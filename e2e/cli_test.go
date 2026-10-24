package e2e

import (
	"strings"
	"testing"
)

func TestUnknownCommand(t *testing.T) {
	e := newTestEnv(t)

	tests := []struct {
		name string
		args []string
		want string
	}{
		{
			name: "トップレベルの未知のコマンドを報告する",
			args: []string{"foo"},
			want: "gdn: unknown command 'foo'\nUse 'gdn --help' to see available commands.",
		},
		{
			name: "wt の未知のサブコマンドを報告する",
			args: []string{"wt", "no-such"},
			want: "gdn: unknown subcommand 'wt no-such'\nUse 'gdn wt --help' to see available subcommands.",
		},
		{
			name: "repo の未知のサブコマンドを報告する",
			args: []string{"repo", "no-such"},
			want: "gdn: unknown subcommand 'repo no-such'\nUse 'gdn repo --help' to see available subcommands.",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := e.gdn(tt.args...)
			if r.stdout != "" || r.code != 1 {
				t.Errorf("stdout = %q, code = %d", r.stdout, r.code)
			}
			e.expectOutput("stderr", r.stderr, tt.want)
		})
	}

	t.Run("サブコマンドなしの wt はヘルプを表示する", func(t *testing.T) {
		r := e.gdn("wt")
		if r.code != 0 || !strings.Contains(r.stdout, "Usage:\n  gdn wt [flags]\n  gdn wt [command]") {
			t.Errorf("code = %d, stdout = %q", r.code, r.stdout)
		}
	})

	t.Run("未知のフラグはエラーになる", func(t *testing.T) {
		r := e.gdn("repo", "list", "--nope")
		if r.code != 1 {
			t.Errorf("code = %d", r.code)
		}
		e.expectOutput("stderr", r.stderr, "gdn: unknown flag: --nope")
	})
}

func TestComplete(t *testing.T) {
	e := newTestEnv(t)

	for _, shell := range []string{"bash", "zsh", "fish", "powershell"} {
		t.Run(shell, func(t *testing.T) {
			r := e.gdn("complete", shell)
			if r.code != 0 || !strings.Contains(r.stdout, "gdn") {
				t.Errorf("code = %d, stderr = %q", r.code, r.stderr)
			}
		})
	}

	t.Run("未対応のシェルはエラー", func(t *testing.T) {
		if r := e.gdn("complete", "tcsh"); r.code != 1 {
			t.Errorf("code = %d", r.code)
		}
	})
}

func TestVersion(t *testing.T) {
	r := newTestEnv(t).gdn("--version")
	if r.code != 0 || strings.TrimSpace(r.stdout) == "" {
		t.Errorf("code = %d, stdout = %q", r.code, r.stdout)
	}
}
