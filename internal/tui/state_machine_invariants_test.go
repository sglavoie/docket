package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
)

type invariantQueueView struct {
	BaseView
	task    *models.Task
	project *models.Project
}

func (v *invariantQueueView) Init() tea.Cmd { return nil }

func (v *invariantQueueView) Update(tea.Msg) (tea.Model, tea.Cmd) {
	return v, nil
}

func (v *invariantQueueView) View() tea.View { return tea.NewView("") }

func (v *invariantQueueView) getTaskRef() (*models.Task, *models.Project) {
	return v.task, v.project
}

func TestTUIStateMachine_ViewStackAnchorsAndNoUnderflow(t *testing.T) {
	t.Run("project anchor", func(t *testing.T) {
		app := newTestAppModel()
		app.viewStack = []ViewType{ViewDashboard, ViewProjectTree, ViewTaskDetail, ViewFocusBoard}

		steps := []struct {
			name        string
			wantHandled bool
			wantView    ViewType
			wantLen     int
		}{
			{name: "overlay pops to anchor", wantHandled: true, wantView: ViewProjectTree, wantLen: 2},
			{name: "anchor pops to dashboard", wantHandled: true, wantView: ViewDashboard, wantLen: 1},
			{name: "dashboard does not underflow", wantHandled: false, wantView: ViewDashboard, wantLen: 1},
		}

		for _, step := range steps {
			handled, _ := app.handleEscKey()
			if handled != step.wantHandled {
				t.Fatalf("%s: handled = %v, want %v", step.name, handled, step.wantHandled)
			}
			if got := app.currentView(); got != step.wantView {
				t.Fatalf("%s: currentView = %v, want %v", step.name, got, step.wantView)
			}
			if got := len(app.viewStack); got != step.wantLen {
				t.Fatalf("%s: stack len = %d, want %d", step.name, got, step.wantLen)
			}
		}
	})

	t.Run("cross-project anchor emits appear message", func(t *testing.T) {
		app := newTestAppModel()
		app.viewStack = []ViewType{ViewDashboard, ViewCrossProjectOutliner, ViewTaskDetail, ViewFocusBoard}

		handled, cmd := app.handleEscKey()
		if !handled {
			t.Fatal("expected esc to be consumed while closing the overlay")
		}
		if got := app.currentView(); got != ViewCrossProjectOutliner {
			t.Fatalf("currentView = %v, want %v", got, ViewCrossProjectOutliner)
		}
		if got := len(app.viewStack); got != 2 {
			t.Fatalf("stack len = %d, want 2", got)
		}
		if cmd == nil {
			t.Fatal("expected ViewDidAppearMsg command when revealing cross-project outliner")
		}
		if _, ok := cmd().(ViewDidAppearMsg); !ok {
			t.Fatalf("expected ViewDidAppearMsg, got %T", cmd())
		}

		handled, _ = app.handleEscKey()
		if !handled {
			t.Fatal("expected esc to pop cross-project outliner to dashboard")
		}
		if got := app.currentView(); got != ViewDashboard {
			t.Fatalf("currentView = %v, want %v", got, ViewDashboard)
		}
		if got := len(app.viewStack); got != 1 {
			t.Fatalf("stack len = %d, want 1", got)
		}

		handled, _ = app.handleEscKey()
		if handled {
			t.Fatal("dashboard esc should be a no-op")
		}
		if got := len(app.viewStack); got != 1 {
			t.Fatalf("stack len = %d, want 1", got)
		}
	})
}

