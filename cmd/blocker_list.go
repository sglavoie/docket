package cmd

import (
	"encoding/json"
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
	blockerListJSON      bool
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
	blockerListCmd.Flags().BoolVar(&blockerListJSON, "json", false, "output as JSON")
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

	// Apply visibility filters to the original blocker list
	var visibleBlockers []*models.Blocker
	for _, b := range blockers {
		isResolved := b.IsResolved()
		if showResolvedBlockers && !isResolved {
			continue
		}
		if !showAllBlockers && !showResolvedBlockers && isResolved {
			continue
		}
		visibleBlockers = append(visibleBlockers, b)
	}

	if blockerListJSON {
		if visibleBlockers == nil {
			visibleBlockers = []*models.Blocker{}
		}
		data, err := json.MarshalIndent(visibleBlockers, "", "  ")
		if err != nil {
			return fmt.Errorf("marshal blockers to JSON: %w", err)
		}
		fmt.Println(string(data))
		return nil
	}

	if len(blockers) == 0 {
		fmt.Println("No blockers found. Add one with: tt blocker add <title>")
		return nil
	}

	if len(visibleBlockers) == 0 {
		if showResolvedBlockers {
			fmt.Println("No resolved blockers found.")
		} else {
			fmt.Println("No active blockers.")
		}
		return nil
	}

	// Build display-formatted rows for table output
	activeStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Bold(true)
	resolvedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("242"))

	t := table.NewWriter()
	t.SetStyle(table.StyleRounded)

	if showAllBlockers || showResolvedBlockers {
		t.AppendHeader(table.Row{"ID", "Title", "Created", "Resolved", "Description"})
	} else {
		t.AppendHeader(table.Row{"ID", "Title", "Created", "Description"})
	}

	for i, b := range visibleBlockers {
		isResolved := b.IsResolved()

		title := b.Title
		if isResolved {
			title = resolvedStyle.Render(title)
		} else {
			title = activeStyle.Render(title)
		}

		desc := b.Description
		if len(desc) > 50 {
			desc = desc[:47] + "..."
		}

		resolved := ""
		if b.ResolvedAt != nil {
			resolved = storage.FormatRelativeTime(*b.ResolvedAt)
		}

		if showAllBlockers || showResolvedBlockers {
			t.AppendRow(table.Row{b.ID[:8], title, storage.FormatRelativeTime(b.CreatedAt), resolved, desc})
		} else {
			t.AppendRow(table.Row{b.ID[:8], title, storage.FormatRelativeTime(b.CreatedAt), desc})
		}

		// Add a separator between active and resolved in --all mode
		if showAllBlockers && i < len(visibleBlockers)-1 {
			if !isResolved && visibleBlockers[i+1].IsResolved() {
				t.AppendSeparator()
			}
		}
	}

	fmt.Printf("Blockers for %s:\n\n", project.Name)
	fmt.Println(t.Render())

	return nil
}
