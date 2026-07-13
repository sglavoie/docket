package tui

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

func TestValidateCrossZoomStackContextChange(t *testing.T) {
	projB := &models.Project{
		ID:    "proj-b",
		Name:  "Beta",
		Tasks: []*models.Task{makeTask("tb1", "Beta task", "")},
	}
	projB.BuildTaskTree()

	m := &OutlinerModel{
		collapsedSections: make(map[string]bool),
		expandedNodes:     make(map[string]bool),
		MultiSelectState:  MultiSelectState{selectedTasks: make(map[string]bool)},
		loadedProjects:    []*models.Project{projB},
		crossZoomStack: []CrossProjectZoomEntry{{
			TaskID:      "ta1",
			ProjectID:   "proj-a",
			CursorIndex: 5,
		}},
	}
	m.cursor = 5

	m.validateCrossZoomStack()

	if len(m.crossZoomStack) != 0 {
		t.Errorf("expected empty zoom stack after context change removed zoomed project, got %d entries", len(m.crossZoomStack))
	}
	if m.cursor != 0 {
		t.Errorf("expected cursor=0 after zoom stack reset, got %d", m.cursor)
	}

	m.activePerspective = 1
	m.crossProjectRebuildRows()
	if len(m.rows) == 0 {
		t.Error("expected non-empty rows from remaining loaded project after zoom stack reset")
	}
}

func TestValidateCrossZoomStackProjectIDScoping(t *testing.T) {
	const sharedUUID = "shared-task-uuid"

	projA := &models.Project{
		ID:    "proj-a",
		Name:  "Alpha",
		Tasks: []*models.Task{makeTask(sharedUUID, "Task in Alpha", "")},
	}
	projA.BuildTaskTree()

	projB := &models.Project{
		ID:    "proj-b",
		Name:  "Beta",
		Tasks: []*models.Task{makeTask(sharedUUID, "Task in Beta", "")},
	}
	projB.BuildTaskTree()

	m := &OutlinerModel{
		collapsedSections: make(map[string]bool),
		expandedNodes:     make(map[string]bool),
		MultiSelectState:  MultiSelectState{selectedTasks: make(map[string]bool)},
		loadedProjects:    []*models.Project{projA, projB},
		crossZoomStack: []CrossProjectZoomEntry{{
			TaskID:      sharedUUID,
			ProjectID:   "proj-a",
			CursorIndex: 3,
		}},
	}
	m.cursor = 3

	m.validateCrossZoomStack()

	if len(m.crossZoomStack) != 1 {
		t.Errorf("stack should be preserved when scoped project has the task: got %d entries", len(m.crossZoomStack))
	}
	if m.cursor != 3 {
		t.Errorf("cursor should be unchanged when stack is valid: want 3, got %d", m.cursor)
	}

	m2 := &OutlinerModel{
		collapsedSections: make(map[string]bool),
		expandedNodes:     make(map[string]bool),
		MultiSelectState:  MultiSelectState{selectedTasks: make(map[string]bool)},
		loadedProjects:    []*models.Project{projB},
		crossZoomStack: []CrossProjectZoomEntry{{
			TaskID:      sharedUUID,
			ProjectID:   "proj-a",
			CursorIndex: 3,
		}},
	}
	m2.cursor = 3

	m2.validateCrossZoomStack()

	if len(m2.crossZoomStack) != 0 {
		t.Errorf("stack should be cleared when scoped project is absent, even if UUID exists elsewhere: got %d entries", len(m2.crossZoomStack))
	}
	if m2.cursor != 0 {
		t.Errorf("cursor should reset to 0 after stack cleared: got %d", m2.cursor)
	}
}

