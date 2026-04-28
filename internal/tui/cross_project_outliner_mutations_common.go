package tui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
)

// crossResolveProject returns the loaded project matching the given ID by
// scanning m.loadedProjects. Returns nil when not found.
func (m *OutlinerModel) crossResolveProject(projectID string) *models.Project {
	for _, p := range m.loadedProjects {
		if p.ID == projectID {
			return p
		}
	}
	return nil
}

// crossGetMutationTarget returns the cursor row, its resolved owning project,
// and ok=true when a valid single-task mutation target exists:
//   - cross-project mode (m.project == nil)
//   - cursor within bounds
//   - row is not a separator or section header
//   - row has a non-nil task
//   - ProjectID resolves to a loaded project
func (m *OutlinerModel) crossGetMutationTarget() (row outlinerRow, proj *models.Project, ok bool) {
	if m.project != nil || len(m.rows) == 0 || m.cursor < 0 || m.cursor >= len(m.rows) {
		return outlinerRow{}, nil, false
	}
	row = m.rows[m.cursor]
	if row.IsSeparator || row.IsSectionHeader || row.task == nil {
		return outlinerRow{}, nil, false
	}
	proj = m.crossResolveProject(row.ProjectID)
	if proj == nil {
		return outlinerRow{}, nil, false
	}
	return row, proj, true
}

// crossPersistFn returns a persistFn scoped to the given project. It records
// an undo snapshot, calls Backup() before the mutation and Save() after,
// ensuring only the owning project is written to disk.
func (m *OutlinerModel) crossPersistFn(_ *models.Project) persistFn {
	svc := m.projectService
	undoCtx := &m.crossUndoCtx
	cfg := m.config
	return func(project *models.Project, taskID string, mutate func(*models.Task)) TaskOpResult {
		if project.GetTaskByID(taskID) == nil {
			return TaskOpResult{StatusMessage: "Task no longer exists in project"}
		}
		// Record undo snapshot before mutation (reads current tasks.md)
		if cfg != nil && cfg.UndoHistoryDepth != nil && *cfg.UndoHistoryDepth > 0 {
			_ = svc.RecordUndoSnapshot(project, undoCtx.Action, undoCtx.Context, *cfg.UndoHistoryDepth, nil)
		}
		if err := svc.Backup(project); err != nil {
			return TaskOpResult{Err: fmt.Errorf("backup failed: %w", err)}
		}
		project.MutateTask(taskID, mutate)
		if err := svc.Save(project); err != nil {
			return TaskOpResult{Err: fmt.Errorf("save failed: %w", err)}
		}
		_ = svc.Load(project)
		return TaskOpResult{}
	}
}

// crossSaveProject returns a tea.Cmd that performs backup -> save -> reload for
// a single project. Used by J/K/right/left in cross-project mode.
func (m *OutlinerModel) crossSaveProject(proj *models.Project) tea.Cmd {
	svc := m.projectService
	return func() tea.Msg {
		if err := svc.Backup(proj); err != nil {
			return ErrorMsg{Err: fmt.Errorf("backup failed: %w", err)}
		}
		if err := svc.Save(proj); err != nil {
			return ErrorMsg{Err: fmt.Errorf("save failed: %w", err)}
		}
		_ = svc.Load(proj)
		return outlinerRefreshMsg{}
	}
}

// crossFindProjectForTask scans m.loadedProjects to find the project that
// contains the task with the given ID. Returns nil when not found.
func (m *OutlinerModel) crossFindProjectForTask(taskID string) *models.Project {
	for _, p := range m.loadedProjects {
		if p.GetTaskByID(taskID) != nil {
			return p
		}
	}
	return nil
}