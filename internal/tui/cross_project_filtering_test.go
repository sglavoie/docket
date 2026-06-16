package tui

import (
	"slices"
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

func TestCrossProjectCustomSectionTabsAggregateProjects(t *testing.T) {
	m := minimalOutliner()
	m.loadedProjects = []*models.Project{
		{ID: "p1", Sections: []string{models.SectionNext, "Sprint-1"}},
		{ID: "p2", Sections: []string{"Design", "Sprint-1"}},
	}

	want := []string{"All", "Design", "Sprint-1"}
	if got := m.customSectionTabs(); !slices.Equal(got, want) {
		t.Fatalf("customSectionTabs() = %v, want %v", got, want)
	}

	m.setCustomSectionTabIndex(2)
	if got := m.customSectionTabIndex(); got != 2 {
		t.Fatalf("customSectionTabIndex() = %d, want 2", got)
	}
	if got := m.activeCustomSectionFilter(); got != "Sprint-1" {
		t.Fatalf("activeCustomSectionFilter() = %q, want Sprint-1", got)
	}
}

func TestCrossProjectEmptyCustomSectionTabFiltersToEmptyRows(t *testing.T) {
	otherTask := makeTask("other", "Other task", models.SectionNext)
	proj := &models.Project{
		ID:       "proj-empty-custom-tab",
		Name:     "EmptyCustomTab",
		Sections: []string{"Beta", "Alpha", models.SectionNext},
		Tasks:    []*models.Task{otherTask},
	}
	proj.BuildTaskTree()

	m := &OutlinerModel{
		collapsedSections: make(map[string]bool),
		expandedNodes:     make(map[string]bool),
		activePerspective: PerspectiveAll,
		loadedProjects:    []*models.Project{proj},
		filter:            NewFilterState(),
		customSectionTab:  make(map[string]int),
		sort:              NewSortState(),
	}

	wantTabs := []string{"All", "Alpha", "Beta"}
	if got := m.customSectionTabs(); !slices.Equal(got, wantTabs) {
		t.Fatalf("customSectionTabs() = %v, want %v", got, wantTabs)
	}

	m.setCustomSectionTabIndex(1)
	if got := m.activeCustomSectionFilter(); got != "Alpha" {
		t.Fatalf("activeCustomSectionFilter() = %q, want Alpha", got)
	}

	m.crossProjectRebuildRows()
	if len(m.rows) != 0 {
		t.Fatalf("empty custom tab rendered rows %v, want none", rowTitles(m.rows))
	}
}

func TestCrossProjectRebuildRowsCustomSectionTabFiltersWithoutForce(t *testing.T) {
	sprintTask := &models.Task{ID: "sprint", Title: "Sprint custom task", CustomSection: "Sprint-1", Status: models.StatusTodo}
	legacySprintTask := &models.Task{ID: "legacy", Title: "Legacy sprint task", Section: "Sprint-1", Status: models.StatusTodo}
	designTask := &models.Task{ID: "design", Title: "Design task", CustomSection: "Design", Status: models.StatusTodo}
	activeTask := makeTask("active", "Active task", "")
	activeTask.Status = models.StatusTodo

	proj := &models.Project{
		ID:       "proj-custom-tabs",
		Name:     "CustomTabs",
		Sections: []string{"Sprint-1", "Design"},
		Tasks:    []*models.Task{sprintTask, legacySprintTask, designTask, activeTask},
	}
	proj.BuildTaskTree()

	m := &OutlinerModel{
		collapsedSections: make(map[string]bool),
		expandedNodes:     make(map[string]bool),
		activePerspective: PerspectiveAll,
		loadedProjects:    []*models.Project{proj},
		filter:            NewFilterState(),
		customSectionTab:  make(map[string]int),
		sort:              NewSortState(),
	}

	m.crossProjectRebuildRows()
	assertRowTitles(t, m.rows, []string{
		"[Active]",
		"Sprint custom task",
		"Legacy sprint task",
		"Design task",
		"Active task",
	})

	tabs := m.customSectionTabs()
	idx := slices.Index(tabs, "Sprint-1")
	if idx < 0 {
		t.Fatalf("Sprint-1 tab missing from %v", tabs)
	}
	m.setCustomSectionTabIndex(idx)

	m.crossProjectRebuildRows()

	assertRowTitles(t, m.rows, []string{
		"[Active]",
		"Sprint custom task",
		"Legacy sprint task",
	})
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
