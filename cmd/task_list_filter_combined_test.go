package cmd

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/testutil"
)

func TestFilterTasks_CombinedFilters_TypeAndStatus(t *testing.T) {
	defer resetFilterVars()
	listShowAll = true
	listTaskType = "task"
	listTaskStatus = "todo"

	tasks := []*models.Task{
		func() *models.Task {
			task := testutil.NewTestTask("Todo task", models.TaskTypeTask)
			task.Status = models.StatusTodo
			return task
		}(),
		func() *models.Task {
			task := testutil.NewTestTask("InProgress task", models.TaskTypeTask)
			task.Status = models.StatusInProgress
			return task
		}(),
		func() *models.Task {
			task := testutil.NewTestTask("Todo bug", models.TaskTypeBug)
			task.Status = models.StatusTodo
			return task
		}(),
	}

	result := filterTasks(tasks)

	if len(result) != 1 {
		t.Errorf("expected 1 task matching type=task AND status=todo, got %d", len(result))
	}
	if len(result) > 0 {
		if result[0].Type != models.TaskTypeTask {
			t.Errorf("expected type task, got %s", result[0].Type)
		}
		if result[0].Status != models.StatusTodo {
			t.Errorf("expected status todo, got %s", result[0].Status)
		}
	}
}

func TestFilterTasks_CombinedFilters_PriorityAndTag(t *testing.T) {
	defer resetFilterVars()
	listShowAll = true
	listPriority = 1
	listTag = "urgent"

	task1 := testutil.NewTestTaskWithPriority("Urgent p1", 1)
	task1.Tags = []string{"urgent"}

	task2 := testutil.NewTestTaskWithPriority("Normal p1", 1)
	task2.Tags = []string{"normal"}

	task3 := testutil.NewTestTaskWithPriority("Urgent p2", 2)
	task3.Tags = []string{"urgent"}

	tasks := []*models.Task{task1, task2, task3}

	result := filterTasks(tasks)

	if len(result) != 1 {
		t.Errorf("expected 1 task matching priority=1 AND tag=urgent, got %d", len(result))
	}
	if len(result) > 0 && result[0].Title != "Urgent p1" {
		t.Errorf("expected 'Urgent p1', got '%s'", result[0].Title)
	}
}

func TestFilterTasks_AllFiltersApplied(t *testing.T) {
	defer resetFilterVars()
	listShowAll = true
	listTaskType = "bug"
	listTaskStatus = "in_progress"
	listPriority = 1
	listTag = "critical"

	matchingTask := func() *models.Task {
		task := testutil.NewTestTask("Critical bug", models.TaskTypeBug)
		task.Status = models.StatusInProgress
		task.Priority = 1
		task.Tags = []string{"critical", "backend"}
		return task
	}()

	wrongType := func() *models.Task {
		task := testutil.NewTestTask("Critical task", models.TaskTypeTask)
		task.Status = models.StatusInProgress
		task.Priority = 1
		task.Tags = []string{"critical"}
		return task
	}()

	wrongStatus := func() *models.Task {
		task := testutil.NewTestTask("Todo bug", models.TaskTypeBug)
		task.Status = models.StatusTodo
		task.Priority = 1
		task.Tags = []string{"critical"}
		return task
	}()

	wrongPriority := func() *models.Task {
		task := testutil.NewTestTask("P2 bug", models.TaskTypeBug)
		task.Status = models.StatusInProgress
		task.Priority = 2
		task.Tags = []string{"critical"}
		return task
	}()

	missingTag := func() *models.Task {
		task := testutil.NewTestTask("Non-critical bug", models.TaskTypeBug)
		task.Status = models.StatusInProgress
		task.Priority = 1
		task.Tags = []string{"minor"}
		return task
	}()

	tasks := []*models.Task{matchingTask, wrongType, wrongStatus, wrongPriority, missingTag}

	result := filterTasks(tasks)

	if len(result) != 1 {
		t.Errorf("expected 1 task matching all filters, got %d", len(result))
	}
	if len(result) > 0 && result[0].Title != "Critical bug" {
		t.Errorf("expected 'Critical bug', got '%s'", result[0].Title)
	}
}

func TestFilterTasks_ParkedComposesWithPriorityAndTag(t *testing.T) {
	defer resetFilterVars()
	listShowAll = true
	listParked = true
	listPriority = 1
	listTag = "urgent"

	matchingTask := testutil.NewTestTaskWithPriority("Parked urgent p1", 1)
	matchingTask.ParkedNote = "resume deploy"
	matchingTask.Tags = []string{"urgent", "backend"}

	nonParked := testutil.NewTestTaskWithPriority("Urgent p1", 1)
	nonParked.Tags = []string{"urgent"}

	wrongPriority := testutil.NewTestTaskWithPriority("Parked urgent p2", 2)
	wrongPriority.ParkedNote = "resume docs"
	wrongPriority.Tags = []string{"urgent"}

	missingTag := testutil.NewTestTaskWithPriority("Parked p1", 1)
	missingTag.ParkedNote = "resume api"
	missingTag.Tags = []string{"backend"}

	tasks := []*models.Task{matchingTask, nonParked, wrongPriority, missingTag}

	result := filterTasks(tasks)

	if len(result) != 1 {
		t.Errorf("expected 1 task matching parked AND priority=1 AND tag=urgent, got %d", len(result))
	}
	if len(result) > 0 && result[0].Title != "Parked urgent p1" {
		t.Errorf("expected 'Parked urgent p1', got '%s'", result[0].Title)
	}
}
