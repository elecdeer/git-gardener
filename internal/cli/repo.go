package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/elecdeer/git-gardener/internal/format"
	"github.com/elecdeer/git-gardener/internal/git"
	"github.com/elecdeer/git-gardener/internal/hook"
	"github.com/elecdeer/git-gardener/internal/repourl"
)

// repoListFields は `repo list --column/--sort` で指定できるフィールド。
var repoListFields = []string{
	"repo", "host", "owner", "name", "path", "branch",
	"worktreeCount", "commitMessage", "updateAt", "updateAtRelative",
}

// repoRow は `repo list` の出力行。JSON では RepoInfo のフィールドの後に updateAtRelative が続く。
type repoRow struct {
	git.RepoInfo
	UpdateAtRelative string `json:"updateAtRelative"`
}

// Field は列名に対応する値を返す。
func (r repoRow) Field(name string) any {
	switch name {
	case "repo":
		return r.Repo
	case "host":
		return r.Host
	case "owner":
		return r.Owner
	case "name":
		return r.Name
	case "path":
		return r.Path
	case "branch":
		return r.Branch
	case "worktreeCount":
		return r.WorktreeCount
	case "commitMessage":
		return r.CommitMessage
	case "updateAt":
		return r.UpdateAt
	case "updateAtRelative":
		return r.UpdateAtRelative
	}
	return nil
}

// listOptions は list 系コマンドに共通するオプション。
type listOptions struct {
	column  string
	sort    string
	reverse bool
	limit   int
	noColor bool
	json    bool
}

// register は list 系コマンド共通のフラグを登録する。
func (o *listOptions) register(cmd *cobra.Command, defaultColumns string, fields []string) {
	f := cmd.Flags()
	f.StringVar(&o.column, "column", defaultColumns, "出力列をカンマ区切りで指定 ("+strings.Join(fields, ", ")+")")
	f.StringVar(&o.sort, "sort", "updateAt", "ソートフィールド（デフォルトは降順）")
	f.BoolVar(&o.reverse, "reverse", false, "ソートを昇順にする")
	f.IntVar(&o.limit, "limit", 0, "上位n件のみ出力（0は無制限）")
	f.BoolVar(&o.noColor, "no-color", false, "ANSIカラーなしで出力（環境変数 NO_COLOR でも無効化）")
	f.BoolVar(&o.json, "json", false, "JSON形式で出力")
	_ = cmd.RegisterFlagCompletionFunc("sort", cobra.FixedCompletions(fields, cobra.ShellCompDirectiveNoFileComp))
}

// writeList は rows をソート・件数制限したうえで、JSON またはタブ区切りで w に出力する。
func writeList[R format.Row](w io.Writer, rows []R, o listOptions, fields []string) error {
	cols, err := format.ParseColumns(o.column, fields)
	if err != nil {
		return err
	}
	if err := format.SortRows(rows, o.sort, o.reverse, fields); err != nil {
		return err
	}
	if o.limit > 0 && len(rows) > o.limit {
		rows = rows[:o.limit]
	}

	var out string
	if o.json {
		out, err = format.JSON(rows)
		if err != nil {
			return err
		}
	} else {
		out = format.Tabular(rows, cols, o.noColor || os.Getenv("NO_COLOR") != "")
	}
	printf(w, "%s", out)
	return nil
}

// newRepoListCommand は `repo list` を作る。
func (a *app) newRepoListCommand() *cobra.Command {
	var opts listOptions
	var includeWorktrees bool
	cmd := &cobra.Command{
		Use:   "list",
		Short: "管理下のリポジトリ一覧をstdoutに出力する",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			root, err := a.config("").Root(ctx)
			if err != nil {
				return err
			}
			infos := a.git.RepoInfos(ctx, root, git.RepoInfoOptions{IncludeWorktrees: includeWorktrees})
			now := time.Now()
			// JSON で空配列を null ではなく [] として出すため、0 件でも nil にしない。
			rows := make([]repoRow, len(infos))
			for i, info := range infos {
				rows[i] = repoRow{RepoInfo: info, UpdateAtRelative: format.RelativeTime(info.UpdateAt, now)}
			}
			return writeList(cmd.OutOrStdout(), rows, opts, repoListFields)
		},
	}
	opts.register(cmd, "repo,updateAtRelative", repoListFields)
	cmd.Flags().BoolVar(&includeWorktrees, "include-worktrees", false, "linked worktreeもリストに含める")
	return cmd
}

