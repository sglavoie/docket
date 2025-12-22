package cmd

import (
	"fmt"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/sglavoie/tt/internal/models"
	"github.com/spf13/cobra"
)

var sectionListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls", "l"},
	Short:   "List all sections in the current project",
	Long: `List all sections with their task counts.

The Backlog section is hidden when it contains no tasks.

Examples:
  tt section list
  tt section ls
  tt sec l`,
	RunE: runSectionList,
}

func init() {
	sectionCmd.AddCommand(sectionListCmd)
}

func runSectionList(cmd *cobra.Command, args []string) error {
	project, _, _, err := LoadCurrentProject()
	if err != nil {
		return err
	}

	sections := project.GetSections()
	counts := project.CountTasksBySection()

	// Filter out empty Backlog section
	var visibleSections []string
	for _, s := range sections {
		if s == models.SectionBacklog && counts[s] == 0 {
			continue
		}
		visibleSections = append(visibleSections, s)
	}

	if len(visibleSections) == 0 {
		fmt.Println("No sections found. Create one with: tt section add <name>")
		return nil
	}

	t := table.NewWriter()
	t.SetStyle(table.StyleRounded)
	t.AppendHeader(table.Row{"Section", "Tasks"})

	for _, s := range visibleSections {
		t.AppendRow(table.Row{s, counts[s]})
	}

	fmt.Println(t.Render())

	return nil
}
