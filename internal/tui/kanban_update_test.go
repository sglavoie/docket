package tui

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/testutil"
	"github.com/sglavoie/tt/internal/tui/keymap"
)

// newTestKanban returns a KanbanModel pre-loaded with the given columns for
// unit testing. No disk I/O is performed.
func newTestKanban(cols []KanbanColumn) *KanbanModel {
	cfg := testutil.NewTestConfig()
	m := &KanbanModel{
		columns:     cols,
		activeCol:   0,
		mode:        KanbanModeSection,
		config:      cfg,
		FilterMixin: FilterMixin{filter: newFilterState()},
	}
	return m
}

// makeCol builds a KanbanColumn with n tasks.
func makeCol(id string, n int) KanbanColumn {
	proj := testutil.NewTestProject("p")
	col := KanbanColumn{ID: id, Label: id}
	for range n {
		t := testutil.NewTestTask("task", models.TaskTypeTask)
		col.Tasks = append(col.Tasks, KanbanCard{Task: t, Project: proj})
	}
	return col
}

// ── cursor up/down ────────────────────────────────────────────────────────────

func TestMoveCursorDown_Basic(t *testing.T) {
	m := newTestKanban([]KanbanColumn{makeCol("inbox", 3)})
	m.moveCursorDown()
	if m.columns[0].Cursor != 1 {
		t.Errorf("want cursor 1, got %d", m.columns[0].Cursor)
	}
}

func TestMoveCursorDown_AtEnd_NoChange(t *testing.T) {
	m := newTestKanban([]KanbanColumn{makeCol("inbox", 2)})
	m.columns[0].Cursor = 1
	m.moveCursorDown()
	if m.columns[0].Cursor != 1 {
		t.Errorf("want cursor stay at 1, got %d", m.columns[0].Cursor)
	}
}

func TestMoveCursorUp_Basic(t *testing.T) {
	m := newTestKanban([]KanbanColumn{makeCol("inbox", 3)})
	m.columns[0].Cursor = 2
	m.moveCursorUp()
	if m.columns[0].Cursor != 1 {
		t.Errorf("want cursor 1, got %d", m.columns[0].Cursor)
	}
}

func TestMoveCursorUp_AtTop_NoChange(t *testing.T) {
	m := newTestKanban([]KanbanColumn{makeCol("inbox", 3)})
	m.moveCursorUp()
	if m.columns[0].Cursor != 0 {
		t.Errorf("want cursor stay at 0, got %d", m.columns[0].Cursor)
	}
}

func TestKanbanArrowNavigationMatchesJKInActiveColumn(t *testing.T) {
	tests := []struct {
		name     string
		arrowKey string
		vimKey   string
		start    int
		want     int
	}{
		{name: "down", arrowKey: "down", vimKey: "j", start: 0, want: 1},
		{name: "up", arrowKey: "up", vimKey: "k", start: 2, want: 1},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			arrow := kanbanAfterNavigationKey(t, tc.arrowKey, tc.start)
			vim := kanbanAfterNavigationKey(t, tc.vimKey, tc.start)

			if arrow.columns[1].Cursor != tc.want {
				t.Fatalf("after %q active column cursor = %d, want %d", tc.arrowKey, arrow.columns[1].Cursor, tc.want)
			}
			if vim.columns[1].Cursor != tc.want {
				t.Fatalf("after %q active column cursor = %d, want %d", tc.vimKey, vim.columns[1].Cursor, tc.want)
			}
			if arrow.columns[1].Cursor != vim.columns[1].Cursor {
				t.Fatalf("%q cursor = %d, %q cursor = %d", tc.arrowKey, arrow.columns[1].Cursor, tc.vimKey, vim.columns[1].Cursor)
			}
			if arrow.columns[0].Cursor != 0 {
				t.Fatalf("%q moved inactive column cursor to %d", tc.arrowKey, arrow.columns[0].Cursor)
			}
		})
	}
}