// newRepoRootCommand は `repo root` を作る。
func (a *app) newRepoRootCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "root",
		Short: "gdn.root を解決した絶対パスを出力する",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			root, err := a.config("").Root(cmd.Context())
			if err != nil {
				return err
			}
			printf(cmd.OutOrStdout(), "%s\n", root)
			return nil
		},
	}
}

// pathExists は path に何らかのファイルが存在するかを返す。
func pathExists(path string) bool {
	_, err := os.Lstat(path)
	return !errors.Is(err, fs.ErrNotExist)
}

// newRepoCloneCommand は `repo clone` を作る。
func (a *app) newRepoCloneCommand() *cobra.Command {
	var opts git.CloneOptions
	cmd := &cobra.Command{
		Use:   "clone <url>",
		Short: "リポジトリを管理構造でクローンする",
		Long: "リポジトリを <gdn.root>/<host>/<owner>/<name> にクローンし、gdn.postCloneHook を実行する。\n" +
			"URLは github.com/user/project や user/project などの省略形も受け付ける。",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			cfg := a.config("")
			root, err := cfg.Root(ctx)
			if err != nil {
				return err
			}
			defaultHost, err := cfg.DefaultHost(ctx)
			if err != nil {
				return err
			}

			repo, err := repourl.Parse(args[0], defaultHost)
			if err != nil {
				return err
			}
			dest := filepath.Join(root, repo.Host, repo.Owner, repo.Name)
			if pathExists(dest) {
				return fmt.Errorf("destination already exists: %s", dest)
			}

			printf(cmd.ErrOrStderr(), "Cloning %s into %s...\n", repo.URL, dest)
			if err := a.git.Clone(ctx, repo.URL, dest, opts); err != nil {
				return err
			}

			hooks, err := cfg.PostCloneHooks(ctx)
			if err != nil {
				return err
			}
			hook.Run(ctx, hooks, dest, cmd.ErrOrStderr())

			printf(cmd.OutOrStdout(), "%s\n", dest)
			return nil
		},
	}
	f := cmd.Flags()
	f.BoolVar(&opts.Shallow, "shallow", false, "shallow clone (--depth 1)")
	f.StringVar(&opts.Branch, "branch", "", "特定ブランチをクローン")
	f.BoolVar(&opts.Submodules, "submodules", false, "submoduleを再帰的にクローン")
	return cmd
}

// configMigration は旧設定キーから gdn.* への移行 1 件分。
type configMigration struct {
	oldKey string
	newKey string
	values []string
	// skip は移行先が既に設定済みのため移行しないことを表す。
	skip bool
}

// legacyConfigKeys は移行対象の旧設定キーと移行先の対応。
var legacyConfigKeys = [][2]string{
	{"wt.basedir", "gdn.wtBasedir"},
	{"wt.hook", "gdn.wtHook"},
	{"wt.deleteHook", "gdn.wtDeleteHook"},
	{"wt.remover", "gdn.wtRemover"},
}

// planConfigMigrations はリポジトリのローカル設定にある旧キーを調べ、移行計画を返す。
func (a *app) planConfigMigrations(ctx context.Context, dir string) ([]configMigration, error) {
	var plan []configMigration
	for _, keys := range legacyConfigKeys {
		values, err := a.git.ConfigGetAll(ctx, dir, git.ScopeLocal, keys[0])
		if err != nil {
			return nil, err
		}
		if len(values) == 0 {
			continue
		}
		existing, err := a.git.ConfigGetAll(ctx, dir, git.ScopeLocal, keys[1])
		if err != nil {
			return nil, err
		}
		plan = append(plan, configMigration{oldKey: keys[0], newKey: keys[1], values: values, skip: len(existing) > 0})
	}
	return plan, nil
}

// describeConfigMigrations は移行計画を人間向けの行にする。
func describeConfigMigrations(plan []configMigration) []string {
	var lines []string
	for _, m := range plan {
		if m.skip {
			lines = append(lines, fmt.Sprintf("Skip %s → %s: %s already set", m.oldKey, m.newKey, m.newKey))
			continue
		}
		for _, v := range m.values {
			lines = append(lines, fmt.Sprintf("Migrate %s=%s → %s=%s", m.oldKey, v, m.newKey, v))
		}
	}
	return lines
}

