package tui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

// reapplySideEffects re-runs state-derived side effects (auto-unblock,
// recurring instances, reorder) after an undo/redo restore. This ensures
// only the user's explicit change is reverted while automated derivations
// are freshly computed. Event-driven workflow rules are deliberately skipped.
func reapplySideEffects(persister Persister, project *models.Project) error {
	project.CheckAndAutoUnblock()
	project.GenerateRecurringInstances()
	project.ReorderRootTasksByCompletion()
	if err := persister.Backup(project); err != nil {
		return fmt.Errorf("failed to backup after undo/redo: %w", err)
	}
	if err := persister.Save(project); err != nil {
		return fmt.Errorf("failed to save after undo/redo: %w", err)
	}
	_ = persister.Load(project)
	return nil
}

type postRestoreHook func([]*models.Project) error

func performUndoForProjectsCmd(persister Persister, project *models.Project, projects []*models.Project, afterRestore postRestoreHook) tea.Cmd {
	if project == nil {
		return nil
	}
	return func() tea.Msg {
		projectsToReload := restoreProjectsForTopTUIHistoryEntry(project, projects, storage.LoadUndoHistory)
		action, err := storage.PerformUndoForProjects(project, projects)
		if err != nil {
			return ErrorMsg{Err: formatUndoRedoRestoreError("undo", err)}
		}
		if err := reloadAndReapplyRestoredProjects(persister, projectsToReload, "undo"); err != nil {
			return ErrorMsg{Err: err}
		}
		if afterRestore != nil {
			if err := afterRestore(projectsToReload); err != nil {
				return ErrorMsg{Err: err}
			}
		}
		return UndoPerformedMsg{Action: action}
	}
}

func reloadAndReapplyRestoredProjects(persister Persister, projects []*models.Project, label string) error {
	for _, project := range uniqueTUIUndoProjects(projects) {
		if err := persister.Load(project); err != nil {
			return fmt.Errorf("failed to reload after %s: %w", label, err)
		}
		if err := reapplySideEffects(persister, project); err != nil {
			return err
		}
	}
	return nil
}

func performRedoForProjectsCmd(persister Persister, project *models.Project, projects []*models.Project, afterRestore postRestoreHook) tea.Cmd {
	if project == nil {
		return nil
	}
	return func() tea.Msg {
		projectsToReload := restoreProjectsForTopTUIHistoryEntry(project, projects, storage.LoadRedoHistory)
		action, err := storage.PerformRedoForProjects(project, projects)
		if err != nil {
			return ErrorMsg{Err: formatUndoRedoRestoreError("redo", err)}
		}
		if err := reloadAndReapplyRestoredProjects(persister, projectsToReload, "redo"); err != nil {
			return ErrorMsg{Err: err}
		}
		if afterRestore != nil {
			if err := afterRestore(projectsToReload); err != nil {
				return ErrorMsg{Err: err}
			}
		}
		return RedoPerformedMsg{Action: action}
	}
}
