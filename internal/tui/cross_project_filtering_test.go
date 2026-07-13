package tui

import (
	"slices"
	"testing"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
)

func TestCrossProjectRebuildRowsActionableFilter(t *testing.T) {
	// Active task in empty section (actionable)
	actionableTask := makeTask("act-1", "Actionable task", "")
	actionableTask.WorkflowState = models.StateReady

	// Task in Inbox section (not actionable)
	inboxTask := makeTask("inbox-1", "Inbox task", models.SectionInbox)

	// Blocked task (not actionable)
	blockedTask := makeTask("blk-1", "Blocked task", "")
	blockedTask.WorkflowState = models.StateWaiting

	// Task with incomplete dependency (not actionable)
	blocker := makeTask("blocker-1", "Blocker", "")
	blocker.WorkflowState = models.StateReady
	depTask := makeTask("dep-1", "Depends on blocker", "")
	depTask.WorkflowState = models.StateReady
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

func TestCrossProjectPinnedTagTabsIgnoreLegacyProjectSections(t *testing.T) {
	m := minimalOutliner()
	m.loadedProjects = []*models.Project{
		{ID: "p1", Sections: []string{models.SectionNext, "Sprint-1"}},
		{ID: "p2", Sections: []string{"Design", "Sprint-1"}},
	}

	if got := m.pinnedTagTabs(); got != nil {
		t.Fatalf("pinnedTagTabs() = %v, want nil", got)
	}
	m.setPinnedTagTabIndex(1)
	if got := m.activePinnedTagFilter(); got != "" {
		t.Fatalf("activePinnedTagFilter() = %q, want empty", got)
	}
	if got := m.pinnedTagTabIndex(); got != 0 {
		t.Fatalf("pinnedTagTabIndex() = %d, want stale index reset to All", got)
	}
}

func TestCrossProjectRebuildRowsIgnoresStalePinnedTagTabWithoutForce(t *testing.T) {
	sprintTask := &models.Task{ID: "sprint", Title: "Sprint tagged task", WorkflowState: models.StateReady}
	legacySprintTask := &models.Task{ID: "legacy", Title: "Legacy sprint task", WorkflowState: models.StateReady}
	designTask := &models.Task{ID: "design", Title: "Design task", WorkflowState: models.StateReady}
	activeTask := makeTask("active", "Active task", "")
	activeTask.WorkflowState = models.StateReady

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
		pinnedTagTab:      make(map[string]int),
		sort:              NewSortState(),
	}

	m.crossProjectRebuildRows()
	assertRowTitles(t, m.rows, []string{
		"[Active]",
		"Sprint tagged task",
		"Legacy sprint task",
		"Design task",
		"Active task",
	})

	m.setPinnedTagTabIndex(1)
	m.crossProjectRebuildRows()
	assertRowTitles(t, m.rows, []string{
		"[Active]",
		"Sprint tagged task",
		"Legacy sprint task",
		"Design task",
		"Active task",
	})
}

func TestCrossProjectRebuildRowsPinnedTagTabFiltersByGlobalPinnedTag(t *testing.T) {
	backendTask := makeTask("backend", "Backend task", "")
	backendTask.WorkflowState = models.StateReady
	backendTask.Tags = []string{"backend"}
	frontendTask := makeTask("frontend", "Frontend task", "")
	frontendTask.WorkflowState = models.StateReady
	frontendTask.Tags = []string{"frontend"}
	proj := &models.Project{
		ID:    "proj-pinned-tabs",
		Name:  "PinnedTabs",
		Tasks: []*models.Task{backendTask, frontendTask},
	}
	proj.BuildTaskTree()

	cfg := config.NewConfig()
	cfg.GlobalPinnedTags = []string{"backend"}
	m := &OutlinerModel{
		PrimaryViewBundle: NewPrimaryViewBundle(cfg, nil),
		collapsedSections: make(map[string]bool),
		expandedNodes:     make(map[string]bool),
		activePerspective: PerspectiveAll,
		loadedProjects:    []*models.Project{proj},
		filter:            NewFilterState(),
		pinnedTagTab:      make(map[string]int),
		sort:              NewSortState(),
	}
	m.setPinnedTagTabIndex(1)

	m.crossForceRebuild = true
	m.crossProjectRebuildRows()

	assertRowTitles(t, m.rows, []string{"[Active]", "Backend task"})
	if got := m.activePinnedTagFilter(); got != "backend" {
		t.Fatalf("activePinnedTagFilter() = %q, want backend", got)
	}
}

func TestCrossProjectPinnedTagTabsHideStaleGlobalPins(t *testing.T) {
	backendTask := makeTask("backend", "Backend task", "")
	backendTask.Tags = []string{"backend"}
	project := &models.Project{
		ID:    "proj-pinned-tabs",
		Name:  "PinnedTabs",
		Tasks: []*models.Task{backendTask},
	}
	project.BuildTaskTree()

	cfg := config.NewConfig()
	cfg.GlobalPinnedTags = []string{"backend", "stale"}
	m := &OutlinerModel{
		PrimaryViewBundle: NewPrimaryViewBundle(cfg, nil),
		loadedProjects:    []*models.Project{project},
		pinnedTagTab:      make(map[string]int),
	}

	got := m.pinnedTagTabs()
	want := []string{"All", "#backend"}
	if !slices.Equal(got, want) {
		t.Fatalf("pinnedTagTabs() = %v, want %v", got, want)
	}
	if !slices.Equal(cfg.GlobalPinnedTagList(), []string{"backend", "stale"}) {
		t.Fatalf("GlobalPinnedTagList() = %v, want stale pin retained", cfg.GlobalPinnedTagList())
	}
}

func collectCrossProjectTaskTitles(m *OutlinerModel) map[string]bool {
	titles := make(map[string]bool)
	for _, row := range m.rows {
		if row.isTaskRow() && row.task != nil {
			titles[row.task.Title] = true
		}
	}
	return titles
}
