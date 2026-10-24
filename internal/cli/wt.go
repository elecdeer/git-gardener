package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/elecdeer/git-gardener/internal/config"
	"github.com/elecdeer/git-gardener/internal/format"
	"github.com/elecdeer/git-gardener/internal/git"
	"github.com/elecdeer/git-gardener/internal/hook"
)

// wtListFields は `wt list --column/--sort` で指定できるフィールド。
var wtListFields = []string{
	"branch", "path", "hash", "upstream", "ahead", "behind", "tracking",
	"commitMessage", "status", "isMain", "isCurrent", "updateAt", "updateAtRelative",
}

// wtRow は `wt list` の出力行。JSON では WtInfo のフィールドの後に updateAtRelative が続く。
type wtRow struct {
	git.WtInfo
	UpdateAtRelative string `json:"updateAtRelative"`
}

// Field は列名に対応する値を返す。
func (r wtRow) Field(name string) any {
	switch name {
	case "branch":
		return r.Branch
	case "path":
		return r.Path
	case "hash":
		return r.Hash
	case "upstream":
		return r.Upstream
	case "ahead":
		return r.Ahead
	case "behind":
		return r.Behind
	case "tracking":
		return r.Tracking
	case "commitMessage":
		return r.CommitMessage
	case "status":
		return r.Status
	case "isMain":
		return r.IsMain
	case "isCurrent":
		return r.IsCurrent
	case "updateAt":
		return r.UpdateAt
	case "updateAtRelative":
		return r.UpdateAtRelative
	}
	return nil
}

// wtTarget は wt 系コマンドの操作対象リポジトリ。
type wtTarget struct {
	// repoPath はメイン worktree のパス。
	repoPath string
	cfg      *config.Config
	// basedir は gdn.wtBasedir を repoPath 基準で解決した絶対パス。
	basedir string
}

// resolveWtTarget は -C の値から操作対象リポジトリを解決する。
func (a *app) resolveWtTarget(ctx context.Context, dirFlag string) (*wtTarget, error) {
	dir, err := resolveDir(dirFlag)
	if err != nil {
		return nil, err
	}
	repoPath, err := a.git.MainWorktreePath(ctx, dir)
	if err != nil {
		return nil, err
	}
	cfg := a.config(repoPath)
	basedir, err := cfg.WtBasedir(ctx, filepath.Base(repoPath))
	if err != nil {
		return nil, err
	}
	if !filepath.IsAbs(basedir) {
		basedir = filepath.Join(repoPath, basedir)
	}
	return &wtTarget{repoPath: repoPath, cfg: cfg, basedir: filepath.Clean(basedir)}, nil
}

// addDirFlag は -C, --dir フラグを登録する。
func addDirFlag(cmd *cobra.Command, dir *string) {
	cmd.Flags().StringVarP(dir, "dir", "C", "", "対象リポジトリのパス（省略時はカレントディレクトリ）")
	_ = cmd.MarkFlagDirname("dir")
}

// completeBranches はブランチ名の補完候補を返す関数を作る。
// worktreeOnly なら worktree に紐づくブランチだけを候補にする。
func (a *app) completeBranches(worktreeOnly bool) cobra.CompletionFunc {
	return func(cmd *cobra.Command, args []string, _ string) ([]string, cobra.ShellCompDirective) {
		if len(args) > 0 {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		dirFlag, _ := cmd.Flags().GetString("dir")
		dir, err := resolveDir(dirFlag)
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		ctx := cmd.Context()
		var names []string
		if worktreeOnly {
			entries, _ := a.git.WorktreeEntries(ctx, dir)
			for _, e := range entries {
				if !e.Bare && !e.Detached() {
					names = append(names, e.Branch)
				}
			}
		} else {
			refs, _ := a.git.BranchRefs(ctx, dir)
			for _, r := range refs {
				names = append(names, r.Name)
			}
		}
		return names, cobra.ShellCompDirectiveNoFileComp
	}
}

// newWtListCommand は `wt list` を作る。
func (a *app) newWtListCommand() *cobra.Command {
	var opts listOptions
	var dir string
	var includeBranches bool
	cmd := &cobra.Command{
		Use:   "list",
		Short: "対象リポジトリのworktree一覧をstdoutに出力する",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			repoDir, err := resolveDir(dir)
			if err != nil {
				return err
			}
			if _, err := a.git.MainWorktreePath(ctx, repoDir); err != nil {
				return err
			}
			infos, err := a.git.WtInfos(ctx, repoDir, git.WtInfoOptions{IncludeBranches: includeBranches})
			if err != nil {
				return err
			}
			now := time.Now()
			// JSON で空配列を null ではなく [] として出すため、0 件でも nil にしない。
			rows := make([]wtRow, len(infos))
			for i, info := range infos {
				rows[i] = wtRow{WtInfo: info, UpdateAtRelative: format.RelativeTime(info.UpdateAt, now)}
			}
			return writeList(cmd.OutOrStdout(), rows, opts, wtListFields)
		},
	}
	opts.register(cmd, "branch,path,hash,updateAtRelative,tracking", wtListFields)
	addDirFlag(cmd, &dir)
	cmd.Flags().BoolVar(&includeBranches, "include-branches", false, "worktreeに紐づいていないローカルブランチも含める")
	return cmd
}

