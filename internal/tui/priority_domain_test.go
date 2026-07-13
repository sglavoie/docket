package tui

import (
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/sglavoie/tt/internal/models"
)

func TestIsEligibleForPriority(t *testing.T) {
	t.Run("active todo task is eligible", func(t *testing.T) {
		task, err := models.NewTask("1", 1, models.TaskTypeTask, "Active")
		if err != nil {
			t.Fatal(err)
		}
		if !IsEligibleForPriority(task) {
			t.Errorf("expected active task to be eligible")
		}
	})

	t.Run("completed task is excluded", func(t *testing.T) {
		task, err := models.NewTask("1", 1, models.TaskTypeTask, "Done")
		if err != nil {
			t.Fatal(err)
		}
		task.WorkflowState = models.StateDone
		if IsEligibleForPriority(task) {
			t.Errorf("expected done task to be excluded")
		}
	})

	t.Run("canceled task is excluded", func(t *testing.T) {
		task, err := models.NewTask("1", 1, models.TaskTypeTask, "Canceled")
		if err != nil {
			t.Fatal(err)
		}
		task.WorkflowState = models.StateCanceled
		if IsEligibleForPriority(task) {
			t.Errorf("expected canceled task to be excluded")
		}
	})

	t.Run("snoozed task is excluded", func(t *testing.T) {
		task, err := models.NewTask("1", 1, models.TaskTypeTask, "Snoozed")
		if err != nil {
			t.Fatal(err)
		}
		future := time.Now().AddDate(0, 0, 3)
		task.SnoozedUntil = &future
		if IsEligibleForPriority(task) {
			t.Errorf("expected snoozed task to be excluded")
		}
	})

	t.Run("parent container task is eligible", func(t *testing.T) {
		parent, err := models.NewTask("parent", 1, models.TaskTypeEpic, "Parent epic")
		if err != nil {
			t.Fatal(err)
		}
		child, err := models.NewTask("child", 2, models.TaskTypeTask, "Child task")
		if err != nil {
			t.Fatal(err)
		}
		parent.Children = []*models.Task{child}

		if !IsEligibleForPriority(parent) {
			t.Errorf("expected parent/container task to remain eligible")
		}
	})
}

func TestIsPriorityBlocked(t *testing.T) {
	t.Run("task with blocked status is blocked", func(t *testing.T) {
		project, err := models.NewProject(uuid.New().String(), "P", "")
		if err != nil {
			t.Fatal(err)
		}
		task, err := models.NewTask("1", 1, models.TaskTypeTask, "Blocked")
		if err != nil {
			t.Fatal(err)
		}
		task.WorkflowState = models.StateWaiting
		project.Tasks = []*models.Task{task}

		if !IsPriorityBlocked(project, task) {
			t.Errorf("expected blocked-status task to be blocked")
		}
	})

	t.Run("task with incomplete dependency is blocked", func(t *testing.T) {
		project, err := models.NewProject(uuid.New().String(), "P", "")
		if err != nil {
			t.Fatal(err)
		}
		blocker, err := models.NewTask("blocker", 1, models.TaskTypeTask, "Blocker")
		if err != nil {
			t.Fatal(err)
		}
		dependent, err := models.NewTask("dep", 2, models.TaskTypeTask, "Dependent")
		if err != nil {
			t.Fatal(err)
		}
		dependent.DependsOn = []string{blocker.ID}
		project.Tasks = []*models.Task{blocker, dependent}

		if !IsPriorityBlocked(project, dependent) {
			t.Errorf("expected task with incomplete dependency to be blocked")
		}
	})

	t.Run("task with completed dependency is not blocked", func(t *testing.T) {
		project, err := models.NewProject(uuid.New().String(), "P", "")
		if err != nil {
			t.Fatal(err)
		}
		blocker, err := models.NewTask("blocker", 1, models.TaskTypeTask, "Blocker")
		if err != nil {
			t.Fatal(err)
		}
		blocker.WorkflowState = models.StateDone
		dependent, err := models.NewTask("dep", 2, models.TaskTypeTask, "Dependent")
		if err != nil {
			t.Fatal(err)
		}
		dependent.DependsOn = []string{blocker.ID}
		project.Tasks = []*models.Task{blocker, dependent}

		if IsPriorityBlocked(project, dependent) {
			t.Errorf("expected task with completed dependency to be unblocked")
		}
	})

	t.Run("task with no dependencies is not blocked", func(t *testing.T) {
		project, err := models.NewProject(uuid.New().String(), "P", "")
		if err != nil {
			t.Fatal(err)
		}
		task, err := models.NewTask("1", 1, models.TaskTypeTask, "Free")
		if err != nil {
			t.Fatal(err)
		}
		project.Tasks = []*models.Task{task}

		if IsPriorityBlocked(project, task) {
			t.Errorf("expected free task to be unblocked")
		}
	})
}