func kanbanAfterNavigationKey(t *testing.T, key string, start int) *KanbanModel {
	t.Helper()

	m := newTestKanban([]KanbanColumn{makeCol("inbox", 3), makeCol("next", 3)})
	m.activeCol = 1
	m.columns[1].Cursor = start
	m.Dispatcher = keymap.NewDispatcher(m.buildKeyMap())

	updated, cmd := m.Update(syntheticKeyStr(key))
	if cmd != nil {
		t.Fatalf("%q returned unexpected command", key)
	}
	kanban, ok := updated.(*KanbanModel)
	if !ok {
		t.Fatalf("%q returned %T, want *KanbanModel", key, updated)
	}
	return kanban
}

func TestMoveCursorDown_EmptyColumn_NoOp(t *testing.T) {
	m := newTestKanban([]KanbanColumn{{ID: "empty"}})
	m.moveCursorDown()
	if m.columns[0].Cursor != 0 {
		t.Errorf("want cursor 0, got %d", m.columns[0].Cursor)
	}
}

// ── jump to bottom ────────────────────────────────────────────────────────────

func TestJumpToBottom(t *testing.T) {
	m := newTestKanban([]KanbanColumn{makeCol("inbox", 5)})
	m.jumpToBottom()
	if m.columns[0].Cursor != 4 {
		t.Errorf("want cursor 4, got %d", m.columns[0].Cursor)
	}
}

func TestJumpToBottom_EmptyColumn_NoOp(t *testing.T) {
	m := newTestKanban([]KanbanColumn{{ID: "empty"}})
	m.jumpToBottom() // should not panic
}

// ── column focus (h/l) ────────────────────────────────────────────────────────

func TestMoveFocusNextCol(t *testing.T) {
	m := newTestKanban([]KanbanColumn{makeCol("a", 1), makeCol("b", 1)})
	m.moveFocusNextCol()
	if m.activeCol != 1 {
		t.Errorf("want activeCol 1, got %d", m.activeCol)
	}
}

func TestMoveFocusNextCol_AtLast_NoChange(t *testing.T) {
	m := newTestKanban([]KanbanColumn{makeCol("a", 1), makeCol("b", 1)})
	m.activeCol = 1
	m.moveFocusNextCol()
	if m.activeCol != 1 {
		t.Errorf("want stay at 1, got %d", m.activeCol)
	}
}

func TestMoveFocusPrevCol(t *testing.T) {
	m := newTestKanban([]KanbanColumn{makeCol("a", 1), makeCol("b", 1)})
	m.activeCol = 1
	m.moveFocusPrevCol()
	if m.activeCol != 0 {
		t.Errorf("want activeCol 0, got %d", m.activeCol)
	}
}

func TestMoveFocusPrevCol_AtFirst_NoChange(t *testing.T) {
	m := newTestKanban([]KanbanColumn{makeCol("a", 1), makeCol("b", 1)})
	m.moveFocusPrevCol()
	if m.activeCol != 0 {
		t.Errorf("want stay at 0, got %d", m.activeCol)
	}
}

func TestHLSingleColumn_NoOp(t *testing.T) {
	m := newTestKanban([]KanbanColumn{makeCol("only", 3)})
	m.moveFocusNextCol() // l at last col
	m.moveFocusPrevCol() // h at first col
	if m.activeCol != 0 {
		t.Errorf("want activeCol 0, got %d", m.activeCol)
	}
}

// ── page up/down ──────────────────────────────────────────────────────────────

func TestPageDown(t *testing.T) {
	m := newTestKanban([]KanbanColumn{makeCol("inbox", 10)})
	m.width = 80
	m.height = 20
	m.pageDown()
	if m.columns[0].Cursor == 0 {
		t.Error("cursor should have moved down")
	}
}

func TestPageUp(t *testing.T) {
	m := newTestKanban([]KanbanColumn{makeCol("inbox", 10)})
	m.width = 80
	m.height = 20
	m.columns[0].Cursor = 8
	m.pageUp()
	if m.columns[0].Cursor >= 8 {
		t.Errorf("cursor should have moved up from 8, got %d", m.columns[0].Cursor)
	}
}

// ── J/K in-column reorder ─────────────────────────────────────────────────────

