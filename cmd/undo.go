package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/storage"
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

		// Reload project to get the restored state
		if err := storage.LoadProject(project); err != nil {
			return fmt.Errorf("failed to reload project after undo: %w", err)
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
