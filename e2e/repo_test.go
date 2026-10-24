package e2e

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

// setupRepoEnv は gdn.root 配下に 3 つのリポジトリを持つ環境を作る。
func setupRepoEnv(t *testing.T) *testEnv {
	e := newTestEnv(t)
	e.initRepo(filepath.Join(e.root, "github.com", "testuser", "myrepo"), "feat: initial commit")
	e.initRepo(filepath.Join(e.root, "github.com", "testuser", "another-repo"), "chore: setup")
	e.initRepo(filepath.Join(e.root, "gitlab.com", "otheruser", "backend"), "feat: backend init")
	return e
}

func TestRepoRoot(t *testing.T) {
	e := setupRepoEnv(t)

	r := e.gdn("repo", "root")

	e.expectOutput("stdout", r.stdout, "<testDir>/gdn-root")
}

func TestRepoList(t *testing.T) {
	e := setupRepoEnv(t)

	t.Run("タブ区切りで一覧を出力する", func(t *testing.T) {
		r := e.gdn("repo", "list", "--no-color", "--sort", "repo", "--reverse")
		e.expectOutput("stdout", r.stdout, ""+
			"github.com/testuser/another-repo\t<time>\n"+
			"github.com/testuser/myrepo\t<time>\n"+
			"gitlab.com/otheruser/backend\t<time>")
	})

	t.Run("--limit で件数を絞る", func(t *testing.T) {
		r := e.gdn("repo", "list", "--limit", "1", "--no-color", "--sort", "repo", "--reverse")
		e.expectOutput("stdout", r.stdout, "github.com/testuser/another-repo\t<time>")
	})

	t.Run("--json で JSON を出力する", func(t *testing.T) {
		r := e.gdn("repo", "list", "--json", "--sort", "repo", "--reverse")
		e.expectOutput("stdout", r.stdout, `[
  {
    "repo": "github.com/testuser/another-repo",
    "host": "github.com",
    "owner": "testuser",
    "name": "another-repo",
    "path": "<testDir>/gdn-root/github.com/testuser/another-repo",
    "branch": "main",
    "worktreeCount": 1,
    "commitMessage": "chore: setup",
    "updateAt": <timestamp>,
    "updateAtRelative": "<time>"
  },
  {
    "repo": "github.com/testuser/myrepo",
    "host": "github.com",
    "owner": "testuser",
    "name": "myrepo",
    "path": "<testDir>/gdn-root/github.com/testuser/myrepo",
    "branch": "main",
    "worktreeCount": 1,
    "commitMessage": "feat: initial commit",
    "updateAt": <timestamp>,
    "updateAtRelative": "<time>"
  },
  {
    "repo": "gitlab.com/otheruser/backend",
    "host": "gitlab.com",
    "owner": "otheruser",
    "name": "backend",
    "path": "<testDir>/gdn-root/gitlab.com/otheruser/backend",
    "branch": "main",
    "worktreeCount": 1,
    "commitMessage": "feat: backend init",
    "updateAt": <timestamp>,
    "updateAtRelative": "<time>"
  }
]`)
	})

	t.Run("--column で出力列を選ぶ", func(t *testing.T) {
		r := e.gdn("repo", "list", "--column", "host,owner,name", "--no-color", "--sort", "repo", "--reverse")
		e.expectOutput("stdout", r.stdout, ""+
			"github.com\ttestuser\tanother-repo\n"+
			"github.com\ttestuser\tmyrepo\n"+
			"gitlab.com\totheruser\tbackend")
	})

	t.Run("未知の列はエラーになる", func(t *testing.T) {
		r := e.gdn("repo", "list", "--column", "repo,nope")
		if r.code != 1 || !strings.HasPrefix(r.stderr, "gdn: unknown column 'nope'") {
			t.Errorf("code = %d, stderr = %q", r.code, r.stderr)
		}
	})

	t.Run("--include-worktrees で linked worktree も root からの相対パスで含める", func(t *testing.T) {
		repo := filepath.Join(e.root, "github.com", "testuser", "myrepo")
		e.git(repo, "worktree", "add", "-b", "feat/x", filepath.Join(e.root, "github.com", "testuser", "myrepo.wt", "feat", "x"))

		r := e.gdn("repo", "list", "--include-worktrees", "--no-color", "--sort", "repo", "--reverse", "--column", "repo,branch,worktreeCount")

		e.expectOutput("stdout", r.stdout, ""+
			"github.com/testuser/another-repo\tmain\t1\n"+
			"github.com/testuser/myrepo\tmain\t2\n"+
			"github.com/testuser/myrepo.wt/feat/x\tfeat/x\t2\n"+
			"gitlab.com/otheruser/backend\tmain\t1")
	})

	t.Run("root が無ければ空の一覧を返す", func(t *testing.T) {
		empty := newTestEnv(t)
		if r := empty.gdn("repo", "list"); r.code != 0 || r.stdout != "" {
			t.Errorf("code = %d, stdout = %q", r.code, r.stdout)
		}
		if r := empty.gdn("repo", "list", "--json"); strings.TrimSpace(r.stdout) != "[]" {
			t.Errorf("json stdout = %q", r.stdout)
		}
	})
}

