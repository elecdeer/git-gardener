package git

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
)

// fieldSep は git の出力フォーマットで使う区切り文字（ASCII Unit Separator）。
const fieldSep = "\x1f"

// RepoInfo は `repo list` の 1 行分の情報。JSON のキー順はフィールド順に従う。
type RepoInfo struct {
	Repo          string `json:"repo"`
	Host          string `json:"host"`
	Owner         string `json:"owner"`
	Name          string `json:"name"`
	Path          string `json:"path"`
	Branch        string `json:"branch"`
	WorktreeCount int    `json:"worktreeCount"`
	CommitMessage string `json:"commitMessage"`
	UpdateAt      int64  `json:"updateAt"`
}

// WtInfo は `wt list` の 1 行分の情報。JSON のキー順はフィールド順に従う。
type WtInfo struct {
	Branch        string `json:"branch"`
	Path          string `json:"path"`
	Hash          string `json:"hash"`
	Upstream      string `json:"upstream"`
	Ahead         int    `json:"ahead"`
	Behind        int    `json:"behind"`
	Tracking      string `json:"tracking"`
	CommitMessage string `json:"commitMessage"`
	Status        string `json:"status"`
	// IsMain はメイン worktree（`git worktree list` の先頭）かどうか。
	IsMain    bool  `json:"isMain"`
	IsCurrent bool  `json:"isCurrent"`
	UpdateAt  int64 `json:"updateAt"`
	// IsDefaultBranch はリポジトリのデフォルトブランチかどうか。削除保護に使う。
	IsDefaultBranch bool `json:"-"`
	// UpstreamGone はリモートで upstream ブランチが削除済みかどうか。prune 判定に使う。
	UpstreamGone bool `json:"-"`
}

// BranchRef はローカルブランチの参照情報。
type BranchRef struct {
	Name          string
	Hash          string
	Upstream      string
	Ahead         int
	Behind        int
	Gone          bool
	CommitMessage string
	UpdateAt      int64
}

// FormatTracking は ahead/behind を ⇡/⇣ 記号で表す。
func FormatTracking(ahead, behind int) string {
	var parts []string
	if ahead > 0 {
		parts = append(parts, "⇡"+strconv.Itoa(ahead))
	}
	if behind > 0 {
		parts = append(parts, "⇣"+strconv.Itoa(behind))
	}
	return strings.Join(parts, " ")
}

// parseTrack は `%(upstream:track,nobracket)` の出力（"ahead 1, behind 2" や "gone"）をパースする。
func parseTrack(track string) (ahead, behind int, gone bool) {
	if track == "gone" {
		return 0, 0, true
	}
	for part := range strings.SplitSeq(track, ", ") {
		kind, n, ok := strings.Cut(part, " ")
		if !ok {
			continue
		}
		count, _ := strconv.Atoi(n)
		switch kind {
		case "ahead":
			ahead = count
		case "behind":
			behind = count
		}
	}
	return ahead, behind, false
}

// BranchRefs はローカルブランチの参照情報を 1 回の for-each-ref でまとめて取得する。
func (g *Git) BranchRefs(ctx context.Context, dir string) ([]BranchRef, error) {
	format := strings.Join([]string{
		"%(refname:short)",
		"%(objectname)",
		"%(upstream:short)",
		"%(upstream:track,nobracket)",
		"%(committerdate:unix)",
		"%(contents:subject)",
	}, "%1f")
	out, err := g.read(ctx, dir, "for-each-ref", "--format="+format, "refs/heads")
	if err != nil {
		return nil, err
	}

	var refs []BranchRef
	for line := range strings.SplitSeq(out, "\n") {
		fields := strings.Split(line, fieldSep)
		if len(fields) != 6 {
			continue
		}
		ahead, behind, gone := parseTrack(fields[3])
		updateAt, _ := strconv.ParseInt(fields[4], 10, 64)
		refs = append(refs, BranchRef{
			Name:          fields[0],
			Hash:          shortHash(fields[1]),
			Upstream:      fields[2],
			Ahead:         ahead,
			Behind:        behind,
			Gone:          gone,
			CommitMessage: fields[5],
			UpdateAt:      updateAt,
		})
	}
	return refs, nil
}

