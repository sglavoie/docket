package tui

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

func TestFocusBoardSortKeysUpdateTodayOrdering(t *testing.T) {
	project := cockpitProject(t, "Today Sort")
	lowPriority := cockpitTask(t, project, "Low priority", models.StatusTodo, models.SectionNext)
	lowPriority.Priority = 3
	highPriority := cockpitTask(t, project, "High priority", models.StatusTodo, models.SectionNext)
	highPriority.Priority = 1

	rows := []boardRow{
		cockpitBoardRow(lowPriority, project),
		cockpitBoardRow(highPriority, project),
	}
	m := cockpitModel(
		rows,
		nil,
		nil,
		map[string]bool{lowPriority.ID: true, highPriority.ID: true},
		OpenLoops{TodayTotal: 2, HasTasks: true},
		boardTabToday,
	)
	sortByPriority := findSharedTaskKeymapAction(t, m.buildKeyMap(), "1")

	model, cmd := sortByPriority()
	if model != m {
		t.Fatalf("sort key returned model %T, want original FocusBoardModel", model)
	}
	if cmd != nil {
		t.Fatal("sort key returned a command, want immediate model update only")
	}
	if m.sort.Field != SortByPriority || !m.sort.Ascending {
		t.Fatalf("sort state after `1` = %+v, want priority ascending", m.sort)
	}
	if got := m.filtered[0].Task.ID; got != highPriority.ID {
		t.Fatalf("priority ascending first row = %q, want high-priority task %q", got, highPriority.ID)
	}

	sortByPriority()
	if m.sort.Field != SortByPriority || m.sort.Ascending {
		t.Fatalf("sort state after second `1` = %+v, want priority descending", m.sort)
	}
	if got := m.filtered[0].Task.ID; got != lowPriority.ID {
		t.Fatalf("priority descending first row = %q, want low-priority task %q", got, lowPriority.ID)
	}

	resetSort := findSharedTaskKeymapAction(t, m.buildKeyMap(), "0")
	resetSort()
	if m.sort.Field != SortByDefault || !m.sort.Ascending {
		t.Fatalf("sort state after `0` = %+v, want default ascending", m.sort)
	}
	if got := m.filtered[0].Task.ID; got != lowPriority.ID {
		t.Fatalf("reset sort first row = %q, want original Today order %q", got, lowPriority.ID)
	}
}

func TestFocusBoardSortSkipsGroupHeaders(t *testing.T) {
	project := cockpitProject(t, "Today Sort Headers")
	lowPriority := cockpitTask(t, project, "Low priority", models.StatusTodo, models.SectionNext)
	lowPriority.Priority = 3
	highPriority := cockpitTask(t, project, "High priority", models.StatusTodo, models.SectionNext)
	highPriority.Priority = 1

	m := newTestFocusBoard()
	m.todayRows = []boardRow{
		{GroupHeader: "Planned"},
		cockpitBoardRow(lowPriority, project),
		cockpitBoardRow(highPriority, project),
	}
	m.tabs.Active = boardTabToday
	m.applyFilter()

	findSharedTaskKeymapAction(t, m.buildKeyMap(), "1")()

	if got := m.filtered[0].GroupHeader; got != "Planned" {
		t.Fatalf("group header moved after sort: first row header = %q", got)
	}
	if got := m.filtered[1].Task.ID; got != highPriority.ID {
		t.Fatalf("sorted first task row = %q, want high-priority task %q", got, highPriority.ID)
	}
}

func TestFocusBoardFpTogglesInlinePreview(t *testing.T) {
	m := newTestFocusBoard()
	m.viewScrollOffset = 4
	action := findSharedTaskKeymapAction(t, m.buildKeyMap(), "f", "p")

	model, cmd := action()
	if model != m {
		t.Fatalf("fp returned model %T, want original FocusBoardModel", model)
	}
	if cmd != nil {
		t.Fatal("fp returned a command, want immediate model update only")
	}
	if !m.previewMode {
		t.Fatal("expected fp to toggle previewMode on")
	}
	if m.viewScrollOffset != 0 {
		t.Fatalf("fp viewScrollOffset = %d, want reset to 0", m.viewScrollOffset)
	}

	action()
	if m.previewMode {
		t.Fatal("expected second fp to toggle previewMode off")
	}
}
