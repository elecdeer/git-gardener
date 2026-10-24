package git

import (
	"context"
	"strings"
)

// ConfigScope は git config を読むスコープ。
type ConfigScope int

const (
	// ScopeAll は system/global/local すべてを対象にする（git config のデフォルト）。
	ScopeAll ConfigScope = iota
	// ScopeGlobal は --global のみを対象にする。
	ScopeGlobal
	// ScopeLocal は --local のみを対象にする。
	ScopeLocal
)

// configArgs は scope に応じた git config の引数を組み立てる。
func configArgs(op string, scope ConfigScope, key string) []string {
	args := []string{"config", op}
	switch scope {
	case ScopeGlobal:
		args = append(args, "--global")
	case ScopeLocal:
		args = append(args, "--local")
	}
	return append(args, key)
}

// ConfigGetAll は key に設定された値をすべて返す。未設定なら空スライス。
// git config はキーが無いと終了コード 1 を返すため、それだけは未設定として扱う。
func (g *Git) ConfigGetAll(ctx context.Context, dir string, scope ConfigScope, key string) ([]string, error) {
	out, err := g.read(ctx, dir, configArgs("--get-all", scope, key)...)
	if err != nil {
		if exitCode(err) == 1 {
			return nil, nil
		}
		return nil, err
	}
	var values []string
	for line := range strings.SplitSeq(out, "\n") {
		if line != "" {
			values = append(values, line)
		}
	}
	return values, nil
}

// ConfigGet は key に設定された最後の値を返す。未設定なら空文字列。
func (g *Git) ConfigGet(ctx context.Context, dir string, scope ConfigScope, key string) (string, error) {
	values, err := g.ConfigGetAll(ctx, dir, scope, key)
	if err != nil || len(values) == 0 {
		return "", err
	}
	return values[len(values)-1], nil
}

// ConfigAdd は --local に key=value を追加する。
func (g *Git) ConfigAdd(ctx context.Context, dir, key, value string) error {
	return g.exec(ctx, dir, "config", "--local", "--add", key, value)
}
