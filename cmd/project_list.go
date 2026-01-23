package cmd

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/storage"
)

var showArchived bool

var projectListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls", "l"},
	Short:   "List all projects",
	Long: `List all projects with their task counts.

Examples:
  tt project list
  tt project list --archived
  tt p ls`,
	RunE: runProjectList,
}

func init() {
	projectCmd.AddCommand(projectListCmd)

	projectListCmd.Flags().BoolVarP(&showArchived, "archived", "a", false, "include archived projects")
}

func runProjectList(cmd *cobra.Command, args []string) error {
	cfg, _, err := LoadConfig()
	if err != nil {
		return err
	}

	var projects = cfg.GetActiveProjects()
	if showArchived {
		projects = cfg.Projects
		sort.Slice(projects, func(i, j int) bool {
			return strings.ToLower(projects[i].Name) < strings.ToLower(projects[j].Name)
		})
	}

	if len(projects) == 0 {
		fmt.Println("No projects found. Create one with: tt project new <name>")
		return nil
	}

	t := table.NewWriter()
	t.SetStyle(table.StyleRounded)

	headers := table.Row{"", "Name", "Context", "Tasks", "In Progress", "Done"}
	if showArchived {
		headers = append(headers, "Archived")
	}
	t.AppendHeader(headers)

	currentStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("82")).Bold(true)
	pinnedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("42")).Bold(true)

	for _, p := range projects {
		if err := storage.LoadProject(p); err != nil {
			continue
		}

		// Current project indicator
		indicator := " "
		name := p.Name
		if p.Pinned {
			name = "📌 " + name
		}
		if p.ID == cfg.CurrentProject {
			indicator = "*"
			name = currentStyle.Render(name)
		} else if p.Pinned {
			name = pinnedStyle.Render(name)
		}

		row := table.Row{
			indicator,
			name,
			p.Context,
			p.CountTasks(),
			p.CountActiveTasks(),
			p.CountCompletedTasks(),
		}

		if showArchived {
			archived := ""
			if p.Archived {
				archived = "yes"
			}
			row = append(row, archived)
		}

		t.AppendRow(row)
	}

	fmt.Println(t.Render())

	return nil
}
