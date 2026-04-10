package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/storage"
)

var redoCmd = &cobra.Command{
	Use:   "redo",
	Short: "Redo the last undone change to tasks",
	Long: `Reapply the most recently undone change to the tasks file.

Each undo operation pushes the undone state onto a redo stack.
Running redo restores the most recent redo entry.

Examples:
  tt redo              # Reapply last undone change`,
	Args: cobra.NoArgs,
	RunE: runRedo,
}

func init() {
	rootCmd.AddCommand(redoCmd)
}

func runRedo(cmd *cobra.Command, args []string) error {
	project, _, _, err := LoadCurrentProject()
	if err != nil {
		return err
	}

	if !storage.RedoHistoryExists(project) {
		return fmt.Errorf("no redo history available for project '%s'", project.Name)
	}

	actionDesc, err := storage.PerformRedo(project)
	if err != nil {
		return fmt.Errorf("redo failed: %w", err)
	}

	if err := storage.LoadProject(project); err != nil {
		return fmt.Errorf("failed to reload project after redo: %w", err)
	}

	fmt.Printf("Redo: %s\n", actionDesc)

	entries, _ := storage.LoadRedoHistory(project)
	if len(entries) > 0 {
		fmt.Printf("%d more redo step(s) available.\n", len(entries))
	}

	return nil
}