// lastCommit は dir の HEAD のコミット時刻（unix）とサブジェクトを返す。コミットが無ければゼロ値。
func (g *Git) lastCommit(ctx context.Context, dir string) (updateAt int64, subject string, err error) {
	out, err := g.read(ctx, dir, "log", "-1", "--format=%ct"+"%x1f"+"%s")
	if err != nil || out == "" {
		return 0, "", err
	}
	ts, subject, _ := strings.Cut(out, fieldSep)
	updateAt, _ = strconv.ParseInt(ts, 10, 64)
	return updateAt, subject, nil
}

// headBranch は dir の HEAD が指すブランチ名を返す。detached HEAD なら "HEAD"。
// コミットが無いリポジトリでも動作するよう rev-parse ではなく symbolic-ref を使う。
func (g *Git) headBranch(ctx context.Context, dir string) string {
	out, err := g.read(ctx, dir, "symbolic-ref", "--short", "-q", "HEAD")
	if err != nil || out == "" {
		return detachedBranch
	}
	return out
}

// DefaultBranch はリポジトリのデフォルトブランチを返す。
// origin/HEAD → init.defaultBranch → "main" の順に判定する。
func (g *Git) DefaultBranch(ctx context.Context, dir string) string {
	if out, err := g.read(ctx, dir, "symbolic-ref", "--short", "refs/remotes/origin/HEAD"); err == nil && out != "" {
		return strings.TrimPrefix(out, "origin/")
	}
	if v, err := g.ConfigGet(ctx, dir, ScopeAll, "init.defaultBranch"); err == nil && v != "" {
		return v
	}
	return "main"
}

// isDirty は worktree に未コミットの変更があるかを返す。
func (g *Git) isDirty(ctx context.Context, dir string) (bool, error) {
	out, err := g.read(ctx, dir, "status", "--porcelain")
	return out != "", err
}

// parallelMap は items の各要素に fn を並列適用し、ok が true の結果だけを元の順序で返す。
func parallelMap[T, R any](items []T, fn func(T) (R, bool)) []R {
	results := make([]R, len(items))
	oks := make([]bool, len(items))
	sem := make(chan struct{}, runtime.NumCPU()*2)
	var wg sync.WaitGroup
	for i, item := range items {
		wg.Go(func() {
			sem <- struct{}{}
			defer func() { <-sem }()
			results[i], oks[i] = fn(item)
		})
	}
	wg.Wait()

	filtered := make([]R, 0, len(items))
	for i, r := range results {
		if oks[i] {
			filtered = append(filtered, r)
		}
	}
	return filtered
}

// WtInfoOptions は WtInfos の挙動を制御する。
type WtInfoOptions struct {
	// IncludeBranches が true なら worktree に紐づいていないローカルブランチも含める。
	IncludeBranches bool
}

