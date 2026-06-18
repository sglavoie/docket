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
	project, cfg, configManager, err := LoadCurrentProject()
	if err != nil {
		return err
	}

	// Try multi-level undo system first
	if storage.UndoHistoryExists(project) {
		projectsToReload, grouped := restoreProjectsForTopHistoryEntry(project, cfg.Projects, loadUndoHistoryEntries)
		actionDesc, err := storage.PerformUndoForProjects(project, cfg.Projects)
		if err != nil {
			return restoreCommandError("undo", grouped, err)
		}

		// Reload restored project(s) and re-run side effects (auto-unblock, recurring
		// generation, reorder) so the restored state is consistent — matching
		// the TUI's reapplySideEffects behavior.
		if err := reapplyRestoreSideEffectsAndSave(projectsToReload, "undo"); err != nil {
			return err
		}
		if err := repairConfigTaskRefsToLoadedProjects(configManager, cfg, projectsToReload); err != nil {
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

	return fmt.Errorf("no undo history available for project '%s'", project.Name)
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
