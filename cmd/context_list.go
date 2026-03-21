package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/spf13/cobra"
)

var contextListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls", "l"},
	Short:   "List all custom contexts",
	Long: `List all custom contexts with their project counts.

Examples:
  tt context list
  tt context ls
  tt ctx l
  tt context list --json`,
	RunE: runContextList,
}

func init() {
	contextCmd.AddCommand(contextListCmd)
	addOutputFlags(contextListCmd)
}

func runContextList(cmd *cobra.Command, args []string) error {
	out, err := getOutputOpts(cmd)
	if err != nil {
		return err
	}

	cfg, _, err := LoadConfig()
	if err != nil {
		return err
	}

	names := cfg.GetCustomContextNames()

	if out.JSON {
		type contextEntry struct {
			Name         string `json:"name"`
			ProjectCount int    `json:"project_count"`
		}
		entries := make([]contextEntry, len(names))
		for i, name := range names {
			entries[i] = contextEntry{Name: name, ProjectCount: cfg.GetCustomContextProjectCount(name)}
		}
		data, err := json.MarshalIndent(entries, "", "  ")
		if err != nil {
			return fmt.Errorf("marshal contexts to JSON: %w", err)
		}
		fmt.Println(string(data))
		return nil
	}

	if out.Quiet {
		return nil
	}

	if len(names) == 0 {
		fmt.Println(NoItemsMsg("custom contexts", "tt context new <name>"))
		return nil
	}

	t := table.NewWriter()
	t.SetStyle(table.StyleRounded)
	t.AppendHeader(table.Row{"Custom Context", "Projects"})

	for _, name := range names {
		count := cfg.GetCustomContextProjectCount(name)
		t.AppendRow(table.Row{name, count})
	}

	fmt.Println(t.Render())

	return nil
}