func TestTUIStateMachine_OutlinerEscOwnershipIsDeterministic(t *testing.T) {
	app, project := newTestAppModelWithProject()
	outliner := NewOutlinerModel(project, app.config, app.configManager)
	outliner.rebuildRows()
	outliner.multiSelectMode = true
	outliner.selectedTasks[project.Tasks[0].ID] = true
	outliner.SetSearchQuery(project.Tasks[0].Title)
	outliner.zoomStack = []ZoomEntry{{TaskID: project.Tasks[0].ID, CursorIndex: 0}}
	app.showView(ViewOutliner, outliner)

	app = sendAppKey(app, syntheticSpecialKey(tea.KeyEsc))
	if app.currentView() != ViewOutliner {
		t.Fatalf("multi-select esc should stay in outliner, got %v", app.currentView())
	}
	if outliner.multiSelectMode {
		t.Fatal("first esc should exit multi-select mode")
	}
	if outliner.SearchQuery() == "" {
		t.Fatal("first esc should not clear search query")
	}
	if len(outliner.zoomStack) != 1 {
		t.Fatalf("first esc should not change zoom stack, got %d entries", len(outliner.zoomStack))
	}

	app = sendAppKey(app, syntheticSpecialKey(tea.KeyEsc))
	if app.currentView() != ViewOutliner {
		t.Fatalf("search esc should stay in outliner, got %v", app.currentView())
	}
	if outliner.SearchQuery() != "" {
		t.Fatalf("second esc should clear search query, got %q", outliner.SearchQuery())
	}
	if len(outliner.zoomStack) != 1 {
		t.Fatalf("second esc should not change zoom stack, got %d entries", len(outliner.zoomStack))
	}

	app = sendAppKey(app, syntheticSpecialKey(tea.KeyEsc))
	if app.currentView() != ViewOutliner {
		t.Fatalf("zoom esc should stay in outliner, got %v", app.currentView())
	}
	if len(outliner.zoomStack) != 0 {
		t.Fatalf("third esc should clear zoom stack, got %d entries", len(outliner.zoomStack))
	}
	if outliner.ShouldCaptureEsc() {
		t.Fatal("outliner should release esc ownership once internal layers are cleared")
	}

	app = sendAppKey(app, syntheticSpecialKey(tea.KeyEsc))
	if app.currentView() != ViewDashboard {
		t.Fatalf("fourth esc should pop the view, got %v", app.currentView())
	}
}

func TestTUIStateMachine_QueueAndDailyPlanStayExclusive(t *testing.T) {
	t.Run("queue to daily plan", func(t *testing.T) {
		app, task := newFocusToggleTestApp(t)

		app = sendAppKey(app, syntheticKey('q'))
		assertExclusiveMembership(t, app, task.ID, true, false)

		app = sendAppKey(app, syntheticKey('w'))
		assertExclusiveMembership(t, app, task.ID, false, true)

		app = sendAppKey(app, syntheticKey('u'))
		assertExclusiveMembership(t, app, task.ID, true, false)

		app = sendAppKey(app, syntheticCtrlKey('r'))
		assertExclusiveMembership(t, app, task.ID, false, true)
	})

	t.Run("daily plan to queue", func(t *testing.T) {
		app, task := newFocusToggleTestApp(t)

		app = sendAppKey(app, syntheticKey('w'))
		assertExclusiveMembership(t, app, task.ID, false, true)

		app = sendAppKey(app, syntheticKey('q'))
		assertExclusiveMembership(t, app, task.ID, true, false)

		app = sendAppKey(app, syntheticKey('u'))
		assertExclusiveMembership(t, app, task.ID, false, true)

		app = sendAppKey(app, syntheticCtrlKey('r'))
		assertExclusiveMembership(t, app, task.ID, true, false)
	})
}

func TestTUIStateMachine_LatestUndoToastWins(t *testing.T) {
	app := newTestAppModel()
	calls := make([]string, 0, 2)

	_ = app.setUndoableStatus("first", func() tea.Cmd {
		calls = append(calls, "first")
		return nil
	})
	_ = app.setUndoableStatus("second", func() tea.Cmd {
		calls = append(calls, "second")
		return nil
	})

	app = sendAppKey(app, syntheticKey('u'))

	if len(calls) != 1 || calls[0] != "second" {
		t.Fatalf("undo calls = %v, want [second]", calls)
	}
	if app.statusMessage != "" {
		t.Fatalf("undo toast should be cleared after inline undo, got %q", app.statusMessage)
	}
	if app.statusUndoCb != nil {
		t.Fatal("inline undo should clear the stored callback")
	}
}

