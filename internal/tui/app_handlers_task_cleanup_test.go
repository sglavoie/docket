package tui

import (
	"os"
	"strings"
	"testing"

	domainapp "github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/testutil"
)

func TestPruneDeletedTaskRefs(t *testing.T) {
	t.Run("removes deleted tasks from in-memory lists", func(t *testing.T) {
		app, project := newTestAppModelWithProject()

		// Ensure project has tasks and is loadable
		task1 := project.Tasks[0]
		task2 := project.Tasks[1]

		// Create a fake "deleted" task not in any project
		deletedTask := &models.Task{ID: "deleted-task-id", Title: "Deleted"}
		deletedProject := &models.Project{ID: "proj-deleted"}

		// Populate in-memory lists with both valid and stale entries
		app.focusQueue = []FocusTask{
			{Task: task1, Project: project},
			{Task: deletedTask, Project: deletedProject},
			{Task: task2, Project: project},
		}
		app.focusQueueIDs = map[TaskIdentity]bool{
			taskIdentityFor(task1, project):              true,
			taskIdentityFor(deletedTask, deletedProject): true,
			taskIdentityFor(task2, project):              true,
		}
		app.dailyPlan = []FocusTask{
			{Task: deletedTask, Project: deletedProject},
		}
		app.dailyPlanIDs = map[TaskIdentity]bool{
			taskIdentityFor(deletedTask, deletedProject): true,
		}
		app.recentTaskVisits = []RecentTaskVisitEntry{
			{Task: task1, Project: project},
			{Task: deletedTask, Project: deletedProject},
		}

		app.pruneDeletedTaskRefs()

		// Focus queue: deleted task should be removed
		if len(app.focusQueue) != 2 {
			t.Fatalf("expected 2 focus queue entries, got %d", len(app.focusQueue))
		}
		if app.focusQueueIDs[taskIdentityFor(deletedTask, deletedProject)] {
			t.Error("deleted task should be removed from focusQueueIDs")
		}

		// Daily plan: deleted task should be removed
		if len(app.dailyPlan) != 0 {
			t.Fatalf("expected 0 daily plan entries, got %d", len(app.dailyPlan))
		}
		if app.dailyPlanIDs[taskIdentityFor(deletedTask, deletedProject)] {
			t.Error("deleted task should be removed from dailyPlanIDs")
		}

		// Recent visits: deleted task should be removed
		if len(app.recentTaskVisits) != 1 {
			t.Fatalf("expected 1 recent visit entry, got %d", len(app.recentTaskVisits))
		}
	})

	t.Run("loads unloaded projects before pruning", func(t *testing.T) {
		// Set up mock repository so persister.Load works without disk I/O
		mockRepo := testutil.NewMockRepository()
		oldRepo := storage.DefaultRepository
		storage.DefaultRepository = mockRepo
		defer func() { storage.DefaultRepository = oldRepo }()

		app, project := newTestAppModelWithProject()
		task1 := project.Tasks[0]

		// Register the loaded project in the mock repo
		mockRepo.Projects[project.ID] = project

		// Create a second project with tasks and register it in the mock
		// repository so persister.Load can populate it.
		storedProject := testutil.NewTestProjectWithTasks("Other Project")
		otherTask := storedProject.Tasks[0]
		mockRepo.Projects[storedProject.ID] = storedProject

		// Create a config entry with the same ID but nil Tasks,
		// simulating a project the user hasn't navigated to yet.
		otherProject := &models.Project{ID: storedProject.ID, Name: "Other Project"}
		addTestAppProject(app, otherProject)

		// Put tasks from both projects in the focus queue
		app.focusQueue = []FocusTask{
			{Task: task1, Project: project},
			{Task: otherTask, Project: otherProject},
		}
		app.focusQueueIDs = map[TaskIdentity]bool{
			taskIdentityFor(task1, project):          true,
			taskIdentityFor(otherTask, otherProject): true,
		}
		app.dailyPlan = []FocusTask{}
		app.dailyPlanIDs = map[TaskIdentity]bool{}
		app.recentTaskVisits = []RecentTaskVisitEntry{}

		app.pruneDeletedTaskRefs()

		// Both tasks should survive — the unloaded project's task must
		// not be incorrectly pruned.
		if len(app.focusQueue) != 2 {
			t.Fatalf("expected 2 focus queue entries, got %d", len(app.focusQueue))
		}
		if !app.focusQueueIDs[taskIdentityFor(otherTask, otherProject)] {
			t.Error("task from unloaded project should NOT be pruned")
		}

		// The other project should be unloaded again (Tasks reset to nil)
		if otherProject.Tasks != nil {
			t.Error("unloaded project should have Tasks reset to nil after pruning")
		}
	})

	t.Run("no-op when all tasks are valid", func(t *testing.T) {
		app, project := newTestAppModelWithProject()
		task1 := project.Tasks[0]

		app.focusQueue = []FocusTask{{Task: task1, Project: project}}
		app.focusQueueIDs = map[TaskIdentity]bool{taskIdentityFor(task1, project): true}
		app.dailyPlan = []FocusTask{}
		app.dailyPlanIDs = map[TaskIdentity]bool{}
		app.recentTaskVisits = []RecentTaskVisitEntry{{Task: task1, Project: project}}

		app.pruneDeletedTaskRefs()

		if len(app.focusQueue) != 1 {
			t.Errorf("expected 1 focus queue entry, got %d", len(app.focusQueue))
		}
		if len(app.recentTaskVisits) != 1 {
			t.Errorf("expected 1 recent visit entry, got %d", len(app.recentTaskVisits))
		}
	})
}

