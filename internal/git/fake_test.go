package git

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"sync"
)

// fakeRunner は git を起動せず、呼び出しを記録して handler の結果を返す Runner。
type fakeRunner struct {
	mu      sync.Mutex
	calls   []Command
	handler func(cmd Command) (string, error)
}

// Run は呼び出しを記録して handler に委譲する。
func (f *fakeRunner) Run(_ context.Context, cmd Command) (string, error) {
	f.mu.Lock()
	f.calls = append(f.calls, cmd)
	f.mu.Unlock()
	if f.handler == nil {
		return "", nil
	}
	return f.handler(cmd)
}

// argsList は記録された呼び出しの引数だけを返す。
func (f *fakeRunner) argsList() [][]string {
	f.mu.Lock()
	defer f.mu.Unlock()
	list := make([][]string, len(f.calls))
	for i, c := range f.calls {
		list[i] = c.Args
	}
	return list
}

// find は引数が args と一致する呼び出しを返す。
func (f *fakeRunner) find(args ...string) (Command, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, c := range f.calls {
		if slices.Equal(c.Args, args) {
			return c, true
		}
	}
	return Command{}, false
}

// called は args[0:len(prefix)] が prefix と一致する呼び出しがあったかを返す。
func (f *fakeRunner) called(prefix ...string) bool {
	for _, args := range f.argsList() {
		if len(args) >= len(prefix) && slices.Equal(args[:len(prefix)], prefix) {
			return true
		}
	}
	return false
}

// failWith は git が終了コード code で失敗したことを表すエラーを返す。
func failWith(code int, args []string) error {
	return &Error{Args: args, ExitCode: code, Stderr: fmt.Sprintf("exit %d", code)}
}

// worktreeEntry は porcelain 出力を組み立てるためのテスト用エントリ。
type worktreeEntry struct {
	path   string
	hash   string
	branch string
}

// porcelain は `git worktree list --porcelain` 形式の出力を生成する。
func porcelain(entries ...worktreeEntry) string {
	var b strings.Builder
	for i, e := range entries {
		if i > 0 {
			b.WriteString("\n")
		}
		fmt.Fprintf(&b, "worktree %s\nHEAD %s\nbranch refs/heads/%s\n", e.path, e.hash, e.branch)
	}
	return b.String()
}
