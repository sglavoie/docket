package tui

import (
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/models"
)

func TestApplyBulkDueDate_SetsDueDate(t *testing.T) {
	project, err := models.NewProject("proj-1", "TestProject", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	task1, err := models.NewGenericTask("task-1", 1, "Task one")
	if err != nil {
		t.Fatal(err)
	}
	task2, err := models.NewGenericTask("task-2", 2, "Task two")
	if err != nil {
		t.Fatal(err)
	}
	project.Tasks = []*models.Task{task1, task2}
	project.BuildTaskTree()

	tomorrow := time.Now().AddDate(0, 0, 1)
	count := applyBulkDueDate(project, project.Tasks, &tomorrow)
	if count != 2 {
		t.Fatalf("expected 2 tasks changed, got %d", count)
	}

	for _, task := range project.Tasks {
		if task.DueDate == nil {
			t.Errorf("task %s: DueDate should be set", task.ID)
		}
	}
}

func TestApplyBulkDueDate_EachTaskOwnsDueDate(t *testing.T) {
	project, err := models.NewProject("proj-1", "TestProject", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	task1, err := models.NewGenericTask("task-1", 1, "Task one")
	if err != nil {
		t.Fatal(err)
	}
	task2, err := models.NewGenericTask("task-2", 2, "Task two")
	if err != nil {
		t.Fatal(err)
	}
	project.Tasks = []*models.Task{task1, task2}
	project.BuildTaskTree()

	tomorrow := time.Now().AddDate(0, 0, 1)
	applyBulkDueDate(project, project.Tasks, &tomorrow)

	// Each task should have its own pointer (not sharing the same one)
	if task1.DueDate == task2.DueDate {
		t.Error("tasks should not share the same DueDate pointer")
	}
	// But the values should be equal
	if !task1.DueDate.Equal(*task2.DueDate) {
		t.Error("tasks should have equal DueDate values")
	}
}

func TestApplyBulkDueDate_ClearsDueDateOnNil(t *testing.T) {
	project, err := models.NewProject("proj-1", "TestProject", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	task, err := models.NewGenericTask("task-1", 1, "Task one")
	if err != nil {
		t.Fatal(err)
	}
	tomorrow := time.Now().AddDate(0, 0, 1)
	task.DueDate = &tomorrow
	project.Tasks = []*models.Task{task}
	project.BuildTaskTree()

	count := applyBulkDueDate(project, project.Tasks, nil)
	if count != 1 {
		t.Fatalf("expected 1 task changed, got %d", count)
	}
	if task.DueDate != nil {
		t.Error("DueDate should be nil after clearing")
	}
}
