package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

var redoGlobal bool

var redoCmd = &cobra.Command{
	Use:   "redo",
	Short: "Redo the last undone change to tasks",
	Long: `Reapply the most recently undone change to the tasks file.

Each undo operation pushes the undone transaction onto the workspace redo
ledger. Running redo restores the most recent redo entry that touched the
current project — including every other project, config artifact, and
attachment the same transaction touched.

With --global, the most recent undone transaction across the whole workspace
is reapplied regardless of which project it touched.

Examples:
  tt redo              # Reapply the current project's last undone change
  tt redo --global     # Redo the latest undone workspace transaction`,
	Args: cobra.NoArgs,
	RunE: runRedo,
}

func init() {
	redoCmd.Flags().BoolVar(&redoGlobal, "global", false, "redo the most recent undone transaction across the whole workspace")
	rootCmd.AddCommand(redoCmd)
}

func runRedo(cmd *cobra.Command, args []string) error {
	ctx, err := loadUndoRedoContext(redoGlobal)
	if err != nil {
		return err
	}

	entry, ledgerHas, err := ctx.ledger.SelectNewestRedo(ctx.scope)
	if err != nil {
		return fmt.Errorf("redo failed: %w", err)
	}

	// Migration-window fallback: mutations not yet routed through the
	// application layer (chiefly TUI flows still on the legacy persist
	// pipeline) record only per-project history. Prefer whichever system holds
	// the more recent entry so interleaved histories restore in order; the
	// fallback dies with the legacy pipeline. Global scope is ledger-only —
	// the legacy system has no workspace-wide notion.
	if !redoGlobal && legacyHistoryIsNewer(ctx.project, entry, ledgerHas, storage.LoadRedoHistory) {
		return runLegacyRedo(ctx.project, ctx.workspace.Runtime.AllProjects(), ctx.workspace.Runtime)
	}
	if !ledgerHas {
		if redoGlobal {
			return fmt.Errorf("no redo history available")
		}
		return fmt.Errorf("no redo history available for project '%s'", ctx.project.Name)
	}

	restoredEntry, actionDesc, err := ctx.ledger.RedoWithEntry(ctx.scope)
	if err != nil {
		return fmt.Errorf("redo failed: %w", err)
	}
	if err := repairConfigRefsAfterLedgerRestore(ctx.workspace.Runtime, restoredEntry.ProjectIDs); err != nil {
		return err
	}

	fmt.Printf("Redo: %s\n", actionDesc)
	if remaining, countErr := ctx.ledger.RedoCount(ctx.scope); countErr == nil && remaining > 0 {
		fmt.Printf("%d more redo step(s) available.\n", remaining)
	}
	return nil
}

// runLegacyRedo restores through the per-project redo history. It is reachable
// only from the migration-window fallback in runRedo and is removed with the
// legacy persist pipeline.
func runLegacyRedo(project *models.Project, projects []*models.Project, runtime *config.RuntimeState) error {
	if !storage.RedoHistoryExists(project) {
		return fmt.Errorf("no redo history available for project '%s'", project.Name)
	}

	projectsToReload, grouped := restoreProjectsForTopHistoryEntry(project, projects, loadRedoHistoryEntries)
	actionDesc, err := storage.PerformRedoForProjects(project, projects)
	if err != nil {
		return restoreCommandError("redo", grouped, err)
	}

	// Reload restored project(s) and re-run side effects (auto-unblock, recurring
	// generation, reorder) so the restored state is consistent — matching
	// the TUI's reapplySideEffects behavior.
	if err := reapplyRestoreSideEffectsAndSave(projectsToReload, "redo"); err != nil {
		return err
	}
	if err := repairConfigTaskRefsToLoadedProjects(runtime, projectsToReload); err != nil {
		return err
	}

	fmt.Printf("Redo: %s\n", actionDesc)

	entries, _ := storage.LoadRedoHistory(project)
	if len(entries) > 0 {
		fmt.Printf("%d more redo step(s) available.\n", len(entries))
	}

	return nil
}
