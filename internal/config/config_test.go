package config

import (
	"context"
	"slices"
	"strings"
	"testing"

	"github.com/elecdeer/git-gardener/internal/git"
)

// configRunner は git config の呼び出しに対して values の値を返す Runner。
// キーは "<スコープ引数> <key>"（例: "--global gdn.root"）、スコープ指定なしは "<key>"。
type configRunner struct {
	values map[string][]string
}

// Run は git config --get/--get-all に応答し、未設定なら終了コード 1 を返す。
func (r configRunner) Run(_ context.Context, cmd git.Command) (string, error) {
	args := slices.DeleteFunc(slices.Clone(cmd.Args), func(a string) bool {
		return a == "config" || a == "--get" || a == "--get-all"
	})
	if v, ok := r.values[strings.Join(args, " ")]; ok {
		return strings.Join(v, "\n") + "\n", nil
	}
	return "", &git.Error{Args: cmd.Args, ExitCode: 1}
}

// newTestConfig はホームディレクトリを /home/user に固定した Config を返す。
func newTestConfig(values map[string][]string) *Config {
	c := New(git.New(configRunner{values: values}), "/repo")
	c.homeDir = func() (string, error) { return "/home/user", nil }
	return c
}

func TestWtBasedir(t *testing.T) {
	tests := []struct {
		name   string
		values map[string][]string
		want   string
	}{
		{"チルダパスをホームディレクトリに展開する", map[string][]string{"gdn.wtBasedir": {"~/worktrees"}}, "/home/user/worktrees"},
		{"チルダのみのパスをホームディレクトリに展開する", map[string][]string{"gdn.wtBasedir": {"~"}}, "/home/user"},
		{"チルダパスで {gitroot} プレースホルダーを展開する", map[string][]string{"gdn.wtBasedir": {"~/worktrees/{gitroot}.wt"}}, "/home/user/worktrees/myrepo.wt"},
		{"設定がない場合はデフォルト値を返す", nil, "../myrepo.wt"},
		{"絶対パスはそのまま返す", map[string][]string{"gdn.wtBasedir": {"/absolute/path"}}, "/absolute/path"},
		{"~user 形式は展開しない", map[string][]string{"gdn.wtBasedir": {"~other/wt"}}, "~other/wt"},
		{"複数設定されている場合は最後（ローカル）の値を使う", map[string][]string{"gdn.wtBasedir": {"/global", "/local"}}, "/local"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := newTestConfig(tt.values).WtBasedir(t.Context(), "myrepo")
			if err != nil || got != tt.want {
				t.Errorf("got (%q, %v), want %q", got, err, tt.want)
			}
		})
	}
}

func TestRoot(t *testing.T) {
	t.Run("グローバル設定のチルダを展開する", func(t *testing.T) {
		got, err := newTestConfig(map[string][]string{"--global gdn.root": {"~/ghq"}}).Root(t.Context())
		if err != nil || got != "/home/user/ghq" {
			t.Errorf("got (%q, %v)", got, err)
		}
	})

	t.Run("ローカル設定は参照しない", func(t *testing.T) {
		got, err := newTestConfig(map[string][]string{"gdn.root": {"/local"}}).Root(t.Context())
		if err != nil || got != "/home/user/gdn" {
			t.Errorf("got (%q, %v)", got, err)
		}
	})

	t.Run("複数設定されている場合はエラー", func(t *testing.T) {
		_, err := newTestConfig(map[string][]string{"--global gdn.root": {"/a", "/b"}}).Root(t.Context())
		if err == nil {
			t.Error("expected an error")
		}
	})
}

func TestDefaults(t *testing.T) {
	c := newTestConfig(nil)
	ctx := t.Context()

	host, err := c.DefaultHost(ctx)
	if err != nil || host != "github.com" {
		t.Errorf("DefaultHost = (%q, %v)", host, err)
	}
	remover, err := c.WtRemover(ctx)
	if err != nil || remover != DefaultWtRemover {
		t.Errorf("WtRemover = (%q, %v)", remover, err)
	}
	hooks, err := c.WtHooks(ctx)
	if err != nil || hooks != nil {
		t.Errorf("WtHooks = (%v, %v)", hooks, err)
	}
}

func TestHooks(t *testing.T) {
	c := newTestConfig(map[string][]string{
		"gdn.wtHook":                 {"cp ../.env .env", "pnpm install"},
		"gdn.wtDeleteHook":           {"echo bye"},
		"--global gdn.postCloneHook": {"echo cloned"},
	})
	ctx := t.Context()

	if got, _ := c.WtHooks(ctx); !slices.Equal(got, []string{"cp ../.env .env", "pnpm install"}) {
		t.Errorf("WtHooks = %v", got)
	}
	if got, _ := c.WtDeleteHooks(ctx); !slices.Equal(got, []string{"echo bye"}) {
		t.Errorf("WtDeleteHooks = %v", got)
	}
	if got, _ := c.PostCloneHooks(ctx); !slices.Equal(got, []string{"echo cloned"}) {
		t.Errorf("PostCloneHooks = %v", got)
	}
}
