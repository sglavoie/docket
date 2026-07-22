package tui

import (
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/testutil"
)

func duplicatePlanningProjects(t *testing.T) (*models.Project, *models.Project, *models.Task, *models.Task) {
	t.Helper()
	sharedID := uuid.NewString()
	newProjectTask := func(name, title string) (*models.Project, *models.Task) {
		project, err := models.NewProject(uuid.NewString(), name, "")
		if err != nil {
			t.Fatalf("NewProject(%s): %v", name, err)
		}
		task, err := models.NewTask(sharedID, 1, models.TaskTypeTask, title)
		if err != nil {
			t.Fatalf("NewTask(%s): %v", title, err)
		}
		task.WorkflowState = models.StateNext
		project.Tasks = []*models.Task{task}
		project.NextShortID = 2
		project.EnsureDefaultSections()
		project.BuildTaskTree()
		return project, task
	}
	alpha, alphaTask := newProjectTask("Alpha", "Alpha shared")
	beta, betaTask := newProjectTask("Beta", "Beta shared")
	return alpha, beta, alphaTask, betaTask
}

func TestPlanningHydrationKeepsDuplicateTaskIDsProjectQualified(t *testing.T) {
	alpha, beta, alphaTask, betaTask := duplicatePlanningProjects(t)
	runtime := testConfigWithProjects(alpha, beta)
	yesterday := time.Now().AddDate(0, 0, -1).Format("2006-01-02")
	runtime.Planning().SetFocusQueue([]config.FocusQueueItem{
		{ProjectID: alpha.ID, TaskID: alphaTask.ID},
		{ProjectID: beta.ID, TaskID: betaTask.ID},
	})
	runtime.Planning().SetDailyPlan([]config.DailyPlanItem{
		{ProjectID: alpha.ID, TaskID: alphaTask.ID, PlannedDate: yesterday},
		{ProjectID: beta.ID, TaskID: betaTask.ID, PlannedDate: yesterday},
	})

	persister := NewMockPersister()
	persister.Projects[alpha.ID] = alpha
	persister.Projects[beta.ID] = beta
	shell := &AppModel{
		runtime:               runtime,
		persister:             persister,
		focusQueueIDs:         make(map[TaskIdentity]bool),
		dailyPlanIDs:          make(map[TaskIdentity]bool),
		dailyPlanCarriedIDs:   make(map[TaskIdentity]bool),
		focusQueue:            []FocusTask{},
		dailyPlan:             []FocusTask{},
		outlinerToggleStates:  make(map[string]*OutlinerToggleState),
		projectLastView:       make(map[string]PresentationID),
		dailyPlanCarriedShown: true,
	}

	loads := make(map[string]error)
	shell.hydrateFocusQueue([]*models.Project{alpha, beta}, loads, true)
	shell.hydrateDailyPlan([]*models.Project{alpha, beta}, loads)

	alphaID := taskIdentityFor(alphaTask, alpha)
	betaID := taskIdentityFor(betaTask, beta)
	if len(shell.focusQueue) != 2 || !shell.focusQueueIDs[alphaID] || !shell.focusQueueIDs[betaID] {
		t.Fatalf("qualified Next hydration = slice:%d map:%v", len(shell.focusQueue), shell.focusQueueIDs)
	}
	if len(shell.dailyPlan) != 2 || !shell.dailyPlanIDs[alphaID] || !shell.dailyPlanIDs[betaID] {
		t.Fatalf("qualified Today hydration = slice:%d map:%v", len(shell.dailyPlan), shell.dailyPlanIDs)
	}
	if shell.dailyPlanCarried != 2 || !shell.dailyPlanCarriedIDs[alphaID] || !shell.dailyPlanCarriedIDs[betaID] {
		t.Fatalf("qualified carry-over hydration = count:%d map:%v", shell.dailyPlanCarried, shell.dailyPlanCarriedIDs)
	}
}

func TestPlanningToggleUndoRedoOnlyTouchesQualifiedTask(t *testing.T) {
	alpha, beta, alphaTask, betaTask := duplicatePlanningProjects(t)
	alphaID := taskIdentityFor(alphaTask, alpha)
	betaID := taskIdentityFor(betaTask, beta)
	shell := &AppModel{
		runtime:      config.EmptyRuntimeState(""),
		dailyPlan:    []FocusTask{{Task: alphaTask, Project: alpha}, {Task: betaTask, Project: beta}},
		dailyPlanIDs: map[TaskIdentity]bool{alphaID: true, betaID: true},
	}

	shell.lastDailyPlanToggle = &dailyPlanToggleAction{Task: betaTask, Project: beta, WasAdd: true}
	shell.lastToggleUndoKind = toggleUndoDailyPlan
	_ = shell.undoDailyPlanToggle()
	if !shell.dailyPlanIDs[alphaID] || shell.dailyPlanIDs[betaID] || len(shell.dailyPlan) != 1 {
		t.Fatalf("undo crossed project boundary: slice:%v map:%v", shell.dailyPlan, shell.dailyPlanIDs)
	}

	_ = shell.redoDailyPlanToggle()
	if !shell.dailyPlanIDs[alphaID] || !shell.dailyPlanIDs[betaID] || len(shell.dailyPlan) != 2 {
		t.Fatalf("redo failed to restore qualified task: slice:%v map:%v", shell.dailyPlan, shell.dailyPlanIDs)
	}
}

