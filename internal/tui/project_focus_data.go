package tui

import (
	"sort"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/focus"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/keymap"
)

// NewProjectFocusViewModel creates a new project focus view model.
func NewProjectFocusViewModel(project *models.Project, runtime *config.RuntimeState) *ProjectFocusViewModel {
	m := &ProjectFocusViewModel{
		project:     project,
		runtime:     runtime,
		FilterMixin: FilterMixin{filter: newFilterState()},
	}
	km := m.buildKeyMap()
	m.Dispatcher = keymap.NewDispatcher(km)
	m.initFilterMixinWithDispatcher(m.Dispatcher, runtime)
	m.loadTasks()
	return m
}

// loadTasks collects Next (queued) tasks from the project. Membership is the
// StateNext workflow state, not presence in the Focus Queue order index, so an
// unindexed StateNext task appears and a stale indexed non-Next task does not.
func (m *ProjectFocusViewModel) loadTasks() {
	m.resetViewScrollOffset()
	m.tasks = nil

	for _, task := range m.project.Tasks {
		if !taskIsNext(task) {
			continue
		}
		// Skip completed tasks
		if task.IsComplete() {
			continue
		}
		// Skip snoozed tasks
		if task.IsSnoozed() {
			continue
		}
		m.tasks = append(m.tasks, task)
	}

	// Order indexed members first (in Focus Queue order), then unindexed
	// StateNext tasks by priority (ascending, 1=highest) then most recently
	// updated. The order index is ordering-only; it never decides membership.
	rank := focus.FocusQueueRank(m.runtime.Planning().GetFocusQueue())
	sort.SliceStable(m.tasks, func(i, j int) bool {
		ri := focus.RankOrUnindexed(rank, m.tasks[i].ID)
		rj := focus.RankOrUnindexed(rank, m.tasks[j].ID)
		if ri != rj {
			return ri < rj
		}
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
	out := make([]*models.Task, 0, len(m.tasks))
	for _, t := range m.tasks {
		if passesCommonFilters(t, &m.filter, m.project, m.runtime.Automation()) {
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
