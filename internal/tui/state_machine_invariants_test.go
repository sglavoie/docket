package tui

import (
	"path/filepath"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/coordinator"
	"github.com/sglavoie/tt/internal/tui/listsurface"
	"github.com/sglavoie/tt/internal/tui/operation"
	"github.com/sglavoie/tt/internal/tui/outliner"
	"github.com/sglavoie/tt/internal/tui/presentation"
	"github.com/sglavoie/tt/internal/tui/taskref"
)

type invariantQueueView struct {
	listsurface.BaseView
	taskID  string
	project *models.Project
}

func (v *invariantQueueView) Init() tea.Cmd { return nil }

func (v *invariantQueueView) Update(tea.Msg) (tea.Model, tea.Cmd) {
	return v, nil
}

func (v *invariantQueueView) View() tea.View { return tea.NewView("") }

func (v *invariantQueueView) GetTaskRef() (*models.Task, *models.Project) {
	if v.project != nil {
		if fresh := v.project.GetTaskByID(v.taskID); fresh != nil {
			return fresh, v.project
		}
	}
	return nil, v.project
}

func TestTUIStateMachine_OutlinerEscOwnershipIsDeterministic(t *testing.T) {
	app, project := newTestAppModelWithProject()
	outlinerView := outliner.NewOutlinerModel(project, app.configManager, app.runtime)
	outlinerView.RebuildRows()
	outlinerView.MultiSelectMode = true
	outlinerView.SelectTaskIdentity(taskref.For(project.Tasks[0], project))
	outlinerView.Focus.Stack = []outliner.FocusEntry{{TaskID: project.Tasks[0].ID, CursorIndex: 0}}
	outlinerView.SetSearchQuery(project.Tasks[0].Title)
	app.showView(presentation.ViewCrossProjectOutliner, outlinerView)
	wireInvariantSimplifiedOutliner(t, app, outlinerView)

	app = sendAppKey(app, syntheticSpecialKey(tea.KeyEsc))
	if app.currentView() != presentation.ViewCrossProjectOutliner {
		t.Fatalf("multi-select esc should stay in outliner, got %v", app.currentView())
	}
	if outlinerView.MultiSelectMode {
		t.Fatal("first esc should exit multi-select mode")
	}
	if outlinerView.SearchQuery() == "" {
		t.Fatal("first esc should not clear search query")
	}
	if len(outlinerView.Focus.Stack) != 1 {
		t.Fatalf("first esc should not change focus stack, got %d entries", len(outlinerView.Focus.Stack))
	}

	app = sendAppKey(app, syntheticSpecialKey(tea.KeyEsc))
	if app.currentView() != presentation.ViewCrossProjectOutliner {
		t.Fatalf("search esc should stay in outliner, got %v", app.currentView())
	}
	if outlinerView.SearchQuery() != "" {
		t.Fatalf("second esc should clear search query, got %q", outlinerView.SearchQuery())
	}
	if len(outlinerView.Focus.Stack) != 1 {
		t.Fatalf("second esc should not change focus stack, got %d entries", len(outlinerView.Focus.Stack))
	}

	app = sendAppKey(app, syntheticSpecialKey(tea.KeyEsc))
	if app.currentView() != presentation.ViewCrossProjectOutliner {
		t.Fatalf("focus esc should stay in outliner, got %v", app.currentView())
	}
	if len(outlinerView.Focus.Stack) != 0 {
		t.Fatalf("third esc should clear focus stack, got %d entries", len(outlinerView.Focus.Stack))
	}
	if outlinerView.ShouldCaptureEsc() {
		t.Fatal("outliner should release esc ownership once internal layers are cleared")
	}

	app = sendAppKey(app, syntheticSpecialKey(tea.KeyEsc))
	if app.currentView() != presentation.ViewCrossProjectOutliner {
		t.Fatalf("fourth esc at the Outliner root should be a no-op, got %v", app.currentView())
	}
}

func TestTUIStateMachine_QueueAndTodayStayExclusive(t *testing.T) {
	// The Outliner is the sole planning surface after the Focus
	// View retirement. On the Planner the global q/w/u/ctrl+r keys are owned by
	// the Today simple grammar (currentViewUsesTodaySimpleGrammar), so this test
	// drives the app-level toggle + undo/redo methods directly — the exact code
	// the Planner's key handling routes into via handleUndoToggle/handleRedoToggle.
	t.Run("queue to daily plan", func(t *testing.T) {
		app, task := newFocusToggleTestApp(t)

		_ = app.toggleQueue(nil)
		assertExclusiveMembership(t, app, task.ID, true, false)

		_ = app.toggleToday(nil)
		assertExclusiveMembership(t, app, task.ID, false, true)

		_ = app.handleUndoToggle()
		assertExclusiveMembership(t, app, task.ID, true, false)

		_ = app.handleRedoToggle()
		assertExclusiveMembership(t, app, task.ID, false, true)
	})

	t.Run("daily plan to queue", func(t *testing.T) {
		app, task := newFocusToggleTestApp(t)

		_ = app.toggleToday(nil)
		assertExclusiveMembership(t, app, task.ID, false, true)

		_ = app.toggleQueue(nil)
		assertExclusiveMembership(t, app, task.ID, true, false)

		_ = app.handleUndoToggle()
		assertExclusiveMembership(t, app, task.ID, false, true)

		_ = app.handleRedoToggle()
		assertExclusiveMembership(t, app, task.ID, true, false)
	})
}

