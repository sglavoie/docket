package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var conflictShowJSON bool

var conflictShowCmd = &cobra.Command{
	Use:               "show <id>",
	Short:             "Show the details of a merge conflict",
	Long:              `Show the base, local, and remote revisions and every structured conflict for one recorded merge conflict.`,
	Args:              cobra.ExactArgs(1),
	RunE:              runConflictShow,
	ValidArgsFunction: completeConflictIDs,
}

func init() {
	conflictCmd.AddCommand(conflictShowCmd)
	conflictShowCmd.Flags().BoolVar(&conflictShowJSON, "json", false, "output as JSON")
}

func runConflictShow(_ *cobra.Command, args []string) error {
	store, _, err := conflictWorkspace()
	if err != nil {
		return err
	}
	id, err := resolveConflictID(store, args[0])
	if err != nil {
		return err
	}
	bundle, err := store.Load(id)
	if err != nil {
		return err
	}
	rec := bundle.Record

	if conflictShowJSON {
		return printJSON(rec)
	}

	fmt.Printf("Conflict %s\n", rec.ID)
	fmt.Printf("  Project:   %s (%s)\n", rec.ProjectName, rec.ProjectSlug)
	if rec.Action != "" {
		fmt.Printf("  Action:    %s\n", rec.Action)
	}
	if rec.Context != "" {
		fmt.Printf("  Context:   %s\n", rec.Context)
	}
	fmt.Printf("  Recorded:  %s\n", rec.CreatedAt.Local().Format("2006-01-02 15:04:05"))
	fmt.Printf("  Revisions: base %s · local %s · remote %s\n",
		shortRev(rec.BaseRevision), shortRev(rec.LocalRevision), shortRev(rec.RemoteRevision))
	// A bundle's documents were rendered at whatever schema was live when it was
	// recorded, and nothing migrates them. Saying so here is what explains a later
	// refusal from `--use local`, so the operator learns it from the surface that
	// exists to explain the bundle rather than from the resolution that fails.
	if bundle.BaseUnreadable != "" {
		fmt.Printf("  Base:      unreadable — %s\n", bundle.BaseUnreadable)
	}
	if bundle.LocalUnreadable != "" {
		fmt.Printf("  Local:     unreadable — %s\n", bundle.LocalUnreadable)
		fmt.Printf("             --use local is unavailable; adopt remote or supply a merged file\n")
	}
	if rec.OperationRetryRequired() {
		fmt.Printf("  Retry:     required — adopt remote to clear this record, then retry the original operation\n")
	}
	fmt.Printf("\n%d conflict(s):\n", len(rec.Conflicts))
	for i, c := range rec.Conflicts {
		fmt.Printf("\n  [%d] %s", i+1, c.Kind)
		if c.TaskTitle != "" {
			fmt.Printf(" — %q", c.TaskTitle)
		}
		fmt.Println()
		// A project comment is located by its own identity, never by a task title.
		// Printing it is what lets the operator find the entry in the stored
		// documents beside the bundle, which is where the bodies live.
		if c.CommentID != "" {
			fmt.Printf("      comment: %s\n", c.CommentID)
		}
		if c.Field != "" {
			fmt.Printf("      field:   %s\n", c.Field)
		}
		if c.Base != "" || c.Local != "" || c.Current != "" {
			fmt.Printf("      base:    %s\n", c.Base)
			fmt.Printf("      local:   %s\n", c.Local)
			fmt.Printf("      remote:  %s\n", c.Current)
		}
		if c.Detail != "" {
			fmt.Printf("      detail:  %s\n", c.Detail)
		}
	}
	if rec.OperationRetryRequired() {
		fmt.Printf("\nClear with: tt conflict resolve %s --use remote\n", shortConflictID(rec.ID))
		return nil
	}
	fmt.Printf("\nResolve with: tt conflict resolve %s --use local|remote|file\n", shortConflictID(rec.ID))
	return nil
}

// shortRev trims a hex revision for display, mirroring the storage layer's
// shortening but tolerant of empty values.
func shortRev(rev string) string {
	if rev == "" {
		return "<none>"
	}
	if len(rev) > 12 {
		return rev[:12]
	}
	return rev
}
