package tui

import (
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
//   - row is a task row (not a separator, section header, or project sub-header)
//   - row has a non-nil task
//   - ProjectID resolves to a loaded project
func (m *OutlinerModel) crossGetMutationTarget() (row outlinerRow, proj *models.Project, ok bool) {
	if m.project != nil || len(m.rows) == 0 || m.cursor < 0 || m.cursor >= len(m.rows) {
		return outlinerRow{}, nil, false
	}
	row = m.rows[m.cursor]
	if !row.isTaskRow() || row.task == nil {
		return outlinerRow{}, nil, false
	}
	proj = m.crossResolveProject(row.ProjectID)
	if proj == nil {
		return outlinerRow{}, nil, false
	}
	return row, proj, true
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