// TestNextMembershipIsExplicitPlanningState locks the schema-6 invariant that
// queue membership is independent from the Backlog lifecycle state.
func TestNextMembershipIsExplicitPlanningState(t *testing.T) {
	t.Run("q toggles explicit membership without inventing a lifecycle state", func(t *testing.T) {
		app, task := newFocusToggleTestAppState(t)
		identity := taskref.For(task, app.currentProject)

		_ = app.toggleQueue(nil)
		if !app.nextOrderIDs[identity] {
			t.Fatal("q did not add explicit Next membership")
		}
		if fresh := app.currentProject.GetTaskByID(task.ID); fresh == nil || fresh.State() != models.StateBacklog {
			t.Fatalf("Next add changed Backlog lifecycle: %v", fresh)
		}

		_ = app.toggleQueue(nil)
		if app.nextOrderIDs[identity] {
			t.Fatal("second q did not remove explicit Next membership")
		}
	})

	t.Run("Today and Next memberships remain exclusive", func(t *testing.T) {
		app, task := newFocusToggleTestAppState(t)
		identity := taskref.For(task, app.currentProject)
		_ = app.toggleQueue(nil)
		_ = app.toggleToday(nil)
		if !app.todayIDs[identity] || app.nextOrderIDs[identity] {
			t.Fatalf("planning memberships after Today add: today=%v next=%v", app.todayIDs[identity], app.nextOrderIDs[identity])
		}
	})
}

// TestNextMembershipConsumersUseExplicitPlanning verifies that live readers use
// the ordered planning collection, exclude unindexed Backlog tasks, and ignore
// stale terminal entries.
func TestNextMembershipConsumersUseExplicitPlanning(t *testing.T) {
	project, err := models.NewProject("next-consumers", "next", "")
	if err != nil {
		t.Fatalf("NewProject: %v", err)
	}
	indexedNext := seedTask(t, 1, "Indexed on deck", models.StateBacklog)
	unindexedNext := seedTask(t, 2, "Unindexed Backlog", models.StateBacklog)
	stale := seedTask(t, 3, "Completed stale entry", models.StateDone)
	project.Tasks = []*models.Task{indexedNext, unindexedNext, stale}
	project.BuildTaskTree()

	// The persisted list names one eligible member and one stale terminal task.
	domains := config.NewDomains()
	domains.Planning.AddToNext(indexedNext.ID, project.ID)
	domains.Planning.AddToNext(stale.ID, project.ID)
	runtime := config.NewRuntimeState("", domains)
	setTestProjects(runtime, []*models.Project{project})

	t.Run("outliner Next follows explicit eligible membership", func(t *testing.T) {
		outlinerView := outliner.NewOutlinerModel(project, nil, runtime)
		outlinerView.ApplyOpenOptions(outliner.OutlinerOpenOptions{Mode: chrome.ModeNext, SetMode: true})
		visible := visibleOutlinerTaskIDs(outlinerView)
		if !visible[indexedNext.ID] {
			t.Fatal("Outliner Next omitted an explicit eligible member")
		}
		if visible[unindexedNext.ID] || visible[stale.ID] {
			t.Fatalf("Outliner Next included nonmember/stale task: %v", visible)
		}
	})
}

