package e2e

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// wtEnv は worktree 操作のテスト環境。
type wtEnv struct {
	*testEnv
	// repo は gdn.root 配下のメインリポジトリ。
	repo string
	// remote は origin として使う bare リポジトリ。
	remote string
}

// setupWtEnv は origin を持つメインリポジトリと、basedir の外にある worktree（feature-a）を用意する。
func setupWtEnv(t *testing.T) *wtEnv {
	e := &wtEnv{testEnv: newTestEnv(t)}
	e.repo = filepath.Join(e.root, "github.com", "testuser", "worktree-demo")
	e.remote = filepath.Join(e.dir, "remote.git")

	// prune がリモートの gone 判定を使うので、bare リポジトリを origin として用意する。
	if err := os.MkdirAll(e.remote, 0o755); err != nil {
		t.Fatal(err)
	}
	e.git(e.remote, "init", "--bare", "--initial-branch=main")
	e.initRepo(e.repo, "feat: initial commit")
	e.git(e.repo, "remote", "add", "origin", e.remote)
	e.git(e.repo, "push", "-u", "origin", "main")
	e.git(e.repo, "worktree", "add", "-b", "feature-a", filepath.Join(e.dir, "feature-a"))
	return e
}

// wtList は `wt list --json` の結果をパースする。
func (e *wtEnv) wtList(t *testing.T, extra ...string) []map[string]any {
	t.Helper()
	r := e.gdn(append([]string{"wt", "list", "-C", e.repo, "--json"}, extra...)...)
	var rows []map[string]any
	if err := json.Unmarshal([]byte(r.stdout), &rows); err != nil {
		t.Fatalf("invalid json: %v\n%s\n%s", err, r.stdout, r.stderr)
	}
	return rows
}

// branches は行の branch 列を返す。
func branches(rows []map[string]any) []string {
	var names []string
	for _, row := range rows {
		names = append(names, row["branch"].(string))
	}
	return names
}

func TestWtRoot(t *testing.T) {
	e := setupWtEnv(t)

	t.Run("wt basedir を出力する", func(t *testing.T) {
		r := e.gdn("wt", "root", "-C", e.repo)
		e.expectOutput("stdout", r.stdout, "<testDir>/gdn-root/github.com/testuser/worktree-demo.wt")
	})

	t.Run("linked worktree から実行してもメインリポジトリ基準で解決する", func(t *testing.T) {
		r := e.gdn("wt", "root", "-C", filepath.Join(e.dir, "feature-a"))
		e.expectOutput("stdout", r.stdout, "<testDir>/gdn-root/github.com/testuser/worktree-demo.wt")
	})

	t.Run("git リポジトリでなければエラー", func(t *testing.T) {
		r := e.gdn("wt", "root", "-C", e.dir)
		e.expectOutput("stderr", r.stderr, "gdn: not a git repository: <testDir>")
	})
}

