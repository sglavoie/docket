package cmd

import (
	"fmt"

	"github.com/sglavoie/dev-helpers/go/tt/internal/config"
	"github.com/sglavoie/dev-helpers/go/tt/internal/storage"
	"github.com/spf13/cobra"
)

var undoCmd = &cobra.Command{
	Use:   "undo",
	Short: "Undo the last change to tasks",
	Long: `Restore the previous state of the tasks file.

Each mutating operation (add, complete, edit, etc.) creates a backup.
Running undo swaps the current state with the backup, making it reversible.
Running undo again will restore the state before the undo.

Examples:
  tt undo              # Restore previous state`,
	Args: cobra.NoArgs,
	RunE: runUndo,
}

func init() {
	rootCmd.AddCommand(undoCmd)
}

func runUndo(cmd *cobra.Command, args []string) error {
	configManager := config.NewManager(GetConfigPath())
	cfg, err := configManager.LoadOrCreate()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	project := cfg.GetCurrentProject()
	if project == nil {
		return fmt.Errorf("no current project selected. Select one with: tt select")
	}

	if err := storage.LoadProject(project); err != nil {
		return fmt.Errorf("failed to load project: %w", err)
	}

	if !storage.BackupExists(project) {
		return fmt.Errorf("no backup available for project '%s'", project.Name)
	}

	// Count tasks before restore
	tasksBefore := len(project.Tasks)

	// Perform the restore (swap current with backup)
	if err := storage.RestoreFromBackup(project); err != nil {
		return fmt.Errorf("failed to restore from backup: %w", err)
	}

	// Reload project to get the restored state
	if err := storage.LoadProject(project); err != nil {
		return fmt.Errorf("failed to reload project after restore: %w", err)
	}

	tasksAfter := len(project.Tasks)

	fmt.Printf("Restored project '%s'\n", project.Name)
	fmt.Printf("Tasks: %d -> %d\n", tasksBefore, tasksAfter)
	fmt.Println("Run 'tt undo' again to reverse this operation.")

	return nil
}
