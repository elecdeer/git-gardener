// Package git は gdn が必要とする git 操作をまとめる。
// git の実行は Runner に委譲するため、テストでは実プロセスを起動せずに呼び出し内容を検証できる。
package git

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"strings"
)

// Command は 1 回の git 呼び出しを表す。
type Command struct {
	// Dir は git を実行するディレクトリ。空ならカレントディレクトリ。
	Dir string
	// Args は git に渡す引数（先頭の "git" は含まない）。
	Args []string
	// ReadOnly が true の場合 GIT_OPTIONAL_LOCKS=0 を付け、status などが index を書き換えないようにする。
	ReadOnly bool
	// Config は `-c key=value` として渡す一時的な設定。
	Config []string
	// Stream が true の場合、git の出力を捕捉せず Runner の進捗出力先へ流す（clone の進捗表示など）。
	Stream bool
}

// Runner は Command を実行して stdout を返す。
type Runner interface {
	Run(ctx context.Context, cmd Command) (string, error)
}

// ExecRunner は git バイナリを子プロセスとして起動する Runner。
type ExecRunner struct {
	// Progress は Stream 指定時の出力先。nil なら os.Stderr。
	Progress io.Writer
}

// Run は git を起動し、失敗時は stderr を含む *Error を返す。
func (r ExecRunner) Run(ctx context.Context, cmd Command) (string, error) {
	args := make([]string, 0, len(cmd.Config)*2+len(cmd.Args))
	for _, c := range cmd.Config {
		args = append(args, "-c", c)
	}
	args = append(args, cmd.Args...)

	c := exec.CommandContext(ctx, "git", args...)
	c.Dir = cmd.Dir
	c.Env = os.Environ()
	if cmd.ReadOnly {
		c.Env = append(c.Env, "GIT_OPTIONAL_LOCKS=0")
	}

	var stdout, stderr bytes.Buffer
	if cmd.Stream {
		progress := r.Progress
		if progress == nil {
			progress = os.Stderr
		}
		c.Stdout = progress
		c.Stderr = progress
	} else {
		c.Stdout = &stdout
		c.Stderr = &stderr
	}

	if err := c.Run(); err != nil {
		code := -1
		if exitErr, ok := errors.AsType[*exec.ExitError](err); ok {
			code = exitErr.ExitCode()
		}
		return stdout.String(), &Error{Args: cmd.Args, Stderr: strings.TrimSpace(stderr.String()), ExitCode: code, Err: err}
	}
	return stdout.String(), nil
}

// Error は git コマンドの失敗を表す。
type Error struct {
	Args   []string
	Stderr string
	// ExitCode は git の終了コード。プロセス起動自体に失敗した場合は -1。
	ExitCode int
	Err      error
}

// Error は失敗したコマンドと git の stderr をまとめたメッセージを返す。
func (e *Error) Error() string {
	msg := "git " + strings.Join(e.Args, " ") + " failed"
	if e.Stderr != "" {
		msg += ":\n" + e.Stderr
	} else if e.Err != nil {
		msg += ": " + e.Err.Error()
	}
	return msg
}

// Unwrap は元のエラー（*exec.ExitError など）を返す。
func (e *Error) Unwrap() error { return e.Err }

// exitCode は err が *Error なら git の終了コードを、それ以外なら -1 を返す。
func exitCode(err error) int {
	if gitErr, ok := errors.AsType[*Error](err); ok {
		return gitErr.ExitCode
	}
	return -1
}

// Git は Runner を使って git 操作を行う。
type Git struct {
	runner Runner
}

// New は Runner を使う Git を返す。
func New(runner Runner) *Git {
	return &Git{runner: runner}
}

// run は Command を実行して stdout をそのまま返す。
func (g *Git) run(ctx context.Context, cmd Command) (string, error) {
	return g.runner.Run(ctx, cmd)
}

// read は読み取り専用コマンドを実行し、前後の空白を除いた stdout を返す。
func (g *Git) read(ctx context.Context, dir string, args ...string) (string, error) {
	out, err := g.run(ctx, Command{Dir: dir, Args: args, ReadOnly: true})
	return strings.TrimSpace(out), err
}

// exec は副作用のあるコマンドを実行する。
func (g *Git) exec(ctx context.Context, dir string, args ...string) error {
	_, err := g.run(ctx, Command{Dir: dir, Args: args})
	return err
}
