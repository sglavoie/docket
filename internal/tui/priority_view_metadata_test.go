package tui

import (
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/models"
)

func TestGetParentTitle(t *testing.T) {
	t.Run("no parent returns empty", func(t *testing.T) {
		project := newTestPriorityProject(t)
		task, err := models.NewTask("1", 1, models.TaskTypeTask, "Orphan task")
		if err != nil {
			t.Fatal(err)
		}
		project.Tasks = []*models.Task{task}

		m := NewPriorityViewModel(project, nil, nil)
		got := m.getParentTitle(task)

		if got != "" {
			t.Errorf("getParentTitle() = %q, want empty", got)
		}
	})

	t.Run("valid parent returns title", func(t *testing.T) {
		project := newTestPriorityProject(t)
		parent, err := models.NewTask("1", 1, models.TaskTypeEpic, "Parent Epic")
		if err != nil {
			t.Fatal(err)
		}
		child, err := models.NewTask("2", 2, models.TaskTypeTask, "Child Task")
		if err != nil {
			t.Fatal(err)
		}
		child.ParentID = &parent.ID

		project.Tasks = []*models.Task{parent, child}
		project.BuildTaskTree()

		m := NewPriorityViewModel(project, nil, nil)
		got := m.getParentTitle(child)

		if got != "Parent Epic" {
			t.Errorf("getParentTitle() = %q, want 'Parent Epic'", got)
		}
	})

	t.Run("missing parent returns empty", func(t *testing.T) {
		project := newTestPriorityProject(t)
		child, err := models.NewTask("1", 1, models.TaskTypeTask, "Orphan with invalid parent")
		if err != nil {
			t.Fatal(err)
		}
		invalidID := "non-existent-id"
		child.ParentID = &invalidID

		project.Tasks = []*models.Task{child}

		m := NewPriorityViewModel(project, nil, nil)
		got := m.getParentTitle(child)

		if got != "" {
			t.Errorf("getParentTitle() = %q, want empty", got)
		}
	})
}

func TestGetBlockingReason(t *testing.T) {
	t.Run("explicitly blocked returns blocked", func(t *testing.T) {
		project := newTestPriorityProject(t)
		task, err := models.NewTask("1", 1, models.TaskTypeTask, "Blocked task")
		if err != nil {
			t.Fatal(err)
		}
		task.Status = models.StatusBlocked

		project.Tasks = []*models.Task{task}

		m := NewPriorityViewModel(project, nil, nil)
		got := m.getBlockingReason(task)

		if got != "(blocked)" {
			t.Errorf("getBlockingReason() = %q, want '(blocked)'", got)
		}
	})

	t.Run("incomplete dependency shows dependency title", func(t *testing.T) {
		project := newTestPriorityProject(t)
		blocker, err := models.NewTask("1", 1, models.TaskTypeTask, "Blocker Task")
		if err != nil {
			t.Fatal(err)
		}
		blocker.Status = models.StatusTodo

		dependent, err := models.NewTask("2", 2, models.TaskTypeTask, "Dependent")
		if err != nil {
			t.Fatal(err)
		}
		dependent.DependsOn = []string{blocker.ID}

		project.Tasks = []*models.Task{blocker, dependent}

		m := NewPriorityViewModel(project, nil, nil)
		got := m.getBlockingReason(dependent)

		if got != "← Blocker Task" {
			t.Errorf("getBlockingReason() = %q, want '← Blocker Task'", got)
		}
	})
}

func TestChangeTaskPriority(t *testing.T) {
	t.Run("changes priority of selected task", func(t *testing.T) {
		project := newTestPriorityProject(t)
		task, err := models.NewTask("1", 1, models.TaskTypeTask, "Test task")
		if err != nil {
			t.Fatal(err)
		}
		task.Priority = 3
		project.Tasks = []*models.Task{task}

		m := NewPriorityViewModel(project, nil, nil)
		m.cursor = 0

		m.changeTaskPriority(1)

		if task.Priority != 1 {
			t.Errorf("expected priority 1, got %d", task.Priority)
		}
	})

	t.Run("does nothing when priority unchanged", func(t *testing.T) {
		project := newTestPriorityProject(t)
		task, err := models.NewTask("1", 1, models.TaskTypeTask, "Test task")
		if err != nil {
			t.Fatal(err)
		}
		task.Priority = 2
		originalUpdatedAt := task.UpdatedAt
		project.Tasks = []*models.Task{task}

		m := NewPriorityViewModel(project, nil, nil)
		m.cursor = 0

		m.changeTaskPriority(2)

		if task.Priority != 2 {
			t.Errorf("expected priority 2, got %d", task.Priority)
		}
		if !task.UpdatedAt.Equal(originalUpdatedAt) {
			t.Error("UpdatedAt should not change when priority is unchanged")
		}
	})

	t.Run("does nothing when no task at cursor", func(t *testing.T) {
		project := newTestPriorityProject(t)
		m := NewPriorityViewModel(project, nil, nil)

		result, _ := m.changeTaskPriority(1)
		if result == nil {
			t.Error("expected non-nil result")
		}
	})

	t.Run("updates UpdatedAt timestamp", func(t *testing.T) {
		project := newTestPriorityProject(t)
		task, err := models.NewTask("1", 1, models.TaskTypeTask, "Test task")
		if err != nil {
			t.Fatal(err)
		}
		task.Priority = 3
		originalUpdatedAt := task.UpdatedAt
		project.Tasks = []*models.Task{task}

		m := NewPriorityViewModel(project, nil, nil)
		m.cursor = 0

		time.Sleep(time.Millisecond)
		m.changeTaskPriority(1)

		if !task.UpdatedAt.After(originalUpdatedAt) {
			t.Error("UpdatedAt should be updated after priority change")
		}
	})
}
