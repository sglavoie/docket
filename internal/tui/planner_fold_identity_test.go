package tui

import (
	"slices"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
)

func duplicatePlannerFoldFixture(t *testing.T) (*FocusBoardModel, TaskIdentity, TaskIdentity) {
	t.Helper()
	alpha := &models.Project{ID: "alpha", Name: "Alpha"}
	beta := &models.Project{ID: "beta", Name: "Beta"}
	alphaRoot := &models.Task{ID: "shared-id", Title: "Alpha root", WorkflowState: models.StateReady}
	betaRoot := &models.Task{ID: "shared-id", Title: "Beta root", WorkflowState: models.StateReady}
	alphaRoot.Children = []*models.Task{{ID: "alpha-child", Title: "Alpha child", WorkflowState: models.StateReady}}
	betaRoot.Children = []*models.Task{{ID: "beta-child", Title: "Beta child", WorkflowState: models.StateReady}}
	alpha.Tasks = []*models.Task{alphaRoot, alphaRoot.Children[0]}
	beta.Tasks = []*models.Task{betaRoot, betaRoot.Children[0]}

	alphaID := taskIdentityFor(alphaRoot, alpha)
	betaID := taskIdentityFor(betaRoot, beta)
	m := newTestFocusBoard()
	m.tabs.Active = boardTabToday
	m.childDepthLimit = -1
	m.dailyPlanIDs = map[TaskIdentity]bool{alphaID: true, betaID: true}
	m.todayRows = []boardRow{
		{Task: alphaRoot, Project: alpha, Source: alpha.Name},
		{Task: betaRoot, Project: beta, Source: beta.Name},
	}
	m.expandTodayChildren()
	m.applyFilter()
	m.rebuildPlannerDispatcher()
	return m, alphaID, betaID
}

func applyPlannerViewMenuItem(t *testing.T, m *FocusBoardModel, label string) {
	t.Helper()
	open := findSharedTaskKeymapAction(t, m.buildKeyMap(), "v")
	_, cmd := open()
	if cmd == nil {
		t.Fatal("opening Planner View menu returned nil command")
	}
	if msg, ok := cmd().(plannerViewMenuMsg); !ok || !msg.Open {
		t.Fatalf("opening Planner View menu emitted %#v", msg)
	}
	movePlannerMenuCursorToLabel(t, m, label)
	m.handlePlannerMenuKey(syntheticSpecialKey(tea.KeyEnter))
}

func TestPlannerViewMenuFoldsDuplicateTaskIDsByIdentity(t *testing.T) {
	m, alphaID, betaID := duplicatePlannerFoldFixture(t)
	m.cursor = 2 // beta root after alpha root + alpha child

	applyPlannerViewMenuItem(t, m, "Toggle selected subtree")
	if len(m.filtered) != 3 {
		t.Fatalf("collapsed row count = %d, want alpha root/child plus beta root", len(m.filtered))
	}
	if !m.foldOverrides[betaID] {
		t.Fatal("beta fold override was not recorded")
	}
	if _, exists := m.foldOverrides[alphaID]; exists {
		t.Fatal("beta fold changed the duplicate alpha task")
	}
	if got := taskIdentityFor(m.filtered[m.cursor].Task, m.filtered[m.cursor].Project); got != betaID {
		t.Fatalf("cursor identity = %#v, want beta root %#v", got, betaID)
	}

	applyPlannerViewMenuItem(t, m, "Toggle selected subtree")
	if len(m.filtered) != 4 {
		t.Fatalf("expanded row count = %d, want 4", len(m.filtered))
	}
}

func TestPlannerViewMenuDepthControlsResetOverridesAndStayTodayOnly(t *testing.T) {
	m, _, _ := duplicatePlannerFoldFixture(t)
	m.cursor = 0
	applyPlannerViewMenuItem(t, m, "Toggle selected subtree")
	if len(m.foldOverrides) != 1 {
		t.Fatalf("fold overrides = %d, want 1", len(m.foldOverrides))
	}

	applyPlannerViewMenuItem(t, m, "Tree depth: 1")
	if m.childDepthLimit != 0 || len(m.foldOverrides) != 0 || len(m.filtered) != 2 {
		t.Fatalf("depth 1 state = limit %d overrides %d rows %d", m.childDepthLimit, len(m.foldOverrides), len(m.filtered))
	}
	applyPlannerViewMenuItem(t, m, "Tree depth: all")
	if m.childDepthLimit != -1 || len(m.filtered) != 4 {
		t.Fatalf("all-depth state = limit %d rows %d", m.childDepthLimit, len(m.filtered))
	}

	m.tabs.Active = boardTabNext
	for _, item := range m.plannerMenuItems() {
		if item.label == "Toggle selected subtree" || item.label == "Tree depth: 1" {
			t.Fatalf("Today-only fold item leaked into Next: %q", item.label)
		}
	}
	for _, retired := range []string{"fa", "f1", "f2", "f3"} {
		if slices.Contains(m.HandledKeys(), retired) {
			t.Fatalf("retired Planner fold grammar %q was restored", retired)
		}
	}
}
