package tui

import (
	"testing"

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
		app.focusQueueIDs = map[string]bool{
			task1.ID:          true,
			"deleted-task-id": true,
			task2.ID:          true,
		}
		app.dailyPlan = []FocusTask{
			{Task: deletedTask, Project: deletedProject},
		}
		app.dailyPlanIDs = map[string]bool{
			"deleted-task-id": true,
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
		if app.focusQueueIDs["deleted-task-id"] {
			t.Error("deleted task should be removed from focusQueueIDs")
		}

		// Daily plan: deleted task should be removed
		if len(app.dailyPlan) != 0 {
			t.Fatalf("expected 0 daily plan entries, got %d", len(app.dailyPlan))
		}
		if app.dailyPlanIDs["deleted-task-id"] {
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
		app.config.Projects = append(app.config.Projects, otherProject)

		// Put tasks from both projects in the focus queue
		app.focusQueue = []FocusTask{
			{Task: task1, Project: project},
			{Task: otherTask, Project: otherProject},
		}
		app.focusQueueIDs = map[string]bool{
			task1.ID:     true,
			otherTask.ID: true,
		}
		app.dailyPlan = []FocusTask{}
		app.dailyPlanIDs = map[string]bool{}
		app.recentTaskVisits = []RecentTaskVisitEntry{}

		app.pruneDeletedTaskRefs()

		// Both tasks should survive — the unloaded project's task must
		// not be incorrectly pruned.
		if len(app.focusQueue) != 2 {
			t.Fatalf("expected 2 focus queue entries, got %d", len(app.focusQueue))
		}
		if !app.focusQueueIDs[otherTask.ID] {
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
		app.focusQueueIDs = map[string]bool{task1.ID: true}
		app.dailyPlan = []FocusTask{}
		app.dailyPlanIDs = map[string]bool{}
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
	app.projectTree = NewProjectTreeModel(project, app.config, nil)

	// Set up a task that will be "deleted"
	deletedTask := &models.Task{ID: "will-be-deleted", Title: "Ghost"}
	app.focusQueue = []FocusTask{{Task: deletedTask, Project: project}}
	app.focusQueueIDs = map[string]bool{"will-be-deleted": true}
	app.dailyPlan = []FocusTask{{Task: deletedTask, Project: project}}
	app.dailyPlanIDs = map[string]bool{"will-be-deleted": true}

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
	cfg := testutil.NewTestConfig()
	project := testutil.NewTestProjectWithTasks("Test")
	cfg.Projects = []*models.Project{project}
	cfg.CurrentProject = project.ID

	app := newTestAppModel()
	app.config = cfg
	app.currentProject = project

	// Simulate an archived task that no longer exists in project
	archivedTask := &models.Task{ID: "archived-task", Title: "Done"}
	app.focusQueue = []FocusTask{{Task: archivedTask, Project: project}}
	app.focusQueueIDs = map[string]bool{"archived-task": true}
	app.dailyPlan = []FocusTask{}
	app.dailyPlanIDs = map[string]bool{}

	msg := TasksArchivedMsg{Count: 1}
	handled, _ := app.handleTaskMessages(msg)

	if !handled {
		t.Fatal("TasksArchivedMsg should be handled")
	}
	if len(app.focusQueue) != 0 {
		t.Errorf("expected focus queue to be pruned after archive, got %d", len(app.focusQueue))
	}
}
