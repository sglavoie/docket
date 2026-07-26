package tui

import (
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/focus"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

type invariantQueueView struct {
	BaseView
	taskID  string
	project *models.Project
}

func (v *invariantQueueView) Init() tea.Cmd { return nil }

func (v *invariantQueueView) Update(tea.Msg) (tea.Model, tea.Cmd) {
	return v, nil
}

func (v *invariantQueueView) View() tea.View { return tea.NewView("") }

// getTaskRef re-resolves the task from the project so it returns the live
// pointer after a ledger-backed queue toggle reloads the project from disk.
func (v *invariantQueueView) getTaskRef() (*models.Task, *models.Project) {
	if v.project != nil {
		if fresh := v.project.GetTaskByID(v.taskID); fresh != nil {
			return fresh, v.project
		}
	}
	return nil, v.project
}

func TestTUIStateMachine_OutlinerEscOwnershipIsDeterministic(t *testing.T) {
	app, project := newTestAppModelWithProject()
	outliner := NewOutlinerModel(project, app.configManager, app.runtime)
	outliner.rebuildRows()
	outliner.multiSelectMode = true
	outliner.SelectTaskIdentity(taskIdentityFor(project.Tasks[0], project))
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
	// The Planner (ViewFocusBoard) is the sole planning surface after the Focus
	// View retirement. On the Planner the global q/w/u/ctrl+r keys are owned by
	// the Today simple grammar (currentViewUsesTodaySimpleGrammar), so this test
	// drives the app-level toggle + undo/redo methods directly — the exact code
	// the Planner's key handling routes into via handleUndoToggle/handleRedoToggle.
	t.Run("queue to daily plan", func(t *testing.T) {
		app, task := newFocusToggleTestApp(t)

		_ = app.toggleQueue()
		assertExclusiveMembership(t, app, task.ID, true, false)

		_ = app.toggleDailyPlan()
		assertExclusiveMembership(t, app, task.ID, false, true)

		_ = app.handleUndoToggle()
		assertExclusiveMembership(t, app, task.ID, true, false)

		_ = app.handleRedoToggle()
		assertExclusiveMembership(t, app, task.ID, false, true)
	})

	t.Run("daily plan to queue", func(t *testing.T) {
		app, task := newFocusToggleTestApp(t)

		_ = app.toggleDailyPlan()
		assertExclusiveMembership(t, app, task.ID, false, true)

		_ = app.toggleQueue()
		assertExclusiveMembership(t, app, task.ID, true, false)

		_ = app.handleUndoToggle()
		assertExclusiveMembership(t, app, task.ID, false, true)

		_ = app.handleRedoToggle()
		assertExclusiveMembership(t, app, task.ID, true, false)
	})
}

// TestNextMembershipDerivesFromWorkflowState locks the FR-007-003 invariant:
// Next (queue) membership is Task.State() == StateNext, not the persisted Focus
// Queue order index. An unindexed StateNext task behaves as a member (q leaves
// Next, adding it to Today removes it from Next), a stale indexed non-Next task
// does not, and the ranking signal follows the state rather than the list.
func TestNextMembershipDerivesFromWorkflowState(t *testing.T) {
	t.Run("q on unindexed StateNext leaves Next", func(t *testing.T) {
		app, task := newFocusToggleTestAppState(t, models.StateNext, ViewOutliner)
		identity := taskIdentityFor(task, app.currentProject)
		if app.focusQueueIDs[identity] {
			t.Fatal("precondition: StateNext task must start unindexed in the order list")
		}

		_ = app.toggleQueue()

		fresh := app.currentProject.GetTaskByID(task.ID)
		if fresh == nil {
			t.Fatal("task missing after toggle")
		}
		if fresh.State() == models.StateNext {
			t.Fatalf("q on an unindexed StateNext task must leave Next, still %v", fresh.State())
		}
		if app.focusQueueIDs[identity] {
			t.Fatal("removed task must not retain an order-index entry")
		}
	})

	t.Run("adding unindexed StateNext task to Today removes it from Next", func(t *testing.T) {
		app, task := newFocusToggleTestAppState(t, models.StateNext, ViewOutliner)
		identity := taskIdentityFor(task, app.currentProject)
		if app.focusQueueIDs[identity] {
			t.Fatal("precondition: StateNext task must start unindexed in the order list")
		}

		_ = app.toggleDailyPlan()

		if !app.dailyPlanIDs[identity] {
			t.Fatal("task should be planned for Today")
		}
		fresh := app.currentProject.GetTaskByID(task.ID)
		if fresh == nil {
			t.Fatal("task missing after toggle")
		}
		if fresh.State() == models.StateNext {
			t.Fatalf("Today and Next are exclusive: task must leave Next, still %v", fresh.State())
		}
		if app.focusQueueIDs[identity] {
			t.Fatal("task must not retain an order-index entry after moving to Today")
		}
	})

	t.Run("ranking signal follows state not the persisted list", func(t *testing.T) {
		project, err := models.NewProject("rank-proj", "rank", "")
		if err != nil {
			t.Fatalf("NewProject: %v", err)
		}
		nextTask := seedTask(t, 1, "On deck", models.StateNext)
		staleTask := seedTask(t, 2, "Left Next", models.StateReady)
		project.Tasks = []*models.Task{nextTask, staleTask}
		project.BuildTaskTree()

		// The persisted Focus Queue still lists the non-Next task (a stale index
		// entry). Membership must ignore it and follow the workflow state.
		domains := config.NewDomains()
		domains.Planning.AddToFocusQueue(staleTask.ID, project.ID)
		cfg := config.NewRuntimeState("", domains)

		ranked := focus.RankCrossProjectTasks([]focus.FocusTask{
			{Task: nextTask, Project: project},
			{Task: staleTask, Project: project},
		}, nil, cfg)

		reasons := make(map[string]string, len(ranked))
		for _, sft := range ranked {
			reasons[sft.Task.ID] = sft.Reason
		}
		if !strings.Contains(reasons[nextTask.ID], "focus queue") {
			t.Fatalf("unindexed StateNext task should carry the Next signal, reason=%q", reasons[nextTask.ID])
		}
		if strings.Contains(reasons[staleTask.ID], "focus queue") {
			t.Fatalf("stale indexed non-Next task must not carry the Next signal, reason=%q", reasons[staleTask.ID])
		}
	})
}

// TestNextMembershipConsumersUseWorkflowState locks FR-008-002 for the live TUI
// readers: Project Focus membership, the Pipeline "Plan" count, and the Priority
// urgency signal all derive Next membership from Task.State()==StateNext rather
// than the Focus Queue order index. An unindexed StateNext task counts as a
// member; a stale indexed non-Next task does not.
func TestNextMembershipConsumersUseWorkflowState(t *testing.T) {
	project, err := models.NewProject("next-consumers", "next", "")
	if err != nil {
		t.Fatalf("NewProject: %v", err)
	}
	indexedNext := seedTask(t, 1, "Indexed on deck", models.StateNext)
	unindexedNext := seedTask(t, 2, "Unindexed on deck", models.StateNext)
	stale := seedTask(t, 3, "Left Next", models.StateReady)
	project.Tasks = []*models.Task{indexedNext, unindexedNext, stale}
	project.BuildTaskTree()

	// The persisted order index lists the indexed StateNext task and the stale
	// non-Next task. Membership must ignore the stale entry and include the
	// unindexed StateNext task absent from the list.
	domains := config.NewDomains()
	domains.Planning.AddToFocusQueue(indexedNext.ID, project.ID)
	domains.Planning.AddToFocusQueue(stale.ID, project.ID)
	runtime := config.NewRuntimeState("", domains)
	setTestProjects(runtime, []*models.Project{project})

	t.Run("outliner Next includes unindexed StateNext and excludes stale index", func(t *testing.T) {
		outliner := NewOutlinerModel(project, nil, runtime)
		outliner.ApplyOpenOptions(OutlinerOpenOptions{Focus: OutlinerNext, SetFocus: true})
		filter := outliner.baseFocusFilter(project)
		if !filter(indexedNext) || !filter(unindexedNext) {
			t.Fatal("Outliner Next must include every StateNext task")
		}
		if filter(stale) {
			t.Fatal("stale indexed non-Next task must not appear in Outliner Next")
		}
	})

	t.Run("pipeline plan count is the StateNext count", func(t *testing.T) {
		var countNext func(task *models.Task) int
		countNext = func(task *models.Task) int {
			n := 0
			if task.State() == models.StateNext {
				n++
			}
			for _, child := range task.Children {
				n += countNext(child)
			}
			return n
		}
		total := 0
		for _, task := range project.GetRootTasks() {
			total += countNext(task)
		}
		if total != 2 {
			t.Fatalf("countNext total = %d, want 2 (both StateNext tasks, stale index ignored)", total)
		}
	})

	t.Run("priority urgency signal follows state not the order index", func(t *testing.T) {
		if !taskIsNext(unindexedNext) {
			t.Fatal("unindexed StateNext task must be a Next member")
		}
		if taskIsNext(stale) {
			t.Fatal("stale indexed non-Next task must not be a Next member")
		}
		// The urgency bonus is driven by taskIsNext, so the unindexed StateNext
		// task carries the +200 Next bonus while the stale indexed task does not.
		withNext := ComputeUrgencyScore(unindexedNext, taskIsNext(unindexedNext))
		withoutNext := ComputeUrgencyScore(unindexedNext, false)
		if withNext-withoutNext != 200 {
			t.Fatalf("Next bonus = %d, want 200", withNext-withoutNext)
		}
		if got := ComputeUrgencyScore(stale, taskIsNext(stale)); got != ComputeUrgencyScore(stale, false) {
			t.Fatal("stale indexed non-Next task must not receive the Next urgency bonus")
		}
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

// TestTUIStateMachine_NonUndoableStatusInvalidatesUndoCallback ensures that
// a plain success/info status replacing an undoable toast also invalidates
// the stored undo callback. Otherwise pressing `u` while a non-undoable
// message is on screen would silently fire the previous destructive
// action's undo.
func TestTUIStateMachine_NonUndoableStatusInvalidatesUndoCallback(t *testing.T) {
	app := newTestAppModel()
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
		outliner := NewOutlinerModel(project, app.configManager, app.runtime)
		outliner.rebuildRows()
		outliner.multiSelectMode = true
		outliner.selectAllVisible()

		if got := outliner.SelectedCount(); got != 2 {
			t.Fatalf("selected count = %d, want 2", got)
		}

		outliner.filter.setStateFilter(testWorkflowStateSet(models.StateNext), true)
		outliner.rebuildRows()

		assertSelectedOutlinerTasksVisible(t, outliner, "next-task")
	})

	t.Run("cross project", func(t *testing.T) {
		app := newTestAppModel()
		outliner := NewOutlinerModel(nil, app.configManager, app.runtime)
		outliner.multiSelectMode = true
		outliner.loadedProjects = []*models.Project{project}
		outliner.crossForceRebuild = true
		outliner.crossProjectRebuildRows()
		outliner.selectAllVisible()

		if got := outliner.SelectedCount(); got != 2 {
			t.Fatalf("selected count = %d, want 2", got)
		}

		outliner.filter.setStateFilter(testWorkflowStateSet(models.StateNext), true)
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
	return newFocusToggleTestAppState(t, models.StateReady, ViewFocusBoard)
}

// newFocusToggleTestAppState builds a ledger-backed AppModel whose single task
// starts in the given workflow state, hosted under hostView (so key-flow tests
// can drive q/w/u/ctrl+r from a real non-Planner task view). The hosted view is
// a QueueToggleable that re-resolves the live task after each reload.
func newFocusToggleTestAppState(t *testing.T, state models.WorkflowState, hostView PresentationID) (*AppModel, *models.Task) {
	t.Helper()

	dataDir := t.TempDir()
	task := seedTask(t, 1, "Toggle target", state)

	project, err := models.NewProject("focus-toggle-proj", "focus-toggle", "")
	if err != nil {
		t.Fatalf("NewProject: %v", err)
	}
	project.Slug = "focus-toggle"
	project.Path = filepath.Join(dataDir, "projects", "focus-toggle")
	project.Tasks = []*models.Task{task}
	project.NextShortID = 2
	project.EnsureDefaultSections()
	project.BuildTaskTree()

	ledger := storage.NewWorkspaceLedger(filepath.Join(dataDir, ".tt", "undo"), testWorkspaceLedgerDepth, []string{dataDir})
	commitSvc := storage.NewCommitServiceWithLedger(filepath.Join(dataDir, ".tt", "transactions"), []string{dataDir}, ledger)
	repo := newScenarioRepo(commitSvc, project)
	repo.seed(t)
	svc := app.NewService(repo, nil, nil)

	cfg := newTestConfigAt(dataDir)
	setTestProjects(cfg, []*models.Project{project})
	cfg.Machine().CurrentProject = project.ID
	manager := config.NewManager(filepath.Join(dataDir, "tt.json"))
	runtime := testRuntimeAt(cfg.DataDir(), cfg)

	appModel := &AppModel{
		configManager:        manager,
		runtime:              runtime,
		deps:                 newShellDeps(runtime, svc, manager, ledger),
		projects:             map[string]*models.Project{project.ID: project},
		persister:            NewProjectService(),
		appService:           svc,
		outlinerToggleStates: make(map[string]*OutlinerToggleState),
		viewStack:            []PresentationID{ViewDashboard},
		width:                80,
		height:               24,
		views:                NewViewRegistry(),
		currentProject:       project,
		focusQueue:           []FocusTask{},
		focusQueueIDs:        make(map[TaskIdentity]bool),
		dailyPlan:            []FocusTask{},
		dailyPlanIDs:         make(map[TaskIdentity]bool),
	}
	appModel.showView(hostView, &invariantQueueView{
		taskID:  task.ID,
		project: project,
	})
	return appModel, task
}

func assertExclusiveMembership(t *testing.T, app *AppModel, taskID string, inQueue, inDaily bool) {
	t.Helper()

	identity := TaskIdentity{ProjectID: app.currentProject.ID, TaskID: taskID}
	if app.focusQueueIDs[identity] != inQueue {
		t.Fatalf("queue membership = %v, want %v", app.focusQueueIDs[identity], inQueue)
	}
	if app.dailyPlanIDs[identity] != inDaily {
		t.Fatalf("daily-plan membership = %v, want %v", app.dailyPlanIDs[identity], inDaily)
	}
	if app.focusQueueIDs[identity] && app.dailyPlanIDs[identity] {
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

	visible := make(map[TaskIdentity]bool)
	for _, row := range m.rows {
		if row.IsSectionHeader || row.IsSeparator || row.task == nil {
			continue
		}
		visible[m.taskIdentityForRow(row)] = true
	}

	if got := m.SelectedCount(); got != len(wantIDs) {
		t.Fatalf("selected task count = %d, want %d", got, len(wantIDs))
	}
	for _, wantID := range wantIDs {
		found := false
		for _, row := range m.rows {
			if row.task != nil && row.task.ID == wantID && m.isRowSelected(row) {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("expected selected task %q, got %v", wantID, m.selectedTaskIdentities)
		}
	}
	for identity := range m.selectedTaskIdentities {
		if !visible[identity] {
			t.Fatalf("selected task %v is no longer visible", identity)
		}
	}
}
