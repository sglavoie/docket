package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/util"
)

var undoGlobal bool

var undoCmd = &cobra.Command{
	Use:   "undo",
	Short: "Undo the last change to tasks",
	Long: `Restore the previous state of the tasks file.

Each mutating operation (add, complete, edit, etc.) records an entry in the
workspace undo ledger. Running undo restores the most recent entry that touched
the current project — including every other project, config artifact, and
attachment the same transaction touched. Multiple levels of undo are supported
(configurable via undo_history_depth).

With --global, the most recent transaction across the whole workspace is undone
regardless of which project it touched.

Examples:
  tt undo              # Restore the current project's previous state
  tt undo --global     # Undo the latest workspace transaction`,
	Args: cobra.NoArgs,
	RunE: runUndo,
}

func init() {
	undoCmd.Flags().BoolVar(&undoGlobal, "global", false, "undo the most recent transaction across the whole workspace")
	rootCmd.AddCommand(undoCmd)
}

func runUndo(cmd *cobra.Command, args []string) error {
	project, workspace, err := LoadRuntimeCurrentProject()
	if err != nil {
		return err
	}

	ledger := workspaceLedgerForRestore(workspace.Runtime)
	scope := storage.ProjectScope(project.ID)
	if undoGlobal {
		scope = storage.GlobalScope()
	}

	entry, ledgerHas, err := ledger.SelectNewestUndo(scope)
	if err != nil {
		return fmt.Errorf("undo failed: %w", err)
	}

	// Migration-window fallback: mutations not yet routed through the
	// application layer (chiefly TUI flows still on the legacy persist
	// pipeline) record only per-project history. Prefer whichever system holds
	// the more recent entry so interleaved histories restore in order; the
	// fallback dies with the legacy pipeline. Global scope is ledger-only —
	// the legacy system has no workspace-wide notion.
	if !undoGlobal && legacyHistoryIsNewer(project, entry, ledgerHas, storage.LoadUndoHistory) {
		return runLegacyUndo(project, workspace.Runtime.AllProjects(), workspace.Runtime)
	}
	if !ledgerHas {
		if undoGlobal {
			return fmt.Errorf("no undo history available")
		}
		return fmt.Errorf("no undo history available for project '%s'", project.Name)
	}

	restoredEntry, actionDesc, err := ledger.UndoWithEntry(scope)
	if err != nil {
		return fmt.Errorf("undo failed: %w", err)
	}
	if err := repairConfigRefsAfterLedgerRestore(workspace.Runtime, restoredEntry.ProjectIDs); err != nil {
		return err
	}

	fmt.Printf("Undone: %s\n", actionDesc)
	if remaining, countErr := ledger.UndoCount(scope); countErr == nil && remaining > 0 {
		fmt.Printf("%d more undo step(s) available.\n", remaining)
	}
	return nil
}

// runLegacyUndo restores through the per-project undo history. It is reachable
// only from the migration-window fallback in runUndo and is removed with the
// legacy persist pipeline.
func runLegacyUndo(project *models.Project, projects []*models.Project, runtime *config.RuntimeState) error {
	projectsToReload, grouped := restoreProjectsForTopHistoryEntry(project, projects, loadUndoHistoryEntries)
	actionDesc, err := storage.PerformUndoForProjects(project, projects)
	if err != nil {
		return restoreCommandError("undo", grouped, err)
	}

	// Reload restored project(s) and re-run side effects (auto-unblock, recurring
	// generation, reorder) so the restored state is consistent — matching
	// the TUI's reapplySideEffects behavior.
	if err := reapplyRestoreSideEffectsAndSave(projectsToReload, "undo"); err != nil {
		return err
	}
	if err := repairConfigTaskRefsToLoadedProjects(runtime, projectsToReload); err != nil {
		return err
	}

	fmt.Printf("Undone: %s\n", actionDesc)

	// Check remaining undo history
	entries, _ := storage.LoadUndoHistory(project)
	if len(entries) > 0 {
		fmt.Printf("%d more undo step(s) available.\n", len(entries))
	}
	return nil
}

// workspaceLedgerForRestore opens the workspace undo ledger the transactional
// commit boundary records to, sized from the configured history depth — the same
// construction newCLIRepository uses, so undo reads exactly what mutations wrote.
func workspaceLedgerForRestore(runtime *config.RuntimeState) *storage.WorkspaceLedger {
	depth := config.DefaultUndoHistoryDepth
	settings := runtime.Settings()
	if settings.UndoHistoryDepth != nil {
		depth = *settings.UndoHistoryDepth
	}
	dataDir := runtime.DataDir()
	return storage.NewWorkspaceLedger(config.UndoLedgerDir(dataDir), depth, []string{dataDir})
}