// newWtRootCommand は `wt root` を作る。
func (a *app) newWtRootCommand() *cobra.Command {
	var dir string
	cmd := &cobra.Command{
		Use:   "root",
		Short: "gdn.wtBasedir を解決した絶対パスを出力する",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			target, err := a.resolveWtTarget(cmd.Context(), dir)
			if err != nil {
				return err
			}
			printf(cmd.OutOrStdout(), "%s\n", target.basedir)
			return nil
		},
	}
	addDirFlag(cmd, &dir)
	return cmd
}

// runWtHooks は gdn.wtHook を worktree のディレクトリで実行する。
func runWtHooks(cmd *cobra.Command, target *wtTarget, wtPath string) error {
	hooks, err := target.cfg.WtHooks(cmd.Context())
	if err != nil {
		return err
	}
	hook.Run(cmd.Context(), hooks, wtPath, cmd.ErrOrStderr())
	return nil
}

// newWtCreateCommand は `wt create` を作る。
func (a *app) newWtCreateCommand() *cobra.Command {
	var base, dir string
	cmd := &cobra.Command{
		Use:   "create <branch>",
		Short: "worktreeを新規作成し、そのパスを出力する",
		Long: "worktreeを <gdn.wtBasedir>/<branch> に作成し、gdn.wtHook を実行する。\n" +
			"ブランチが既にあればそれを使い、無ければ --base（省略時はHEAD）から作成する。\n" +
			"そのブランチの worktree が既にある場合はエラー。",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			branch := args[0]
			target, err := a.resolveWtTarget(ctx, dir)
			if err != nil {
				return err
			}

			entries, err := a.git.WorktreeEntries(ctx, target.repoPath)
			if err != nil {
				return err
			}
			for _, e := range entries {
				if e.Branch == branch && !e.Detached() {
					return fmt.Errorf("worktree for branch '%s' already exists: %s", branch, e.Path)
				}
			}

			wtPath, err := a.git.AddWorktree(ctx, target.repoPath, branch, filepath.Join(target.basedir, branch), base)
			if err != nil {
				return err
			}
			if err := runWtHooks(cmd, target, wtPath); err != nil {
				return err
			}
			printf(cmd.OutOrStdout(), "%s\n", wtPath)
			return nil
		},
	}
	cmd.Flags().StringVar(&base, "base", "", "分岐元ブランチ（省略時はHEAD）")
	addDirFlag(cmd, &dir)
	return cmd
}

// newWtSwitchCommand は `wt switch` を作る。
func (a *app) newWtSwitchCommand() *cobra.Command {
	var base, dir string
	cmd := &cobra.Command{
		Use:   "switch <branch>",
		Short: "指定ブランチのworktreeのパスを出力する（存在しない場合は作成する）",
		Long: "指定ブランチのworktreeのパスを出力する。worktreeが無ければ <gdn.wtBasedir>/<branch> に作成し、\n" +
			"gdn.wtHook を実行する。",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: a.completeBranches(false),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			target, err := a.resolveWtTarget(ctx, dir)
			if err != nil {
				return err
			}
			wtPath, created, err := a.git.SwitchWorktree(ctx, target.repoPath, args[0], target.basedir, base)
			if err != nil {
				return err
			}
			if created {
				if err := runWtHooks(cmd, target, wtPath); err != nil {
					return err
				}
			}
			printf(cmd.OutOrStdout(), "%s\n", wtPath)
			return nil
		},
	}
	cmd.Flags().StringVar(&base, "base", "", "新規作成時の分岐元ブランチ（省略時はHEAD）")
	addDirFlag(cmd, &dir)
	return cmd
}