func TestSaveCrossOutlinerStatePersistsFoldState(t *testing.T) {
	app := newTestAppModel()
	app.crossOutliner = NewOutlinerModel(nil, app.config, app.configManager)
	app.crossOutliner.expandedNodes["collapsed-task"] = false
	app.crossOutliner.expandedNodes["expanded-task"] = true

	app.saveCrossOutlinerState()

	if !app.config.CrossOutlinerFoldState["collapsed-task"] {
		t.Errorf("CrossOutlinerFoldState should contain collapsed-task=true, got %v", app.config.CrossOutlinerFoldState)
	}
	if _, ok := app.config.CrossOutlinerFoldState["expanded-task"]; ok {
		t.Errorf("CrossOutlinerFoldState should not contain expanded-task (sparse), got %v", app.config.CrossOutlinerFoldState)
	}
	if len(app.config.CrossOutlinerFoldState) != 1 {
		t.Errorf("CrossOutlinerFoldState should have exactly 1 entry, got %d", len(app.config.CrossOutlinerFoldState))
	}
}

func TestSaveCrossOutlinerStateClearsFoldStateWhenAllExpanded(t *testing.T) {
	app := newTestAppModel()
	app.crossOutliner = NewOutlinerModel(nil, app.config, app.configManager)
	app.config.CrossOutlinerFoldState = map[string]bool{"stale": true}
	app.crossOutliner.expandedNodes["task1"] = true

	app.saveCrossOutlinerState()

	if app.config.CrossOutlinerFoldState != nil {
		t.Errorf("CrossOutlinerFoldState should be nil when no nodes are collapsed, got %v", app.config.CrossOutlinerFoldState)
	}
}

func TestNewOutlinerModelRestoresCrossFoldState(t *testing.T) {
	app := newTestAppModel()
	app.config.CrossOutlinerFoldState = map[string]bool{
		"task-a": true,
		"task-b": true,
	}

	m := NewOutlinerModel(nil, app.config, app.configManager)

	if expanded, ok := m.expandedNodes["task-a"]; !ok || expanded {
		t.Errorf("expandedNodes[task-a] should be false (collapsed), got ok=%v expanded=%v", ok, expanded)
	}
	if expanded, ok := m.expandedNodes["task-b"]; !ok || expanded {
		t.Errorf("expandedNodes[task-b] should be false (collapsed), got ok=%v expanded=%v", ok, expanded)
	}
	if m.isExpanded("task-a") {
		t.Errorf("isExpanded(task-a) should be false after restore")
	}
}

func TestHandleCrossProjectsLoadedResetsMissingPersistedPinnedTagTab(t *testing.T) {
	app := newTestAppModel()
	app.config.GlobalPinnedTags = []string{"missing"}
	app.config.CrossOutlinerPinnedTagTab = "missing"
	project := &models.Project{
		ID:   "alpha",
		Name: "Alpha",
		Tasks: []*models.Task{
			{ID: "backend", Title: "Backend", WorkflowState: models.StateReady, Tags: []string{"backend"}},
		},
	}
	project.BuildTaskTree()
	m := NewOutlinerModel(nil, app.config, app.configManager)

	m.handleCrossProjectsLoaded(crossProjectsLoadedMsg{
		projects:   []*models.Project{project},
		generation: 0,
	})

	if got := m.activePinnedTagFilter(); got != "" {
		t.Fatalf("activePinnedTagFilter() = %q, want empty", got)
	}
}

func TestHandleCrossProjectsLoadedClearsInMemoryPinnedTagTab(t *testing.T) {
	app := newTestAppModel()
	project := &models.Project{
		ID:       "alpha",
		Name:     "Alpha",
		Sections: []string{models.SectionNext, "Sprint-1"},
	}
	m := NewOutlinerModel(nil, app.config, app.configManager)
	m.loadedProjects = []*models.Project{project}
	m.setPinnedTagTabIndex(1)

	m.handleCrossProjectsLoaded(crossProjectsLoadedMsg{
		projects:   []*models.Project{project},
		generation: 0,
	})

	if got := m.activePinnedTagFilter(); got != "" {
		t.Fatalf("activePinnedTagFilter() = %q, want empty", got)
	}
}