// legacyHistoryIsNewer reports whether the legacy per-project history holds an
// entry more recent than the selected ledger entry (or the only entry at all),
// in which case the fallback path should restore it first.
func legacyHistoryIsNewer(project *models.Project, ledgerEntry storage.LedgerEntry, ledgerHas bool, load historyLoader) bool {
	entries, err := load(project)
	if err != nil || len(entries) == 0 {
		return false
	}
	if !ledgerHas {
		return true
	}
	return entries[0].Timestamp.After(ledgerEntry.Timestamp)
}

// repairConfigRefsAfterLedgerRestore re-points focus queue, daily plan, and
// recent-visit references to the project where each task exists after a ledger
// restore. The restore may have rewritten the config file itself (artifacts that
// rode the undone transaction), so the config is reloaded from disk first —
// repairing against the stale in-memory copy would clobber restored bytes.
func repairConfigRefsAfterLedgerRestore(runtime *config.RuntimeState, projectIDs []string) error {
	if runtime == nil || len(projectIDs) == 0 {
		return nil
	}
	if err := runtime.Reload(); err != nil {
		return fmt.Errorf("failed to reload config after restore: %w", err)
	}
	restored := make([]*models.Project, 0, len(projectIDs))
	for _, id := range projectIDs {
		p := runtime.GetProject(id)
		if p == nil {
			continue
		}
		if err := storage.LoadProject(p); err != nil {
			continue
		}
		restored = append(restored, p)
	}
	return repairConfigTaskRefsToLoadedProjects(runtime, restored)
}

type historyLoader func(*models.Project) ([]storage.UndoEntry, error)

func loadUndoHistoryEntries(project *models.Project) ([]storage.UndoEntry, error) {
	return storage.LoadUndoHistory(project)
}

func loadRedoHistoryEntries(project *models.Project) ([]storage.UndoEntry, error) {
	return storage.LoadRedoHistory(project)
}

func restoreProjectsForTopHistoryEntry(project *models.Project, candidates []*models.Project, load historyLoader) ([]*models.Project, bool) {
	entries, err := load(project)
	if err != nil || len(entries) == 0 {
		return []*models.Project{project}, false
	}

	latest := entries[0]
	if latest.GroupID == "" && len(latest.GroupProjectIDs) == 0 {
		return []*models.Project{project}, false
	}

	projectsByID := make(map[string]*models.Project, len(candidates)+1)
	if id := restoreProjectID(project); id != "" {
		projectsByID[id] = project
	}
	for _, candidate := range candidates {
		if id := restoreProjectID(candidate); id != "" {
			projectsByID[id] = candidate
		}
	}

	projects := make([]*models.Project, 0, len(latest.GroupProjectIDs))
	seen := make(map[string]bool, len(latest.GroupProjectIDs))
	for _, id := range latest.GroupProjectIDs {
		if seen[id] {
			continue
		}
		seen[id] = true
		if candidate := projectsByID[id]; candidate != nil {
			projects = append(projects, candidate)
		}
	}
	if len(projects) == 0 {
		projects = append(projects, project)
	}
	return projects, true
}

func restoreProjectID(project *models.Project) string {
	if project == nil {
		return ""
	}
	if project.ID != "" {
		return project.ID
	}
	return project.Slug
}

func restoreCommandError(label string, grouped bool, err error) error {
	if !grouped {
		return fmt.Errorf("%s failed: %w", label, err)
	}
	return fmt.Errorf(
		"%s failed: grouped history is incomplete or diverged; grouped restore aborted. "+
			"Ensure every project from the original grouped change is still configured and has matching history, then try again: %w",
		label,
		err,
	)
}

func reapplyRestoreSideEffectsAndSave(projects []*models.Project, label string) error {
	for _, project := range projects {
		if err := storage.LoadProject(project); err != nil {
			return fmt.Errorf("failed to reload project after %s: %w", label, err)
		}
		if err := reapplySideEffectsAndSave(project); err != nil {
			return fmt.Errorf("failed to reapply side effects after %s: %w", label, err)
		}
	}
	return nil
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