func TestReorderCardDown(t *testing.T) {
	col := makeCol("inbox", 3)
	firstID := col.Tasks[0].Task.ID
	secondID := col.Tasks[1].Task.ID
	m := newTestKanban([]KanbanColumn{col})
	m.reorderCardDown() // cursor at 0 → swap 0 and 1
	if m.columns[0].Tasks[0].Task.ID != secondID {
		t.Error("task at index 0 should now be the original second task")
	}
	if m.columns[0].Tasks[1].Task.ID != firstID {
		t.Error("task at index 1 should now be the original first task")
	}
	if m.columns[0].Cursor != 1 {
		t.Errorf("cursor should follow: want 1, got %d", m.columns[0].Cursor)
	}
}

func TestReorderCardUp(t *testing.T) {
	col := makeCol("inbox", 3)
	firstID := col.Tasks[0].Task.ID
	secondID := col.Tasks[1].Task.ID
	m := newTestKanban([]KanbanColumn{col})
	m.columns[0].Cursor = 1
	m.reorderCardUp()
	if m.columns[0].Tasks[0].Task.ID != secondID {
		t.Error("task at index 0 should now be the original second task")
	}
	if m.columns[0].Tasks[1].Task.ID != firstID {
		t.Error("task at index 1 should now be the original first task")
	}
	if m.columns[0].Cursor != 0 {
		t.Errorf("cursor should follow: want 0, got %d", m.columns[0].Cursor)
	}
}

func TestReorderCardDown_AtLast_NoOp(t *testing.T) {
	col := makeCol("inbox", 2)
	m := newTestKanban([]KanbanColumn{col})
	m.columns[0].Cursor = 1
	lastID := m.columns[0].Tasks[1].Task.ID
	m.reorderCardDown()
	if m.columns[0].Tasks[1].Task.ID != lastID {
		t.Error("last task should not move")
	}
}

func TestReorderCardUp_AtFirst_NoOp(t *testing.T) {
	col := makeCol("inbox", 2)
	m := newTestKanban([]KanbanColumn{col})
	firstID := m.columns[0].Tasks[0].Task.ID
	m.reorderCardUp()
	if m.columns[0].Tasks[0].Task.ID != firstID {
		t.Error("first task should not move")
	}
}

// ── clampCursor ───────────────────────────────────────────────────────────────

func TestClampCursor_OutOfBounds(t *testing.T) {
	m := newTestKanban([]KanbanColumn{makeCol("inbox", 2)})
	m.columns[0].Cursor = 99
	m.clampCursor()
	if m.columns[0].Cursor != 1 {
		t.Errorf("want clamped to 1, got %d", m.columns[0].Cursor)
	}
}

func TestClampCursor_EmptyColumn(t *testing.T) {
	m := newTestKanban([]KanbanColumn{{ID: "empty", Cursor: 5}})
	m.clampCursor()
	if m.columns[0].Cursor != 0 {
		t.Errorf("want clamped to 0, got %d", m.columns[0].Cursor)
	}
}

func TestClampCursorAllCols(t *testing.T) {
	m := newTestKanban([]KanbanColumn{
		{ID: "a", Tasks: []KanbanCard{{Task: testutil.NewTestTask("t", models.TaskTypeTask)}}, Cursor: 0},
		{ID: "b", Cursor: 99}, // empty with out-of-bounds cursor
	})
	m.clampCursorAllCols()
	if m.columns[1].Cursor != 0 {
		t.Errorf("want clamped to 0, got %d", m.columns[1].Cursor)
	}
}

// ── axis mode cycling ─────────────────────────────────────────────────────────

func TestCycleAxisMode_AdvancesToNext(t *testing.T) {
	m := newTestKanban(nil)
	m.mode = KanbanModeSection
	m.allCards = nil
	m.columns = nil
	m.cycleAxisMode()
	if m.mode != KanbanModeStatus {
		t.Errorf("want KanbanModeStatus after cycling from Section, got %v", m.mode)
	}
}

func TestCycleAxisMode_WrapsAround(t *testing.T) {
	m := newTestKanban(nil)
	m.mode = KanbanModeScoreBuckets
	m.allCards = nil
	m.columns = nil
	m.cycleAxisMode()
	if m.mode != KanbanModeSection {
		t.Errorf("want wrap to Section, got %v", m.mode)
	}
}

func TestCycleAxisMode_OnlyEnabledModes(t *testing.T) {
	m := newTestKanban(nil)
	m.mode = KanbanModeSection
	m.enabledModes = []KanbanMode{KanbanModeSection, KanbanModePriority}
	m.allCards = nil
	m.columns = nil
	m.cycleAxisMode()
	if m.mode != KanbanModePriority {
		t.Errorf("want skip to Priority (only enabled), got %v", m.mode)
	}
}