// applyConfigMigrations は移行計画をローカル設定に書き込む。旧キーは残す。
func (a *app) applyConfigMigrations(ctx context.Context, dir string, plan []configMigration) error {
	for _, m := range plan {
		if m.skip {
			continue
		}
		for _, v := range m.values {
			if err := a.git.ConfigAdd(ctx, dir, m.newKey, v); err != nil {
				return err
			}
		}
	}
	return nil
}

// printConfigMigrations は移行計画があれば w に出力する。
func printConfigMigrations(w io.Writer, plan []configMigration) {
	lines := describeConfigMigrations(plan)
	if len(lines) == 0 {
		return
	}
	printf(w, "Config migrations:\n")
	for _, line := range lines {
		printf(w, "  %s\n", line)
	}
}

// newRepoMigrateCommand は `repo migrate` を作る。
func (a *app) newRepoMigrateCommand() *cobra.Command {
	var dryRun, yes bool
	cmd := &cobra.Command{
		Use:   "migrate [path]",
		Short: "既存のローカルリポジトリをgdn管理のディレクトリ構造へ移行する",
		Long: "既存のローカルリポジトリを <gdn.root>/<host>/<owner>/<name> へ移動し、gdn.postCloneHook を実行する。\n" +
			"リポジトリのローカル設定にある wt.* を gdn.* へ移行する。",
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			stdout, stderr := cmd.OutOrStdout(), cmd.ErrOrStderr()

			path := ""
			if len(args) == 1 {
				path = args[0]
			}
			dir, err := resolveDir(path)
			if err != nil {
				return err
			}
			// サブディレクトリや linked worktree から実行された場合もリポジトリ本体を移動する。
			source, err := a.git.MainWorktreePath(ctx, dir)
			if err != nil {
				return err
			}
			if a.git.IsBareRepo(ctx, source) {
				return fmt.Errorf("bare repository migration is not supported: %s", source)
			}

			cfg := a.config(source)
			root, err := cfg.Root(ctx)
			if err != nil {
				return err
			}
			defaultHost, err := cfg.DefaultHost(ctx)
			if err != nil {
				return err
			}
			remoteURL, err := a.git.ConfigGet(ctx, source, git.ScopeLocal, "remote.origin.url")
			if err != nil {
				return err
			}
			if remoteURL == "" {
				return fmt.Errorf("no remote 'origin' found in %s", source)
			}
			repo, err := repourl.Parse(remoteURL, defaultHost)
			if err != nil {
				return err
			}
			dest := filepath.Join(root, repo.Host, repo.Owner, repo.Name)

			plan, err := a.planConfigMigrations(ctx, source)
			if err != nil {
				return err
			}

			if git.SamePath(source, dest) {
				printf(stderr, "Repository is already in the correct location.\n")
				printConfigMigrations(stdout, plan)
				if dryRun {
					return nil
				}
				return a.applyConfigMigrations(ctx, source, plan)
			}

			if dryRun {
				printf(stdout, "[DRY RUN] Would move:\n  %s\n  → %s\n", source, dest)
				printConfigMigrations(stdout, plan)
				return nil
			}

			if pathExists(dest) {
				return fmt.Errorf("destination already exists: %s", dest)
			}
			// 移動後は source で git config を実行できなくなるため、先に読んでおく。
			hooks, err := cfg.PostCloneHooks(ctx)
			if err != nil {
				return err
			}
			question := fmt.Sprintf("Move repository?\n  From: %s\n  To:   %s\nProceed?", source, dest)
			if err := confirm(cmd, yes, question); err != nil {
				return err
			}

			if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
				return err
			}
			if err := os.Rename(source, dest); err != nil {
				return err
			}
			// メイン worktree を移動すると linked worktree からの参照が切れるため修復する。
			if err := a.git.RepairWorktrees(ctx, dest); err != nil {
				printf(stderr, "gdn: warning: %v\n", err)
			}
			if err := a.applyConfigMigrations(ctx, dest, plan); err != nil {
				return err
			}
			printConfigMigrations(stderr, plan)
			hook.Run(ctx, hooks, dest, stderr)

			printf(stdout, "%s\n", dest)
			return nil
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "実際には移動せず変更予定を表示")
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "確認プロンプトをスキップ")
	return cmd
}
