package cmd

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/testutil"
)

func TestTasksToSelectorItems(t *testing.T) {
	t.Run("empty slice returns empty items", func(t *testing.T) {
		items := TasksToSelectorItems([]*models.Task{})
		if len(items) != 0 {
			t.Errorf("TasksToSelectorItems([]) length = %d, want 0", len(items))
		}
	})

	t.Run("tasks produce correct structure", func(t *testing.T) {
		task1, err := models.NewTask("uuid-1", 1, models.TaskTypeTask, "First Task")
		if err != nil {
			t.Fatal(err)
		}
		task1.WorkflowState = models.StateReady
		task2, err := models.NewTask("uuid-2", 42, models.TaskTypeBug, "Bug Report")
		if err != nil {
			t.Fatal(err)
		}
		task2.WorkflowState = models.StateInProgress

		items := TasksToSelectorItems([]*models.Task{task1, task2})

		if len(items) != 2 {
			t.Fatalf("TasksToSelectorItems() length = %d, want 2", len(items))
		}

		item := items[0]
		if item.ID != "uuid-1" {
			t.Errorf("item[0].ID = %q, want %q", item.ID, "uuid-1")
		}
		if item.Data != task1 {
			t.Error("item[0].Data does not reference original task")
		}
		if len(item.Headers) != 4 {
			t.Errorf("item[0].Headers length = %d, want 4", len(item.Headers))
		}
		if len(item.Columns) != 4 {
			t.Errorf("item[0].Columns length = %d, want 4", len(item.Columns))
		}
		if item.Columns[0] != "#1" {
			t.Errorf("item[0].Columns[0] = %q, want %q", item.Columns[0], "#1")
		}
		if item.Columns[3] != "First Task" {
			t.Errorf("item[0].Columns[3] = %q, want %q", item.Columns[3], "First Task")
		}

		if items[1].Columns[0] != "#42" {
			t.Errorf("item[1].Columns[0] = %q, want %q", items[1].Columns[0], "#42")
		}
	})

	t.Run("headers are consistent", func(t *testing.T) {
		task, err := models.NewTask("uuid-1", 1, models.TaskTypeTask, "Task")
		if err != nil {
			t.Fatal(err)
		}
		items := TasksToSelectorItems([]*models.Task{task})

		expectedHeaders := []string{"ID", "Type", "Status", "Title"}
		for i, h := range expectedHeaders {
			if items[0].Headers[i] != h {
				t.Errorf("Headers[%d] = %q, want %q", i, items[0].Headers[i], h)
			}
		}
	})
}

func TestProjectsToSelectorItems(t *testing.T) {
	t.Run("empty slice returns empty items", func(t *testing.T) {
		items := ProjectsToSelectorItems([]*models.Project{}, false)
		if len(items) != 0 {
			t.Errorf("ProjectsToSelectorItems([]) length = %d, want 0", len(items))
		}
	})

	t.Run("projects without active column", func(t *testing.T) {
		project := testutil.NewTestProject("My Project")
		project.Context = "work"

		items := ProjectsToSelectorItems([]*models.Project{project}, false)

		if len(items) != 1 {
			t.Fatalf("ProjectsToSelectorItems() length = %d, want 1", len(items))
		}

		item := items[0]
		if item.ID != project.ID {
			t.Errorf("item.ID = %q, want %q", item.ID, project.ID)
		}
		if item.Data != project {
			t.Error("item.Data does not reference original project")
		}
		if len(item.Headers) != 3 {
			t.Errorf("item.Headers length = %d, want 3 (without Active)", len(item.Headers))
		}
		if len(item.Columns) != 3 {
			t.Errorf("item.Columns length = %d, want 3 (without Active)", len(item.Columns))
		}
		if item.Columns[0] != "My Project" {
			t.Errorf("item.Columns[0] = %q, want %q", item.Columns[0], "My Project")
		}
		if item.Columns[1] != "work" {
			t.Errorf("item.Columns[1] = %q, want %q", item.Columns[1], "work")
		}
	})

	t.Run("projects with active column", func(t *testing.T) {
		project := testutil.NewTestProjectWithTasks("Test Project")

		items := ProjectsToSelectorItems([]*models.Project{project}, true)

		if len(items) != 1 {
			t.Fatalf("ProjectsToSelectorItems() length = %d, want 1", len(items))
		}

		item := items[0]
		if len(item.Headers) != 4 {
			t.Errorf("item.Headers length = %d, want 4 (with Active)", len(item.Headers))
		}
		if len(item.Columns) != 4 {
			t.Errorf("item.Columns length = %d, want 4 (with Active)", len(item.Columns))
		}
		if item.Headers[3] != "Active" {
			t.Errorf("item.Headers[3] = %q, want %q", item.Headers[3], "Active")
		}
	})
}