func TestCycleAxisMode_SingleEnabled_NoChange(t *testing.T) {
	m := newTestKanban(nil)
	m.mode = KanbanModeSection
	m.enabledModes = []KanbanMode{KanbanModeSection}
	m.cycleAxisMode()
	if m.mode != KanbanModeSection {
		t.Errorf("single enabled mode should not change, got %v", m.mode)
	}
}

// ── toggleAxisMode ────────────────────────────────────────────────────────────

func TestToggleAxisMode_DisableOneOfMany(t *testing.T) {
	m := newTestKanban(nil)
	m.enabledModes = []KanbanMode{KanbanModeSection, KanbanModeStatus, KanbanModePriority}
	m.toggleAxisMode(KanbanModeStatus)
	for _, em := range m.enabledModes {
		if em == KanbanModeStatus {
			t.Error("Status should have been removed from enabled modes")
		}
	}
}

func TestToggleAxisMode_GuardLastMode(t *testing.T) {
	m := newTestKanban(nil)
	m.enabledModes = []KanbanMode{KanbanModeSection}
	m.toggleAxisMode(KanbanModeSection) // should be blocked
	if len(m.enabledModes) != 1 {
		t.Error("last mode should remain enabled")
	}
	if m.statusMsg == "" {
		t.Error("expected error status message when trying to disable last mode")
	}
}

func TestToggleAxisMode_EnablePreviouslyDisabled(t *testing.T) {
	m := newTestKanban(nil)
	m.enabledModes = []KanbanMode{KanbanModeSection}
	m.toggleAxisMode(KanbanModePriority) // enable it
	found := false
	for _, em := range m.enabledModes {
		if em == KanbanModePriority {
			found = true
		}
	}
	if !found {
		t.Error("Priority should have been added to enabled modes")
	}
}

// ── adjustHorizontalScroll ────────────────────────────────────────────────────

func TestAdjustHorizontalScroll_ClipsToActive(t *testing.T) {
	cols := []KanbanColumn{
		makeCol("a", 1), makeCol("b", 1), makeCol("c", 1),
		makeCol("d", 1), makeCol("e", 1),
	}
	m := newTestKanban(cols)
	m.width = 100
	m.activeCol = 4
	m.adjustHorizontalScroll()
	// With width=100 and minColWidth=25: maxVisible = (100+1)/26 = 3
	// active=4, offset must make col 4 visible (offset ≥ 2)
	if m.colScrollOff > m.activeCol {
		t.Errorf("scroll offset %d should not exceed activeCol %d", m.colScrollOff, m.activeCol)
	}
}

func TestAdjustHorizontalScroll_ZeroWidth_NoOp(t *testing.T) {
	m := newTestKanban([]KanbanColumn{makeCol("a", 1), makeCol("b", 1)})
	m.width = 0
	m.adjustHorizontalScroll() // should not panic
}

// ── bulkMoveSelectedTasks ─────────────────────────────────────────────────────

func TestBulkMoveSelectedTasks_StatusMessageReflectsActuallyMovedCount(t *testing.T) {
	proj := testutil.NewTestProject("p")
	t1 := testutil.NewTestTaskWithStatus("done1", models.StatusDone)
	t2 := testutil.NewTestTaskWithStatus("done2", models.StatusDone)
	proj.Tasks = []*models.Task{t1, t2}

	doneCol := KanbanColumn{ID: "done", Label: "Done", Tasks: []KanbanCard{
		{Task: t1, Project: proj},
		{Task: t2, Project: proj},
	}}
	canceledCol := KanbanColumn{ID: "canceled", Label: "Canceled"}

	m := newTestKanban([]KanbanColumn{doneCol, canceledCol})
	m.mode = KanbanModeStatus
	m.multiSelectMode = true
	m.ToggleSelection(t1.ID)
	m.ToggleSelection(t2.ID)

	m.bulkMoveSelectedTasks(+1)

	if want := "Moved 0 tasks → Canceled"; m.statusMsg != want {
		t.Errorf("status msg: want %q, got %q", want, m.statusMsg)
	}
}
