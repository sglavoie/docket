package cmd

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/storage"
)

var (
	conflictResolveUse  string
	conflictResolveFile string
)

var conflictResolveCmd = &cobra.Command{
	Use:   "resolve <id>",
	Short: "Resolve a merge conflict",
	Long: `Resolve a recorded merge conflict.

  --use local    keep your local edits (re-merged against the remote if it moved)
  --use remote   discard local edits and adopt the current on-disk state
  --use file     commit a hand-merged file supplied with --file

Before committing, the current remote revision is re-checked: if the remote
changed since the conflict was recorded, your resolution is re-merged against it,
and a fresh conflict is recorded if that re-merge cannot be resolved
automatically.

A bundle's stored documents were rendered at whatever task schema was live when
the conflict was recorded, and a schema migration never rewrites them. When the
current schema can no longer read them, local resolution refuses by name: adopt
remote to clear the record, or supply your own merged file.

When the conflict came from an aborted multi-artifact transaction, the bundle
cannot safely replay only its project document. In that case local and file
resolution fail closed: adopt remote to clear the record, then retry the original
operation so every project, configuration artifact, and attachment is rebuilt in
one transaction.`,
	Args:              cobra.ExactArgs(1),
	RunE:              runConflictResolve,
	ValidArgsFunction: completeConflictIDs,
}

func init() {
	conflictCmd.AddCommand(conflictResolveCmd)
	conflictResolveCmd.Flags().StringVar(&conflictResolveUse, "use", "", "resolution strategy: local, remote, or file")
	conflictResolveCmd.Flags().StringVar(&conflictResolveFile, "file", "", "path to a hand-merged file (with --use file)")
	_ = conflictResolveCmd.RegisterFlagCompletionFunc("use", cobra.FixedCompletions(
		[]cobra.Completion{"local", "remote", "file"}, cobra.ShellCompDirectiveNoFileComp))
}

func runConflictResolve(_ *cobra.Command, args []string) error {
	resolution, err := parseResolution(conflictResolveUse)
	if err != nil {
		return err
	}
	if resolution == storage.ResolveUseFile && conflictResolveFile == "" {
		return fmt.Errorf("--use file requires --file <path>")
	}

	store, commit, err := conflictWorkspace()
	if err != nil {
		return err
	}
	id, err := resolveConflictID(store, args[0])
	if err != nil {
		return err
	}

	result, err := storage.ResolveConflict(store, commit, id, resolution, conflictResolveFile)
	if err != nil {
		if errors.Is(err, storage.ErrConflictDocumentUnreadable) {
			return fmt.Errorf("%w\nAdopt the current on-disk state with --use remote to clear this record, or hand-merge with --use file", err)
		}
		if errors.Is(err, storage.ErrConflictRetryRequired) {
			return fmt.Errorf("cannot apply only the project document: adopt remote with --use remote, then retry the original operation: %w", err)
		}
		if errors.Is(err, storage.ErrRemoteReconflict) {
			fmt.Printf("The remote changed during resolution and the re-merge conflicts.\n")
			fmt.Printf("A new conflict was recorded: %s\n", shortConflictID(result.NewConflictID))
			fmt.Printf("Review it with: tt conflict show %s\n", shortConflictID(result.NewConflictID))
			return err
		}
		var revConflict *storage.RevisionConflictError
		if errors.As(err, &revConflict) {
			return fmt.Errorf("the project changed again during resolution; re-run: %w", err)
		}
		return err
	}

	switch {
	case result.RetryRequired:
		fmt.Printf("Cleared conflict %s without applying the aborted transaction. Retry the original operation.\n", shortConflictID(id))
	case result.Committed && result.Remerged:
		fmt.Printf("Resolved conflict %s: committed the re-merged result (revision %s).\n", shortConflictID(id), shortRev(result.Revision))
	case result.Committed:
		fmt.Printf("Resolved conflict %s using %s (revision %s).\n", shortConflictID(id), resolution, shortRev(result.Revision))
	default:
		fmt.Printf("Resolved conflict %s: adopted the current remote state.\n", shortConflictID(id))
	}
	return nil
}

// parseResolution maps the --use flag to a storage resolution strategy.
func parseResolution(use string) (storage.ConflictResolution, error) {
	switch use {
	case "local":
		return storage.ResolveUseLocal, nil
	case "remote":
		return storage.ResolveUseRemote, nil
	case "file":
		return storage.ResolveUseFile, nil
	case "":
		return "", fmt.Errorf("--use is required (local, remote, or file)")
	default:
		return "", fmt.Errorf("unknown --use value %q (expected local, remote, or file)", use)
	}
}
