package git

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// detachedBranch は detached HEAD の worktree に割り当てるブランチ名。
const detachedBranch = "HEAD"

// WorktreeEntry は `git worktree list --porcelain` の 1 エントリ。
type WorktreeEntry struct {
	Path string
	// Branch は refs/heads/ を除いたブランチ名。detached HEAD の場合は "HEAD"。
	Branch string
	// Hash は HEAD の短縮コミットハッシュ（7 桁）。
	Hash string
	Bare bool
}

// Detached は worktree が detached HEAD かどうかを返す。
func (e WorktreeEntry) Detached() bool {
	return !e.Bare && e.Branch == detachedBranch
}

// ParseWorktreePorcelain は `git worktree list --porcelain` の出力をパースする。
func ParseWorktreePorcelain(raw string) []WorktreeEntry {
	var entries []WorktreeEntry
	var current *WorktreeEntry
	flush := func() {
		if current != nil {
			entries = append(entries, *current)
		}
	}

	for line := range strings.SplitSeq(raw, "\n") {
		switch {
		case strings.HasPrefix(line, "worktree "):
			flush()
			current = &WorktreeEntry{Path: strings.TrimPrefix(line, "worktree "), Branch: detachedBranch}
		case current == nil:
			continue
		case strings.HasPrefix(line, "branch "):
			current.Branch = strings.TrimPrefix(strings.TrimPrefix(line, "branch "), "refs/heads/")
		case strings.HasPrefix(line, "HEAD "):
			current.Hash = shortHash(strings.TrimPrefix(line, "HEAD "))
		case line == "bare":
			current.Bare = true
		}
	}
	flush()
	return entries
}

// shortHash はコミットハッシュを 7 桁に切り詰める。
func shortHash(hash string) string {
	if len(hash) > 7 {
		return hash[:7]
	}
	return hash
}

// WorktreeEntries は dir が属するリポジトリの worktree 一覧を返す。先頭がメイン worktree。
func (g *Git) WorktreeEntries(ctx context.Context, dir string) ([]WorktreeEntry, error) {
	out, err := g.read(ctx, dir, "worktree", "list", "--porcelain")
	if err != nil {
		return nil, err
	}
	return ParseWorktreePorcelain(out), nil
}

// MainWorktreePath は dir が属するリポジトリのメイン worktree のパスを返す。
// linked worktree 内から実行した場合もメインリポジトリのパスを返す。
func (g *Git) MainWorktreePath(ctx context.Context, dir string) (string, error) {
	entries, err := g.WorktreeEntries(ctx, dir)
	if err != nil || len(entries) == 0 {
		return "", fmt.Errorf("not a git repository: %s", dir)
	}
	return entries[0].Path, nil
}

// IsBareRepo は dir が bare リポジトリかどうかを返す。
func (g *Git) IsBareRepo(ctx context.Context, dir string) bool {
	out, err := g.read(ctx, dir, "rev-parse", "--is-bare-repository")
	return err == nil && out == "true"
}

// localBranchExists はブランチがローカルに存在するかを返す。
func (g *Git) localBranchExists(ctx context.Context, dir, branch string) bool {
	_, err := g.read(ctx, dir, "rev-parse", "--verify", "--quiet", "refs/heads/"+branch)
	return err == nil
}

// remoteBranchExists は同名のリモート追跡ブランチが存在するかを返す。
func (g *Git) remoteBranchExists(ctx context.Context, dir, branch string) bool {
	out, err := g.read(ctx, dir, "branch", "-r", "--list", "*/"+branch)
	return err == nil && out != ""
}

// realpathOrAbs はシンボリックリンクを解決した絶対パスを返す。存在しない場合は絶対パス化だけ行う。
func realpathOrAbs(path string) string {
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		path = resolved
	}
	if abs, err := filepath.Abs(path); err == nil {
		return abs
	}
	return filepath.Clean(path)
}

// SamePath は 2 つのパスが同じ場所を指すかを返す。
// 両方存在する場合は os.SameFile で比較し、シンボリックリンクや macOS の大文字小文字の表記揺れ
// （~/dev と ~/Dev など）も同一とみなす。
func SamePath(a, b string) bool {
	if sa, err := os.Stat(a); err == nil {
		if sb, err := os.Stat(b); err == nil {
			return os.SameFile(sa, sb)
		}
	}
	return realpathOrAbs(a) == realpathOrAbs(b)
}

