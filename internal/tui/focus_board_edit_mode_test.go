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
		t.Error("editMode should be canceled by FocusBoardRefreshMsg")
	}
}

// TestFocusBoardAllTab_IncludesNonActiveTasks verifies that the "All" tab shows
// all non-complete, non-snoozed tasks — not just active/due-soon ones.
func TestFocusBoardAllTab_IncludesNonActiveTasks(t *testing.T) {
	project, _ := models.NewProject("proj", "proj", "/tmp/test-proj")

	// Active task (in m.rows)
	activeTask := makeBoardTask(t, "id-active", 1, "Active Task")
	activeTask.Active = true
	activeRow := boardRow{Task: activeTask, Project: project, Source: project.Name}

	// Non-active, non-due task (only in m.allRows, not in m.rows)
	nextTask := makeBoardTask(t, "id-next", 2, "Next Task")
	nextTask.Active = false
	nextTask.Section = models.SectionNext
	nextRow := boardRow{Task: nextTask, Project: project, Source: project.Name}

	m := newTestFocusBoard()
	m.rows = []boardRow{activeRow}
	m.allRows = []boardRow{activeRow, nextRow}
	m.nextRows = []boardRow{nextRow}

	// Active/Due tab should only show active tasks
	m.tabs.Active = boardTabActiveDue
	m.applyFilter()
	if len(m.filtered) != 1 {
		t.Errorf("Active/Due tab: expected 1 item, got %d", len(m.filtered))
	}

	// All tab should show both tasks
	m.tabs.Active = boardTabAll
	m.applyFilter()
	if len(m.filtered) != 2 {
		t.Errorf("All tab: expected 2 items, got %d", len(m.filtered))
	}

	// Verify both tasks are present
	ids := map[string]bool{}
	for _, r := range m.filtered {
		ids[r.Task.ID] = true
	}
	if !ids["id-active"] {
		t.Error("All tab: missing active task")
	}
	if !ids["id-next"] {
		t.Error("All tab: missing next task")
	}
}

// TestFocusBoardBuildTagCmd_UsesAllRows verifies that the tag filter picker
// collects tags from allRows (all non-complete, non-snoozed tasks), not just
// the active/due-soon subset in m.rows.
func TestFocusBoardBuildTagCmd_UsesAllRows(t *testing.T) {
	project, _ := models.NewProject("proj", "proj", "/tmp/test-proj")

	// Active task with tag "urgent" — present in both rows and allRows
	activeTask := makeBoardTask(t, "id-active", 1, "Active Task")
	activeTask.Active = true
	activeTask.Tags = []string{"urgent"}
	activeRow := boardRow{Task: activeTask, Project: project, Source: project.Name}

	// Non-active task with tag "backlog" — only in allRows, NOT in rows
	nextTask := makeBoardTask(t, "id-next", 2, "Next Task")
	nextTask.Active = false
	nextTask.Tags = []string{"backlog"}
	nextRow := boardRow{Task: nextTask, Project: project, Source: project.Name}

	m := newTestFocusBoard()
	m.rows = []boardRow{activeRow}
	m.allRows = []boardRow{activeRow, nextRow}

	cmd := m.buildTagCmd()
	if cmd == nil {
		t.Fatal("expected non-nil command from buildTagCmd")
	}
	msg := cmd()
	tagMsg, ok := msg.(ShowTagFilterMsg)
	if !ok {
		t.Fatalf("expected ShowTagFilterMsg, got %T", msg)
	}

	tagNames := map[string]bool{}
	for _, ti := range tagMsg.Tags {
		tagNames[ti.Name] = true
	}
	if !tagNames["urgent"] {
		t.Error("buildTagCmd should include 'urgent' tag from active task")
	}
	if !tagNames["backlog"] {
		t.Error("buildTagCmd should include 'backlog' tag from non-active task in allRows")
	}
}
