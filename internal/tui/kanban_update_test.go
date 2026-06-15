package tui

import (
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/testutil"
	"github.com/sglavoie/tt/internal/tui/keymap"
	"github.com/sglavoie/tt/internal/util"
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

func TestKanbanPersistentScrollPreservesWindowUntilCursorLeaves(t *testing.T) {
	m := newTestKanban([]KanbanColumn{makeCol("inbox", 12)})
	m.width = 80
	m.height = 10

	m.columns[0].Cursor = len(m.columns[0].Tasks) - 1
	m.clampScrollToVisible(0)
	bottomOffset := m.columns[0].ScrollOffset
	if bottomOffset == 0 {
		t.Fatal("expected bottom clamp to establish a non-zero scroll offset")
	}

	m.columns[0].Cursor--
	m.clampScrollToVisible(0)
	if m.columns[0].ScrollOffset != bottomOffset {
		t.Fatalf("scroll offset changed while cursor stayed visible: got %d, want %d", m.columns[0].ScrollOffset, bottomOffset)
	}

	m.columns[0].Cursor = 0
	m.clampScrollToVisible(0)
	if m.columns[0].ScrollOffset != 0 {
		t.Fatalf("scroll offset should clamp when cursor jumps above viewport: got %d", m.columns[0].ScrollOffset)
	}
}

func TestKanbanPersistentScrollOffsetsArePerColumn(t *testing.T) {
	m := newTestKanban([]KanbanColumn{makeCol("todo", 12), makeCol("done", 12)})
	m.width = 80
	m.height = 10

	m.columns[0].Cursor = len(m.columns[0].Tasks) - 1
	m.clampScrollToVisible(0)
	firstOffset := m.columns[0].ScrollOffset
	if firstOffset == 0 {
		t.Fatal("expected first column to establish a non-zero scroll offset")
	}

	m.activeCol = 1
	m.columns[1].Cursor = 1
	m.clampScrollToVisible(1)

	if m.columns[0].ScrollOffset != firstOffset {
		t.Fatalf("inactive column offset changed: got %d, want %d", m.columns[0].ScrollOffset, firstOffset)
	}
	if m.columns[1].ScrollOffset != 0 {
		t.Fatalf("second column should maintain its own offset: got %d", m.columns[1].ScrollOffset)
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

func TestKanbanBulkPickerMessagesUseSelectedRefs(t *testing.T) {
	projectA, tasksA := newSharedBulkProject("Alpha", "cursor", "selected-alpha")
	projectB, tasksB := newSharedBulkProject("Beta", "selected-beta")

	m := newTestKanban([]KanbanColumn{
		{ID: "todo", Label: "Todo", Tasks: []KanbanCard{
			{Task: tasksA[0], Project: projectA},
			{Task: tasksA[1], Project: projectA},
		}},
		{ID: "next", Label: "Next", Tasks: []KanbanCard{
			{Task: tasksB[0], Project: projectB},
		}},
	})
	m.MultiSelectState = MultiSelectState{
		multiSelectMode: true,
		selectedTasks: map[string]bool{
			tasksA[1].ID: true,
			tasksB[0].ID: true,
		},
	}

	km := m.buildKeyMap()
	want := []TaskProjectRef{
		{Task: tasksA[1], Project: projectA},
		{Task: tasksB[0], Project: projectB},
	}

	tagAction := findSharedTaskKeymapAction(t, km, "t", "t")
	tagMsg, ok := runSharedTaskKeymapAction(t, tagAction).(ShowBulkTagSelectorMsg)
	if !ok {
		t.Fatalf("tt: expected ShowBulkTagSelectorMsg, got %T", tagMsg)
	}
	assertTaskProjectRefs(t, tagMsg.TaskRefs, want)

	dueAction := findSharedTaskKeymapAction(t, km, "d", "d")
	dueMsg, ok := runSharedTaskKeymapAction(t, dueAction).(SetDueDateMsg)
	if !ok {
		t.Fatalf("dd: expected SetDueDateMsg, got %T", dueMsg)
	}
	assertTaskProjectRefs(t, dueMsg.TaskRefs, want)

	snoozeAction := findSharedTaskKeymapAction(t, km, "z", "d")
	snoozeMsg, ok := runSharedTaskKeymapAction(t, snoozeAction).(SetSnoozeDateMsg)
	if !ok {
		t.Fatalf("zd: expected SetSnoozeDateMsg, got %T", snoozeMsg)
	}
	assertTaskProjectRefs(t, snoozeMsg.TaskRefs, want)
}

func TestKanbanBulkQuickDateAndSnoozeApplySelectedCards(t *testing.T) {
	alpha, alphaTask := newDiskKanbanProject(t, "alpha-proj", "Alpha", "alpha-task")
	beta, betaTask := newDiskKanbanProject(t, "beta-proj", "Beta", "beta-task")

	m := newTestKanban([]KanbanColumn{
		{ID: "todo", Label: "Todo", Tasks: []KanbanCard{{Task: alphaTask, Project: alpha}}},
		{ID: "next", Label: "Next", Tasks: []KanbanCard{{Task: betaTask, Project: beta}}},
	})
	m.svc = NewProjectService()
	m.loadedProjects = []*models.Project{alpha, beta}
	m.MultiSelectState = MultiSelectState{
		multiSelectMode: true,
		selectedTasks: map[string]bool{
			alphaTask.ID: true,
			betaTask.ID:  true,
		},
	}

	model, cmd := m.applySelectedBulkDueDate(0, false)
	if cmd != nil {
		t.Fatalf("expected inline bulk due-date mutation, got command %T", cmd())
	}
	m = model.(*KanbanModel)

	today := util.StartOfDay(time.Now())
	for name, task := range map[string]*models.Task{
		"alpha": alpha.GetTaskByID(alphaTask.ID),
		"beta":  beta.GetTaskByID(betaTask.ID),
	} {
		if task == nil {
			t.Fatalf("%s task not found", name)
		}
		if task.DueDate == nil || !task.DueDate.Equal(today) {
			t.Fatalf("%s due date = %v, want %v", name, task.DueDate, today)
		}
	}

	m.multiSelectMode = true
	m.selectedTasks = map[string]bool{alphaTask.ID: true, betaTask.ID: true}
	model, cmd = m.applySelectedBulkSnooze("w")
	if cmd != nil {
		t.Fatalf("expected inline bulk snooze mutation, got command %T", cmd())
	}
	m = model.(*KanbanModel)

	nextWeek := util.StartOfDay(time.Now()).AddDate(0, 0, 7)
	for name, task := range map[string]*models.Task{
		"alpha": alpha.GetTaskByID(alphaTask.ID),
		"beta":  beta.GetTaskByID(betaTask.ID),
	} {
		if task == nil {
			t.Fatalf("%s task not found", name)
		}
		if task.SnoozedUntil == nil || !task.SnoozedUntil.Equal(nextWeek) {
			t.Fatalf("%s snooze = %v, want %v", name, task.SnoozedUntil, nextWeek)
		}
	}
	if m.multiSelectMode || len(m.selectedTasks) != 0 {
		t.Fatal("expected Kanban bulk field mutations to clear selection")
	}
}

func TestKanbanBulkKeymapActionsMutateSelectedCardsOnly(t *testing.T) {
	tests := []struct {
		name       string
		keys       []string
		setup      func(*kanbanBulkActionFixture)
		assert     func(*testing.T, *kanbanBulkActionFixture)
		wantStatus string
	}{
		{
			name: "status prefix mark done",
			keys: []string{",", "d"},
			assert: func(t *testing.T, f *kanbanBulkActionFixture) {
				t.Helper()
				assertKanbanBulkTask(t, f.projectA, f.cursor.ID, func(task *models.Task) {
					if task.Status != models.StatusTodo {
						t.Fatalf("cursor status = %s, want %s", task.Status, models.StatusTodo)
					}
				})
				for _, ref := range f.selectedRefs {
					assertKanbanBulkTask(t, ref.Project, ref.Task.ID, func(task *models.Task) {
						if task.Status != models.StatusDone {
							t.Fatalf("selected status = %s, want %s", task.Status, models.StatusDone)
						}
					})
				}
			},
			wantStatus: "Changed 2 tasks to done",
		},
		{
			name: "top-level mark done",
			keys: []string{"D"},
			assert: func(t *testing.T, f *kanbanBulkActionFixture) {
				t.Helper()
				assertKanbanBulkTask(t, f.projectA, f.cursor.ID, func(task *models.Task) {
					if task.Status != models.StatusTodo {
						t.Fatalf("cursor status = %s, want %s", task.Status, models.StatusTodo)
					}
				})
				for _, ref := range f.selectedRefs {
					assertKanbanBulkTask(t, ref.Project, ref.Task.ID, func(task *models.Task) {
						if task.Status != models.StatusDone {
							t.Fatalf("selected status = %s, want %s", task.Status, models.StatusDone)
						}
					})
				}
			},
			wantStatus: "Changed 2 tasks to done",
		},
		{
			name: "priority",
			keys: []string{"!", "3"},
			assert: func(t *testing.T, f *kanbanBulkActionFixture) {
				t.Helper()
				assertKanbanBulkTask(t, f.projectA, f.cursor.ID, func(task *models.Task) {
					if task.Priority != 1 {
						t.Fatalf("cursor priority = %d, want 1", task.Priority)
					}
				})
				for _, ref := range f.selectedRefs {
					assertKanbanBulkTask(t, ref.Project, ref.Task.ID, func(task *models.Task) {
						if task.Priority != 3 {
							t.Fatalf("selected priority = %d, want 3", task.Priority)
						}
					})
				}
			},
			wantStatus: "Set priority 3 on 2 tasks",
		},
		{
			name: "set stage",
			keys: []string{"s", "1"},
			assert: func(t *testing.T, f *kanbanBulkActionFixture) {
				t.Helper()
				assertKanbanBulkTask(t, f.projectA, f.cursor.ID, func(task *models.Task) {
					if task.Stage != "Cursor" {
						t.Fatalf("cursor stage = %q, want %q", task.Stage, "Cursor")
					}
				})
				for _, ref := range f.selectedRefs {
					assertKanbanBulkTask(t, ref.Project, ref.Task.ID, func(task *models.Task) {
						if task.Stage != "Design" {
							t.Fatalf("selected stage = %q, want %q", task.Stage, "Design")
						}
					})
				}
			},
			wantStatus: "Set stage on 2 tasks",
		},
		{
			name: "clear stage",
			keys: []string{"s", "0"},
			setup: func(f *kanbanBulkActionFixture) {
				for _, ref := range f.selectedRefs {
					ref.Task.Stage = "Build"
				}
				f.cursor.Stage = "Cursor"
			},
			assert: func(t *testing.T, f *kanbanBulkActionFixture) {
				t.Helper()
				assertKanbanBulkTask(t, f.projectA, f.cursor.ID, func(task *models.Task) {
					if task.Stage != "Cursor" {
						t.Fatalf("cursor stage = %q, want %q", task.Stage, "Cursor")
					}
				})
				for _, ref := range f.selectedRefs {
					assertKanbanBulkTask(t, ref.Project, ref.Task.ID, func(task *models.Task) {
						if task.Stage != "" {
							t.Fatalf("selected stage = %q, want empty", task.Stage)
						}
					})
				}
			},
			wantStatus: "Cleared stage on 2 tasks",
		},
		{
			name: "toggle active",
			keys: []string{"*"},
			assert: func(t *testing.T, f *kanbanBulkActionFixture) {
				t.Helper()
				assertKanbanBulkTask(t, f.projectA, f.cursor.ID, func(task *models.Task) {
					if task.Active {
						t.Fatal("cursor active = true, want false")
					}
				})
				for _, ref := range f.selectedRefs {
					assertKanbanBulkTask(t, ref.Project, ref.Task.ID, func(task *models.Task) {
						if !task.Active {
							t.Fatal("selected active = false, want true")
						}
					})
				}
			},
			wantStatus: "Toggled active on 2 tasks",
		},
		{
			name: "toggle pin",
			keys: []string{"t", "p"},
			assert: func(t *testing.T, f *kanbanBulkActionFixture) {
				t.Helper()
				assertKanbanBulkTask(t, f.projectA, f.cursor.ID, func(task *models.Task) {
					if task.Pinned {
						t.Fatal("cursor pinned = true, want false")
					}
				})
				for _, ref := range f.selectedRefs {
					assertKanbanBulkTask(t, ref.Project, ref.Task.ID, func(task *models.Task) {
						if !task.Pinned {
							t.Fatal("selected pinned = false, want true")
						}
					})
				}
			},
			wantStatus: "Toggled pin on 2 tasks",
		},
		{
			name: "toggle dim",
			keys: []string{"t", "h"},
			assert: func(t *testing.T, f *kanbanBulkActionFixture) {
				t.Helper()
				assertKanbanBulkTask(t, f.projectA, f.cursor.ID, func(task *models.Task) {
					if task.Dimmed {
						t.Fatal("cursor dimmed = true, want false")
					}
				})
				for _, ref := range f.selectedRefs {
					assertKanbanBulkTask(t, ref.Project, ref.Task.ID, func(task *models.Task) {
						if !task.Dimmed {
							t.Fatal("selected dimmed = false, want true")
						}
					})
				}
			},
			wantStatus: "Toggled dim on 2 tasks",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fixture := newKanbanBulkActionFixture(t)
			if tt.setup != nil {
				tt.setup(fixture)
				saveAndReloadKanbanBulkFixture(t, fixture)
			}

			fixture.model = runKanbanKeymapAction(t, fixture.model, tt.keys...)
			if fixture.model.statusMsg != tt.wantStatus {
				t.Fatalf("status = %q, want %q", fixture.model.statusMsg, tt.wantStatus)
			}
			assertKanbanSelectionCleared(t, fixture.model)
			tt.assert(t, fixture)
		})
	}
}

func TestKanbanBulkSectionMoveSelectedCardsOnlyForSameProject(t *testing.T) {
	project, tasks := newDiskKanbanProjectWithTasks(t, "alpha-proj", "Alpha", "cursor", "selected-one", "selected-two")
	m := newTestKanban([]KanbanColumn{
		{ID: "todo", Label: "Todo", Tasks: []KanbanCard{
			{Task: tasks[0], Project: project},
			{Task: tasks[1], Project: project},
			{Task: tasks[2], Project: project},
		}},
	})
	m.svc = NewProjectService()
	m.loadedProjects = []*models.Project{project}
	m.MultiSelectState = MultiSelectState{
		multiSelectMode: true,
		selectedTasks: map[string]bool{
			tasks[1].ID: true,
			tasks[2].ID: true,
		},
	}

	m = runKanbanKeymapAction(t, m, "m", "n")
	if m.statusMsg != "Moved 2 tasks to Next" {
		t.Fatalf("status = %q, want %q", m.statusMsg, "Moved 2 tasks to Next")
	}
	assertKanbanSelectionCleared(t, m)
	assertKanbanBulkTask(t, project, tasks[0].ID, func(task *models.Task) {
		if task.Section != "" {
			t.Fatalf("cursor section = %q, want empty", task.Section)
		}
	})
	for _, task := range tasks[1:] {
		assertKanbanBulkTask(t, project, task.ID, func(task *models.Task) {
			if task.Section != models.SectionNext {
				t.Fatalf("selected section = %q, want %q", task.Section, models.SectionNext)
			}
		})
	}
}

func TestKanbanBulkSectionMoveRejectsCrossProjectSelection(t *testing.T) {
	fixture := newKanbanBulkActionFixture(t)

	fixture.model = runKanbanKeymapAction(t, fixture.model, "m", "n")
	if fixture.model.statusMsg != "bulk section move not supported across projects" {
		t.Fatalf("status = %q, want %q", fixture.model.statusMsg, "bulk section move not supported across projects")
	}
	assertKanbanSelectionPreserved(t, fixture.model, fixture.selectedRefs...)
	assertKanbanBulkTask(t, fixture.projectA, fixture.cursor.ID, func(task *models.Task) {
		if task.Section != "" {
			t.Fatalf("cursor section = %q, want empty", task.Section)
		}
	})
	for _, ref := range fixture.selectedRefs {
		assertKanbanBulkTask(t, ref.Project, ref.Task.ID, func(task *models.Task) {
			if task.Section != "" {
				t.Fatalf("selected section = %q, want empty", task.Section)
			}
		})
	}
}

func TestKanbanBulkDeleteRejectsSelectedCards(t *testing.T) {
	fixture := newKanbanBulkActionFixture(t)

	fixture.model = runKanbanKeymapAction(t, fixture.model, "t", "D")
	if fixture.model.statusMsg != "bulk delete not supported in Kanban" {
		t.Fatalf("status = %q, want %q", fixture.model.statusMsg, "bulk delete not supported in Kanban")
	}
	assertKanbanSelectionPreserved(t, fixture.model, fixture.selectedRefs...)
	for _, ref := range append([]TaskProjectRef{{Task: fixture.cursor, Project: fixture.projectA}}, fixture.selectedRefs...) {
		assertKanbanBulkTask(t, ref.Project, ref.Task.ID, func(task *models.Task) {
			if task == nil {
				t.Fatalf("task %q was deleted", ref.Task.ID)
			}
		})
	}
}

func newDiskKanbanProject(t *testing.T, id, name, taskID string) (*models.Project, *models.Task) {
	t.Helper()

	project, tasks := newDiskKanbanProjectWithTasks(t, id, name, taskID)
	return project, tasks[0]
}

func newDiskKanbanProjectWithTasks(t *testing.T, id, name string, taskIDs ...string) (*models.Project, []*models.Task) {
	t.Helper()

	project := &models.Project{ID: id, Name: name, Path: t.TempDir()}
	tasks := make([]*models.Task, 0, len(taskIDs))
	for i, taskID := range taskIDs {
		task, err := models.NewTask(taskID, i+1, models.TaskTypeTask, name+" "+taskID)
		if err != nil {
			t.Fatalf("NewTask: %v", err)
		}
		task.Project = project
		project.Tasks = append(project.Tasks, task)
		tasks = append(tasks, task)
	}
	project.NextShortID = len(project.Tasks) + 1
	project.BuildTaskTree()
	if err := storage.SaveProject(project); err != nil {
		t.Fatalf("save project: %v", err)
	}
	if err := storage.LoadProject(project); err != nil {
		t.Fatalf("load project: %v", err)
	}
	for i, taskID := range taskIDs {
		loaded := project.GetTaskByID(taskID)
		if loaded == nil {
			t.Fatalf("task %q not found after load", taskID)
		}
		tasks[i] = loaded
	}
	return project, tasks
}

type kanbanBulkActionFixture struct {
	model        *KanbanModel
	projectA     *models.Project
	projectB     *models.Project
	cursor       *models.Task
	selectedRefs []TaskProjectRef
}

func newKanbanBulkActionFixture(t *testing.T) *kanbanBulkActionFixture {
	t.Helper()

	projectA, tasksA := newDiskKanbanProjectWithTasks(t, "alpha-proj", "Alpha", "cursor", "selected-alpha")
	projectB, tasksB := newDiskKanbanProjectWithTasks(t, "beta-proj", "Beta", "selected-beta")

	tasksA[0].Priority = 1
	tasksA[0].Stage = "Cursor"
	tasksA[1].Priority = 1
	tasksB[0].Priority = 1
	saveAndReloadKanbanBulkProject(t, projectA)
	saveAndReloadKanbanBulkProject(t, projectB)
	tasksA[0] = projectA.GetTaskByID(tasksA[0].ID)
	tasksA[1] = projectA.GetTaskByID(tasksA[1].ID)
	tasksB[0] = projectB.GetTaskByID(tasksB[0].ID)

	m := newTestKanban([]KanbanColumn{
		{ID: "todo", Label: "Todo", Tasks: []KanbanCard{
			{Task: tasksA[0], Project: projectA},
			{Task: tasksA[1], Project: projectA},
		}},
		{ID: "next", Label: "Next", Tasks: []KanbanCard{
			{Task: tasksB[0], Project: projectB},
		}},
	})
	m.svc = NewProjectService()
	m.loadedProjects = []*models.Project{projectA, projectB}
	m.config.Stages = []string{"Design", "Build"}
	m.MultiSelectState = MultiSelectState{
		multiSelectMode: true,
		selectedTasks: map[string]bool{
			tasksA[1].ID: true,
			tasksB[0].ID: true,
		},
	}

	return &kanbanBulkActionFixture{
		model:    m,
		projectA: projectA,
		projectB: projectB,
		cursor:   tasksA[0],
		selectedRefs: []TaskProjectRef{
			{Task: tasksA[1], Project: projectA},
			{Task: tasksB[0], Project: projectB},
		},
	}
}

func runKanbanKeymapAction(t *testing.T, m *KanbanModel, keys ...string) *KanbanModel {
	t.Helper()

	action := findSharedTaskKeymapAction(t, m.buildKeyMap(), keys...)
	model, cmd := action()
	if cmd != nil {
		t.Fatalf("binding %v returned unexpected command %T", keys, cmd)
	}
	kanban, ok := model.(*KanbanModel)
	if !ok {
		t.Fatalf("binding %v returned %T, want *KanbanModel", keys, model)
	}
	return kanban
}

func saveAndReloadKanbanBulkFixture(t *testing.T, f *kanbanBulkActionFixture) {
	t.Helper()

	saveAndReloadKanbanBulkProject(t, f.projectA)
	saveAndReloadKanbanBulkProject(t, f.projectB)
	f.cursor = f.projectA.GetTaskByID(f.cursor.ID)
	for i := range f.selectedRefs {
		f.selectedRefs[i].Task = f.selectedRefs[i].Project.GetTaskByID(f.selectedRefs[i].Task.ID)
	}
}

func saveAndReloadKanbanBulkProject(t *testing.T, project *models.Project) {
	t.Helper()

	if err := storage.SaveProject(project); err != nil {
		t.Fatalf("save project: %v", err)
	}
	if err := storage.LoadProject(project); err != nil {
		t.Fatalf("load project: %v", err)
	}
}

func assertKanbanBulkTask(t *testing.T, project *models.Project, taskID string, assert func(*models.Task)) {
	t.Helper()

	task := project.GetTaskByID(taskID)
	if task == nil {
		t.Fatalf("task %q not found", taskID)
	}
	assert(task)
}

func assertKanbanSelectionCleared(t *testing.T, m *KanbanModel) {
	t.Helper()

	if m.multiSelectMode || m.SelectedCount() != 0 {
		t.Fatalf("selection = mode %v count %d, want cleared", m.multiSelectMode, m.SelectedCount())
	}
}

func assertKanbanSelectionPreserved(t *testing.T, m *KanbanModel, refs ...TaskProjectRef) {
	t.Helper()

	if !m.multiSelectMode {
		t.Fatal("multi-select mode = false, want preserved")
	}
	if m.SelectedCount() != len(refs) {
		t.Fatalf("selection count = %d, want %d", m.SelectedCount(), len(refs))
	}
	for _, ref := range refs {
		if !m.IsSelected(ref.Task.ID) {
			t.Fatalf("task %q selection was not preserved", ref.Task.ID)
		}
	}
}