func TestPlannerDuplicateTaskIDsKeepExpansionOrderingAndCursorIndependent(t *testing.T) {
	alpha, beta, alphaTask, betaTask := duplicatePlanningProjects(t)
	alphaChild, err := models.NewTask(uuid.NewString(), 2, models.TaskTypeTask, "Alpha child")
	if err != nil {
		t.Fatalf("NewTask(alpha child): %v", err)
	}
	betaChild := *alphaChild
	betaChild.Title = "Beta child"
	alphaTask.Children = []*models.Task{alphaChild}
	betaTask.Children = []*models.Task{&betaChild}

	alphaID := taskIdentityFor(alphaTask, alpha)
	betaID := taskIdentityFor(betaTask, beta)
	alphaRow := boardRow{Task: alphaTask, Project: alpha, Source: alpha.Name}
	betaRow := boardRow{Task: betaTask, Project: beta, Source: beta.Name}

	t.Run("child expansion", func(t *testing.T) {
		m := newTestFocusBoard()
		m.childDepthLimit = -1
		m.todayRows = []boardRow{alphaRow, betaRow}
		m.dailyPlanIDs = map[TaskIdentity]bool{alphaID: true, betaID: true}
		m.expandTodayChildren()
		if len(m.todayRows) != 4 {
			t.Fatalf("expanded rows = %d, want both roots and both same-ID children", len(m.todayRows))
		}
		if m.todayRows[1].Project.ID != alpha.ID || m.todayRows[3].Project.ID != beta.ID {
			t.Fatalf("expanded child ownership = %s/%s, want alpha/beta", m.todayRows[1].Project.ID, m.todayRows[3].Project.ID)
		}
	})

	t.Run("Today reorder", func(t *testing.T) {
		m := newTestFocusBoard()
		plan := []FocusTask{{Task: alphaTask, Project: alpha}, {Task: betaTask, Project: beta}}
		m.dailyPlan = &plan
		m.dailyPlanIDs = map[TaskIdentity]bool{alphaID: true, betaID: true}
		m.todayRows = []boardRow{alphaRow, betaRow}
		m.filtered = append([]boardRow(nil), m.todayRows...)
		m.tabs.Active = boardTabToday
		m.cursor = 0
		m.handleDailyPlanReorder(1)
		if got := taskIdentityFor(plan[0].Task, plan[0].Project); got != betaID {
			t.Fatalf("Today first identity = %+v, want beta %+v", got, betaID)
		}
	})

	t.Run("Next reorder", func(t *testing.T) {
		m := newTestFocusBoard()
		queue := []FocusTask{{Task: alphaTask, Project: alpha}, {Task: betaTask, Project: beta}}
		m.focusQueue = &queue
		m.nextRows = []boardRow{alphaRow, betaRow}
		m.filtered = append([]boardRow(nil), m.nextRows...)
		m.tabs.Active = boardTabNext
		m.cursor = 0
		m.handleNextReorder(1)
		if got := taskIdentityFor(queue[0].Task, queue[0].Project); got != betaID {
			t.Fatalf("Next first identity = %+v, want beta %+v", got, betaID)
		}
	})

	t.Run("cursor restoration", func(t *testing.T) {
		m := newTestFocusBoard()
		m.filtered = []boardRow{betaRow, alphaRow}
		m.restoreCursorAfterReload(focusBoardReloadIntent{valid: true, identity: alphaID})
		if m.cursor != 1 {
			t.Fatalf("cursor = %d, want alpha's project-qualified row at 1", m.cursor)
		}
	})
}

func TestPlanningRenderingAndOpenLoopsDoNotBleedAcrossDuplicateTaskIDs(t *testing.T) {
	alpha, beta, alphaTask, betaTask := duplicatePlanningProjects(t)
	alphaTask.WorkflowState = models.StateReady
	betaTask.WorkflowState = models.StateReady
	alphaID := taskIdentityFor(alphaTask, alpha)

	outliner := NewOutlinerModel(nil, nil, nil)
	outliner.dailyPlanIDs = map[TaskIdentity]bool{alphaID: true}
	if got := outliner.renderTaskQueueIndicator(alphaTask, alpha); got == "  " {
		t.Fatal("planned alpha task lost its Today indicator")
	}
	if got := outliner.renderTaskQueueIndicator(betaTask, beta); got != "  " {
		t.Fatalf("beta duplicate inherited alpha's Today indicator: %q", got)
	}

	repo := testutil.NewMockRepository()
	repo.Projects[alpha.ID] = alpha
	repo.Projects[beta.ID] = beta
	oldRepo := storage.DefaultRepository
	storage.DefaultRepository = repo
	defer func() { storage.DefaultRepository = oldRepo }()

	runtime := testConfigWithProjects(alpha, beta)
	loops := ComputeOpenLoops(runtime, map[TaskIdentity]bool{alphaID: true})
	if loops.TodayTotal != 1 {
		t.Fatalf("Today open-loop total = %d, want only alpha's qualified task", loops.TodayTotal)
	}
}

func TestPlanningArtifactMembershipIsProjectQualified(t *testing.T) {
	alpha, beta, alphaTask, betaTask := duplicatePlanningProjects(t)
	planning := &config.PlanningState{
		FocusQueue: []config.FocusQueueItem{{ProjectID: alpha.ID, TaskID: alphaTask.ID}},
		DailyPlan:  []config.DailyPlanItem{{ProjectID: alpha.ID, TaskID: alphaTask.ID}},
	}
	if !focusQueueContains(planning, taskIdentityFor(alphaTask, alpha)) ||
		!dailyPlanContains(planning, taskIdentityFor(alphaTask, alpha)) {
		t.Fatal("alpha membership was not found")
	}
	if focusQueueContains(planning, taskIdentityFor(betaTask, beta)) ||
		dailyPlanContains(planning, taskIdentityFor(betaTask, beta)) {
		t.Fatal("beta duplicate inherited alpha's persisted membership")
	}
}
