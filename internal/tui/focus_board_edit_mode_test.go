package tui

import (
	"testing"

	"charm.land/bubbles/v2/textinput"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
)

// newTestFocusBoard creates a minimal FocusBoardModel for unit tests.
func newTestFocusBoard() *FocusBoardModel {
	dp := []FocusTask(nil)
	return &FocusBoardModel{
		config:         &config.Config{},
		projectService: NewProjectService(),
		dailyPlan:      &dp,
		dailyPlanIDs:   map[string]bool{},
		queueIDs:       map[string]bool{},
		carriedOverIDs: map[string]bool{},
		addInput:       textinput.New(),
		editInput:      textinput.New(),
		FilterMixin:    FilterMixin{filter: newFilterState()},
		sort:           NewSortState(),
		tabs:           TabState{Names: focusBoardTabNames},
	}
}

// TestFocusBoardLoadItems_CancelsEditMode verifies that loadItems cancels
// editMode so that a stale editIdx cannot apply changes to the wrong task.
func TestFocusBoardLoadItems_CancelsEditMode(t *testing.T) {
	project, _ := models.NewProject("proj", "proj", "/tmp/test-proj")
	task := makeBoardTask(t, "id-a", 1, "Alpha")
	row := boardRow{Task: task, Project: project, Source: project.Name}

	m := newTestFocusBoard()
	m.filtered = []boardRow{row}
	m.editMode = true
	m.editIdx = 0
	m.editInput.Focus()

	if !m.editMode {
		t.Fatal("precondition: editMode should be true")
	}

	// loadItems rebuilds the data; editIdx would become stale.
	m.loadItems()

	if m.editMode {
		t.Error("editMode should be false after loadItems")
	}
}

// TestFocusBoardLoadItems_CancelsAddMode verifies that loadItems cancels
// addMode so input state is not left dangling after a data reload.
func TestFocusBoardLoadItems_CancelsAddMode(t *testing.T) {
	m := newTestFocusBoard()
	m.addMode = true
	m.addInput.Focus()

	if !m.addMode {
		t.Fatal("precondition: addMode should be true")
	}

	m.loadItems()

	if m.addMode {
		t.Error("addMode should be false after loadItems")
	}
}

// TestFocusBoardRefreshMsg_CancelsEditMode verifies that FocusBoardRefreshMsg
// exits edit mode before rebuilding data so editIdx cannot point at a wrong row.
func TestFocusBoardRefreshMsg_CancelsEditMode(t *testing.T) {
	project, _ := models.NewProject("proj", "proj", "/tmp/test-proj")
	task := makeBoardTask(t, "id-a", 1, "Alpha")
	row := boardRow{Task: task, Project: project, Source: project.Name}

	m := newTestFocusBoard()
	m.filtered = []boardRow{row}
	m.editMode = true
	m.editIdx = 0
	m.editInput.Focus()

	m.Update(FocusBoardRefreshMsg{})

	if m.editMode {
		t.Error("editMode should be cancelled by FocusBoardRefreshMsg")
	}
}
