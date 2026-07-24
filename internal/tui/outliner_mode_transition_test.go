package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
)

func mustMode(t *testing.T, id OutlinerModeID) OutlinerMode {
	t.Helper()
	mode, ok := OutlinerModeByID(id)
	if !ok {
		t.Fatalf("mode %q missing from catalog", id)
	}
	return mode
}

// TestModeTransitionPreservesContext proves scope, pinned tag, search, cursor
// identity, and column state survive a mode switch.
func TestModeTransitionPreservesContext(t *testing.T) {
	all := mustMode(t, ModeAll)
	today := mustMode(t, ModeToday)

	before := OutlinerModeSwitch{
		Mode:         ModeAll,
		ProjectScope: "proj-1",
		PinnedTag:    "backend",
		Search:       "invoice",
		CursorTaskID: "task-9",
		Columns:      []string{"priority", "status"},
	}
	after := ApplyOutlinerModeTransition(before, all, today)

	if after.Mode != ModeToday {
		t.Errorf("mode = %q, want %q", after.Mode, ModeToday)
	}
	if after.ProjectScope != "proj-1" || after.PinnedTag != "backend" || after.Search != "invoice" || after.CursorTaskID != "task-9" {
		t.Errorf("context not preserved: %+v", after)
	}
	if len(after.Columns) != 2 || after.Columns[0] != "priority" || after.Columns[1] != "status" {
		t.Errorf("column state not preserved: %v", after.Columns)
	}
}

// TestModeTransitionDeactivatesManualSortForFixedOrder proves entering a
// fixed-order mode marks manual sort inactive but preserves the stored
// tree-mode sort so it can be restored later.
func TestModeTransitionDeactivatesManualSortForFixedOrder(t *testing.T) {
	all := mustMode(t, ModeAll)
	today := mustMode(t, ModeToday)

	before := OutlinerModeSwitch{
		Mode:             ModeAll,
		ActiveSort:       SortState{Field: SortByPriority, Ascending: true},
		ManualSortActive: true,
	}
	after := ApplyOutlinerModeTransition(before, all, today)

	if after.ManualSortActive {
		t.Error("manual sort should be inactive in a fixed-order mode")
	}
	if after.ActiveSort.Field != SortByDefault {
		t.Errorf("fixed-order active sort = %v, want default", after.ActiveSort.Field)
	}
	if after.TreeSort.Field != SortByPriority {
		t.Errorf("stored tree-mode sort was destroyed: %v", after.TreeSort.Field)
	}

	// Returning to a tree-order mode restores the stashed manual sort.
	restored := ApplyOutlinerModeTransition(after, today, all)
	if !restored.ManualSortActive || restored.ActiveSort.Field != SortByPriority {
		t.Errorf("tree-order restore = active %v sort %v, want active priority", restored.ManualSortActive, restored.ActiveSort.Field)
	}
}

// TestModeTransitionTreeToTreeKeepsSort proves switching between two tree-order
// modes preserves the active manual sort.
func TestModeTransitionTreeToTreeKeepsSort(t *testing.T) {
	all := mustMode(t, ModeAll)
	inbox := mustMode(t, ModeInbox)

	before := OutlinerModeSwitch{
		Mode:             ModeAll,
		ActiveSort:       SortState{Field: SortByDueDate, Ascending: false},
		ManualSortActive: true,
	}
	after := ApplyOutlinerModeTransition(before, all, inbox)
	if !after.ManualSortActive || after.ActiveSort.Field != SortByDueDate || after.ActiveSort.Ascending {
		t.Errorf("tree→tree sort = %+v, want due-date descending active", after.ActiveSort)
	}
}

