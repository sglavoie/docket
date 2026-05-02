package tui

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

func TestCrossProjectRebuildRowsActionableFilter(t *testing.T) {
	// Active task in empty section (actionable)
	actionableTask := makeTask("act-1", "Actionable task", "")
	actionableTask.Status = models.StatusTodo

	// Task in Inbox section (not actionable)
	inboxTask := makeTask("inbox-1", "Inbox task", models.SectionInbox)
	inboxTask.Status = models.StatusTodo

	// Blocked task (not actionable)
	blockedTask := makeTask("blk-1", "Blocked task", "")
	blockedTask.Status = models.StatusBlocked

	// Task with incomplete dependency (not actionable)
	blocker := makeTask("blocker-1", "Blocker", "")
	blocker.Status = models.StatusTodo
	depTask := makeTask("dep-1", "Depends on blocker", "")
	depTask.Status = models.StatusTodo
	depTask.DependsOn = []string{"blocker-1"}

	proj := &models.Project{
		ID:   "proj-actionable",
		Name: "ActionableTest",
		Tasks: []*models.Task{
			actionableTask, inboxTask, blockedTask, blocker, depTask,
		},
	}
	proj.BuildTaskTree()

	m := &OutlinerModel{
		collapsedSections: make(map[string]bool),
		expandedNodes:     make(map[string]bool),
		MultiSelectState:  MultiSelectState{selectedTasks: make(map[string]bool)},
		activePerspective: PerspectiveAll,
		loadedProjects:    []*models.Project{proj},
		filter:            NewFilterState(),
	}

	// Without actionable filter, all tasks should appear
	m.crossForceRebuild = true
	m.crossProjectRebuildRows()
	allTaskTitles := collectCrossProjectTaskTitles(m)
	if !allTaskTitles["Actionable task"] {
		t.Error("expected Actionable task without filter")
	}
	if !allTaskTitles["Inbox task"] {
		t.Error("expected Inbox task without filter")
	}
	if !allTaskTitles["Blocked task"] {
		t.Error("expected Blocked task without filter")
	}

	// Enable actionable filter
	m.filter.Actionable = true
	m.crossForceRebuild = true
	m.crossProjectRebuildRows()
	filteredTitles := collectCrossProjectTaskTitles(m)

	if !filteredTitles["Actionable task"] {
		t.Error("actionable filter should keep actionable task")
	}
	if !filteredTitles["Blocker"] {
		t.Error("actionable filter should keep blocker (it has no blockers itself)")
	}
	if filteredTitles["Inbox task"] {
		t.Error("actionable filter should hide Inbox task")
	}
	if filteredTitles["Blocked task"] {
		t.Error("actionable filter should hide blocked task")
	}
	if filteredTitles["Depends on blocker"] {
		t.Error("actionable filter should hide task with incomplete dependencies")
	}
}

func TestCrossProjectRebuildRowsSectionFilter(t *testing.T) {
	nextTask := makeTask("next-1", "Next task", models.SectionNext)
	inboxTask := makeTask("inbox-1", "Inbox task", models.SectionInbox)
	activeTask := makeTask("active-1", "Active task", "")

	proj := &models.Project{
		ID:    "proj-section-filter",
		Name:  "SectionFilterTest",
		Tasks: []*models.Task{nextTask, inboxTask, activeTask},
	}
	proj.BuildTaskTree()

	m := &OutlinerModel{
		collapsedSections: make(map[string]bool),
		expandedNodes:     make(map[string]bool),
		MultiSelectState:  MultiSelectState{selectedTasks: make(map[string]bool)},
		activePerspective: PerspectiveAll,
		loadedProjects:    []*models.Project{proj},
		filter:            NewFilterState(),
	}

	// Enable section filter for Next only
	m.filter.Sections[models.SectionNext] = true
	m.crossForceRebuild = true
	m.crossProjectRebuildRows()
	titles := collectCrossProjectTaskTitles(m)

	if !titles["Next task"] {
		t.Error("section filter should keep Next task")
	}
	if titles["Inbox task"] {
		t.Error("section filter should hide Inbox task")
	}
	if titles["Active task"] {
		t.Error("section filter should hide Active task (empty section)")
	}
}

func collectCrossProjectTaskTitles(m *OutlinerModel) map[string]bool {
	titles := make(map[string]bool)
	for _, row := range m.rows {
		if !row.IsSectionHeader && !row.IsSeparator && row.task != nil {
			titles[row.task.Title] = true
		}
	}
	return titles
}
