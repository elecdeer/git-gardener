// Package config は git config の gdn.* 名前空間から設定を読み出す。
package config

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/elecdeer/git-gardener/internal/git"
)

// DefaultWtRemover は gdn.wtRemover 未設定時に使う削除方法。
const DefaultWtRemover = "git worktree remove"

// Config は gdn.* 設定の読み出し口。
type Config struct {
	git *git.Git
	// dir は git config を実行するディレクトリ。リポジトリ配下ならローカル設定も対象になる。
	dir string
	// homeDir はチルダ展開に使うホームディレクトリを返す。テストで差し替える。
	homeDir func() (string, error)
}

// New は dir を基準に設定を読む Config を返す。
func New(g *git.Git, dir string) *Config {
	return &Config{git: g, dir: dir, homeDir: os.UserHomeDir}
}

// expandHome は先頭の "~" をホームディレクトリに展開する。"~user" 形式は扱わない。
func (c *Config) expandHome(path string) (string, error) {
	if path != "~" && !strings.HasPrefix(path, "~/") {
		return path, nil
	}
	home, err := c.homeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, strings.TrimPrefix(path, "~")), nil
}

// Root は gdn.root（グローバル設定のみ）を解決した絶対パスを返す。デフォルトは ~/gdn。
func (c *Config) Root(ctx context.Context) (string, error) {
	roots, err := c.git.ConfigGetAll(ctx, c.dir, git.ScopeGlobal, "gdn.root")
	if err != nil {
		return "", err
	}
	if len(roots) > 1 {
		return "", errors.New("multiple gdn.root config values are not allowed")
	}
	root := "~/gdn"
	if len(roots) == 1 {
		root = roots[0]
	}
	root, err = c.expandHome(root)
	if err != nil {
		return "", err
	}
	return filepath.Abs(root)
}

// DefaultHost は gdn.defaultHost（グローバル設定のみ）を返す。デフォルトは github.com。
func (c *Config) DefaultHost(ctx context.Context) (string, error) {
	host, err := c.git.ConfigGet(ctx, c.dir, git.ScopeGlobal, "gdn.defaultHost")
	if host == "" && err == nil {
		host = "github.com"
	}
	return host, err
}

// PostCloneHooks は gdn.postCloneHook（グローバル設定のみ）をすべて返す。
func (c *Config) PostCloneHooks(ctx context.Context) ([]string, error) {
	return c.git.ConfigGetAll(ctx, c.dir, git.ScopeGlobal, "gdn.postCloneHook")
}

// WtBasedir は gdn.wtBasedir を返す。{gitroot} を repoName に、先頭の ~ をホームに展開する。
// 相対パスはそのまま返すので、呼び出し側でリポジトリのパスを基準に解決する。デフォルトは ../{gitroot}.wt。
func (c *Config) WtBasedir(ctx context.Context, repoName string) (string, error) {
	basedir, err := c.git.ConfigGet(ctx, c.dir, git.ScopeAll, "gdn.wtBasedir")
	if err != nil {
		return "", err
	}
	if basedir == "" {
		basedir = "../{gitroot}.wt"
	}
	return c.expandHome(strings.ReplaceAll(basedir, "{gitroot}", repoName))
}

// WtHooks は gdn.wtHook をすべて返す。
func (c *Config) WtHooks(ctx context.Context) ([]string, error) {
	return c.git.ConfigGetAll(ctx, c.dir, git.ScopeAll, "gdn.wtHook")
}

// WtDeleteHooks は gdn.wtDeleteHook をすべて返す。
func (c *Config) WtDeleteHooks(ctx context.Context) ([]string, error) {
	return c.git.ConfigGetAll(ctx, c.dir, git.ScopeAll, "gdn.wtDeleteHook")
}

// WtRemover は gdn.wtRemover を返す。デフォルトは "git worktree remove"。
func (c *Config) WtRemover(ctx context.Context) (string, error) {
	remover, err := c.git.ConfigGet(ctx, c.dir, git.ScopeAll, "gdn.wtRemover")
	if remover == "" && err == nil {
		remover = DefaultWtRemover
	}
	return remover, err
}
