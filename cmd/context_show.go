package cmd

import (
	"fmt"
	"os"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/tterrors"
)

var contextShowCmd = &cobra.Command{
	Use:   "show <name>",
	Short: "Show details of a custom context",
	Long: `Show the projects included in a custom context.

Examples:
  tt context show Work
  tt ctx show "Side Projects"`,
	Args: cobra.ExactArgs(1),
	RunE: runContextShow,
}

func init() {
	contextCmd.AddCommand(contextShowCmd)
}

func runContextShow(cmd *cobra.Command, args []string) error {
	if len(args) < 1 {
		return tterrors.NewRequiredFieldError("context name")
	}

	name := args[0]

	cfg, _, err := LoadConfig()
	if err != nil {
		return err
	}

	projectIDs := cfg.GetCustomContext(name)
	if projectIDs == nil {
		return fmt.Errorf("custom context '%s' not found", name)
	}

	fmt.Printf("Custom Context: %s\n\n", name)

	if len(projectIDs) == 0 {
		fmt.Println("No projects in this custom context.")
		return nil
	}

	t := table.NewWriter()
	t.SetStyle(table.StyleRounded)
	t.AppendHeader(table.Row{"Project", "Context", "Tasks", "Status"})

	for _, id := range projectIDs {
		p := cfg.GetProject(id)
		if p == nil {
			t.AppendRow(table.Row{"(deleted)", "-", "-", "removed"})
			continue
		}
		status := "active"
		if p.Archived {
			status = "archived"
		}
		taskCount := 0
		if err := storage.LoadProject(p); err != nil {
			fmt.Fprintf(os.Stderr, "warning: could not load project %q: %v\n", p.Name, err)
		} else {
			taskCount = p.CountTasks()
		}
		t.AppendRow(table.Row{p.Name, p.Context, taskCount, status})
	}

	fmt.Println(t.Render())
	return nil
}