// removeWorktree は gdn.wtDeleteHook を実行したうえで worktree を削除する。
// gdn.wtRemover が設定されていればそのコマンドでディレクトリを削除し、git の管理情報を prune する。
func (a *app) removeWorktree(cmd *cobra.Command, target *wtTarget, wtPath string, force bool) error {
	ctx := cmd.Context()
	hooks, err := target.cfg.WtDeleteHooks(ctx)
	if err != nil {
		return err
	}
	hook.Run(ctx, hooks, wtPath, cmd.ErrOrStderr())

	remover, err := target.cfg.WtRemover(ctx)
	if err != nil {
		return err
	}
	if remover == config.DefaultWtRemover {
		return a.git.RemoveWorktree(ctx, target.repoPath, wtPath, force)
	}
	if err := hook.RunWithPath(ctx, remover, wtPath, target.repoPath, cmd.ErrOrStderr()); err != nil {
		return err
	}
	return a.git.PruneWorktrees(ctx, target.repoPath)
}

// newWtDeleteCommand は `wt delete` を作る。
func (a *app) newWtDeleteCommand() *cobra.Command {
	var force bool
	var dir string
	cmd := &cobra.Command{
		Use:               "delete <branch>",
		Short:             "worktreeとブランチを削除する（デフォルトブランチとメインworktreeは保護）",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: a.completeBranches(true),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			branch := args[0]
			target, err := a.resolveWtTarget(ctx, dir)
			if err != nil {
				return err
			}

			infos, err := a.git.WtInfos(ctx, target.repoPath, git.WtInfoOptions{})
			if err != nil {
				return err
			}
			var wt *git.WtInfo
			for i := range infos {
				if infos[i].Branch == branch {
					wt = &infos[i]
					break
				}
			}
			switch {
			case wt == nil:
				return fmt.Errorf("worktree for branch '%s' not found", branch)
			case wt.IsDefaultBranch:
				return fmt.Errorf("cannot delete default branch worktree: %s", branch)
			case wt.IsMain:
				return fmt.Errorf("cannot delete the main worktree: %s", wt.Path)
			}

			if err := a.removeWorktree(cmd, target, wt.Path, force); err != nil {
				return err
			}
			if err := a.git.DeleteBranch(ctx, target.repoPath, branch, force); err != nil {
				printf(cmd.OutOrStdout(), "Deleted worktree: %s\n", branch)
				printf(cmd.ErrOrStderr(), "gdn: warning: branch '%s' was not deleted: %v\nhint: use --force to delete it anyway\n", branch, err)
				return nil
			}
			printf(cmd.OutOrStdout(), "Deleted worktree and branch: %s\n", branch)
			return nil
		},
	}
	cmd.Flags().BoolVar(&force, "force", false, "未コミットの変更や未マージのコミットがあっても強制削除")
	addDirFlag(cmd, &dir)
	return cmd
}

// newWtPruneCommand は `wt prune` を作る。
func (a *app) newWtPruneCommand() *cobra.Command {
	var dryRun, yes, noFetch bool
	var dir string
	cmd := &cobra.Command{
		Use:   "prune",
		Short: "リモートで削除済みのブランチのworktreeとブランチを一括削除する",
		Long: "upstream がリモートで削除済み（gone）のブランチについて、worktreeとローカルブランチを削除する。\n" +
			"判定前に git fetch --all --prune を実行する。メインworktreeとデフォルトブランチは対象外。",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			stdout, stderr := cmd.OutOrStdout(), cmd.ErrOrStderr()
			target, err := a.resolveWtTarget(ctx, dir)
			if err != nil {
				return err
			}

			prunable, err := a.git.PrunableWorktrees(ctx, target.repoPath, !noFetch)
			if err != nil {
				return err
			}
			if len(prunable) == 0 {
				printf(stdout, "No worktrees to prune.\n")
				return nil
			}

			if dryRun {
				printf(stdout, "[DRY RUN] Would prune these branches:\n")
				for _, wt := range prunable {
					printf(stdout, "  %s (%s)\n", wt.Branch, wt.Path)
				}
				return nil
			}

			var b strings.Builder
			b.WriteString("Branches to prune:\n")
			for _, wt := range prunable {
				fmt.Fprintf(&b, "  %s (%s)\n", wt.Branch, wt.Path)
			}
			fmt.Fprintf(&b, "Prune %d worktree(s)?", len(prunable))
			if err := confirm(cmd, yes, b.String()); err != nil {
				return err
			}

			pruned := 0
			for _, wt := range prunable {
				// 1 件の失敗（未コミットの変更がある等）で残りを止めないよう、警告して次へ進む。
				if err := a.removeWorktree(cmd, target, wt.Path, false); err != nil {
					printf(stderr, "gdn: warning: skipped '%s': %v\n", wt.Branch, err)
					continue
				}
				if err := a.git.DeleteBranch(ctx, target.repoPath, wt.Branch, false); err != nil {
					printf(stderr, "gdn: warning: branch '%s' was not deleted: %v\nhint: delete it with 'git branch -D %s'\n", wt.Branch, err, wt.Branch)
				}
				printf(stdout, "Pruned: %s\n", wt.Branch)
				pruned++
			}
			printf(stdout, "Pruned %d worktree(s).\n", pruned)
			return nil
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "実際には削除せず対象を表示")
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "確認プロンプトをスキップ")
	cmd.Flags().BoolVar(&noFetch, "no-fetch", false, "判定前の git fetch --all --prune を行わない")
	addDirFlag(cmd, &dir)
	return cmd
}

