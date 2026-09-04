package tui

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/testutil"
	"github.com/sglavoie/tt/internal/tui/outliner"
	"github.com/sglavoie/tt/internal/tui/taskref"
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
		app.nextOrder = []outliner.PlanningTask{
			{Task: task1, Project: project},
			{Task: deletedTask, Project: deletedProject},
			{Task: task2, Project: project},
		}
		app.nextOrderIDs = map[taskref.TaskIdentity]bool{
			taskref.For(task1, project):              true,
			taskref.For(deletedTask, deletedProject): true,
			taskref.For(task2, project):              true,
		}
		app.today = []outliner.PlanningTask{
			{Task: deletedTask, Project: deletedProject},
		}
		app.todayIDs = map[taskref.TaskIdentity]bool{
			taskref.For(deletedTask, deletedProject): true,
		}
		app.pruneDeletedTaskRefs()

		// Focus queue: deleted task should be removed
		if len(app.nextOrder) != 2 {
			t.Fatalf("expected 2 focus queue entries, got %d", len(app.nextOrder))
		}
		if app.nextOrderIDs[taskref.For(deletedTask, deletedProject)] {
			t.Error("deleted task should be removed from nextOrderIDs")
		}

		// Daily plan: deleted task should be removed
		if len(app.today) != 0 {
			t.Fatalf("expected 0 daily plan entries, got %d", len(app.today))
		}
		if app.todayIDs[taskref.For(deletedTask, deletedProject)] {
			t.Error("deleted task should be removed from todayIDs")
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
		app.nextOrder = []outliner.PlanningTask{
			{Task: task1, Project: project},
			{Task: otherTask, Project: otherProject},
		}
		app.nextOrderIDs = map[taskref.TaskIdentity]bool{
			taskref.For(task1, project):          true,
			taskref.For(otherTask, otherProject): true,
		}
		app.today = []outliner.PlanningTask{}
		app.todayIDs = map[taskref.TaskIdentity]bool{}
		app.pruneDeletedTaskRefs()

		// Both tasks should survive — the unloaded project's task must
		// not be incorrectly pruned.
		if len(app.nextOrder) != 2 {
			t.Fatalf("expected 2 focus queue entries, got %d", len(app.nextOrder))
		}
		if !app.nextOrderIDs[taskref.For(otherTask, otherProject)] {
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

		app.nextOrder = []outliner.PlanningTask{{Task: task1, Project: project}}
		app.nextOrderIDs = map[taskref.TaskIdentity]bool{taskref.For(task1, project): true}
		app.today = []outliner.PlanningTask{}
		app.todayIDs = map[taskref.TaskIdentity]bool{}
		app.pruneDeletedTaskRefs()

		if len(app.nextOrder) != 1 {
			t.Errorf("expected 1 focus queue entry, got %d", len(app.nextOrder))
		}
	})
}