func TestTaskDeletedMsgCleansConfigRefs(t *testing.T) {
	app, project := newTestAppModelWithProject()
	app.outliner = NewOutlinerModel(project, nil, app.runtime)

	// Set up a task that will be "deleted"
	deletedTask := &models.Task{ID: "will-be-deleted", Title: "Ghost"}
	app.focusQueue = []FocusTask{{Task: deletedTask, Project: project}}
	app.focusQueueIDs = map[TaskIdentity]bool{taskIdentityFor(deletedTask, project): true}
	app.dailyPlan = []FocusTask{{Task: deletedTask, Project: project}}
	app.dailyPlanIDs = map[TaskIdentity]bool{taskIdentityFor(deletedTask, project): true}

	msg := TaskDeletedMsg{Task: deletedTask}
	handled, _ := app.handleTaskMessages(msg)

	if !handled {
		t.Fatal("TaskDeletedMsg should be handled")
	}
	if len(app.focusQueue) != 0 {
		t.Errorf("expected focus queue to be pruned, got %d entries", len(app.focusQueue))
	}
	if len(app.dailyPlan) != 0 {
		t.Errorf("expected daily plan to be pruned, got %d entries", len(app.dailyPlan))
	}
}

func TestTasksArchivedMsgCleansConfigRefs(t *testing.T) {
	cfg := newTestConfig()
	project := testutil.NewTestProjectWithTasks("Test")
	setTestProjects(cfg, []*models.Project{project})
	cfg.Machine().CurrentProject = project.ID

	app := newTestAppModel()
	app.runtime = testRuntimeAt(cfg.DataDir(), cfg)
	setTestAppProjects(app, project)
	setTestCurrentProject(app, project.ID)

	// Simulate an archived task that no longer exists in project
	archivedTask := &models.Task{ID: "archived-task", Title: "Done"}
	app.focusQueue = []FocusTask{{Task: archivedTask, Project: project}}
	app.focusQueueIDs = map[TaskIdentity]bool{taskIdentityFor(archivedTask, project): true}
	app.dailyPlan = []FocusTask{}
	app.dailyPlanIDs = map[TaskIdentity]bool{}

	msg := TasksArchivedMsg{Count: 1}
	handled, _ := app.handleTaskMessages(msg)

	if !handled {
		t.Fatal("TasksArchivedMsg should be handled")
	}
	if len(app.focusQueue) != 0 {
		t.Errorf("expected focus queue to be pruned after archive, got %d", len(app.focusQueue))
	}
}

func TestTasksArchivedMachineFailurePublishesPlanningAndRetriesCleanup(t *testing.T) {
	project := newSession4Project(t, "Partial Task Archive")
	fixture := newSession4LifecycleFixture(t, true, project)
	model := fixture.app
	taskID := project.Tasks[0].ID

	if err := commitPlanningToRuntime(model.runtime, func(planning *config.PlanningState) {
		planning.SetFocusQueue([]config.FocusQueueItem{{
			ProjectID: project.ID,
			TaskID:    taskID,
		}})
	}); err != nil {
		t.Fatalf("seed planning reference: %v", err)
	}
	machine := cloneMachineState(model.runtime.Machine())
	machine.SetRecentTaskVisits([]config.RecentTaskVisit{{
		ProjectID: project.ID,
		TaskID:    taskID,
	}})
	if err := model.runtime.CommitMachine(machine); err != nil {
		t.Fatalf("seed machine reference: %v", err)
	}

	machineLock := blockTrashRestoreMachineCommit(t, model)
	handled, cmd := model.handleTaskArchiveMessages(TasksArchivedMsg{
		Count: 1,
		ArchivedTaskLocators: []domainapp.TaskLocator{{
			ProjectID: project.ID,
			TaskID:    taskID,
		}},
	})
	if !handled || cmd == nil {
		t.Fatalf("partial task archive = handled %v cmd %v, want published result and retry commands", handled, cmd)
	}
	if model.err != nil {
		t.Fatalf("partial task archive surfaced a total failure: %v", model.err)
	}
	if len(model.runtime.Planning().FocusQueue) != 0 {
		t.Fatalf("synced planning cleanup was not published: %+v", model.runtime.Planning().FocusQueue)
	}
	if len(model.runtime.Machine().RecentTaskVisits) != 1 {
		t.Fatalf("failed machine commit changed recent visits: %+v", model.runtime.Machine().RecentTaskVisits)
	}
	if model.pendingMachineDomainRepair.empty() || !model.machineDomainRepairRetryScheduled {
		t.Fatal("partial task archive did not retain and schedule machine cleanup")
	}
	if !strings.Contains(model.statusMessage, "machine-state cleanup incomplete") {
		t.Fatalf("partial task archive status = %q, want retry warning", model.statusMessage)
	}

	if err := os.RemoveAll(machineLock); err != nil {
		t.Fatalf("repair machine lock: %v", err)
	}
	retryHandled, retryCmd := model.handleGlobalLifecycleMessage(retryMachineDomainRepairMsg{})
	if !retryHandled || retryCmd != nil {
		t.Fatalf("successful retry = handled %v cmd %v, want handled and settled", retryHandled, retryCmd)
	}
	if visits := model.runtime.Machine().RecentTaskVisits; len(visits) != 0 {
		t.Fatalf("machine cleanup was not applied: %+v", visits)
	}
	if !model.pendingMachineDomainRepair.empty() || model.machineDomainRepairRetryScheduled {
		t.Fatal("successful retry retained pending machine cleanup")
	}
}
