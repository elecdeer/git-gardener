package git

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestParseWorktreePorcelain(t *testing.T) {
	raw := "worktree /project.git\nbare\n\n" +
		"worktree /project.wt/feat\nHEAD 0123456789abcdef\nbranch refs/heads/feat/foo\n\n" +
		"worktree /project.wt/detached\nHEAD fedcba9876543210\ndetached\n"

	got := ParseWorktreePorcelain(raw)

	want := []WorktreeEntry{
		{Path: "/project.git", Branch: "HEAD", Bare: true},
		{Path: "/project.wt/feat", Branch: "feat/foo", Hash: "0123456"},
		{Path: "/project.wt/detached", Branch: "HEAD", Hash: "fedcba9"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
	if got[0].Detached() || got[1].Detached() || !got[2].Detached() {
		t.Errorf("Detached() = %v, %v, %v; want false, false, true", got[0].Detached(), got[1].Detached(), got[2].Detached())
	}
}

func TestMainWorktreePath(t *testing.T) {
	tests := []struct {
		name    string
		entries []worktreeEntry
		dir     string
	}{
		{
			name: "worktree 内から実行してもメインリポジトリのパスを返す",
			entries: []worktreeEntry{
				{"/project", "abc1234", "main"},
				{"/project.wt/feat/foo", "def5678", "feat/foo"},
			},
			dir: "/project.wt/feat/foo",
		},
		{
			name:    "メイン worktree から実行した場合はそのパスをそのまま返す",
			entries: []worktreeEntry{{"/project", "abc1234", "main"}},
			dir:     "/project",
		},
		{
			name: "worktree が複数あってもリストの先頭（メイン）を返す",
			entries: []worktreeEntry{
				{"/project", "abc1234", "main"},
				{"/project.wt/feat/a", "aaa0001", "feat/a"},
				{"/project.wt/feat/b", "bbb0002", "feat/b"},
			},
			dir: "/project.wt/feat/b",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := &fakeRunner{handler: func(Command) (string, error) { return porcelain(tt.entries...), nil }}

			got, err := New(runner).MainWorktreePath(t.Context(), tt.dir)

			if err != nil || got != "/project" {
				t.Fatalf("got (%q, %v), want (%q, nil)", got, err, "/project")
			}
		})
	}

	t.Run("読み取り専用 Git コマンドでは optional lock を無効化する", func(t *testing.T) {
		runner := &fakeRunner{handler: func(Command) (string, error) {
			return porcelain(worktreeEntry{"/project", "abc1234", "main"}), nil
		}}

		_, _ = New(runner).MainWorktreePath(t.Context(), "/project")

		cmd, ok := runner.find("worktree", "list", "--porcelain")
		if !ok || !cmd.ReadOnly {
			t.Errorf("worktree list was not run as read-only: %+v", runner.calls)
		}
	})

	t.Run("git リポジトリでなければエラーを返す", func(t *testing.T) {
		runner := &fakeRunner{handler: func(cmd Command) (string, error) { return "", failWith(128, cmd.Args) }}

		_, err := New(runner).MainWorktreePath(t.Context(), "/not-repo")

		if err == nil || err.Error() != "not a git repository: /not-repo" {
			t.Errorf("err = %v", err)
		}
	})
}

func TestSwitchWorktree(t *testing.T) {
	t.Run("既存 worktree があればそのパスを返し、status などを実行しない", func(t *testing.T) {
		runner := &fakeRunner{handler: func(Command) (string, error) {
			return porcelain(
				worktreeEntry{"/project", "abc1234", "main"},
				worktreeEntry{"/project.wt/feat/existing", "def5678", "feat/existing"},
			), nil
		}}

		path, created, err := New(runner).SwitchWorktree(t.Context(), "/project", "feat/existing", "/project.wt", "")

		if err != nil || path != "/project.wt/feat/existing" || created {
			t.Fatalf("got (%q, %v, %v)", path, created, err)
		}
		want := [][]string{{"worktree", "list", "--porcelain"}}
		if got := runner.argsList(); !reflect.DeepEqual(got, want) {
			t.Errorf("calls = %v, want %v", got, want)
		}
	})

	t.Run("worktree が無ければ basedir/branch に作成する", func(t *testing.T) {
		base := t.TempDir()
		runner := &fakeRunner{handler: func(cmd Command) (string, error) {
			switch cmd.Args[0] {
			case "worktree":
				if cmd.Args[1] == "list" {
					return porcelain(worktreeEntry{"/project", "abc1234", "main"}), nil
				}
				return "", nil
			case "rev-parse":
				return "", failWith(1, cmd.Args)
			}
			return "", nil
		}}

		path, created, err := New(runner).SwitchWorktree(t.Context(), "/project", "feat/new", base, "")

		want := filepath.Join(base, "feat/new")
		if err != nil || path != want || !created {
			t.Fatalf("got (%q, %v, %v), want (%q, true, nil)", path, created, err, want)
		}
	})
}

// branchHandler は rev-parse（ローカルブランチ確認）と branch -r（リモート確認）の結果を切り替える handler を作る。
func branchHandler(local bool, remoteOut string) func(Command) (string, error) {
	return func(cmd Command) (string, error) {
		switch cmd.Args[0] {
		case "rev-parse":
			if local {
				return "abc1234\n", nil
			}
			return "", failWith(1, cmd.Args)
		case "branch":
			return remoteOut, nil
		}
		return "", nil
	}
}

func TestAddWorktree(t *testing.T) {
	tests := []struct {
		name      string
		local     bool
		remoteOut string
		base      string
		wantArgs  []string
		wantCfg   []string
	}{
		{
			name:     "ローカルにもリモートにも存在しない場合は -b で新規ブランチを作成する",
			wantArgs: []string{"worktree", "add", "-b", "feat/new", "/project.wt/feat/new"},
		},
		{
			name:     "ローカルにもリモートにも存在しない場合に base を分岐元として渡し、upstream を継承しない",
			base:     "main",
			wantArgs: []string{"worktree", "add", "-b", "feat/new", "/project.wt/feat/new", "main"},
			wantCfg:  []string{"branch.autoSetupMerge=false"},
		},
		{
			name:      "リモートにのみ存在する場合は -b なしで worktree を作成する",
			remoteOut: "  origin/feat/new\n",
			wantArgs:  []string{"worktree", "add", "/project.wt/feat/new", "feat/new"},
		},
		{
			name:      "リモートにのみ存在する場合は base を無視する",
			remoteOut: "  origin/feat/new\n",
			base:      "main",
			wantArgs:  []string{"worktree", "add", "/project.wt/feat/new", "feat/new"},
		},
		{
			name:     "ローカルに存在する場合は既存ブランチを追加する",
			local:    true,
			wantArgs: []string{"worktree", "add", "/project.wt/feat/new", "feat/new"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := &fakeRunner{handler: branchHandler(tt.local, tt.remoteOut)}

			_, err := New(runner).AddWorktree(t.Context(), "/project", "feat/new", "/project.wt/feat/new", tt.base)

			if err != nil {
				t.Fatal(err)
			}
			cmd, ok := runner.find(tt.wantArgs...)
			if !ok {
				t.Fatalf("worktree add %v was not called: %v", tt.wantArgs, runner.argsList())
			}
			if cmd.ReadOnly {
				t.Error("worktree add must not disable optional locks")
			}
			if !reflect.DeepEqual(cmd.Config, tt.wantCfg) {
				t.Errorf("config = %v, want %v", cmd.Config, tt.wantCfg)
			}
		})
	}

	t.Run("ローカルに存在する場合はリモート確認をスキップする", func(t *testing.T) {
		runner := &fakeRunner{handler: branchHandler(true, "")}

		_, _ = New(runner).AddWorktree(t.Context(), "/project", "feat/local", "/project.wt/feat/local", "")

		if runner.called("branch") {
			t.Errorf("branch -r should not be called: %v", runner.argsList())
		}
	})
}

func TestAddWorktreeWhenPathExists(t *testing.T) {
	// setup は project.wt/main を作成先とする一時ディレクトリを用意する。
	setup := func(t *testing.T) (tmp, wtDir string) {
		tmp, err := filepath.EvalSymlinks(t.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		return tmp, filepath.Join(tmp, "project.wt", "main")
	}
	writeFile := func(t *testing.T, path, content string) {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	t.Run("別ブランチの worktree が使っているパスならそのブランチ名を含むエラーを返し、worktree add を実行しない", func(t *testing.T) {
		tmp, wtDir := setup(t)
		writeFile(t, filepath.Join(wtDir, ".git"), "gitdir: /project/.git/worktrees/main\n")
		runner := &fakeRunner{handler: func(Command) (string, error) {
			return porcelain(
				worktreeEntry{filepath.Join(tmp, "project"), "abc1234", "develop"},
				worktreeEntry{wtDir, "def5678", "feat/other"},
			), nil
		}}

		_, err := New(runner).AddWorktree(t.Context(), filepath.Join(tmp, "project"), "main", wtDir, "")

		want := "cannot create worktree for 'main': '" + wtDir + "' is already used by the worktree for branch 'feat/other'\n" +
			"hint: move it with 'git worktree move " + wtDir + " <new-path>'"
		if err == nil || err.Error() != want {
			t.Errorf("err = %v, want %q", err, want)
		}
		if runner.called("worktree", "add") {
			t.Error("worktree add should not be called")
		}
	})

	t.Run("detached HEAD の worktree が使っているパスならその旨を含むエラーを返す", func(t *testing.T) {
		tmp, wtDir := setup(t)
		writeFile(t, filepath.Join(wtDir, ".git"), "gitdir: /project/.git/worktrees/main\n")
		runner := &fakeRunner{handler: func(Command) (string, error) {
			return "worktree " + filepath.Join(tmp, "project") + "\nHEAD abc1234\nbranch refs/heads/develop\n\n" +
				"worktree " + wtDir + "\nHEAD def5678\ndetached\n", nil
		}}

		_, err := New(runner).AddWorktree(t.Context(), filepath.Join(tmp, "project"), "main", wtDir, "")

		if err == nil || !strings.Contains(err.Error(), "is already used by a detached HEAD worktree") {
			t.Errorf("err = %v", err)
		}
	})

	t.Run("worktree ではない空でないディレクトリならエラーを返し、worktree add を実行しない", func(t *testing.T) {
		tmp, wtDir := setup(t)
		writeFile(t, filepath.Join(wtDir, "file.txt"), "content\n")
		runner := &fakeRunner{handler: func(Command) (string, error) {
			return porcelain(worktreeEntry{filepath.Join(tmp, "project"), "abc1234", "develop"}), nil
		}}

		_, err := New(runner).AddWorktree(t.Context(), filepath.Join(tmp, "project"), "main", wtDir, "")

		want := "cannot create worktree for 'main': '" + wtDir + "' already exists and is not an empty directory"
		if err == nil || err.Error() != want {
			t.Errorf("err = %v, want %q", err, want)
		}
		if runner.called("worktree", "add") {
			t.Error("worktree add should not be called")
		}
	})

	t.Run("同名のファイルが置かれている場合もエラーを返す", func(t *testing.T) {
		tmp, wtDir := setup(t)
		writeFile(t, wtDir, "content\n")
		runner := &fakeRunner{handler: func(Command) (string, error) {
			return porcelain(worktreeEntry{filepath.Join(tmp, "project"), "abc1234", "develop"}), nil
		}}

		_, err := New(runner).AddWorktree(t.Context(), filepath.Join(tmp, "project"), "main", wtDir, "")

		if err == nil {
			t.Error("expected an error")
		}
	})

	t.Run("空ディレクトリなら Git に任せて worktree add を実行する", func(t *testing.T) {
		tmp, wtDir := setup(t)
		if err := os.MkdirAll(wtDir, 0o755); err != nil {
			t.Fatal(err)
		}
		runner := &fakeRunner{handler: branchHandler(true, "")}

		_, err := New(runner).AddWorktree(t.Context(), filepath.Join(tmp, "project"), "main", wtDir, "")

		if err != nil {
			t.Fatal(err)
		}
		if _, ok := runner.find("worktree", "add", wtDir, "main"); !ok {
			t.Errorf("worktree add was not called: %v", runner.argsList())
		}
	})
}

func TestSamePath(t *testing.T) {
	tmp := t.TempDir()
	real := filepath.Join(tmp, "real")
	link := filepath.Join(tmp, "link")
	if err := os.Mkdir(real, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(real, link); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name string
		a, b string
		want bool
	}{
		{"シンボリックリンク経由でも同じ場所なら true", real, link, true},
		{"末尾の区切りや . を含んでも同じ場所なら true", real + "/", filepath.Join(tmp, ".", "real"), true},
		{"存在しないパス同士は文字列として比較する", filepath.Join(tmp, "x", "..", "y"), filepath.Join(tmp, "y"), true},
		{"異なる場所なら false", real, tmp, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := SamePath(tt.a, tt.b); got != tt.want {
				t.Errorf("SamePath(%q, %q) = %v, want %v", tt.a, tt.b, got, tt.want)
			}
		})
	}
}
