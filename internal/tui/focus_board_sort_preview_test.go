package tui

import (
	"slices"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
)

func TestPlannerViewMenuUpdatesTodayOrdering(t *testing.T) {
	project := cockpitProject(t, "Planner Sort")
	lowPriority := cockpitTask(t, project, "Low priority", models.StatusTodo, models.SectionNext)
	lowPriority.Priority = 3
	highPriority := cockpitTask(t, project, "High priority", models.StatusTodo, models.SectionNext)
	highPriority.Priority = 1

	m := cockpitModel(
		[]boardRow{cockpitBoardRow(lowPriority, project), cockpitBoardRow(highPriority, project)},
		nil, nil,
		map[TaskIdentity]bool{
			taskIdentityFor(lowPriority, project):  true,
			taskIdentityFor(highPriority, project): true,
		},
		OpenLoops{TodayTotal: 2, HasTasks: true},
		boardTabToday,
	)
	m.rebuildPlannerDispatcher()

	open := findSharedTaskKeymapAction(t, m.buildKeyMap(), "v")
	_, cmd := open()
	if msg, ok := cmd().(plannerViewMenuMsg); !ok || !msg.Open || !m.menuOpen {
		t.Fatalf("v emitted %#v with menuOpen=%v, want open Planner View menu", msg, m.menuOpen)
	}

	movePlannerMenuCursorToLabel(t, m, "Sort: Priority")
	m.handlePlannerMenuKey(syntheticSpecialKey(tea.KeyEnter))
	if m.sort.Field != SortByPriority || !m.sort.Ascending {
		t.Fatalf("View menu sort state = %+v, want priority ascending", m.sort)
	}
	if got := m.filtered[0].Task.ID; got != highPriority.ID {
		t.Fatalf("priority sort first row = %q, want %q", got, highPriority.ID)
	}
	if m.menuOpen {
		t.Fatal("applying a View menu item should close the menu")
	}

	for _, retired := range []string{"0", "1", "2", "3", "4", "5", "fp"} {
		if slices.Contains(m.HandledKeys(), retired) {
			t.Fatalf("retired sort/preview key %q remains bound", retired)
		}
	}
}

func movePlannerMenuCursorToLabel(t *testing.T, m *FocusBoardModel, label string) {
	t.Helper()
	for index, item := range m.plannerMenuItems() {
		if item.label != label {
			continue
		}
		for m.menuCursor < index {
			m.handlePlannerMenuKey(syntheticKey('j'))
		}
		for m.menuCursor > index {
			m.handlePlannerMenuKey(syntheticKey('k'))
		}
		return
	}
	t.Fatalf("Planner View menu missing %q", label)
}

func TestPlannerViewMenuTogglesInlinePreview(t *testing.T) {
	m, _, _ := focusBoardTaskActionFixture(t)
	m.viewScrollOffset = 4

	findSharedTaskKeymapAction(t, m.buildKeyMap(), "v")()
	m.handlePlannerMenuKey(syntheticSpecialKey(tea.KeyEnter))
	if !m.previewMode {
		t.Fatal("Preview menu item did not enable inline preview")
	}

	findSharedTaskKeymapAction(t, m.buildKeyMap(), "v")()
	m.handlePlannerMenuKey(syntheticSpecialKey(tea.KeyEnter))
	if m.previewMode {
		t.Fatal("second Preview menu selection did not disable inline preview")
	}
}
