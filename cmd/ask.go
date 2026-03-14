package cmd

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/external"
	"github.com/sglavoie/tt/internal/storage"
)

var (
	askApply     bool
	askNoContext bool
)

var askCmd = &cobra.Command{
	Use:   "ask [query]",
	Short: "Ask the AI assistant about your tasks",
	Long: `Ask the AI assistant a question about your project and tasks.

The assistant has context about your current project's tasks and can suggest
tt commands to make changes.

Examples:
  tt ask "what are my high priority tasks?"
  tt ask "create a bug for the login timeout issue" --apply
  tt ask --no-context "how do I use tt to manage dependencies?"`,
	Args: cobra.MinimumNArgs(1),
	RunE: runAsk,
}

func init() {
	rootCmd.AddCommand(askCmd)

	askCmd.Flags().BoolVar(&askApply, "apply", false, "extract and run tt commands from the response")
	askCmd.Flags().BoolVar(&askNoContext, "no-context", false, "skip project context in the prompt")
}

func runAsk(cmd *cobra.Command, args []string) error {
	if !external.ClaudeAvailable() {
		return fmt.Errorf("claude CLI not found. Install it from https://claude.ai/cli")
	}

	query := strings.Join(args, " ")
	var systemPrompt string

	if !askNoContext {
		project, _, _, err := LoadCurrentProject()
		if err == nil {
			if loadErr := storage.LoadProject(project); loadErr != nil {
				fmt.Fprintf(os.Stderr, "warning: failed to load project tasks: %v\n", loadErr)
			} else {
				systemPrompt = external.BuildTaskContext(project)
			}
		}
	}

	if CLIReference != "" {
		systemPrompt += "\n\n" + external.BuildCLIContext(CLIReference, query)
	}

	ctx := context.Background()
	response, err := external.ClaudeAsk(ctx, query, systemPrompt)
	if err != nil {
		return fmt.Errorf("failed to get response: %w", err)
	}

	fmt.Print(response)

	if askApply {
		commands := external.ExtractTTCommands(response)
		if len(commands) == 0 {
			return nil
		}

		fmt.Println("\n--- Extracted commands ---")
		for i, c := range commands {
			fmt.Printf("  %d. %s\n", i+1, c)
		}

		fmt.Print("\nRun these commands? [y/N] ")
		reader := bufio.NewReader(os.Stdin)
		answer, err := reader.ReadString('\n')
		if err != nil {
			return nil
		}
		answer = strings.TrimSpace(strings.ToLower(answer))

		if answer == "y" || answer == "yes" {
			for _, c := range commands {
				parts := strings.Fields(c)
				if len(parts) < 2 {
					continue
				}
				if parts[0] != "tt" {
					fmt.Fprintf(os.Stderr, "warning: skipping non-tt command: %s\n", c)
					continue
				}
				fmt.Printf("\n> %s\n", c)
				execCmd := exec.Command(parts[0], parts[1:]...)
				execCmd.Stdout = os.Stdout
				execCmd.Stderr = os.Stderr
				if err := execCmd.Run(); err != nil {
					fmt.Fprintf(os.Stderr, "Command failed: %v\n", err)
				}
			}
		}
	}

	return nil
}
