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
		config:            &config.Config{},
		projectService:    NewProjectService(),
		dailyPlan:         &dp,
		dailyPlanIDs:      map[string]bool{},
		queueIDs:          map[string]bool{},
		carriedOverIDs:    map[string]bool{},
		addInput:          textinput.New(),
		editInput:         textinput.New(),
		FilterMixin:       FilterMixin{filter: newFilterState()},
		sort:              NewSortState(),
		tabs:              TabState{Names: focusBoardTabNames},
		activePerspective: PerspectiveInactive,
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

// TestFocusBoardBuildTagCmd_UsesAllRows verifies that the tag filter picker
// collects tags from allRows (all non-complete, non-snoozed tasks), not just
// the active/due-soon subset in m.rows.
func TestFocusBoardBuildTagCmd_UsesAllRows(t *testing.T) {
	project, _ := models.NewProject("proj", "proj", "/tmp/test-proj")

	// Active task with tag "urgent" — present in both rows and allRows
	activeTask := makeBoardTask(t, "id-active", 1, "Active Task")
	activeTask.Tags = []string{"urgent"}
	activeRow := boardRow{Task: activeTask, Project: project, Source: project.Name}

	// Non-active task with tag "backlog" — only in allRows, NOT in rows
	nextTask := makeBoardTask(t, "id-next", 2, "Next Task")
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

func TestFocusBoardNextTab_ExcludesNonGTDSectionTasks(t *testing.T) {
	project, _ := models.NewProject("proj", "proj", "/tmp/test-proj")

	nextTask := makeBoardTask(t, "id-next", 1, "Next Task")
	nextTask.SetSection("")
	nextRow := boardRow{Task: nextTask, Project: project, Source: project.Name}

	explicitNext := makeBoardTask(t, "id-explicit-next", 2, "Explicit Next")
	explicitNext.SetSection(models.SectionNext)
	explicitNextRow := boardRow{Task: explicitNext, Project: project, Source: project.Name}

	legacyTask := makeBoardTask(t, "id-legacy", 3, "Legacy Section Task")
	legacyTask.SetSection("MyBoard")
	legacyRow := boardRow{Task: legacyTask, Project: project, Source: project.Name}

	m := newTestFocusBoard()
	m.allRows = []boardRow{nextRow, explicitNextRow, legacyRow}
	m.nextRows = []boardRow{nextRow, explicitNextRow}

	m.tabs.Active = boardTabNext
	m.applyFilter()

	// Only the two genuine Next tasks should appear
	if len(m.filtered) != 2 {
		t.Errorf("Next tab: expected 2 items, got %d", len(m.filtered))
	}
	ids := map[string]bool{}
	for _, r := range m.filtered {
		ids[r.Task.ID] = true
	}
	if !ids["id-next"] {
		t.Error("Next tab: missing default-section task")
	}
	if !ids["id-explicit-next"] {
		t.Error("Next tab: missing explicit-Next task")
	}
	if ids["id-legacy"] {
		t.Error("Next tab: non-GTD section task should not appear in Next tab")
	}
}