func TestWtList(t *testing.T) {
	e := setupWtEnv(t)

	t.Run("index を更新せず index.lock も作成しない", func(t *testing.T) {
		indexPath := filepath.Join(e.repo, ".git", "index")
		before, err := os.Stat(indexPath)
		if err != nil {
			t.Fatal(err)
		}
		// mtime を未来にして、通常の git status なら index を書き換える状況を作る。
		future := time.Now().Add(time.Minute)
		if err := os.Chtimes(filepath.Join(e.repo, "README.md"), future, future); err != nil {
			t.Fatal(err)
		}
		tracePath := filepath.Join(e.dir, "wt-list-trace.json")

		r := e.gdnWith("", []string{"GIT_TRACE2_EVENT=" + tracePath}, "wt", "list", "-C", e.repo, "--json")

		after, err := os.Stat(indexPath)
		if err != nil {
			t.Fatal(err)
		}
		trace, err := os.ReadFile(tracePath)
		if err != nil {
			t.Fatal(err)
		}
		var lockEvents []string
		for line := range strings.SplitSeq(string(trace), "\n") {
			if strings.Contains(line, `"label":"do_write_index"`) && strings.Contains(line, "index.lock") {
				lockEvents = append(lockEvents, line)
			}
		}
		if r.stderr != "" || !after.ModTime().Equal(before.ModTime()) || len(lockEvents) > 0 {
			t.Errorf("stderr = %q, index mtime changed = %v, lock events = %v",
				r.stderr, !after.ModTime().Equal(before.ModTime()), lockEvents)
		}
	})

	t.Run("タブ区切りで一覧を出力する", func(t *testing.T) {
		r := e.gdn("wt", "list", "-C", e.repo, "--no-color", "--sort", "branch", "--reverse")
		e.expectOutput("stdout", r.stdout, ""+
			"feature-a\t<testDir>/feature-a\t<hash>\t<time>\n"+
			"main\t<testDir>/gdn-root/github.com/testuser/worktree-demo\t<hash>\t<time>")
	})

	t.Run("--column で出力列を選ぶ", func(t *testing.T) {
		r := e.gdn("wt", "list", "-C", e.repo, "--no-color", "--sort", "branch", "--reverse", "--column", "branch,hash")
		e.expectOutput("stdout", r.stdout, "feature-a\t<hash>\nmain\t<hash>")
	})

	t.Run("--limit で件数を絞る", func(t *testing.T) {
		r := e.gdn("wt", "list", "-C", e.repo, "--no-color", "--sort", "branch", "--reverse", "--limit", "1")
		e.expectOutput("stdout", r.stdout, "feature-a\t<testDir>/feature-a\t<hash>\t<time>")
	})

	t.Run("--json で JSON を出力する", func(t *testing.T) {
		r := e.gdn("wt", "list", "-C", e.repo, "--json", "--sort", "branch", "--reverse")
		e.expectOutput("stdout", r.stdout, `[
  {
    "branch": "feature-a",
    "path": "<testDir>/feature-a",
    "hash": "<hash>",
    "upstream": "",
    "ahead": 0,
    "behind": 0,
    "tracking": "",
    "commitMessage": "feat: initial commit",
    "status": "clean",
    "isMain": false,
    "isCurrent": false,
    "updateAt": <timestamp>,
    "updateAtRelative": "<time>"
  },
  {
    "branch": "main",
    "path": "<testDir>/gdn-root/github.com/testuser/worktree-demo",
    "hash": "<hash>",
    "upstream": "origin/main",
    "ahead": 0,
    "behind": 0,
    "tracking": "",
    "commitMessage": "feat: initial commit",
    "status": "clean",
    "isMain": true,
    "isCurrent": true,
    "updateAt": <timestamp>,
    "updateAtRelative": "<time>"
  }
]`)
	})

	t.Run("未コミットの変更や ahead を反映する", func(t *testing.T) {
		wt := filepath.Join(e.dir, "feature-a")
		e.git(wt, "push", "-u", "origin", "feature-a")
		if err := os.WriteFile(filepath.Join(wt, "new.txt"), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
		e.git(wt, "add", "new.txt")
		e.git(wt, "commit", "-m", "feat: add new")
		if err := os.WriteFile(filepath.Join(wt, "dirty.txt"), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}

		r := e.gdn("wt", "list", "-C", e.repo, "--no-color", "--sort", "branch", "--reverse", "--column", "branch,status,upstream,tracking,commitMessage,isCurrent")

		e.expectOutput("stdout", r.stdout, ""+
			"feature-a\tdirty\torigin/feature-a\t⇡1\tfeat: add new\n"+
			"main\tclean\torigin/main\t\tfeat: initial commit\t✓")
	})

	t.Run("--include-branches で worktree の無いブランチも含める", func(t *testing.T) {
		e.git(e.repo, "branch", "orphan-branch")

		rows := e.wtList(t, "--include-branches", "--sort", "branch", "--reverse")

		if got := strings.Join(branches(rows), ","); got != "feature-a,main,orphan-branch" {
			t.Fatalf("branches = %s", got)
		}
		if rows[2]["path"] != "" || rows[2]["status"] != "" {
			t.Errorf("branch-only row = %v", rows[2])
		}
	})
}

func TestWtCreateAndSwitch(t *testing.T) {
	e := setupWtEnv(t)
	basedir := filepath.Join(e.root, "github.com", "testuser", "worktree-demo.wt")

	t.Run("worktree を新規作成する", func(t *testing.T) {
		r := e.gdn("wt", "create", "feature-b", "-C", e.repo)

		e.expectOutput("stdout", r.stdout, "<testDir>/gdn-root/github.com/testuser/worktree-demo.wt/feature-b")
		if got := e.git(strings.TrimSpace(r.stdout), "rev-parse", "--abbrev-ref", "HEAD"); got != "feature-b" {
			t.Errorf("HEAD = %q", got)
		}
	})

	t.Run("既に worktree があるブランチはエラー", func(t *testing.T) {
		r := e.gdn("wt", "create", "feature-a", "-C", e.repo)
		e.expectOutput("stderr", r.stderr, "gdn: worktree for branch 'feature-a' already exists: <testDir>/feature-a")
	})

	t.Run("--base で分岐元を指定し、分岐元の upstream を継承しない", func(t *testing.T) {
		r := e.gdn("wt", "create", "feature-based", "-C", e.repo, "--base", "origin/main")
		if r.stderr != "" {
			t.Fatalf("stderr = %q", r.stderr)
		}
		wt := strings.TrimSpace(r.stdout)

		if got := e.git(wt, "rev-parse", "--abbrev-ref", "HEAD"); got != "feature-based" {
			t.Errorf("HEAD = %q", got)
		}
		if got, _ := e.gitMaybe(wt, "rev-parse", "--abbrev-ref", "feature-based@{upstream}"); got != "" {
			t.Errorf("upstream = %q, want none", got)
		}
	})

	t.Run("既存の worktree へ切り替える", func(t *testing.T) {
		r := e.gdn("wt", "switch", "feature-a", "-C", e.repo)
		e.expectOutput("stdout", r.stdout, "<testDir>/feature-a")
	})

	t.Run("worktree が無ければ作成して切り替える", func(t *testing.T) {
		r := e.gdn("wt", "switch", "feature-c", "-C", e.repo)
		e.expectOutput("stdout", r.stdout, "<testDir>/gdn-root/github.com/testuser/worktree-demo.wt/feature-c")
	})

	t.Run("gdn.wtHook は作成時だけ新しい worktree で実行する", func(t *testing.T) {
		e.git(e.repo, "config", "gdn.wtHook", "echo hooked >> hook.log")

		created := e.gdn("wt", "switch", "feature-hook", "-C", e.repo)
		existing := e.gdn("wt", "switch", "feature-hook", "-C", e.repo)

		log, err := os.ReadFile(filepath.Join(basedir, "feature-hook", "hook.log"))
		if err != nil || string(log) != "hooked\n" {
			t.Errorf("hook.log = %q, %v", log, err)
		}
		if created.stdout != existing.stdout {
			t.Errorf("switch paths differ: %q vs %q", created.stdout, existing.stdout)
		}
		e.git(e.repo, "config", "--unset", "gdn.wtHook")
	})

	t.Run("作成先が別ブランチの worktree に使われていれば報告する", func(t *testing.T) {
		// worktree 作成後に中でブランチを切り替え、ディレクトリ名とブランチ名がずれた状態を作る。
		wt := strings.TrimSpace(e.gdn("wt", "create", "feature-d", "-C", e.repo).stdout)
		e.git(wt, "switch", "-c", "feature-d-renamed")

		r := e.gdn("wt", "switch", "feature-d", "-C", e.repo)

		if r.stdout != "" {
			t.Errorf("stdout = %q", r.stdout)
		}
		e.expectOutput("stderr", r.stderr, ""+
			"gdn: cannot create worktree for 'feature-d': '<testDir>/gdn-root/github.com/testuser/worktree-demo.wt/feature-d' is already used by the worktree for branch 'feature-d-renamed'\n"+
			"hint: move it with 'git worktree move <testDir>/gdn-root/github.com/testuser/worktree-demo.wt/feature-d <new-path>'")
	})
}

func TestWtDelete(t *testing.T) {
	e := setupWtEnv(t)
	e.gdn("wt", "create", "feature-b", "-C", e.repo)

	t.Run("worktree とブランチを削除する", func(t *testing.T) {
		r := e.gdn("wt", "delete", "feature-b", "-C", e.repo, "--force")

		e.expectOutput("stdout", r.stdout, "Deleted worktree and branch: feature-b")
		if got := branches(e.wtList(t, "--include-branches")); strings.Contains(strings.Join(got, ","), "feature-b") {
			t.Errorf("feature-b still exists: %v", got)
		}
	})

	t.Run("未マージのブランチは worktree だけ削除して警告する", func(t *testing.T) {
		wt := strings.TrimSpace(e.gdn("wt", "create", "unmerged", "-C", e.repo).stdout)
		e.git(wt, "commit", "--allow-empty", "-m", "wip")

		r := e.gdn("wt", "delete", "unmerged", "-C", e.repo)

		e.expectOutput("stdout", r.stdout, "Deleted worktree: unmerged")
		if r.code != 0 || !strings.Contains(r.stderr, "gdn: warning: branch 'unmerged' was not deleted") {
			t.Errorf("code = %d, stderr = %q", r.code, r.stderr)
		}
		if exists(wt) {
			t.Error("worktree should be removed")
		}
	})

	t.Run("gdn.wtRemover が設定されていればそのコマンドで削除し、git の管理情報も消す", func(t *testing.T) {
		wt := strings.TrimSpace(e.gdn("wt", "create", "custom-removed", "-C", e.repo).stdout)
		e.git(e.repo, "config", "gdn.wtRemover", "rm -rf")
		defer e.git(e.repo, "config", "--unset", "gdn.wtRemover")

		r := e.gdn("wt", "delete", "custom-removed", "-C", e.repo)

		e.expectOutput("stdout", r.stdout, "Deleted worktree and branch: custom-removed")
		if exists(wt) {
			t.Error("worktree directory should be removed")
		}
		if out := e.git(e.repo, "worktree", "list"); strings.Contains(out, "custom-removed") {
			t.Errorf("worktree metadata remains: %s", out)
		}
	})

	t.Run("デフォルトブランチは削除できない", func(t *testing.T) {
		r := e.gdn("wt", "delete", "main", "-C", e.repo)
		e.expectOutput("stderr", r.stderr, "gdn: cannot delete default branch worktree: main")
	})

	t.Run("存在しないブランチはエラー", func(t *testing.T) {
		r := e.gdn("wt", "delete", "no-such-branch", "-C", e.repo)
		e.expectOutput("stderr", r.stderr, "gdn: worktree for branch 'no-such-branch' not found")
	})
}

func TestWtPrune(t *testing.T) {
	e := setupWtEnv(t)
	e.gdn("wt", "create", "feature-c", "-C", e.repo)
	e.gdn("wt", "create", "feature-alive", "-C", e.repo)
	// feature-a / feature-c を origin に push して upstream を張り、
	// リモートから ref を削除することで [gone] 状態を作る。
	for _, b := range []string{"feature-a", "feature-c", "feature-alive"} {
		e.git(e.repo, "push", "-u", "origin", b)
	}
	e.git(e.remote, "branch", "-D", "feature-a", "feature-c")

	t.Run("--dry-run で対象を表示する", func(t *testing.T) {
		r := e.gdn("wt", "prune", "-C", e.repo, "--dry-run")
		e.expectOutput("stdout", r.stdout, ""+
			"[DRY RUN] Would prune these branches:\n"+
			"  feature-a (<testDir>/feature-a)\n"+
			"  feature-c (<testDir>/gdn-root/github.com/testuser/worktree-demo.wt/feature-c)")
	})

	t.Run("確認入力が無ければ中止する", func(t *testing.T) {
		r := e.gdn("wt", "prune", "-C", e.repo)
		if r.code != 1 || !strings.Contains(r.stderr, "Prune 2 worktree(s)? [y/N]: ") {
			t.Errorf("code = %d, stderr = %q", r.code, r.stderr)
		}
		if !exists(filepath.Join(e.dir, "feature-a")) {
			t.Error("aborted prune must not remove worktrees")
		}
	})

	t.Run("upstream がリモートで削除された worktree を削除する", func(t *testing.T) {
		r := e.gdnWith("y\n", nil, "wt", "prune", "-C", e.repo)
		e.expectOutput("stdout", r.stdout, "Pruned: feature-a\nPruned: feature-c\nPruned 2 worktree(s).")
		if got := strings.Join(branches(e.wtList(t, "--include-branches", "--sort", "branch", "--reverse")), ","); got != "feature-alive,main" {
			t.Errorf("remaining branches = %s", got)
		}
	})

	t.Run("対象が無ければその旨を表示する", func(t *testing.T) {
		r := e.gdn("wt", "prune", "-C", e.repo, "--dry-run")
		e.expectOutput("stdout", r.stdout, "No worktrees to prune.")
	})
}

func TestWtMigrate(t *testing.T) {
	t.Run("すべて正しい場所にあれば何もしない", func(t *testing.T) {
		e := newTestEnv(t)
		repo := filepath.Join(e.root, "github.com", "testuser", "tidy")
		e.initRepo(repo, "feat: initial")
		e.git(repo, "worktree", "add", "-b", "feature", filepath.Join(e.root, "github.com", "testuser", "tidy.wt", "feature"))

		r := e.gdn("wt", "migrate", "-C", repo, "--dry-run")

		e.expectOutput("stdout", r.stdout, "All worktrees are already in the correct location.")
	})

	e := newTestEnv(t)
	repo := filepath.Join(e.root, "github.com", "testuser", "migrate-wt-demo")
	e.initRepo(repo, "feat: initial")
	// basedir の外に worktree を作る。
	outOfPlace := filepath.Join(e.dir, "out-of-place-wt")
	e.git(repo, "worktree", "add", "-b", "feature-x", outOfPlace)
	e.git(repo, "worktree", "add", "--detach", filepath.Join(e.dir, "detached-wt"))
	target := filepath.Join(e.root, "github.com", "testuser", "migrate-wt-demo.wt", "feature-x")

	t.Run("--dry-run で移動予定を表示する（detached HEAD は対象外）", func(t *testing.T) {
		r := e.gdn("wt", "migrate", "-C", repo, "--dry-run")
		e.expectOutput("stdout", r.stdout, ""+
			"[DRY RUN] Would move:\n"+
			"  <testDir>/out-of-place-wt\n"+
			"  → <testDir>/gdn-root/github.com/testuser/migrate-wt-demo.wt/feature-x")
	})

	t.Run("worktree を basedir 配下へ移動し、git の管理情報も更新する", func(t *testing.T) {
		r := e.gdn("wt", "migrate", "-C", repo, "--yes")

		e.expectOutput("stdout", r.stdout, ""+
			"Moved: feature-x → <testDir>/gdn-root/github.com/testuser/migrate-wt-demo.wt/feature-x\n"+
			"Migrated 1 worktree(s).")
		if !exists(target) || exists(outOfPlace) {
			t.Fatal("worktree was not moved")
		}
		if got := e.git(target, "rev-parse", "--abbrev-ref", "HEAD"); got != "feature-x" {
			t.Errorf("HEAD = %q", got)
		}
		if out := e.git(repo, "worktree", "list", "--porcelain"); !strings.Contains(out, "worktree "+target+"\n") {
			t.Errorf("git does not know the new path:\n%s", out)
		}
	})
}
