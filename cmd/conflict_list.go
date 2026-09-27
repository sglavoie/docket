package cmd

import (
	"fmt"
	"strings"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/storage"
)

var conflictListJSON bool

var conflictListCmd = &cobra.Command{
	Use:   "list",
	Short: "List unresolved merge conflicts",
	Long:  `List every persisted merge conflict with its project, conflict count, and age.`,
	Example: `  tt conflict list
  tt conflict list --json`,
	Args:              noArgs,
	ValidArgsFunction: cobra.NoFileCompletions,
	RunE:              runConflictList,
}

func init() {
	conflictCmd.AddCommand(conflictListCmd)
	conflictListCmd.Flags().BoolVar(&conflictListJSON, "json", false, "print the result as JSON")
}

func runConflictList(_ *cobra.Command, _ []string) error {
	store, _, err := conflictWorkspace()
	if err != nil {
		return err
	}
	records, err := store.List()
	if err != nil {
		return err
	}

	if conflictListJSON {
		// An empty workspace is an empty list, not JSON null, so a script can
		// iterate the result without special-casing the clean state.
		if records == nil {
			records = []storage.ConflictRecord{}
		}
		return printJSON(records)
	}

	if len(records) == 0 {
		fmt.Println("No unresolved conflicts.")
		return nil
	}

	t := table.NewWriter()
	t.SetStyle(table.StyleRounded)
	t.AppendHeader(table.Row{"ID", "Project", "Conflicts", "Action", "Recorded"})
	for _, r := range records {
		t.AppendRow(table.Row{
			shortConflictID(r.ID),
			r.ProjectName,
			len(r.Conflicts),
			r.Action,
			r.CreatedAt.Local().Format("2006-01-02 15:04"),
		})
	}
	fmt.Println(t.Render())
	fmt.Printf("\nResolve one with: tt conflict resolve <id> --use local|remote|file\n")
	return nil
}

// shortConflictID trims a conflict UUID to a readable prefix for table display;
// the prefix is still an unambiguous selector for show/resolve.
func shortConflictID(id string) string {
	if len(id) > 8 {
		return id[:8]
	}
	return id
}

// resolveConflictID resolves a possibly-abbreviated id against the store,
// returning the full id or an error listing candidates on ambiguity. It lets the
// user pass the short id printed by `tt conflict list`.
func resolveConflictID(store *storage.ConflictStore, ref string) (string, error) {
	records, err := store.List()
	if err != nil {
		return "", err
	}
	if len(ref) < 4 {
		for _, r := range records {
			if r.ID == ref {
				return r.ID, nil
			}
		}
		return "", fmt.Errorf("conflict ID prefix must be at least 4 characters, got %q; run 'tt conflict list' to see IDs", ref)
	}
	var matches []string
	for _, r := range records {
		if r.ID == ref {
			return r.ID, nil // exact match wins outright
		}
		if len(r.ID) >= len(ref) && r.ID[:len(ref)] == ref {
			matches = append(matches, r.ID)
		}
	}
	switch len(matches) {
	case 1:
		return matches[0], nil
	case 0:
		return "", fmt.Errorf("no conflict matches %q; run 'tt conflict list' to see IDs", ref)
	default:
		return "", fmt.Errorf("conflict ID %q is ambiguous; candidates: %s", ref, strings.Join(matches, ", "))
	}
}
