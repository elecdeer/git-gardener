package git

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestFormatTracking(t *testing.T) {
	tests := []struct {
		ahead, behind int
		want          string
	}{
		{0, 0, ""},
		{2, 0, "⇡2"},
		{0, 3, "⇣3"},
		{1, 4, "⇡1 ⇣4"},
	}
	for _, tt := range tests {
		if got := FormatTracking(tt.ahead, tt.behind); got != tt.want {
			t.Errorf("FormatTracking(%d, %d) = %q, want %q", tt.ahead, tt.behind, got, tt.want)
		}
	}
}

func TestParseTrack(t *testing.T) {
	tests := []struct {
		track         string
		ahead, behind int
		gone          bool
	}{
		{"", 0, 0, false},
		{"gone", 0, 0, true},
		{"ahead 2", 2, 0, false},
		{"behind 5", 0, 5, false},
		{"ahead 1, behind 3", 1, 3, false},
	}
	for _, tt := range tests {
		ahead, behind, gone := parseTrack(tt.track)
		if ahead != tt.ahead || behind != tt.behind || gone != tt.gone {
			t.Errorf("parseTrack(%q) = (%d, %d, %v), want (%d, %d, %v)", tt.track, ahead, behind, gone, tt.ahead, tt.behind, tt.gone)
		}
	}
}

func TestBranchRefs(t *testing.T) {
	line := func(fields ...string) string { return strings.Join(fields, fieldSep) }
	runner := &fakeRunner{handler: func(Command) (string, error) {
		return line("main", "0123456789", "origin/main", "", "1700000000", "feat: init") + "\n" +
			line("feat/x", "abcdef0123", "origin/feat/x", "gone", "1700000100", "wip: tab\tin subject") + "\n", nil
	}}

	got, err := New(runner).BranchRefs(t.Context(), "/project")

	if err != nil {
		t.Fatal(err)
	}
	want := []BranchRef{
		{Name: "main", Hash: "0123456", Upstream: "origin/main", CommitMessage: "feat: init", UpdateAt: 1700000000},
		{Name: "feat/x", Hash: "abcdef0", Upstream: "origin/feat/x", Gone: true, CommitMessage: "wip: tab\tin subject", UpdateAt: 1700000100},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestDefaultBranch(t *testing.T) {
	tests := []struct {
		name    string
		handler func(Command) (string, error)
		want    string
	}{
		{
			name:    "origin/HEAD があればそのブランチ名を返す",
			handler: func(Command) (string, error) { return "origin/develop\n", nil },
			want:    "develop",
		},
		{
			name: "origin/HEAD が無ければ init.defaultBranch を返す",
			handler: func(cmd Command) (string, error) {
				if cmd.Args[0] == "symbolic-ref" {
					return "", failWith(128, cmd.Args)
				}
				return "trunk\n", nil
			},
			want: "trunk",
		},
		{
			name:    "どちらも無ければ main を返す",
			handler: func(cmd Command) (string, error) { return "", failWith(1, cmd.Args) },
			want:    "main",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := New(&fakeRunner{handler: tt.handler}).DefaultBranch(t.Context(), "/project"); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestConfigGetAll(t *testing.T) {
	t.Run("スコープに応じた引数で git config を呼ぶ", func(t *testing.T) {
		runner := &fakeRunner{handler: func(Command) (string, error) { return "a\nb\n", nil }}

		got, err := New(runner).ConfigGetAll(t.Context(), "/project", ScopeGlobal, "gdn.wtHook")

		if err != nil || !reflect.DeepEqual(got, []string{"a", "b"}) {
			t.Fatalf("got (%v, %v)", got, err)
		}
		if _, ok := runner.find("config", "--get-all", "--global", "gdn.wtHook"); !ok {
			t.Errorf("unexpected calls: %v", runner.argsList())
		}
	})

	t.Run("終了コード 1（キーが無い）は未設定として扱う", func(t *testing.T) {
		runner := &fakeRunner{handler: func(cmd Command) (string, error) { return "", failWith(1, cmd.Args) }}

		got, err := New(runner).ConfigGetAll(t.Context(), "/project", ScopeAll, "gdn.wtHook")

		if err != nil || got != nil {
			t.Errorf("got (%v, %v), want (nil, nil)", got, err)
		}
	})

	t.Run("それ以外の失敗はエラーを返す", func(t *testing.T) {
		runner := &fakeRunner{handler: func(cmd Command) (string, error) { return "", failWith(3, cmd.Args) }}

		_, err := New(runner).ConfigGetAll(t.Context(), "/project", ScopeAll, "gdn.wtHook")

		var gitErr *Error
		if !errors.As(err, &gitErr) || gitErr.ExitCode != 3 {
			t.Errorf("err = %v", err)
		}
	})

	t.Run("ConfigGet は最後の値を返す", func(t *testing.T) {
		runner := &fakeRunner{handler: func(Command) (string, error) { return "global\nlocal\n", nil }}

		got, err := New(runner).ConfigGet(t.Context(), "/project", ScopeAll, "gdn.wtBasedir")

		if err != nil || got != "local" {
			t.Errorf("got (%q, %v)", got, err)
		}
	})
}

func TestWorktreeRepoName(t *testing.T) {
	root := t.TempDir()
	inside := filepath.Join(root, "github.com", "user", "repo.wt", "feat", "x")
	if err := os.MkdirAll(inside, 0o755); err != nil {
		t.Fatal(err)
	}
	outside := t.TempDir()

	if got := worktreeRepoName(root, inside); got != "github.com/user/repo.wt/feat/x" {
		t.Errorf("inside root: got %q", got)
	}
	if got := worktreeRepoName(root, outside); got != outside {
		t.Errorf("outside root: got %q, want %q", got, outside)
	}
}

func TestPrunableWorktrees(t *testing.T) {
	tmp := t.TempDir()
	mainDir := filepath.Join(tmp, "project")
	goneDir := filepath.Join(tmp, "project.wt", "gone")
	aliveDir := filepath.Join(tmp, "project.wt", "alive")
	for _, d := range []string{mainDir, goneDir, aliveDir} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	ref := func(name, track string) string {
		return strings.Join([]string{name, "0123456789", "origin/" + name, track, "1700000000", "msg"}, fieldSep)
	}

	runner := &fakeRunner{handler: func(cmd Command) (string, error) {
		switch cmd.Args[0] {
		case "worktree":
			return porcelain(
				worktreeEntry{mainDir, "abc1234", "main"},
				worktreeEntry{goneDir, "def5678", "gone"},
				worktreeEntry{aliveDir, "aaa0001", "alive"},
			), nil
		case "for-each-ref":
			// main の upstream が gone でもデフォルトブランチなので対象外になることを確認する。
			return ref("main", "gone") + "\n" + ref("gone", "gone") + "\n" + ref("alive", "ahead 1") + "\n", nil
		case "symbolic-ref":
			return "", failWith(128, cmd.Args)
		case "config":
			return "", failWith(1, cmd.Args)
		}
		return "", nil
	}}

	got, err := New(runner).PrunableWorktrees(t.Context(), mainDir, false)

	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Branch != "gone" {
		t.Errorf("got %+v, want only 'gone'", got)
	}
	if runner.called("fetch") {
		t.Error("fetch should not be called when fetch=false")
	}
}
