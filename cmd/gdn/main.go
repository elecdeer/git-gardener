// Command gdn は ghq と git worktree を統合したようなリポジトリ管理CLI。
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"runtime/debug"

	"github.com/elecdeer/git-gardener/internal/cli"
	"github.com/elecdeer/git-gardener/internal/git"
)

// version はリリースビルド時に -ldflags "-X main.version=..." で埋め込まれる。
var version = ""

// resolveVersion は埋め込まれたバージョン、なければ go install 時のモジュールバージョンを返す。
func resolveVersion() string {
	if version != "" {
		return version
	}
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
		return info.Main.Version
	}
	return "dev"
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	root := cli.NewRootCommand(resolveVersion(), git.ExecRunner{})
	if err := root.ExecuteContext(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "gdn: %v\n", err)
		stop()
		os.Exit(1)
	}
}
