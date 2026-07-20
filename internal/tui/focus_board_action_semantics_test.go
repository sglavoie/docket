package tui

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

func TestFocusBoardActionSemanticsResolveSelectedRow(t *testing.T) {
	project := cockpitProject(t, "Action Semantics")
	task := cockpitTask(t, project, "Draft proposal", models.StatusTodo, models.SectionNext)
	m := cockpitModel([]boardRow{cockpitBoardRow(task, project)}, nil, nil, map[string]bool{task.ID: true}, OpenLoops{TodayTotal: 1, HasTasks: true}, boardTabToday)

	cases := []struct {
		key  string
		want CockpitCommand
	}{
		{key: "a", want: CockpitCommandQuickCapture},
		{key: "/", want: CockpitCommandSearchFilter},
		{key: "enter", want: CockpitCommandOpenTask},
		{key: "x", want: CockpitCommandCompleteTask},
		{key: "p", want: CockpitCommandPrioritizeTask},
	}

	for _, tc := range cases {
		if got := m.resolveCockpitActionForKey(tc.key).Command; got != tc.want {
			t.Fatalf("key %q resolved to %s, want %s", tc.key, got, tc.want)
		}
	}
}

func TestFocusBoardActionSemanticsPlanOrPrioritizeByContext(t *testing.T) {
	t.Run("selected task prioritizes", func(t *testing.T) {
		project := cockpitProject(t, "Selected")
		task := cockpitTask(t, project, "Rank me", models.StatusTodo, models.SectionNext)
		m := cockpitModel([]boardRow{cockpitBoardRow(task, project)}, nil, nil, map[string]bool{task.ID: true}, OpenLoops{TodayTotal: 1, HasTasks: true}, boardTabToday)

		_, cmd := m.executeCockpitAction(m.resolveCockpitActionForKey("p"))
		msg, ok := cmd().(ShowPriorityPickerMsg)
		if !ok {
			t.Fatalf("p on selected row emitted %T, want ShowPriorityPickerMsg", cmd())
		}
		if msg.Task != task || msg.Project != project {
			t.Fatalf("priority target = %+v/%+v, want selected task/project", msg.Task, msg.Project)
		}
	})

	t.Run("first run creates project", func(t *testing.T) {
		m := cockpitModel(nil, nil, nil, nil, OpenLoops{}, boardTabToday)

		_, cmd := m.executeCockpitAction(m.resolveCockpitActionForKey("p"))
		if _, ok := cmd().(CreateProjectMsg); !ok {
			t.Fatalf("p on first-run cockpit emitted %T, want CreateProjectMsg", cmd())
		}
	})

	t.Run("backlog ready opens planning", func(t *testing.T) {
		project := cockpitProject(t, "Backlog")
		task := cockpitTask(t, project, "Choose me later", models.StatusTodo, models.SectionNext)
		m := cockpitModel(nil, []boardRow{cockpitBoardRow(task, project)}, nil, nil, OpenLoops{HasTasks: true}, boardTabToday)

		_, cmd := m.executeCockpitAction(m.resolveCockpitActionForKey("p"))
		msg, ok := cmd().(ShowFocusBoardMsg)
		if !ok {
			t.Fatalf("p on backlog-ready cockpit emitted %T, want ShowFocusBoardMsg", cmd())
		}
		if !msg.SetTab || msg.Tab != boardTabBacklog {
			t.Fatalf("p on backlog-ready cockpit opened tab %d (SetTab=%v), want Backlog tab", msg.Tab, msg.SetTab)
		}
	})

	t.Run("inbox only opens triage", func(t *testing.T) {
		m := cockpitModel(nil, nil, nil, nil, OpenLoops{Inbox: 1, HasTasks: true}, boardTabToday)

		_, cmd := m.executeCockpitAction(m.resolveCockpitActionForKey("p"))
		if _, ok := cmd().(ShowGlobalInboxProcessorMsg); !ok {
			t.Fatalf("p on inbox-only cockpit emitted %T, want ShowGlobalInboxProcessorMsg", cmd())
		}
	})
}

func TestFocusBoardCockpitCommandsHaveActionsAndExecutorPaths(t *testing.T) {
	for _, command := range AllCockpitCommands() {
		t.Run(string(command), func(t *testing.T) {
			m := cockpitModelForCommandExecution(t, command)
			beforeTab := m.tabs.Active
			action := m.cockpitActionForPaletteCommand(command)
			if action.IsZero() {
				t.Fatalf("%s mapped to zero action", command)
			}
			if action.Command != command {
				t.Fatalf("%s mapped to %s action", command, action.Command)
			}

			_, cmd := m.executeCockpitAction(action)
			if command == CockpitCommandSwitchTab {
				if m.tabs.Active == beforeTab {
					t.Fatalf("%s did not advance the active tab", command)
				}
				return
			}
			if cmd == nil {
				t.Fatalf("%s executed without a command", command)
			}
		})
	}
}

func TestFocusBoardActionSemanticsDismissAndUnblock(t *testing.T) {
	t.Run("x dismisses dependency-blocked work", func(t *testing.T) {
		project := cockpitProject(t, "Blocked")
		blocker := cockpitTask(t, project, "Finish blocker", models.StatusTodo, models.SectionNext)
		blocked := cockpitTask(t, project, "Depends on blocker", models.StatusBlocked, models.SectionNext)
		if err := blocked.AddDependency(blocker.ID); err != nil {
			t.Fatal(err)
		}
		m := cockpitModel([]boardRow{cockpitBoardRow(blocked, project)}, nil, nil, map[string]bool{blocked.ID: true}, OpenLoops{TodayTotal: 1, HasTasks: true}, boardTabToday)

		if got := m.resolveCockpitActionForKey("x").Command; got != CockpitCommandDismissTask {
			t.Fatalf("x on dependency-blocked task = %s, want %s", got, CockpitCommandDismissTask)
		}
		if got := m.resolveCockpitActionForKey("u").Command; got != CockpitCommandNone {
			t.Fatalf("u on dependency-blocked task = %s, want no local unblock", got)
		}
	})

	t.Run("u unblocks resolved blocked row", func(t *testing.T) {
		project := cockpitProject(t, "Unblock")
		blocked := cockpitTask(t, project, "No longer blocked", models.StatusBlocked, models.SectionNext)
		m := cockpitModel([]boardRow{cockpitBoardRow(blocked, project)}, nil, nil, map[string]bool{blocked.ID: true}, OpenLoops{TodayTotal: 1, HasTasks: true}, boardTabToday)

		if got := m.resolveCockpitActionForKey("u").Command; got != CockpitCommandUnblockTask {
			t.Fatalf("u on resolved blocked task = %s, want %s", got, CockpitCommandUnblockTask)
		}
	})
}

func cockpitModelForCommandExecution(t *testing.T, command CockpitCommand) *FocusBoardModel {
	t.Helper()
	switch command {
	case CockpitCommandDismissTask:
		project := cockpitProject(t, "Dismiss Command")
		task := cockpitTask(t, project, "Blocked elsewhere", models.StatusBlocked, models.SectionNext)
		task.DependsOn = []string{"external-project:blocked-task"}
		return cockpitModel(
			[]boardRow{cockpitBoardRow(task, project)},
			nil,
			nil,
			map[string]bool{task.ID: true},
			OpenLoops{TodayTotal: 1, HasTasks: true},
			boardTabToday,
		)
	case CockpitCommandUnblockTask:
		m, _, _ := newFlowLoopFocusBoard(t, models.StatusBlocked)
		return m
	default:
		m, _, _ := newFlowLoopFocusBoard(t, models.StatusTodo)
		return m
	}
}
