package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

// newCompleteCommand は `complete <shell>` を作る。
func newCompleteCommand() *cobra.Command {
	return &cobra.Command{
		Use:       "complete <bash|zsh|fish|powershell>",
		Short:     "シェル補完スクリプトを出力する",
		ValidArgs: []string{"bash", "zsh", "fish", "powershell"},
		Args:      cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
		RunE: func(cmd *cobra.Command, args []string) error {
			root, out := cmd.Root(), cmd.OutOrStdout()
			switch args[0] {
			case "bash":
				return root.GenBashCompletionV2(out, true)
			case "zsh":
				return root.GenZshCompletion(out)
			case "fish":
				return root.GenFishCompletion(out, true)
			case "powershell":
				return root.GenPowerShellCompletionWithDesc(out)
			}
			return fmt.Errorf("unsupported shell: %s", args[0])
		},
	}
}