// WtInfos は dir が属するリポジトリの worktree 情報を返す。
// bare リポジトリ本体や実体の無い worktree は除外する。
func (g *Git) WtInfos(ctx context.Context, dir string, opts WtInfoOptions) ([]WtInfo, error) {
	entries, err := g.WorktreeEntries(ctx, dir)
	if err != nil {
		return nil, err
	}
	refs, err := g.BranchRefs(ctx, dir)
	if err != nil {
		return nil, err
	}
	refByName := make(map[string]BranchRef, len(refs))
	for _, ref := range refs {
		refByName[ref.Name] = ref
	}

	defaultBranch := g.DefaultBranch(ctx, dir)
	currentWt, _ := g.read(ctx, dir, "rev-parse", "--show-toplevel")

	type indexed struct {
		index int
		entry WorktreeEntry
	}
	items := make([]indexed, 0, len(entries))
	for i, entry := range entries {
		if !entry.Bare {
			items = append(items, indexed{i, entry})
		}
	}

	infos := parallelMap(items, func(item indexed) (WtInfo, bool) {
		entry := item.entry
		// prunable（ディレクトリが消えた）worktree は情報を取れないので除外する。
		if _, err := os.Stat(entry.Path); err != nil {
			return WtInfo{}, false
		}
		// コミットの無いリポジトリでは log が失敗するが、空の情報として扱う。
		updateAt, subject, _ := g.lastCommit(ctx, entry.Path)
		dirty, err := g.isDirty(ctx, entry.Path)
		if err != nil {
			return WtInfo{}, false
		}
		status := "clean"
		if dirty {
			status = "dirty"
		}
		info := WtInfo{
			Branch:          entry.Branch,
			Path:            entry.Path,
			Hash:            entry.Hash,
			CommitMessage:   subject,
			Status:          status,
			IsMain:          item.index == 0,
			IsCurrent:       currentWt != "" && SamePath(entry.Path, currentWt),
			UpdateAt:        updateAt,
			IsDefaultBranch: !entry.Detached() && entry.Branch == defaultBranch,
		}
		if ref, ok := refByName[entry.Branch]; ok && !entry.Detached() {
			info.Upstream = ref.Upstream
			info.Ahead = ref.Ahead
			info.Behind = ref.Behind
			info.UpstreamGone = ref.Gone
		}
		info.Tracking = FormatTracking(info.Ahead, info.Behind)
		return info, true
	})

	if opts.IncludeBranches {
		checkedOut := make(map[string]bool, len(entries))
		for _, entry := range entries {
			checkedOut[entry.Branch] = true
		}
		for _, ref := range refs {
			if checkedOut[ref.Name] {
				continue
			}
			infos = append(infos, WtInfo{
				Branch:          ref.Name,
				Hash:            ref.Hash,
				Upstream:        ref.Upstream,
				Ahead:           ref.Ahead,
				Behind:          ref.Behind,
				Tracking:        FormatTracking(ref.Ahead, ref.Behind),
				CommitMessage:   ref.CommitMessage,
				UpdateAt:        ref.UpdateAt,
				IsDefaultBranch: ref.Name == defaultBranch,
				UpstreamGone:    ref.Gone,
			})
		}
	}

	return infos, nil
}

// PrunableWorktrees はリモートで upstream が削除済み（gone）のブランチを持つ worktree を返す。
// メイン worktree とデフォルトブランチは対象外。fetch が true なら先に `git fetch --all --prune` を行う。
func (g *Git) PrunableWorktrees(ctx context.Context, dir string, fetch bool) ([]WtInfo, error) {
	if fetch {
		// オフライン等による fetch の失敗は致命的にせず、既存の追跡情報で判定を続ける。
		_ = g.exec(ctx, dir, "fetch", "--all", "--prune")
	}

	infos, err := g.WtInfos(ctx, dir, WtInfoOptions{})
	if err != nil {
		return nil, err
	}
	var prunable []WtInfo
	for _, info := range infos {
		if info.UpstreamGone && !info.IsMain && !info.IsDefaultBranch {
			prunable = append(prunable, info)
		}
	}
	return prunable, nil
}

// RepoInfoOptions は RepoInfos の挙動を制御する。
type RepoInfoOptions struct {
	// IncludeWorktrees が true なら linked worktree も行として含める。
	IncludeWorktrees bool
}

// repoDir は root 配下で見つかったリポジトリ候補。
type repoDir struct {
	path  string
	host  string
	owner string
	name  string
}

// listDirs は dir 直下の隠しディレクトリ以外のディレクトリ名を返す。
func listDirs(dir string) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() && !strings.HasPrefix(e.Name(), ".") {
			names = append(names, e.Name())
		}
	}
	return names
}

// hasGitDir は dir 直下に .git ディレクトリがあるかを返す。
// linked worktree（.git がファイル）や bare リポジトリは対象外にする。
func hasGitDir(dir string) bool {
	s, err := os.Stat(filepath.Join(dir, ".git"))
	return err == nil && s.IsDir()
}