func TestTUIStateMachine_LatestUndoToastWins(t *testing.T) {
	app := newTestAppModel()
	wireInvariantSimplifiedRoot(t, app)
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

// TestTUIStateMachine_NonUndoableStatusInvalidatesUndoCallback ensures that
// a plain success/info status replacing an undoable toast also invalidates
// the stored undo callback. Otherwise pressing `u` while a non-undoable
// message is on screen would silently fire the previous destructive
// action's undo.
func TestTUIStateMachine_NonUndoableStatusInvalidatesUndoCallback(t *testing.T) {
	app := newTestAppModel()
	wireInvariantSimplifiedRoot(t, app)
	called := false

	_ = app.setUndoableStatus("destructive · u to undo", func() tea.Cmd {
		called = true
		return nil
	})

	// A non-undoable status arrives (e.g. tab switch, refresh).
	app.setSuccessStatus("saved")

	app = sendAppKey(app, syntheticKey('u'))

	if called {
		t.Fatal("pressing u after a non-undoable status replaced the toast should not invoke the previous undo callback")
	}
	if app.statusUndoCb != nil {
		t.Fatal("non-undoable status should clear statusUndoCb")
	}
}

func wireInvariantSimplifiedRoot(t *testing.T, app *AppModel) {
	t.Helper()
	outlinerView := outliner.NewOutlinerModel(nil, app.configManager, app.runtime)
	app.crossOutliner = outlinerView
	app.viewNav.views.Register(presentation.ViewCrossProjectOutliner, outlinerView)
	wireInvariantSimplifiedOutliner(t, app, outlinerView)
}

func wireInvariantSimplifiedOutliner(t *testing.T, app *AppModel, outlinerView *outliner.OutlinerModel) {
	t.Helper()
	app.operationRegistry = operation.NewOperationRegistry()
	app.operationExecutor = operation.NewOperationExecutor(app.operationRegistry)
	app.crossOutliner = outlinerView
	app.viewNav.views.Register(presentation.ViewCrossProjectOutliner, outlinerView)
	if err := app.registerCoreInputOperations(); err != nil {
		t.Fatalf("register core operations: %v", err)
	}
	if err := app.registerTaskActionOperations(); err != nil {
		t.Fatalf("register task action operations: %v", err)
	}
	if err := app.registerFilterOperations(); err != nil {
		t.Fatalf("register filter operations: %v", err)
	}
	if err := app.registerSortOperations(); err != nil {
		t.Fatalf("register sort operations: %v", err)
	}
	if err := app.registerProjectOperations(); err != nil {
		t.Fatalf("register project operations: %v", err)
	}
	if err := app.registerProjectActionSheetOperation(); err != nil {
		t.Fatalf("register project action sheet operation: %v", err)
	}
	if err := app.registerChecklistRowOperations(); err != nil {
		t.Fatalf("register checklist row operations: %v", err)
	}
	if err := app.operationRegistry.RegisterInputStrategy(SimplifiedInputStrategy()); err != nil {
		t.Fatalf("register simplified input strategy: %v", err)
	}
	app.wireOutlinerOperationExecutor(outlinerView)
}

func TestTUIStateMachine_OutlinerSelectionSurvivesVisibilityRebuilds(t *testing.T) {
	project := &models.Project{
		ID:   "selection-project",
		Name: "Selection Project",
		Tasks: []*models.Task{
			makeTask("next-task", "Next task", models.StateBacklog),
			makeTask("waiting-task", "Waiting task", models.StateWaiting),
		},
	}
	project.BuildTaskTree()

	t.Run("single project", func(t *testing.T) {
		app := newTestAppModel()
		outlinerView := outliner.NewOutlinerModel(project, app.configManager, app.runtime)
		outlinerView.RebuildRows()
		outlinerView.MultiSelectMode = true
		outlinerView.SelectAllVisible()

		if got := outlinerView.SelectedCount(); got != 2 {
			t.Fatalf("selected count = %d, want 2", got)
		}

		outlinerView.Filter.SetStateFilter(testWorkflowStateSet(models.StateBacklog))
		outlinerView.AdoptFilterCarriers()
		outlinerView.RebuildRows()

		if !outlinerView.MultiSelectMode || outlinerView.SelectedCount() != 2 {
			t.Fatalf("single-project selection after filter = active %v count %d, want active/2", outlinerView.MultiSelectMode, outlinerView.SelectedCount())
		}
	})

	t.Run("cross project", func(t *testing.T) {
		app := newTestAppModel()
		outlinerView := outliner.NewOutlinerModel(nil, app.configManager, app.runtime)
		outlinerView.MultiSelectMode = true
		outlinerView.Population.Projects = []*models.Project{project}
		outlinerView.Population.Invalidate()
		outlinerView.CrossProjectRebuildRows()
		outlinerView.SelectAllVisible()

		if got := outlinerView.SelectedCount(); got != 2 {
			t.Fatalf("selected count = %d, want 2", got)
		}

		outlinerView.Filter.SetStateFilter(testWorkflowStateSet(models.StateBacklog))
		outlinerView.AdoptFilterCarriers()
		outlinerView.Population.Invalidate()
		outlinerView.CrossProjectRebuildRows()

		if !outlinerView.MultiSelectMode || outlinerView.SelectedCount() != 2 {
			t.Fatalf("cross-project selection after filter = active %v count %d, want active/2", outlinerView.MultiSelectMode, outlinerView.SelectedCount())
		}
	})
}

func sendAppKey(app *AppModel, key tea.KeyPressMsg) *AppModel {
	model, _ := app.Update(key)
	return model.(*AppModel)
}

func newFocusToggleTestApp(t *testing.T) (*AppModel, *models.Task) {
	t.Helper()
	return newFocusToggleTestAppState(t)
}

// newFocusToggleTestAppState builds a ledger-backed AppModel whose single task
// starts in the given workflow state under the canonical Outliner root.
func newFocusToggleTestAppState(t *testing.T) (*AppModel, *models.Task) {
	t.Helper()

	dataDir := t.TempDir()
	task := seedTask(t, 1, "Toggle target", models.StateBacklog)

	project, err := models.NewProject("focus-toggle-proj", "focus-toggle", "")
	if err != nil {
		t.Fatalf("NewProject: %v", err)
	}
	project.Slug = "focus-toggle"
	project.Path = filepath.Join(dataDir, "projects", "focus-toggle")
	project.Tasks = []*models.Task{task}
	project.NextShortID = 2
	project.BuildTaskTree()

	ledger := storage.NewWorkspaceLedger(filepath.Join(dataDir, ".tt", "undo"), []string{dataDir})
	commitSvc := storage.NewCommitServiceWithLedger(filepath.Join(dataDir, ".tt", "transactions"), []string{dataDir}, ledger)
	repo := newScenarioRepo(commitSvc, project)
	repo.seed(t)
	svc := app.NewServiceWithArtifacts(repo, nil, nil, nil)

	cfg := newTestConfigAt(dataDir)
	setTestProjects(cfg, []*models.Project{project})
	manager := config.NewManager(filepath.Join(dataDir, "tt.json"))
	runtime := testRuntimeAt(cfg.DataDir(), cfg)

	appModel := &AppModel{
		configManager:     manager,
		runtime:           runtime,
		deps:              newShellDeps(ledger),
		projects:          map[string]*models.Project{project.ID: project},
		persister:         coordinator.NewProjectService(),
		appService:        svc,
		viewNav:           newViewNavigationController(),
		currentProject:    project,
		nextOrder:         []outliner.PlanningTask{},
		nextOrderIDs:      make(map[taskref.TaskIdentity]bool),
		today:             []outliner.PlanningTask{},
		todayIDs:          make(map[taskref.TaskIdentity]bool),
		operationRegistry: operation.NewOperationRegistry(),
	}
	appModel.operationExecutor = operation.NewOperationExecutor(appModel.operationRegistry)
	outlinerView := outliner.NewOutlinerModel(project, manager, runtime)
	outlinerView.RebuildRows()
	outlinerView.MoveCursorToTask(task.ID)
	appModel.crossOutliner = outlinerView
	appModel.viewNav.views.Register(presentation.ViewCrossProjectOutliner, outlinerView)
	if err := appModel.registerCoreInputOperations(); err != nil {
		t.Fatalf("register core operations: %v", err)
	}
	if err := appModel.registerTaskActionOperations(); err != nil {
		t.Fatalf("register task action operations: %v", err)
	}
	if err := appModel.registerFilterOperations(); err != nil {
		t.Fatalf("register filter operations: %v", err)
	}
	if err := appModel.registerSortOperations(); err != nil {
		t.Fatalf("register sort operations: %v", err)
	}
	if err := appModel.registerProjectOperations(); err != nil {
		t.Fatalf("register project operations: %v", err)
	}
	if err := appModel.registerProjectActionSheetOperation(); err != nil {
		t.Fatalf("register project action sheet operation: %v", err)
	}
	if err := appModel.registerChecklistRowOperations(); err != nil {
		t.Fatalf("register checklist row operations: %v", err)
	}
	if err := appModel.operationRegistry.RegisterInputStrategy(SimplifiedInputStrategy()); err != nil {
		t.Fatalf("register simplified input: %v", err)
	}
	appModel.wireOutlinerOperationExecutor(outlinerView)
	return appModel, task
}

func assertExclusiveMembership(t *testing.T, app *AppModel, taskID string, inQueue, inDaily bool) {
	t.Helper()

	identity := taskref.TaskIdentity{ProjectID: app.currentProject.ID, TaskID: taskID}
	if app.nextOrderIDs[identity] != inQueue {
		t.Fatalf("queue membership = %v, want %v", app.nextOrderIDs[identity], inQueue)
	}
	if app.todayIDs[identity] != inDaily {
		t.Fatalf("daily-plan membership = %v, want %v", app.todayIDs[identity], inDaily)
	}
	if app.nextOrderIDs[identity] && app.todayIDs[identity] {
		t.Fatal("task must never be in both queue and daily plan")
	}
	if got := taskMembershipCount(app.nextOrder, taskID); got != boolCount(inQueue) {
		t.Fatalf("queue slice membership count = %d, want %d", got, boolCount(inQueue))
	}
	if got := taskMembershipCount(app.today, taskID); got != boolCount(inDaily) {
		t.Fatalf("daily-plan slice membership count = %d, want %d", got, boolCount(inDaily))
	}
}

func taskMembershipCount(tasks []outliner.PlanningTask, taskID string) int {
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
