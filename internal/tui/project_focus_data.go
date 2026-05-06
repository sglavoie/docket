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
		project: project,
		cfg:     cfg,
	}
	km := m.buildKeyMap()
	m.Dispatcher = keymap.NewDispatcher(km)
	m.loadTasks()
	return m
}

// loadTasks collects active-flagged tasks from the project.
func (m *ProjectFocusViewModel) loadTasks() {
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

// computeSubTabs builds dynamic sub-tab names from unique CustomSection values.
func (m *ProjectFocusViewModel) computeSubTabs() {
	seen := map[string]bool{}
	var names []string
	for _, task := range m.tasks {
		section := task.CustomSection
		if section == "" {
			section = "(none)"
		}
		if !seen[section] {
			seen[section] = true
			names = append(names, section)
		}
	}
	sort.Strings(names)
	m.subTabNames = names

	if m.subTab > len(m.subTabNames) {
		m.subTab = 0
	}
}

// activeSubTabFilter returns the selected sub-tab value, or "" if no sub-tab filtering.
func (m *ProjectFocusViewModel) activeSubTabFilter() string {
	if len(m.subTabNames) <= 1 || m.subTab <= 0 || m.subTab > len(m.subTabNames) {
		return ""
	}
	return m.subTabNames[m.subTab-1]
}

// filteredTasks returns tasks filtered by the active sub-tab.
func (m *ProjectFocusViewModel) filteredTasks() []*models.Task {
	filter := m.activeSubTabFilter()
	if filter == "" {
		return m.tasks
	}
	var result []*models.Task
	for _, task := range m.tasks {
		section := task.CustomSection
		if section == "" {
			section = "(none)"
		}
		if section == filter {
			result = append(result, task)
		}
	}
	return result
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
