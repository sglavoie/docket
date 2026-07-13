package cmd

import (
	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/storage"
)

// conflictCmd is the parent for the conflict-management surfaces. Three-way merge
// conflicts (see `internal/storage/project_merge.go`) are persisted as bundles
// under `<data>/.tt/conflicts/`; these subcommands list, inspect, and resolve
// them without ever committing a partial merge.
var conflictCmd = &cobra.Command{
	Use:   "conflict",
	Short: "List, inspect, and resolve merge conflicts",
	Long: `Manage persisted three-way merge conflicts.

When a concurrent change to the same project cannot be merged automatically, tt
records the base, local, and remote documents plus structured conflict metadata.
Use these commands to review what diverged and to resolve each conflict by
keeping your local edits, adopting the remote state, or supplying a merged file.`,
}

func init() {
	rootCmd.AddCommand(conflictCmd)
}

// conflictWorkspace resolves the current workspace's conflict store and a commit
// service that shares its data directory, so resolutions commit through the same
// transactional boundary as every other mutation.
func conflictWorkspace() (*storage.ConflictStore, *storage.CommitService, error) {
	_, manager, err := LoadConfig()
	if err != nil {
		return nil, nil, err
	}
	dataDir := config.WorkspaceRootFromConfigPath(manager.GetConfigPath())
	store := storage.NewConflictStore(config.ConflictsDir(dataDir))
	commit := storage.NewCommitService(config.TransactionsDir(dataDir), []string{dataDir})
	return store, commit, nil
}
