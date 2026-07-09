package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
)

func TestFocusBoardTaskPrefixCreateUsesSelectedProject(t *testing.T) {
	m, task, project := focusBoardTaskActionFixture(t)
	km := m.buildKeyMap()

	tests := []struct {
		name     string
		key      string
		wantSkip bool
	}{
		{name: "new task", key: "n", wantSkip: true},
		{name: "new task with template", key: "N", wantSkip: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			action := findSharedTaskKeymapAction(t, km, "t", tt.key)
			create, ok := runSharedTaskKeymapAction(t, action).(CreateTaskMsg)
			if !ok {
				t.Fatalf("t%s emitted %T, want CreateTaskMsg", tt.key, create)
			}
			if create.ParentTask != task {
				t.Fatalf("t%s ParentTask = %v, want %v", tt.key, create.ParentTask, task)
			}
			if create.Project != project {
				t.Fatalf("t%s Project = %v, want %v", tt.key, create.Project, project)
			}
			if create.SkipTemplatePicker != tt.wantSkip {
				t.Fatalf("t%s SkipTemplatePicker = %v, want %v", tt.key, create.SkipTemplatePicker, tt.wantSkip)
			}
		})
	}
}

func TestFocusBoardSharedDoneKeysUseFlowAwareCompletion(t *testing.T) {
	tests := []struct {
		name string
		keys []string
	}{
		{name: "top-level done", keys: []string{"D"}},
		{name: "status done", keys: []string{",", "d"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m, _, tasks := newFlowLoopFocusBoard(t, models.StatusTodo, models.StatusTodo)
			m.cursor = 0

			action := findSharedTaskKeymapAction(t, m.buildKeyMap(), tt.keys...)
			model, cmd := action()

			if model != m {
				t.Fatalf("%v returned model %T, want original FocusBoardModel", tt.keys, model)
			}
			if cmd == nil {
				t.Fatalf("%v returned nil command, want completion side-effect command", tt.keys)
			}
			if got := m.filtered[m.cursor].Task.ID; got != tasks[1].ID {
				t.Fatalf("%v selected %q, want next actionable task %q", tt.keys, got, tasks[1].ID)
			}
			if got := focusBoardRowStatus(t, m.todayRows, tasks[0].ID); got != models.StatusDone {
				t.Fatalf("%v completed task status = %s, want %s", tt.keys, got, models.StatusDone)
			}
			if !strings.Contains(m.statusMessage, "done") {
				t.Fatalf("%v statusMessage = %q, want completion status", tt.keys, m.statusMessage)
			}
		})
	}
}

func TestFocusBoardStatusChangeActionIgnoresUnknownStatus(t *testing.T) {
	m, task, _ := focusBoardTaskActionFixture(t)

	action := m.focusBoardStatusChangeAction("not-a-status")
	model, cmd := action()

	if model != m {
		t.Fatalf("invalid status returned model %T, want original FocusBoardModel", model)
	}
	if cmd != nil {
		t.Fatal("invalid status returned a command, want nil command")
	}
	if task.Status != models.StatusTodo {
		t.Fatalf("invalid status changed task status to %s, want %s", task.Status, models.StatusTodo)
	}
}

func TestFocusBoardStatusChangeActionUsesTaskOpsForValidNonDoneStatus(t *testing.T) {
	m, _, tasks := newFlowLoopFocusBoard(t, models.StatusTodo)
	m.cursor = 0

	action := m.focusBoardStatusChangeAction(string(models.StatusBlocked))
	model, cmd := action()

	if model != m {
		t.Fatalf("valid status returned model %T, want original FocusBoardModel", model)
	}
	if cmd == nil {
		t.Fatal("valid status returned nil command, want status-change command")
	}
	if _, ok := cmd().(TaskStatusChangedMsg); !ok {
		t.Fatalf("valid status command emitted unexpected message")
	}
	if got := focusBoardRowStatus(t, m.todayRows, tasks[0].ID); got != models.StatusBlocked {
		t.Fatalf("valid status changed task status to %s, want %s", got, models.StatusBlocked)
	}
	if !strings.Contains(m.statusMessage, string(models.StatusBlocked)) {
		t.Fatalf("statusMessage = %q, want blocked status", m.statusMessage)
	}
}

func TestFocusBoardTaskPrefixDeleteShowsConfirm(t *testing.T) {
	m, _, _ := focusBoardTaskActionFixture(t)

	action := findSharedTaskKeymapAction(t, m.buildKeyMap(), "t", "D")
	model, cmd := action()

	if model != m {
		t.Fatalf("tD returned model %T, want original FocusBoardModel", model)
	}
	if cmd != nil {
		t.Fatal("tD returned a command, want confirm-only nil command")
	}
	if !m.IsConfirmActive() {
		t.Fatal("tD should open the delete confirmation dialog")
	}
}

func TestFocusBoardSharedTaskActionsUseSelectedProject(t *testing.T) {
	m, task, project := focusBoardTaskActionFixture(t)
	km := m.buildKeyMap()

	tests := []struct {
		name string
		keys []string
		want func(tea.Msg) bool
	}{
		{
			name: "view task details",
			keys: []string{"o"},
			want: func(msg tea.Msg) bool {
				got, ok := msg.(ShowTaskDetailMsg)
				return ok && got.Task == task && got.Project == project
			},
		},
		{
			name: "task prefix park",
			keys: []string{"t", "P"},
			want: func(msg tea.Msg) bool {
				got, ok := msg.(ShowParkInputMsg)
				return ok && got.Task == task && got.Project == project
			},
		},
		{
			name: "move to project",
			keys: []string{"ctrl+b"},
			want: func(msg tea.Msg) bool {
				got, ok := msg.(ShowProjectSelectorForMoveMsg)
				return ok && got.Task == task && got.SourceProject == project
			},
		},
		{
			name: "quick note",
			keys: []string{"ctrl+."},
			want: func(msg tea.Msg) bool {
				got, ok := msg.(OpenQuickNoteMsg)
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

	if action := findSharedTaskKeymapAction(t, km, "O"); action == nil {
		t.Fatal("O binding should be present for Today open URL")
	}
}

func focusBoardTaskActionFixture(t *testing.T) (*FocusBoardModel, *models.Task, *models.Project) {
	t.Helper()

	project := cockpitProject(t, "Today Task Actions")
	task := cockpitTask(t, project, "selected task", models.StatusTodo, models.SectionNext)
	m := newTestFocusBoard()
	m.filtered = []boardRow{cockpitBoardRow(task, project)}
	m.cursor = 0
	return m, task, project
}

func focusBoardRowStatus(t *testing.T, rows []boardRow, taskID string) models.TaskStatus {
	t.Helper()
	for _, row := range rows {
		if row.Task != nil && row.Task.ID == taskID {
			return row.Task.Status
		}
	}
	t.Fatalf("task %q not found in focus board rows", taskID)
	return ""
}
