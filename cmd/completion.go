package cmd

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

// commandAlias is the short name the Homebrew cask and `just install` link to
// the binary. The completion scripts register it beside the binary's own name.
const commandAlias = "dkt"

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
    docket completion fish > ~/.config/fish/completions/docket.fish

Each script also completes the short alias dkt. Fish looks completions up by
command name, so it needs a second file for dkt:

    echo 'complete -c dkt -w docket' > ~/.config/fish/completions/dkt.fish`,
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
	return writeCompletion(os.Stdout, args[0])
}

// writeCompletion writes the script for shell, registered for the binary and
// its alias alike. Cobra's scripts ask the command they complete for
// candidates, so the alias needs only to be named where the script registers.
func writeCompletion(w io.Writer, shell string) error {
	name := rootCmd.Name()
	var script bytes.Buffer
	var err error
	switch shell {
	case "bash":
		err = rootCmd.GenBashCompletion(&script)
	case "zsh":
		err = rootCmd.GenZshCompletion(&script)
	case "fish":
		err = rootCmd.GenFishCompletion(&script, true)
	default:
		return nil
	}
	if err != nil {
		return err
	}
	out := script.String()
	switch shell {
	case "bash":
		out = strings.ReplaceAll(out, "-F __start_"+name+" "+name+"\n", "-F __start_"+name+" "+name+" "+commandAlias+"\n")
	case "zsh":
		out = strings.Replace(out, "#compdef "+name+"\n", "#compdef "+name+" "+commandAlias+"\n", 1)
		out = strings.Replace(out, "compdef _"+name+" "+name+"\n", "compdef _"+name+" "+name+" "+commandAlias+"\n", 1)
	case "fish":
		out += fmt.Sprintf("\ncomplete -c %s -w %s\n", commandAlias, name)
	}
	_, err = io.WriteString(w, out)
	return err
}
