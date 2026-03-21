package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/models"
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
  tt sec l
  tt section list --json`,
	RunE: runSectionList,
}

func init() {
	sectionCmd.AddCommand(sectionListCmd)
	addOutputFlags(sectionListCmd)
}

func runSectionList(cmd *cobra.Command, args []string) error {
	out, err := getOutputOpts(cmd)
	if err != nil {
		return err
	}

	project, _, _, err := LoadCurrentProject()
	if err != nil {
		return err
	}

	sections := project.GetSections()
	counts := project.CountTasksBySection()

	// Filter out empty Next section
	var visibleSections []string
	for _, s := range sections {
		if s == models.SectionNext && counts[s] == 0 {
			continue
		}
		visibleSections = append(visibleSections, s)
	}

	if out.JSON {
		type sectionEntry struct {
			Name  string `json:"name"`
			Tasks int    `json:"tasks"`
		}
		entries := make([]sectionEntry, len(visibleSections))
		for i, s := range visibleSections {
			entries[i] = sectionEntry{Name: s, Tasks: counts[s]}
		}
		data, err := json.MarshalIndent(entries, "", "  ")
		if err != nil {
			return fmt.Errorf("marshal sections to JSON: %w", err)
		}
		fmt.Println(string(data))
		return nil
	}

	if out.Quiet {
		return nil
	}

	if len(visibleSections) == 0 {
		fmt.Println(NoItemsMsg("sections", "tt section add <name>"))
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
