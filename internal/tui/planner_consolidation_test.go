package tui

import (
	"slices"
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

func focusBoardTaskActionFixture(t *testing.T) (*FocusBoardModel, *models.Task, *models.Project) {
	t.Helper()
	project := cockpitProject(t, "Planner Task Actions")
	task := cockpitTask(t, project, "selected task", models.StatusTodo, models.SectionNext)
	row := cockpitBoardRow(task, project)
	m := newTestFocusBoard()
	m.todayRows = []boardRow{row}
	m.nextRows = []boardRow{row}
	m.backlogRows = []boardRow{row}
	m.allRows = []boardRow{row}
	m.tabs.Active = boardTabToday
	m.applyFilter()
	m.cursor = 0
	m.rebuildPlannerDispatcher()
	return m, task, project
}

func suggestedCommandNames(palette *CommandPaletteModel) []string {
	var names []string
	for _, item := range palette.filteredItems {
		if item.IsSuggested && item.Item.Command != nil {
			names = append(names, item.Item.Command.Name)
		}
	}
	return names
}

func TestPlannerExposesExactlyThreeTabs(t *testing.T) {
	if want := []string{"Today", "Next", "Backlog"}; !slices.Equal(focusBoardTabNames, want) {
		t.Fatalf("Planner tabs = %v, want %v", focusBoardTabNames, want)
	}

	m, _, _ := focusBoardTaskActionFixture(t)
	next := findSharedTaskKeymapAction(t, m.buildKeyMap(), "tab")
	_, cmd := next()
	msg, ok := cmd().(PlannerRouteChangedMsg)
	if !ok || msg.Tab != PlannerNext || m.tabs.Active != boardTabNext {
		t.Fatalf("tab result = %#v at board tab %d, want Planner Next", msg, m.tabs.Active)
	}

	previous := findSharedTaskKeymapAction(t, m.buildKeyMap(), "shift+tab")
	_, cmd = previous()
	msg, ok = cmd().(PlannerRouteChangedMsg)
	if !ok || msg.Tab != PlannerToday || m.tabs.Active != boardTabToday {
		t.Fatalf("shift+tab result = %#v at board tab %d, want Planner Today", msg, m.tabs.Active)
	}

	_, cmd = previous()
	msg, ok = cmd().(PlannerRouteChangedMsg)
	if !ok || msg.Tab != PlannerBacklog || m.tabs.Active != boardTabBacklog {
		t.Fatalf("shift+tab wrap result = %#v at board tab %d, want Planner Backlog", msg, m.tabs.Active)
	}
}

func TestPlannerReorderCapabilityOnlyExistsForPlanningHorizons(t *testing.T) {
	m, _, _ := focusBoardTaskActionFixture(t)
	for _, tc := range []struct {
		tab         int
		wantReorder bool
	}{
		{boardTabToday, true},
		{boardTabNext, true},
		{boardTabBacklog, false},
	} {
		m.tabs.Active = tc.tab
		m.applyFilter()
		keys := m.HandledKeys()
		for _, key := range []string{"J", "K"} {
			if got := slices.Contains(keys, key); got != tc.wantReorder {
				t.Fatalf("tab %q key %q present = %v, want %v", focusBoardTabNames[tc.tab], key, got, tc.wantReorder)
			}
		}
	}
}

func TestPlannerSelectionUsesStableTaskIdentity(t *testing.T) {
	m, task, project := focusBoardTaskActionFixture(t)
	selectAction := findSharedTaskKeymapAction(t, m.buildKeyMap(), "space")
	selectAction()

	id := TaskIdentity{ProjectID: project.ID, TaskID: task.ID}
	if _, ok := m.selection[id]; !ok {
		t.Fatalf("space did not select stable identity %+v", id)
	}
	if !m.Capabilities().Has(CapabilityPlannerSelection) {
		t.Fatal("selected Planner row did not expose the selection capability")
	}

	selectAction = findSharedTaskKeymapAction(t, m.buildKeyMap(), "space")
	selectAction()
	if len(m.selection) != 0 {
		t.Fatalf("second space left selection = %+v, want empty", m.selection)
	}
}

func TestCompletedWorkRemainsReachableOutsidePlanner(t *testing.T) {
	state, err := NewTaskWorkspaceState(TaskWorkspaceParams{
		Scope: AppEntityScope(), Layout: WorkspaceList, Focus: WorkspaceActive,
	})
	if err != nil {
		t.Fatalf("NewTaskWorkspaceState: %v", err)
	}
	var completed bool
	for _, item := range TaskWorkspaceViewMenu(state, TaskWorkspaceSnapshot{}, nil) {
		if item.ID == "focus:completed" && item.Label == "Completed" {
			completed = true
			break
		}
	}
	if !completed {
		t.Fatal("Tasks View menu does not expose Completed")
	}

	route, ok, err := routeForPresentation(ViewRecentActivity, "", "")
	if err != nil || !ok {
		t.Fatalf("Review Activity route: ok=%v err=%v", ok, err)
	}
	params, ok := route.Params().(ReviewParams)
	if !ok || route.ID() != ReviewHub || params.Module != ReviewActivity {
		t.Fatalf("Recent Activity route = %s %#v, want Review/Activity", route.ID(), route.Params())
	}
}
