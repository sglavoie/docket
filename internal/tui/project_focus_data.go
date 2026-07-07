package tui

import (
	"sort"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/keymap"
)

// NewProjectFocusViewModel creates a new project focus view model.
func NewProjectFocusViewModel(project *models.Project, cfg *config.Config) *ProjectFocusViewModel {
	m := &ProjectFocusViewModel{
		project:     project,
		cfg:         cfg,
		FilterMixin: FilterMixin{filter: newFilterState()},
	}
	km := m.buildKeyMap()
	m.Dispatcher = keymap.NewDispatcher(km)
	m.initFilterMixinWithDispatcher(m.Dispatcher, cfg)
	m.loadTasks()
	return m
}

// loadTasks collects active-flagged tasks from the project.
func (m *ProjectFocusViewModel) loadTasks() {
	m.resetViewScrollOffset()
	m.tasks = nil

	for _, task := range m.project.Tasks {
		if !task.Active {
			continue
		}
		// Skip completed tasks
		if task.Status.IsComplete() {
			continue
		}
		// Skip snoozed tasks
		if task.IsSnoozed() {
			continue
		}
		m.tasks = append(m.tasks, task)
	}

	// Sort by priority (ascending, 1=highest), then by UpdatedAt (most recent first)
	sort.Slice(m.tasks, func(i, j int) bool {
		if m.tasks[i].Priority != m.tasks[j].Priority {
			return m.tasks[i].Priority < m.tasks[j].Priority
		}
		return m.tasks[i].UpdatedAt.After(m.tasks[j].UpdatedAt)
	})

	m.computeSubTabs()
}

// computeSubTabs builds dynamic sub-tab names.
func (m *ProjectFocusViewModel) computeSubTabs() {
	m.subTabNames = nil

	if m.subTab > len(m.subTabNames) {
		m.subTab = 0
	}
}

// filteredTasks returns tasks filtered by the active sub-tab and the shared
// F-prefix filter state.
func (m *ProjectFocusViewModel) filteredTasks() []*models.Task {
	if !hasActiveCommonFilters(&m.filter) {
		return m.tasks
	}
	projectID := ""
	if m.project != nil {
		projectID = m.project.ID
	}
	out := make([]*models.Task, 0, len(m.tasks))
	for _, t := range m.tasks {
		if passesCommonFilters(t, &m.filter, projectID, m.cfg) {
			out = append(out, t)
		}
	}
	return out
}

// getTaskAtCursor returns the task at the current cursor position.
func (m *ProjectFocusViewModel) getTaskAtCursor() *models.Task {
	tasks := m.filteredTasks()
	if m.cursor < 0 || m.cursor >= len(tasks) {
		return nil
	}
	return tasks[m.cursor]
}

// HasActiveSearch returns false (no search in this view).
func (m *ProjectFocusViewModel) HasActiveSearch() bool {
	return false
}