func TestRepoClone(t *testing.T) {
	e := newTestEnv(t)
	source := filepath.Join(e.dir, "source-repo")
	e.initRepo(source, "feat: hello")

	t.Run("ローカルリポジトリをクローンする", func(t *testing.T) {
		r := e.gdn("repo", "clone", source)

		e.expectOutput("stdout", r.stdout, "<testDir>/gdn-root/localhost/local/source-repo")
		if got := e.git(strings.TrimSpace(r.stdout), "log", "-1", "--format=%s"); got != "feat: hello" {
			t.Errorf("cloned log = %q", got)
		}
	})

	t.Run("クローン先が既にあればエラー", func(t *testing.T) {
		r := e.gdn("repo", "clone", source)
		if r.code != 1 {
			t.Errorf("code = %d", r.code)
		}
		e.expectOutput("stderr", r.stderr, "gdn: destination already exists: <testDir>/gdn-root/localhost/local/source-repo")
	})

	t.Run("gdn.postCloneHook をクローン先で実行する", func(t *testing.T) {
		other := filepath.Join(e.dir, "hooked-repo")
		e.initRepo(other, "feat: hooked")
		// GIT_CONFIG_GLOBAL がテスト用ファイルを指しているので、--global はそこへ書き込まれる。
		e.git(e.dir, "config", "--global", "gdn.postCloneHook", "touch cloned-marker")

		r := e.gdn("repo", "clone", other)

		if r.code != 0 || !exists(filepath.Join(strings.TrimSpace(r.stdout), "cloned-marker")) {
			t.Errorf("hook did not run: code = %d, stderr = %q", r.code, r.stderr)
		}
	})
}

