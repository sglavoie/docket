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
	project, cfg, configManager, err := LoadCurrentProject()
	if err != nil {
		return err
	}

	if !storage.RedoHistoryExists(project) {
		return fmt.Errorf("no redo history available for project '%s'", project.Name)
	}

	projectsToReload, grouped := restoreProjectsForTopHistoryEntry(project, cfg.Projects, loadRedoHistoryEntries)
	actionDesc, err := storage.PerformRedoForProjects(project, cfg.Projects)
	if err != nil {
		return restoreCommandError("redo", grouped, err)
	}

	// Reload restored project(s) and re-run side effects (auto-unblock, recurring
	// generation, reorder) so the restored state is consistent — matching
	// the TUI's reapplySideEffects behavior.
	if err := reapplyRestoreSideEffectsAndSave(projectsToReload, "redo"); err != nil {
		return err
	}
	if err := repairConfigTaskRefsToLoadedProjects(configManager, cfg, projectsToReload); err != nil {
		return err
	}

	fmt.Printf("Redo: %s\n", actionDesc)

	entries, _ := storage.LoadRedoHistory(project)
	if len(entries) > 0 {
		fmt.Printf("%d more redo step(s) available.\n", len(entries))
	}

	return nil
}
