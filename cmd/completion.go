package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

var completionCmd = &cobra.Command{
	Use:   "completion {bash|zsh|fish}",
	Short: "Generate shell completion scripts",
	Long: `Generate shell completion scripts for docket.

To load completions:

  Bash:
    source <(docket completion bash)

    # To load completions for each session, add to ~/.bashrc:
    echo 'source <(docket completion bash)' >> ~/.bashrc

  Zsh:
    # If shell completion is not already enabled, add to ~/.zshrc:
    autoload -Uz compinit && compinit

    source <(docket completion zsh)

    # To load completions for each session, add to ~/.zshrc:
    echo 'source <(docket completion zsh)' >> ~/.zshrc

  Fish:
    docket completion fish | source

    # To load completions for each session:
    docket completion fish > ~/.config/fish/completions/docket.fish`,
	Example: `  docket completion zsh
  docket completion fish > ~/.config/fish/completions/docket.fish`,
	Args:      cobra.MatchAll(exactlyOneArg("shell (bash, zsh, or fish)"), argsRefusal(cobra.OnlyValidArgs)),
	ValidArgs: []string{"bash", "zsh", "fish"},
	RunE:      runCompletion,
}

func init() {
	rootCmd.AddCommand(completionCmd)
}

func runCompletion(cmd *cobra.Command, args []string) error {
	switch args[0] {
	case "bash":
		return rootCmd.GenBashCompletion(os.Stdout)
	case "zsh":
		return rootCmd.GenZshCompletion(os.Stdout)
	case "fish":
		return rootCmd.GenFishCompletion(os.Stdout, true)
	}
	return nil
}
