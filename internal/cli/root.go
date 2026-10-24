// Package cli は gdn のコマンドツリーを定義する。
package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/elecdeer/git-gardener/internal/config"
	"github.com/elecdeer/git-gardener/internal/git"
	"github.com/elecdeer/git-gardener/internal/prompt"
)

// ErrAborted は確認プロンプトで中止された場合のエラー。
var ErrAborted = errors.New("aborted")

// app はコマンド間で共有する依存。
type app struct {
	git *git.Git
}

// config は dir を基準に gdn.* を読む Config を返す。
func (a *app) config(dir string) *config.Config {
	return config.New(a.git, dir)
}

// NewRootCommand は gdn のルートコマンドを返す。
func NewRootCommand(version string, runner git.Runner) *cobra.Command {
	a := &app{git: git.New(runner)}

	root := &cobra.Command{
		Use:     "gdn",
		Short:   "ghq と git worktree を統合したようなリポジトリ管理CLI",
		Version: version,
		Args:    cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				//lint:ignore ST1005 利用者向けの案内文として 2 行目まで含めてそのまま表示する
				return fmt.Errorf("unknown command '%s'\nUse 'gdn --help' to see available commands.", args[0])
			}
			return cmd.Help()
		},
		SilenceErrors: true,
		SilenceUsage:  true,
	}
	root.SetVersionTemplate("{{.Version}}\n")
	// 補完スクリプト生成は `gdn complete <shell>` で提供するため、cobra 既定の completion コマンドは使わない。
	root.CompletionOptions.DisableDefaultCmd = true

	root.AddCommand(
		newParentCommand("repo", "ghqライクなリポジトリのクローン・一覧管理",
			a.newRepoListCommand(),
			a.newRepoRootCommand(),
			a.newRepoCloneCommand(),
			a.newRepoMigrateCommand(),
		),
		newParentCommand("wt", "git worktreeの作成・切り替え・削除・整理",
			a.newWtListCommand(),
			a.newWtRootCommand(),
			a.newWtCreateCommand(),
			a.newWtSwitchCommand(),
			a.newWtDeleteCommand(),
			a.newWtPruneCommand(),
			a.newWtMigrateCommand(),
		),
		newCompleteCommand(),
	)
	return root
}

// newParentCommand はサブコマンドをまとめる親コマンドを作る。
// 未知のサブコマンド名で呼ばれた場合はエラーにし、引数なしならヘルプを表示する。
func newParentCommand(name, short string, children ...*cobra.Command) *cobra.Command {
	cmd := &cobra.Command{
		Use:   name,
		Short: short,
		Args:  cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				//lint:ignore ST1005 利用者向けの案内文として 2 行目まで含めてそのまま表示する
				return fmt.Errorf("unknown subcommand '%s %s'\nUse 'gdn %s --help' to see available subcommands.", name, args[0], name)
			}
			return cmd.Help()
		},
	}
	cmd.AddCommand(children...)
	return cmd
}

// resolveDir は -C の値（空ならカレントディレクトリ）を絶対パスにする。
func resolveDir(dir string) (string, error) {
	if dir == "" {
		return os.Getwd()
	}
	return filepath.Abs(dir)
}

// confirm は yes が false の場合に確認プロンプトを出し、拒否されたら ErrAborted を返す。
func confirm(cmd *cobra.Command, yes bool, question string) error {
	if yes {
		return nil
	}
	ok, err := prompt.Confirm(cmd.InOrStdin(), cmd.ErrOrStderr(), question)
	if err != nil {
		return err
	}
	if !ok {
		return ErrAborted
	}
	return nil
}

// printf は w に書き込む。stdout/stderr への書き込み失敗は回復できないため無視する。
func printf(w io.Writer, format string, args ...any) {
	_, _ = fmt.Fprintf(w, format, args...)
}
