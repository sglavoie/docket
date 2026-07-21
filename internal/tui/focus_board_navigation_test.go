package tui

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

func TestPlannerNavigationDetailAndPeekGrammar(t *testing.T) {
	m, task, project := focusBoardTaskActionFixture(t)
	km := m.buildKeyMap()

	tests := []struct {
		name string
		keys []string
		want func(any) bool
	}{
		{
			name: "enter opens details",
			keys: []string{"enter"},
			want: func(msg any) bool {
				got, ok := msg.(ShowTaskDetailMsg)
				return ok && got.Task == task && got.Project == project
			},
		},
		{
			name: "shift enter shares the task target for shell Peek",
			keys: []string{"shift+enter"},
			want: func(msg any) bool {
				got, ok := msg.(ShowTaskDetailMsg)
				return ok && got.Task == task && got.Project == project
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			action := findSharedTaskKeymapAction(t, km, tt.keys...)
			msg := runSharedTaskKeymapAction(t, action)
			if !tt.want(msg) {
				t.Fatalf("%v emitted %#v with wrong refs", tt.keys, msg)
			}
		})
	}
}

func TestFocusBoardNavigationGotoTopSkipsGroupHeader(t *testing.T) {
	project := cockpitProject(t, "Today Navigation")
	first := cockpitTask(t, project, "First parent", models.StatusTodo, models.SectionNext)
	second := cockpitTask(t, project, "Second parent", models.StatusTodo, models.SectionNext)
	header := boardRow{GroupHeader: "Planned"}

	m := newTestFocusBoard()
	m.filtered = []boardRow{
		header,
		cockpitBoardRow(first, project),
		cockpitBoardRow(second, project),
	}
	m.cursor = 2

	action := findSharedTaskKeymapAction(t, m.buildKeyMap(), "g")
	model, _ := action()

	if model != m {
		t.Fatalf("g returned model %T, want original FocusBoardModel", model)
	}
	if m.cursor != 1 {
		t.Fatalf("g cursor = %d, want first selectable parent row 1", m.cursor)
	}
	if m.filtered[m.cursor].GroupHeader != "" {
		t.Fatalf("g landed on group header at cursor %d", m.cursor)
	}
}

func TestFocusBoardNavigationParentJumpsUseVisibleTopLevelRows(t *testing.T) {
	project := cockpitProject(t, "Parent Navigation")
	first := cockpitTask(t, project, "First parent", models.StatusTodo, models.SectionNext)
	child := cockpitTask(t, project, "Child task", models.StatusTodo, models.SectionNext)
	second := cockpitTask(t, project, "Second parent", models.StatusTodo, models.SectionNext)
	header := boardRow{GroupHeader: "Later"}

	m := newTestFocusBoard()
	m.filtered = []boardRow{
		cockpitBoardRow(first, project),
		{Task: child, Project: project, Source: project.Name, TreePrefix: "child"},
		header,
		cockpitBoardRow(second, project),
	}
	km := m.buildKeyMap()

	nextParent := findSharedTaskKeymapAction(t, km, "n")
	prevParent := findSharedTaskKeymapAction(t, km, "N")

	m.cursor = 1
	nextParent()
	if m.cursor != 3 {
		t.Fatalf("n from child cursor = %d, want next top-level parent 3", m.cursor)
	}
	if m.filtered[m.cursor].GroupHeader != "" {
		t.Fatalf("n landed on group header at cursor %d", m.cursor)
	}

	nextParent()
	if m.cursor != 0 {
		t.Fatalf("n should wrap to first top-level parent, got cursor %d", m.cursor)
	}

	prevParent()
	if m.cursor != 3 {
		t.Fatalf("N should wrap to last top-level parent, got cursor %d", m.cursor)
	}
	if m.filtered[m.cursor].GroupHeader != "" {
		t.Fatalf("N landed on group header at cursor %d", m.cursor)
	}
}
