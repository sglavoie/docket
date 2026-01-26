package cmd

import (
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
  tt ctx l`,
	RunE: runContextList,
}

func init() {
	contextCmd.AddCommand(contextListCmd)
}

func runContextList(cmd *cobra.Command, args []string) error {
	cfg, _, err := LoadConfig()
	if err != nil {
		return err
	}

	names := cfg.GetCustomContextNames()
	if len(names) == 0 {
		fmt.Println("No custom contexts found. Create one with: tt context new <name>")
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
