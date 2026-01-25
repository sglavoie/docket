package cmd

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

var (
	showAllBlockers      bool
	showResolvedBlockers bool
	showActiveBlockers   bool
)

var blockerListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls", "l"},
	Short:   "List blockers",
	Long: `List blockers for the current project.

By default, shows only active (unresolved) blockers.

Examples:
  tt blocker list           # Show active blockers
  tt blocker list --all     # Show all blockers
  tt blocker list --resolved # Show only resolved blockers
  tt b ls`,
	RunE: runBlockerList,
}

func init() {
	blockerCmd.AddCommand(blockerListCmd)

	blockerListCmd.Flags().BoolVarP(&showAllBlockers, "all", "a", false, "show all blockers")
	blockerListCmd.Flags().BoolVar(&showResolvedBlockers, "resolved", false, "show only resolved blockers")
	blockerListCmd.Flags().BoolVar(&showActiveBlockers, "active", false, "show only active blockers (default)")
}

func runBlockerList(cmd *cobra.Command, args []string) error {
	project, _, _, err := LoadCurrentProject()
	if err != nil {
		return err
	}

	blockers, err := storage.LoadBlockers(project)
	if err != nil {
		return fmt.Errorf("failed to load blockers: %w", err)
	}

	if len(blockers) == 0 {
		fmt.Println("No blockers found. Add one with: tt blocker add <title>")
		return nil
	}

	// Filter blockers based on flags
	var filtered []*struct {
		id          string
		title       string
		created     string
		resolved    string
		description string
	}

	activeStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Bold(true)
	resolvedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("242"))

	for _, b := range blockers {
		isResolved := b.IsResolved()

		// Apply filters
		if showResolvedBlockers && !isResolved {
			continue
		}
		if !showAllBlockers && !showResolvedBlockers && isResolved {
			continue
		}

		resolved := ""
		if b.ResolvedAt != nil {
			resolved = storage.FormatRelativeTime(*b.ResolvedAt)
		}

		desc := b.Description
		if len(desc) > 50 {
			desc = desc[:47] + "..."
		}

		filtered = append(filtered, &struct {
			id          string
			title       string
			created     string
			resolved    string
			description string
		}{
			id:          b.ID[:8],
			title:       b.Title,
			created:     storage.FormatRelativeTime(b.CreatedAt),
			resolved:    resolved,
			description: desc,
		})
	}

	if len(filtered) == 0 {
		if showResolvedBlockers {
			fmt.Println("No resolved blockers found.")
		} else {
			fmt.Println("No active blockers.")
		}
		return nil
	}

	t := table.NewWriter()
	t.SetStyle(table.StyleRounded)

	if showAllBlockers || showResolvedBlockers {
		t.AppendHeader(table.Row{"ID", "Title", "Created", "Resolved", "Description"})
	} else {
		t.AppendHeader(table.Row{"ID", "Title", "Created", "Description"})
	}

	for i, b := range filtered {
		// Check if this is resolved (by checking the original blocker)
		isResolved := blockers[findBlockerIndex(blockers, b.id)].IsResolved()

		title := b.title
		if isResolved {
			title = resolvedStyle.Render(title)
		} else {
			title = activeStyle.Render(title)
		}

		if showAllBlockers || showResolvedBlockers {
			t.AppendRow(table.Row{b.id, title, b.created, b.resolved, b.description})
		} else {
			t.AppendRow(table.Row{b.id, title, b.created, b.description})
		}

		// Add a separator between active and resolved in --all mode
		if showAllBlockers && i < len(filtered)-1 {
			nextIsResolved := blockers[findBlockerIndex(blockers, filtered[i+1].id)].IsResolved()
			if !isResolved && nextIsResolved {
				t.AppendSeparator()
			}
		}
	}

	fmt.Printf("Blockers for %s:\n\n", project.Name)
	fmt.Println(t.Render())

	return nil
}

func findBlockerIndex(blockers []*models.Blocker, idPrefix string) int {
	for i, b := range blockers {
		if b.ID[:8] == idPrefix {
			return i
		}
	}
	return 0
}
