package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/util"
)

var undoCmd = &cobra.Command{
	Use:   "undo",
	Short: "Undo the last change to tasks",
	Long: `Restore the previous state of the tasks file.

Each mutating operation (add, complete, edit, etc.) creates an undo snapshot.
Running undo restores the most recent snapshot.
Multiple levels of undo are supported (configurable via undo_history_depth).

Examples:
  tt undo              # Restore previous state`,
	Args: cobra.NoArgs,
	RunE: runUndo,
}

func init() {
	rootCmd.AddCommand(undoCmd)
}

func runUndo(cmd *cobra.Command, args []string) error {
	project, _, _, err := LoadCurrentProject()
	if err != nil {
		return err
	}

	// Try multi-level undo system first
	if storage.UndoHistoryExists(project) {
		actionDesc, err := storage.PerformUndo(project)
		if err != nil {
			return fmt.Errorf("undo failed: %w", err)
		}

		// Reload project and re-run side effects (auto-unblock, recurring
		// generation, reorder) so the restored state is consistent — matching
		// the TUI's reapplySideEffects behavior.
		if err := storage.LoadProject(project); err != nil {
			return fmt.Errorf("failed to reload project after undo: %w", err)
		}
		if err := reapplySideEffectsAndSave(project); err != nil {
			return fmt.Errorf("failed to reapply side effects after undo: %w", err)
		}

		fmt.Printf("Undone: %s\n", actionDesc)

		// Check remaining undo history
		entries, _ := storage.LoadUndoHistory(project)
		if len(entries) > 0 {
			fmt.Printf("%d more undo step(s) available.\n", len(entries))
		}
		return nil
	}

	return fmt.Errorf("no undo history available for project '%s'", project.Name)
}

// reapplySideEffectsAndSave re-runs state-derived side effects after an
// undo/redo restore, then saves the result. This ensures automated
// derivations (auto-unblock, recurring instances, completion reorder) are
// freshly computed based on the restored snapshot.
func reapplySideEffectsAndSave(project *models.Project) error {
	project.CheckAndAutoUnblock()
	project.GenerateRecurringInstances()
	project.ReorderRootTasksByCompletion()

	if err := os.MkdirAll(project.Path, 0o755); err != nil {
		return fmt.Errorf("failed to ensure project directory: %w", err)
	}
	lockPath := filepath.Join(project.Path, storage.LockFileName)
	return util.WithFileLock(lockPath, true, func() error {
		if err := storage.BackupProject(project); err != nil {
			return fmt.Errorf("failed to create backup: %w", err)
		}
		return storage.SaveProject(project)
	})
}