func TestTUIStateMachine_OutlinerSelectionStaysScopedToVisibleTasks(t *testing.T) {
	project := &models.Project{
		ID:   "selection-project",
		Name: "Selection Project",
		Tasks: []*models.Task{
			makeTask("next-task", "Next task", models.SectionNext),
			makeTask("waiting-task", "Waiting task", models.SectionWaiting),
		},
	}
	project.BuildTaskTree()

	t.Run("single project", func(t *testing.T) {
		app := newTestAppModel()
		outliner := NewOutlinerModel(project, app.config, app.configManager)
		outliner.rebuildRows()
		outliner.multiSelectMode = true
		outliner.selectAllVisible()

		if got := outliner.SelectedCount(); got != 2 {
			t.Fatalf("selected count = %d, want 2", got)
		}

		outliner.filter.Sections = map[string]bool{models.SectionNext: true}
		outliner.rebuildRows()

		assertSelectedOutlinerTasksVisible(t, outliner, "next-task")
	})

	t.Run("cross project", func(t *testing.T) {
		app := newTestAppModel()
		outliner := NewOutlinerModel(nil, app.config, app.configManager)
		outliner.multiSelectMode = true
		outliner.loadedProjects = []*models.Project{project}
		outliner.crossForceRebuild = true
		outliner.crossProjectRebuildRows()
		outliner.selectAllVisible()

		if got := outliner.SelectedCount(); got != 2 {
			t.Fatalf("selected count = %d, want 2", got)
		}

		outliner.filter.Sections = map[string]bool{models.SectionNext: true}
		outliner.crossForceRebuild = true
		outliner.crossProjectRebuildRows()

		assertSelectedOutlinerTasksVisible(t, outliner, "next-task")
	})
}

func sendAppKey(app *AppModel, key tea.KeyPressMsg) *AppModel {
	model, _ := app.Update(key)
	return model.(*AppModel)
}

func newFocusToggleTestApp(t *testing.T) (*AppModel, *models.Task) {
	t.Helper()

	app, project := newTestAppModelWithProject()
	app.focusQueueIDs = make(map[string]bool)
	app.dailyPlanIDs = make(map[string]bool)
	app.showView(ViewFocusView, &invariantQueueView{
		task:    project.Tasks[0],
		project: project,
	})
	return app, project.Tasks[0]
}

func assertExclusiveMembership(t *testing.T, app *AppModel, taskID string, inQueue, inDaily bool) {
	t.Helper()

	if app.focusQueueIDs[taskID] != inQueue {
		t.Fatalf("queue membership = %v, want %v", app.focusQueueIDs[taskID], inQueue)
	}
	if app.dailyPlanIDs[taskID] != inDaily {
		t.Fatalf("daily-plan membership = %v, want %v", app.dailyPlanIDs[taskID], inDaily)
	}
	if app.focusQueueIDs[taskID] && app.dailyPlanIDs[taskID] {
		t.Fatal("task must never be in both queue and daily plan")
	}
	if got := taskMembershipCount(app.focusQueue, taskID); got != boolCount(inQueue) {
		t.Fatalf("queue slice membership count = %d, want %d", got, boolCount(inQueue))
	}
	if got := taskMembershipCount(app.dailyPlan, taskID); got != boolCount(inDaily) {
		t.Fatalf("daily-plan slice membership count = %d, want %d", got, boolCount(inDaily))
	}
}

func taskMembershipCount(tasks []FocusTask, taskID string) int {
	count := 0
	for _, ft := range tasks {
		if ft.Task != nil && ft.Task.ID == taskID {
			count++
		}
	}
	return count
}

func boolCount(v bool) int {
	if v {
		return 1
	}
	return 0
}

func assertSelectedOutlinerTasksVisible(t *testing.T, m *OutlinerModel, wantIDs ...string) {
	t.Helper()

	visibleIDs := make(map[string]bool)
	for _, row := range m.rows {
		if row.IsSectionHeader || row.IsSeparator || row.task == nil {
			continue
		}
		visibleIDs[row.task.ID] = true
	}

	if got := len(m.selectedTasks); got != len(wantIDs) {
		t.Fatalf("selected task count = %d, want %d", got, len(wantIDs))
	}
	for _, wantID := range wantIDs {
		if !m.selectedTasks[wantID] {
			t.Fatalf("expected selected task %q, got %v", wantID, m.selectedTasks)
		}
	}
	for id := range m.selectedTasks {
		if !visibleIDs[id] {
			t.Fatalf("selected task %q is no longer visible", id)
		}
	}
}