func TestRepoMigrate(t *testing.T) {
	e := setupRepoEnv(t)
	migrant := filepath.Join(e.dir, "migrant-repo")
	e.initRepo(migrant, "feat: initial")
	e.git(migrant, "remote", "add", "origin", "https://github.com/testuser/migrant.git")
	e.git(migrant, "config", "--add", "wt.hook", "echo hi")
	dest := filepath.Join(e.root, "github.com", "testuser", "migrant")

	t.Run("--dry-run で移動予定と設定移行予定を表示し、何も変更しない", func(t *testing.T) {
		r := e.gdn("repo", "migrate", migrant, "--dry-run")

		e.expectOutput("stdout", r.stdout, ""+
			"[DRY RUN] Would move:\n"+
			"  <testDir>/migrant-repo\n"+
			"  → <testDir>/gdn-root/github.com/testuser/migrant\n"+
			"Config migrations:\n"+
			"  Migrate wt.hook=echo hi → gdn.wtHook=echo hi")
		if exists(dest) {
			t.Error("dry-run must not move the repository")
		}
		if got, _ := e.gitMaybe(migrant, "config", "--local", "--get", "gdn.wtHook"); got != "" {
			t.Errorf("dry-run must not migrate config, got gdn.wtHook=%q", got)
		}
	})

	t.Run("確認で n と答えると中止する", func(t *testing.T) {
		r := e.gdnWith("n\n", nil, "repo", "migrate", migrant)

		if r.code != 1 || !strings.Contains(r.stderr, "Proceed? [y/N]: ") || !strings.HasSuffix(r.stderr, "gdn: aborted\n") {
			t.Errorf("code = %d, stderr = %q", r.code, r.stderr)
		}
		if exists(dest) || !exists(migrant) {
			t.Error("aborted migrate must not move the repository")
		}
	})

	t.Run("gdn の構造へ移動し、設定を移行する", func(t *testing.T) {
		r := e.gdn("repo", "migrate", migrant, "--yes")

		e.expectOutput("stdout", r.stdout, "<testDir>/gdn-root/github.com/testuser/migrant")
		if exists(migrant) || !exists(dest) {
			t.Fatal("repository was not moved")
		}
		if got := e.git(dest, "log", "-1", "--format=%s"); got != "feat: initial" {
			t.Errorf("log = %q", got)
		}
		if got := e.git(dest, "config", "--local", "--get", "gdn.wtHook"); got != "echo hi" {
			t.Errorf("gdn.wtHook = %q", got)
		}
	})

	t.Run("既に正しい場所にあれば報告する", func(t *testing.T) {
		r := e.gdn("repo", "migrate", dest, "--yes")
		e.expectOutput("stderr", r.stderr, "Repository is already in the correct location.")
	})

	t.Run("サブディレクトリから実行してもリポジトリ全体を対象にする", func(t *testing.T) {
		r := e.gdn("repo", "migrate", filepath.Join(dest, ".git"), "--dry-run")
		e.expectOutput("stderr", r.stderr, "Repository is already in the correct location.")
	})

	t.Run("origin が無ければエラー", func(t *testing.T) {
		noRemote := filepath.Join(e.dir, "no-remote-repo")
		e.initRepo(noRemote, "initial")

		r := e.gdn("repo", "migrate", noRemote, "--yes")

		e.expectOutput("stderr", r.stderr, "gdn: no remote 'origin' found in <testDir>/no-remote-repo")
	})

	t.Run("移動先が既にあればエラー", func(t *testing.T) {
		conflict := filepath.Join(e.dir, "conflict-repo")
		e.initRepo(conflict, "initial")
		e.git(conflict, "remote", "add", "origin", "https://github.com/testuser/myrepo.git")

		r := e.gdn("repo", "migrate", conflict, "--yes")

		e.expectOutput("stderr", r.stderr, "gdn: destination already exists: <testDir>/gdn-root/github.com/testuser/myrepo")
	})

	t.Run("linked worktree を持つリポジトリを移動しても worktree から参照できる", func(t *testing.T) {
		withWt := filepath.Join(e.dir, "with-wt")
		e.initRepo(withWt, "initial")
		e.git(withWt, "remote", "add", "origin", "git@github.com:testuser/with-wt.git")
		wtPath := filepath.Join(e.dir, "with-wt-feature")
		e.git(withWt, "worktree", "add", "-b", "feature", wtPath)

		r := e.gdn("repo", "migrate", withWt, "--yes")
		if r.code != 0 {
			t.Fatalf("stderr = %q", r.stderr)
		}

		if got := e.git(wtPath, "rev-parse", "--abbrev-ref", "HEAD"); got != "feature" {
			t.Errorf("worktree HEAD = %q", got)
		}
		var entries []map[string]any
		list := e.gdn("wt", "list", "-C", wtPath, "--json")
		if err := json.Unmarshal([]byte(list.stdout), &entries); err != nil || len(entries) != 2 {
			t.Errorf("wt list after migrate: %v %q %q", err, list.stdout, list.stderr)
		}
	})
}
