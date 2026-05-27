package cmd

import (
	"fmt"

	"charm.land/lipgloss/v2"
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
	addOutputFlags(blockerListCmd)
}

func runBlockerList(cmd *cobra.Command, args []string) error {
	out, err := getOutputOpts(cmd)
	if err != nil {
		return err
	}

	// --active and --resolved are mutually exclusive filters: honoring both
	// silently would either show nothing or pick one arbitrarily, neither of
	// which matches user intent. Reject the combination explicitly.
	if showActiveBlockers && showResolvedBlockers {
		return fmt.Errorf("--active and --resolved are mutually exclusive")
	}

	project, _, _, err := LoadCurrentProject()
	if err != nil {
		return err
	}

	blockers, err := storage.LoadBlockers(project)
	if err != nil {
		return fmt.Errorf("failed to load blockers: %w", err)
	}

	visibleBlockers := filterBlockersForList(blockers, showAllBlockers, showResolvedBlockers, showActiveBlockers)

	if out.JSON {
		if visibleBlockers == nil {
			visibleBlockers = []*models.Blocker{}
		}
		return printJSON(visibleBlockers)
	}

	if out.Quiet {
		return nil
	}

	if len(blockers) == 0 {
		fmt.Println(NoItemsMsg("blockers", "tt blocker add <title>"))
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

		shortID := shortBlockerID(b.ID)
		if showAllBlockers || showResolvedBlockers {
			t.AppendRow(table.Row{shortID, title, storage.FormatRelativeTime(b.CreatedAt), resolved, desc})
		} else {
			t.AppendRow(table.Row{shortID, title, storage.FormatRelativeTime(b.CreatedAt), desc})
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

// filterBlockersForList applies the visibility flags to produce the list shown
// to the user. The caller is responsible for rejecting mutually-exclusive flag
// combinations before calling this helper. Filter semantics:
//   - active: only unresolved blockers (overrides all/resolved)
//   - resolved: only resolved blockers
//   - all: both active and resolved
//   - default (none set): only active blockers
func filterBlockersForList(blockers []*models.Blocker, all, resolved, active bool) []*models.Blocker {
	var visible []*models.Blocker
	for _, b := range blockers {
		isResolved := b.IsResolved()
		switch {
		case active:
			if isResolved {
				continue
			}
		case resolved:
			if !isResolved {
				continue
			}
		case all:
			// no filter
		default:
			if isResolved {
				continue
			}
		}
		visible = append(visible, b)
	}
	return visible
}

// shortBlockerID returns at most the first 8 characters of the blocker ID,
// safely handling shorter IDs (e.g. from manually-edited blockers.md files)
// instead of panicking with a slice-bounds error.
func shortBlockerID(id string) string {
	if len(id) > 8 {
		return id[:8]
	}
	return id
}