// assertWtDirAvailable は worktree の作成先が使用可能か確認する。
// Git は空ディレクトリへの `worktree add` を許可するため、中身があるディレクトリやファイルだけを衝突とみなす。
// 作成先が別ブランチの worktree として使われている場合（worktree 内でブランチを切り替えた場合など）は、
// そのブランチ名をメッセージに含める。
func (g *Git) assertWtDirAvailable(ctx context.Context, dir, branch, wtDir string) error {
	names, err := os.ReadDir(wtDir)
	if errors.Is(err, fs.ErrNotExist) || (err == nil && len(names) == 0) {
		return nil
	}

	entries, _ := g.WorktreeEntries(ctx, dir)
	for _, entry := range entries {
		if !SamePath(entry.Path, wtDir) {
			continue
		}
		usedBy := fmt.Sprintf("the worktree for branch '%s'", entry.Branch)
		if entry.Detached() {
			usedBy = "a detached HEAD worktree"
		}
		return fmt.Errorf("cannot create worktree for '%s': '%s' is already used by %s\n"+
			"hint: move it with 'git worktree move %s <new-path>'", branch, wtDir, usedBy, wtDir)
	}

	return fmt.Errorf("cannot create worktree for '%s': '%s' already exists and is not an empty directory", branch, wtDir)
}

// AddWorktree は branch の worktree を wtDir に作成し、その絶対パスを返す。
// ローカルまたはリモートに既存のブランチがあればそれを使い、無ければ base（空なら HEAD）から新規作成する。
func (g *Git) AddWorktree(ctx context.Context, dir, branch, wtDir, base string) (string, error) {
	if err := g.assertWtDirAvailable(ctx, dir, branch, wtDir); err != nil {
		return "", err
	}

	cmd := Command{Dir: dir}
	if g.localBranchExists(ctx, dir, branch) || g.remoteBranchExists(ctx, dir, branch) {
		// 既存ブランチ（リモートのみの場合は DWIM で追跡ブランチが作られる）は -b なしで追加する。
		// リモートブランチは既に分岐元を持つため base は渡さない。
		cmd.Args = []string{"worktree", "add", wtDir, branch}
	} else {
		cmd.Args = []string{"worktree", "add", "-b", branch, wtDir}
		if base != "" {
			cmd.Args = append(cmd.Args, base)
			// 新ブランチが派生元の upstream（例: origin/main）を継承しないようにする。
			cmd.Config = []string{"branch.autoSetupMerge=false"}
		}
	}
	if _, err := g.run(ctx, cmd); err != nil {
		return "", err
	}

	return filepath.Abs(wtDir)
}

// SwitchWorktree は branch の worktree のパスを返す。存在しなければ wtBasedir/branch に作成する。
// created は新規作成したかどうか。
func (g *Git) SwitchWorktree(ctx context.Context, dir, branch, wtBasedir, base string) (path string, created bool, err error) {
	entries, err := g.WorktreeEntries(ctx, dir)
	if err != nil {
		return "", false, err
	}
	for _, entry := range entries {
		if entry.Branch == branch && !entry.Detached() {
			return entry.Path, false, nil
		}
	}

	path, err = g.AddWorktree(ctx, dir, branch, filepath.Join(wtBasedir, branch), base)
	return path, err == nil, err
}

// RemoveWorktree は `git worktree remove` で worktree を削除する。
func (g *Git) RemoveWorktree(ctx context.Context, dir, path string, force bool) error {
	args := []string{"worktree", "remove"}
	if force {
		args = append(args, "--force")
	}
	return g.exec(ctx, dir, append(args, path)...)
}

// PruneWorktrees は実体の無くなった worktree の管理情報を削除する。
func (g *Git) PruneWorktrees(ctx context.Context, dir string) error {
	return g.exec(ctx, dir, "worktree", "prune")
}

// MoveWorktree は `git worktree move` で worktree を移動する。
func (g *Git) MoveWorktree(ctx context.Context, dir, from, to string) error {
	return g.exec(ctx, dir, "worktree", "move", from, to)
}

// RepairWorktrees はリポジトリ移動後に worktree との相互リンクを修復する。
func (g *Git) RepairWorktrees(ctx context.Context, dir string) error {
	return g.exec(ctx, dir, "worktree", "repair")
}

// DeleteBranch はローカルブランチを削除する。force なら未マージでも削除する。
func (g *Git) DeleteBranch(ctx context.Context, dir, branch string, force bool) error {
	flag := "-d"
	if force {
		flag = "-D"
	}
	return g.exec(ctx, dir, "branch", flag, branch)
}