func TestReloadCrossProjectsClearsInMemoryPinnedTagTab(t *testing.T) {
	app := newTestAppModel()
	project := &models.Project{
		ID:       "alpha",
		Name:     "Alpha",
		Sections: []string{models.SectionNext, "Sprint-1"},
	}
	m := NewOutlinerModel(nil, app.config, app.configManager)
	m.loadedProjects = []*models.Project{project}
	m.setPinnedTagTabIndex(1)

	_ = m.reloadCrossProjects()
	if m.loadedProjects != nil {
		t.Fatalf("reloadCrossProjects should clear loadedProjects before async load, got %v", m.loadedProjects)
	}

	reloadedProject := &models.Project{
		ID:       "alpha",
		Name:     "Alpha",
		Sections: []string{models.SectionNext, "Sprint-1"},
	}
	m.handleCrossProjectsLoaded(crossProjectsLoadedMsg{
		projects:   []*models.Project{reloadedProject},
		generation: m.crossLoadGen,
	})

	if got := m.activePinnedTagFilter(); got != "" {
		t.Fatalf("activePinnedTagFilter() = %q, want empty after reload", got)
	}
}

func TestReloadCrossProjectsResetsMissingInMemoryPinnedTagTab(t *testing.T) {
	app := newTestAppModel()
	app.config.GlobalPinnedTags = []string{"backend"}
	backendTask := &models.Task{ID: "backend", Title: "Backend", WorkflowState: models.StateReady, Tags: []string{"backend"}}
	project := &models.Project{
		ID:    "alpha",
		Name:  "Alpha",
		Tasks: []*models.Task{backendTask},
	}
	project.BuildTaskTree()
	m := NewOutlinerModel(nil, app.config, app.configManager)
	m.loadedProjects = []*models.Project{project}
	m.setPinnedTagTabIndex(1)

	_ = m.reloadCrossProjects()

	reloadedProject := &models.Project{
		ID:   "alpha",
		Name: "Alpha",
		Tasks: []*models.Task{
			{ID: "frontend", Title: "Frontend", WorkflowState: models.StateReady, Tags: []string{"frontend"}},
		},
	}
	reloadedProject.BuildTaskTree()
	m.handleCrossProjectsLoaded(crossProjectsLoadedMsg{
		projects:   []*models.Project{reloadedProject},
		generation: m.crossLoadGen,
	})

	if got := m.activePinnedTagFilter(); got != "" {
		t.Fatalf("activePinnedTagFilter() = %q, want empty after selected tab disappeared", got)
	}
	if got := m.pinnedTagTabIndex(); got != 0 {
		t.Fatalf("pinnedTagTabIndex() = %d, want All tab index 0", got)
	}
}

func TestCrossProjectStatePreservationAfterReload(t *testing.T) {
	const collapsedSection = models.SectionNext

	proj := &models.Project{
		ID:   "proj-reload",
		Name: "Reload",
		Tasks: []*models.Task{
			makeTask("r1", "Next task 1", models.SectionNext),
			makeTask("r2", "Next task 2", models.SectionNext),
			makeTask("r3", "Inbox task", models.SectionInbox),
		},
	}
	proj.BuildTaskTree()

	m := &OutlinerModel{
		collapsedSections: map[string]bool{collapsedSection: true},
		expandedNodes:     make(map[string]bool),
		MultiSelectState:  MultiSelectState{selectedTasks: make(map[string]bool)},
		activePerspective: 3,
		crossLoadGen:      0,
	}
	m.cursor = 1

	newProj := &models.Project{
		ID:   "proj-reload-2",
		Name: "Reload2",
		Tasks: []*models.Task{
			makeTask("n1", "New task", models.SectionNext),
		},
	}
	newProj.BuildTaskTree()

	msg := crossProjectsLoadedMsg{
		projects:   []*models.Project{newProj},
		generation: 0,
	}

	m.handleCrossProjectsLoaded(msg)

	if m.activePerspective != 3 {
		t.Errorf("activePerspective: want 3, got %d", m.activePerspective)
	}
	if !m.collapsedSections[collapsedSection] {
		t.Errorf("collapsedSections[%q] should still be true after reload", collapsedSection)
	}

	rowCount := len(m.rows)
	if rowCount == 0 {
		if m.cursor != 0 {
			t.Errorf("cursor must be 0 when no rows exist, got %d", m.cursor)
		}
	} else if m.cursor < 0 || m.cursor >= rowCount {
		t.Errorf("cursor %d is out of bounds for %d rows", m.cursor, rowCount)
	}
}