func selectModeThroughLivePicker(t *testing.T, m *OutlinerModel, query string) {
	t.Helper()
	m.Update(keyPress("P"))
	if m.modePicker == nil {
		t.Fatal("P did not open the live mode picker through the dispatcher")
	}
	for _, r := range query {
		m.Update(syntheticKey(r))
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if m.modePicker != nil {
		t.Fatalf("mode picker remained open after selecting query %q", query)
	}
}

// TestLiveOutlinerModeTransitionRestoresTreeSort exercises the real dispatcher
// and P picker: a manual tree sort is inactive in Due Date but remains durable
// and returns with the same visible order when All is selected again.
func TestLiveOutlinerModeTransitionRestoresTreeSort(t *testing.T) {
	low := &models.Task{ID: "low", Title: "Low", WorkflowState: models.StateReady, Priority: 3}
	high := &models.Task{ID: "high", Title: "High", WorkflowState: models.StateReady, Priority: 1}
	project := &models.Project{ID: "project", Name: "Project", Tasks: []*models.Task{low, high}}
	project.BuildTaskTree()

	m := NewOutlinerModel(project, nil, newTestConfig())
	m.SetSize(100, 30)
	m.selectOutlinerMode(ModeAll)

	// The real sort binding, not a direct field write.
	m.Update(keyPress("1"))
	if m.sort.Field != SortByPriority || m.treeSort != m.sort {
		t.Fatalf("live priority sort = active %+v tree %+v", m.sort, m.treeSort)
	}
	if got := taskRowIDs(m); len(got) != 2 || got[0] != high.ID || got[1] != low.ID {
		t.Fatalf("priority row order = %v, want [high low]", got)
	}

	selectModeThroughLivePicker(t, m, "date")
	if got := m.activeOutlinerMode(); got != ModeDueDateLens {
		t.Fatalf("picker selected %q, want Due Date", got)
	}
	if m.sort != NewSortState() {
		t.Fatalf("Due Date active sort = %+v, want inactive default", m.sort)
	}
	if m.treeSort.Field != SortByPriority {
		t.Fatalf("Due Date destroyed tree sort: %+v", m.treeSort)
	}

	selectModeThroughLivePicker(t, m, "all")
	if m.sort.Field != SortByPriority || m.treeSort != m.sort {
		t.Fatalf("All did not restore priority sort: active %+v tree %+v", m.sort, m.treeSort)
	}
	if got := taskRowIDs(m); len(got) != 2 || got[0] != high.ID || got[1] != low.ID {
		t.Fatalf("restored row order = %v, want [high low]", got)
	}
}

func TestModeTransitionFixedToFixedPreservesTreeSort(t *testing.T) {
	m := NewOutlinerModel(nil, nil, newTestConfig())
	m.selectOutlinerMode(ModeAll)
	m.toggleSort(SortByPriority)

	for _, id := range []OutlinerModeID{
		ModeToday,
		ModeNext,
		ModeBacklog,
		ModePriorityLens,
		ModeDueDateLens,
		ModeDailyReview,
		ModeActivity,
	} {
		m.selectOutlinerMode(id)
		if m.sort != NewSortState() {
			t.Fatalf("%s active sort = %+v, want default", id, m.sort)
		}
		if m.treeSort.Field != SortByPriority {
			t.Fatalf("%s tree sort = %+v, want preserved priority", id, m.treeSort)
		}
	}

	m.selectOutlinerMode(ModeInbox)
	if m.sort.Field != SortByPriority || m.treeSort != m.sort {
		t.Fatalf("fixed→tree restore = active %+v tree %+v", m.sort, m.treeSort)
	}
}

func TestModeTransitionRejectsManualSortInFixedMode(t *testing.T) {
	m := NewOutlinerModel(nil, nil, newTestConfig())
	m.selectOutlinerMode(ModeAll)
	m.toggleSort(SortByPriority)
	m.selectOutlinerMode(ModeToday)
	activeBefore, treeBefore := m.sort, m.treeSort

	// Keyboard and palette commands both route through toggleSort.
	m.Update(keyPress("2"))
	m.toggleSort(SortByScore)
	m.resetSort()

	if m.sort != activeBefore || m.treeSort != treeBefore {
		t.Fatalf("fixed-mode sort changed: active %+v→%+v tree %+v→%+v",
			activeBefore, m.sort, treeBefore, m.treeSort)
	}
	if !strings.Contains(m.statusMessage, "Ordering is automatic in Today mode") {
		t.Fatalf("fixed-mode status = %q", m.statusMessage)
	}
}
