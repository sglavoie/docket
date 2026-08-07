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
		task, err := models.NewGenericTask(sharedID, 1, title)
		if err != nil {
			t.Fatalf("NewTask(%s): %v", title, err)
		}
		task.WorkflowState = models.StateBacklog
		project.Tasks = []*models.Task{task}
		project.NextShortID = 2
		project.BuildTaskTree()
		return project, task
	}
	alpha, alphaTask := newProjectTask("Alpha", "Alpha shared")
	beta, betaTask := newProjectTask("Beta", "Beta shared")
	return alpha, beta, alphaTask, betaTask
}

func TestPlanningHydrationKeepsDuplicateTaskIDsProjectQualified(t *testing.T) {
	alpha, beta, alphaTask, betaTask := duplicatePlanningProjects(t)
	todaySharedID := uuid.NewString()
	alphaToday, err := models.NewGenericTask(todaySharedID, 2, "Alpha Today shared")
	if err != nil {
		t.Fatalf("NewTask(alpha Today): %v", err)
	}
	betaToday, err := models.NewGenericTask(todaySharedID, 2, "Beta Today shared")
	if err != nil {
		t.Fatalf("NewTask(beta Today): %v", err)
	}
	alphaToday.WorkflowState = models.StateBacklog
	betaToday.WorkflowState = models.StateBacklog
	alpha.Tasks = append(alpha.Tasks, alphaToday)
	beta.Tasks = append(beta.Tasks, betaToday)
	alpha.BuildTaskTree()
	beta.BuildTaskTree()
	runtime := testConfigWithProjects(alpha, beta)
	runtime.Planning().SetNext([]config.ProjectTaskLocator{
		{ProjectID: alpha.ID, TaskID: alphaTask.ID},
		{ProjectID: beta.ID, TaskID: betaTask.ID},
	})
	runtime.Planning().SetToday([]config.ProjectTaskLocator{
		{ProjectID: alpha.ID, TaskID: alphaToday.ID},
		{ProjectID: beta.ID, TaskID: betaToday.ID},
	})

	persister := NewMockPersister()
	persister.Projects[alpha.ID] = alpha
	persister.Projects[beta.ID] = beta
	shell := &AppModel{
		runtime:      runtime,
		persister:    persister,
		nextOrderIDs: make(map[TaskIdentity]bool),
		todayIDs:     make(map[TaskIdentity]bool),
		nextOrder:    []PlanningTask{},
		today:        []PlanningTask{},
	}

	loads := make(map[string]error)
	shell.hydrateNextOrder([]*models.Project{alpha, beta}, loads, true)
	shell.hydrateToday([]*models.Project{alpha, beta}, loads)

	alphaID := taskIdentityFor(alphaTask, alpha)
	betaID := taskIdentityFor(betaTask, beta)
	alphaTodayID := taskIdentityFor(alphaToday, alpha)
	betaTodayID := taskIdentityFor(betaToday, beta)
	if len(shell.nextOrder) != 2 || !shell.nextOrderIDs[alphaID] || !shell.nextOrderIDs[betaID] {
		t.Fatalf("qualified Next hydration = slice:%d map:%v", len(shell.nextOrder), shell.nextOrderIDs)
	}
	if len(shell.today) != 2 || !shell.todayIDs[alphaTodayID] || !shell.todayIDs[betaTodayID] {
		t.Fatalf("qualified Today hydration = slice:%d map:%v", len(shell.today), shell.todayIDs)
	}
}

func TestPlanningWorkspacePulseDoesNotBleedAcrossDuplicateTaskIDs(t *testing.T) {
	alpha, beta, alphaTask, betaTask := duplicatePlanningProjects(t)
	alphaTask.WorkflowState = models.StateBacklog
	betaTask.WorkflowState = models.StateBacklog
	alphaID := taskIdentityFor(alphaTask, alpha)

	repo := testutil.NewMockRepository()
	repo.Projects[alpha.ID] = alpha
	repo.Projects[beta.ID] = beta
	oldRepo := storage.DefaultRepository
	storage.DefaultRepository = repo
	defer func() { storage.DefaultRepository = oldRepo }()

	runtime := testConfigWithProjects(alpha, beta)
	pulse := ComputeWorkspacePulse(runtime, map[TaskIdentity]bool{alphaID: true}, time.Now())
	if pulse.Today != 1 {
		t.Fatalf("Today workspace pulse = %d, want only alpha's qualified task", pulse.Today)
	}
}

func TestPlanningArtifactMembershipIsProjectQualified(t *testing.T) {
	alpha, beta, alphaTask, betaTask := duplicatePlanningProjects(t)
	planning := &config.PlanningState{
		Next:  []config.ProjectTaskLocator{{ProjectID: alpha.ID, TaskID: alphaTask.ID}},
		Today: []config.ProjectTaskLocator{{ProjectID: alpha.ID, TaskID: alphaTask.ID}},
	}
	if !nextOrderContains(planning, taskIdentityFor(alphaTask, alpha)) ||
		!todayContains(planning, taskIdentityFor(alphaTask, alpha)) {
		t.Fatal("alpha membership was not found")
	}
	if nextOrderContains(planning, taskIdentityFor(betaTask, beta)) ||
		todayContains(planning, taskIdentityFor(betaTask, beta)) {
		t.Fatal("beta duplicate inherited alpha's persisted membership")
	}
}
