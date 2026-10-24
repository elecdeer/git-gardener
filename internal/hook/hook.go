// Package hook は git config に設定されたシェルコマンド（フック・remover）を実行する。
package hook

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
)

// command は script を sh -c で実行するコマンドを組み立てる。
// 出力は out に流し、gdn の stdout（パスなどの機械可読な出力）を汚さないようにする。
func command(ctx context.Context, script, dir string, out io.Writer, args ...string) *exec.Cmd {
	// sh -c の $0 には "gdn" を渡し、args を "$@" として参照できるようにする。
	cmd := exec.CommandContext(ctx, "sh", append([]string{"-c", script, "gdn"}, args...)...)
	cmd.Dir = dir
	cmd.Env = os.Environ()
	cmd.Stdout = out
	cmd.Stderr = out
	return cmd
}

// Run は hooks を dir で順に実行する。
// フックの失敗は致命的にせず、警告を out に出して後続のフックを続ける。
func Run(ctx context.Context, hooks []string, dir string, out io.Writer) {
	for _, h := range hooks {
		if err := command(ctx, h, dir, out).Run(); err != nil {
			fmt.Fprintf(out, "gdn: warning: hook failed: %s: %v\n", h, err)
		}
	}
}

// RunWithPath は remover のようにパスを末尾の引数として受け取るコマンドを実行する。
// script には `"$@"` が付加されるため、"trash-put" のような設定値でもパスを安全に渡せる。
func RunWithPath(ctx context.Context, script, path, dir string, out io.Writer) error {
	if err := command(ctx, script+` "$@"`, dir, out, path).Run(); err != nil {
		return fmt.Errorf("%s %s: %w", script, path, err)
	}
	return nil
}