// wtMove は worktree の移動 1 件分。
type wtMove struct {
	branch string
	from   string
	to     string
}

// newWtMigrateCommand は `wt migrate` を作る。
func (a *app) newWtMigrateCommand() *cobra.Command {
	var dryRun, yes bool
	var dir string
	cmd := &cobra.Command{
		Use:   "migrate",
		Short: "既存リポジトリのworktreeをgdn.wtBasedir配下の正規化された構造へ移行する",
		Long: "linked worktree を <gdn.wtBasedir>/<branch> へ git worktree move で移動する。\n" +
			"detached HEAD の worktree は対象外。bare リポジトリは未対応。",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			stdout, stderr := cmd.OutOrStdout(), cmd.ErrOrStderr()
			target, err := a.resolveWtTarget(ctx, dir)
			if err != nil {
				return err
			}
			if a.git.IsBareRepo(ctx, target.repoPath) {
				return fmt.Errorf("bare repository migration is not yet supported")
			}

			entries, err := a.git.WorktreeEntries(ctx, target.repoPath)
			if err != nil {
				return err
			}
			var moves []wtMove
			for i, e := range entries {
				if i == 0 || e.Bare || e.Detached() {
					continue
				}
				to := filepath.Join(target.basedir, e.Branch)
				if !git.SamePath(e.Path, to) {
					moves = append(moves, wtMove{branch: e.Branch, from: e.Path, to: to})
				}
			}

			if len(moves) == 0 {
				printf(stdout, "All worktrees are already in the correct location.\n")
				return nil
			}

			if dryRun {
				printf(stdout, "[DRY RUN] Would move:\n")
				for _, m := range moves {
					printf(stdout, "  %s\n  → %s\n", m.from, m.to)
				}
				return nil
			}

			var b strings.Builder
			fmt.Fprintf(&b, "Migrate %d worktree(s)?\n", len(moves))
			for _, m := range moves {
				fmt.Fprintf(&b, "  %s: %s → %s\n", m.branch, m.from, m.to)
			}
			b.WriteString("Proceed?")
			if err := confirm(cmd, yes, b.String()); err != nil {
				return err
			}

			moved := 0
			for _, m := range moves {
				if pathExists(m.to) {
					printf(stderr, "gdn: warning: target already exists for '%s', skipping: %s\n", m.branch, m.to)
					continue
				}
				if err := os.MkdirAll(filepath.Dir(m.to), 0o755); err != nil {
					return err
				}
				// ディレクトリを rename するだけでは git の管理情報が古いパスを指したままになるため git に移動させる。
				if err := a.git.MoveWorktree(ctx, target.repoPath, m.from, m.to); err != nil {
					printf(stderr, "gdn: warning: failed to move '%s': %v\n", m.branch, err)
					continue
				}
				printf(stdout, "Moved: %s → %s\n", m.branch, m.to)
				moved++
			}
			printf(stdout, "Migrated %d worktree(s).\n", moved)
			return nil
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "実際には移動せず変更予定を表示")
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "確認プロンプトをスキップ")
	addDirFlag(cmd, &dir)
	return cmd
}
