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