// findRepos は root/<host>/<owner>/<name> の階層からリポジトリを探す。
func findRepos(root string) []repoDir {
	var repos []repoDir
	for _, host := range listDirs(root) {
		hostPath := filepath.Join(root, host)
		for _, owner := range listDirs(hostPath) {
			ownerPath := filepath.Join(hostPath, owner)
			for _, name := range listDirs(ownerPath) {
				path := filepath.Join(ownerPath, name)
				if hasGitDir(path) {
					repos = append(repos, repoDir{path: path, host: host, owner: owner, name: name})
				}
			}
		}
	}
	return repos
}

// RepoInfos は root 配下の管理リポジトリの情報を返す。root が存在しなければ空。
func (g *Git) RepoInfos(ctx context.Context, root string, opts RepoInfoOptions) []RepoInfo {
	groups := parallelMap(findRepos(root), func(r repoDir) ([]RepoInfo, bool) {
		entries, err := g.WorktreeEntries(ctx, r.path)
		if err != nil {
			return nil, false
		}
		// コミットの無いリポジトリでは log が失敗するが、空の情報として扱う。
		updateAt, subject, _ := g.lastCommit(ctx, r.path)
		base := RepoInfo{
			Repo:          r.host + "/" + r.owner + "/" + r.name,
			Host:          r.host,
			Owner:         r.owner,
			Name:          r.name,
			Path:          r.path,
			Branch:        g.headBranch(ctx, r.path),
			WorktreeCount: len(entries),
			CommitMessage: subject,
			UpdateAt:      updateAt,
		}
		infos := []RepoInfo{base}
		if !opts.IncludeWorktrees {
			return infos, true
		}

		for _, entry := range entries[1:] {
			updateAt, subject, err := g.lastCommit(ctx, entry.Path)
			if err != nil {
				continue
			}
			wt := base
			wt.Repo = worktreeRepoName(root, entry.Path)
			wt.Path = entry.Path
			wt.Branch = entry.Branch
			wt.CommitMessage = subject
			wt.UpdateAt = updateAt
			infos = append(infos, wt)
		}
		return infos, true
	})

	var repos []RepoInfo
	for _, group := range groups {
		repos = append(repos, group...)
	}
	return repos
}

// worktreeRepoName は linked worktree の repo 列の値を返す。
// root 配下なら root からの相対パス（`<repo root>/<値>` で cd できる形）、外なら絶対パス。
// パス文字列の前方一致ではなく祖先ディレクトリを SamePath で辿り、表記揺れを吸収する。
func worktreeRepoName(root, path string) string {
	for dir := filepath.Dir(path); dir != filepath.Dir(dir); dir = filepath.Dir(dir) {
		if !SamePath(dir, root) {
			continue
		}
		if rel, err := filepath.Rel(dir, path); err == nil {
			return filepath.ToSlash(rel)
		}
	}
	return path
}

// CloneOptions は Clone のオプション。
type CloneOptions struct {
	Shallow    bool
	Branch     string
	Submodules bool
}

// Clone は url を dest にクローンする。git の進捗は stderr に流す。
func (g *Git) Clone(ctx context.Context, url, dest string, opts CloneOptions) error {
	args := []string{"clone"}
	if opts.Shallow {
		args = append(args, "--depth", "1")
	}
	if opts.Branch != "" {
		args = append(args, "--branch", opts.Branch)
	}
	if opts.Submodules {
		args = append(args, "--recurse-submodules")
	}
	args = append(args, url, dest)
	_, err := g.run(ctx, Command{Args: args, Stream: true})
	return err
}

// RepoRoot は dir を含む worktree のトップレベルディレクトリを返す。
func (g *Git) RepoRoot(ctx context.Context, dir string) (string, error) {
	return g.read(ctx, dir, "rev-parse", "--show-toplevel")
}
