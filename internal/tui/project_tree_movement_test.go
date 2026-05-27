package tui

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

func newSortedProjectTreeModel(t *testing.T) (*ProjectTreeModel, []*TreeNode) {
	t.Helper()
	t1 := &models.Task{ID: "t1", Title: "T1", Type: models.TaskTypeTask, Status: models.StatusTodo}
	t2 := &models.Task{ID: "t2", Title: "T2", Type: models.TaskTypeTask, Status: models.StatusTodo}
	project := &models.Project{Tasks: []*models.Task{t1, t2}}

	m := &ProjectTreeModel{
		project:  project,
		viewMode: ViewModeAll,
		sort:     SortState{Field: SortByPriority, Ascending: true},
	}
	visible := []*TreeNode{
		{Task: t1, Visible: true},
		{Task: t2, Visible: true},
	}
	m.nodes = visible
	return m, visible
}

func TestHandleMovementKeys_K_BlockedBySortActive(t *testing.T) {
	m, visible := newSortedProjectTreeModel(t)
	m.cursor = 1

	handled, cmd := m.handleMovementKeys("K", visible)
	if !handled {
		t.Fatal("K should be handled even when blocked by sort")
	}
	if cmd == nil {
		t.Fatal("expected non-nil tea.Cmd when sort is active")
	}
	msg := cmd()
	blocked, ok := msg.(MoveBlockedMsg)
	if !ok {
		t.Fatalf("expected MoveBlockedMsg, got %T", msg)
	}
	if blocked.Reason != "cannot reorder while a sort is active" {
		t.Errorf("Reason = %q, want sort-active reason", blocked.Reason)
	}
	// Cursor must not have moved.
	if m.cursor != 1 {
		t.Errorf("cursor = %d, want 1 (no movement)", m.cursor)
	}
}

func TestHandleMovementKeys_J_BlockedBySortActive(t *testing.T) {
	m, visible := newSortedProjectTreeModel(t)
	m.cursor = 0

	handled, cmd := m.handleMovementKeys("J", visible)
	if !handled {
		t.Fatal("J should be handled even when blocked by sort")
	}
	if cmd == nil {
		t.Fatal("expected non-nil tea.Cmd when sort is active")
	}
	msg := cmd()
	blocked, ok := msg.(MoveBlockedMsg)
	if !ok {
		t.Fatalf("expected MoveBlockedMsg, got %T", msg)
	}
	if blocked.Reason != "cannot reorder while a sort is active" {
		t.Errorf("Reason = %q, want sort-active reason", blocked.Reason)
	}
	if m.cursor != 0 {
		t.Errorf("cursor = %d, want 0 (no movement)", m.cursor)
	}
}

func TestMoveTaskToTop_BlockedBySortActive(t *testing.T) {
	m, visible := newSortedProjectTreeModel(t)
	m.cursor = 1

	_, cmd := m.moveTaskToTop(visible)
	if cmd == nil {
		t.Fatal("expected non-nil tea.Cmd when sort is active")
	}
	msg := cmd()
	blocked, ok := msg.(MoveBlockedMsg)
	if !ok {
		t.Fatalf("expected MoveBlockedMsg, got %T", msg)
	}
	if blocked.Reason != "cannot reorder while a sort is active" {
		t.Errorf("Reason = %q, want sort-active reason", blocked.Reason)
	}
}

func TestMoveTaskToBottom_BlockedBySortActive(t *testing.T) {
	m, visible := newSortedProjectTreeModel(t)
	m.cursor = 0

	_, cmd := m.moveTaskToBottom(visible)
	if cmd == nil {
		t.Fatal("expected non-nil tea.Cmd when sort is active")
	}
	msg := cmd()
	blocked, ok := msg.(MoveBlockedMsg)
	if !ok {
		t.Fatalf("expected MoveBlockedMsg, got %T", msg)
	}
	if blocked.Reason != "cannot reorder while a sort is active" {
		t.Errorf("Reason = %q, want sort-active reason", blocked.Reason)
	}
}

// TestMoveTaskToTop_BlockedByCompletionLeavesCursor verifies that when a
// completed root task cannot be moved above an uncompleted sibling, the cursor
// stays put instead of jumping to the first sibling's old position.
func TestMoveTaskToTop_BlockedByCompletionLeavesCursor(t *testing.T) {
	b := &models.Task{ID: "b", Title: "B", Type: models.TaskTypeTask, Status: models.StatusTodo}
	c := &models.Task{ID: "c", Title: "C", Type: models.TaskTypeTask, Status: models.StatusTodo}
	d := &models.Task{ID: "d", Title: "D", Type: models.TaskTypeTask, Status: models.StatusTodo}
	a := &models.Task{ID: "a", Title: "A", Type: models.TaskTypeTask, Status: models.StatusDone}
	project := &models.Project{Tasks: []*models.Task{b, c, d, a}}

	m := &ProjectTreeModel{
		project:  project,
		viewMode: ViewModeAll,
		sort:     SortState{Field: SortByDefault, Ascending: true},
	}
	visible := []*TreeNode{
		{Task: b, Visible: true},
		{Task: c, Visible: true},
		{Task: d, Visible: true},
		{Task: a, Visible: true},
	}
	m.nodes = visible
	m.cursor = 3 // on the completed task A

	_, _ = m.moveTaskToTop(visible)

	// Order unchanged: A still last.
	if project.Tasks[3].ID != "a" {
		t.Fatalf("expected A to remain at index 3, got order %s,%s,%s,%s",
			project.Tasks[0].ID, project.Tasks[1].ID, project.Tasks[2].ID, project.Tasks[3].ID)
	}
	// Cursor must NOT have jumped to B's position (index 0); it should stay on A.
	if m.cursor != 3 {
		t.Errorf("cursor = %d, want 3 (cursor must stay on blocked task, not jump to first sibling)", m.cursor)
	}
}

// TestHandleMovementKeys_J_DefaultSortNotBlocked ensures the guard only fires
// for explicit sorts — SortByDefault must allow J to proceed normally.
func TestHandleMovementKeys_J_DefaultSortNotBlocked(t *testing.T) {
	m, visible := newSortedProjectTreeModel(t)
	m.sort = SortState{Field: SortByDefault, Ascending: true}
	m.cursor = 0

	handled, cmd := m.handleMovementKeys("J", visible)
	if !handled {
		t.Fatal("J should be handled")
	}
	// With default sort and two siblings, a real swap will be attempted and
	// produce a non-blocked cmd path. The key assertion is that the message
	// returned is NOT MoveBlockedMsg with sort-active reason.
	if cmd != nil {
		if msg := cmd(); msg != nil {
			if blocked, ok := msg.(MoveBlockedMsg); ok {
				if blocked.Reason == "cannot reorder while a sort is active" {
					t.Error("default sort should not produce sort-active block")
				}
			}
		}
	}
}