func TestSortPriorityTasksByUrgency(t *testing.T) {
	t.Run("sorts higher score first", func(t *testing.T) {
		low := &PriorityTask{Task: &models.Task{ID: "low"}, UrgencyScore: 1000}
		mid := &PriorityTask{Task: &models.Task{ID: "mid"}, UrgencyScore: 3000}
		high := &PriorityTask{Task: &models.Task{ID: "high"}, UrgencyScore: 5000}

		tasks := []*PriorityTask{low, high, mid}
		SortPriorityTasksByUrgency(tasks)

		wantOrder := []string{"high", "mid", "low"}
		for i, want := range wantOrder {
			if tasks[i].Task.ID != want {
				t.Errorf("position %d: got %q, want %q", i, tasks[i].Task.ID, want)
			}
		}
	})

	t.Run("preserves insertion order for equal scores", func(t *testing.T) {
		// Five tasks with identical UrgencyScore. A stable sort must preserve
		// insertion order so the cross-project and single-project views do not
		// flip rows between reloads.
		ids := []string{"a", "b", "c", "d", "e"}
		tasks := make([]*PriorityTask, len(ids))
		for i, id := range ids {
			tasks[i] = &PriorityTask{Task: &models.Task{ID: id}, UrgencyScore: 3000}
		}

		SortPriorityTasksByUrgency(tasks)

		for i, want := range ids {
			if tasks[i].Task.ID != want {
				t.Errorf("position %d: got %q, want %q", i, tasks[i].Task.ID, want)
			}
		}
	})

	t.Run("empty slice is a no-op", func(t *testing.T) {
		var tasks []*PriorityTask
		SortPriorityTasksByUrgency(tasks)
		if len(tasks) != 0 {
			t.Errorf("expected empty slice, got len %d", len(tasks))
		}
	})
}

func TestCategorizeByDueDate_Helper(t *testing.T) {
	// Spot-check the helper directly without the view model. The exhaustive
	// table is in TestCategorizeByDueDate.
	makeTask := func(daysOffset *int) *models.Task {
		task, err := models.NewTask("1", 1, models.TaskTypeTask, "T")
		if err != nil {
			t.Fatal(err)
		}
		if daysOffset != nil {
			due := time.Now().AddDate(0, 0, *daysOffset)
			task.DueDate = &due
		}
		return task
	}

	if got := CategorizeByDueDate(makeTask(intPtr(-1))); got != PriorityGroupOverdue {
		t.Errorf("overdue: got %v", got)
	}
	if got := CategorizeByDueDate(makeTask(intPtr(0))); got != PriorityGroupToday {
		t.Errorf("today: got %v", got)
	}
	if got := CategorizeByDueDate(makeTask(intPtr(3))); got != PriorityGroupThisWeek {
		t.Errorf("this week: got %v", got)
	}
	if got := CategorizeByDueDate(makeTask(intPtr(10))); got != PriorityGroupLater {
		t.Errorf("later: got %v", got)
	}
	if got := CategorizeByDueDate(makeTask(nil)); got != PriorityGroupBacklog {
		t.Errorf("backlog (no due date): got %v", got)
	}
}
