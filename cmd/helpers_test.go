package cmd

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/testutil"
)

func TestFindTaskByIdentifier(t *testing.T) {
	project := testutil.NewTestProject("Test Project")

	task1 := models.NewTask("uuid-1", 1, models.TaskTypeTask, "First Task")
	task2 := models.NewTask("uuid-2", 42, models.TaskTypeTask, "Second Task")
	task3 := models.NewTask("uuid-3", 100, models.TaskTypeBug, "Bug Report")
	project.Tasks = []*models.Task{task1, task2, task3}

	tests := []struct {
		name       string
		identifier string
		wantTask   *models.Task
	}{
		{
			name:       "find by short ID",
			identifier: "1",
			wantTask:   task1,
		},
		{
			name:       "find by short ID larger number",
			identifier: "42",
			wantTask:   task2,
		},
		{
			name:       "find by UUID",
			identifier: "uuid-2",
			wantTask:   task2,
		},
		{
			name:       "find by title prefix case insensitive",
			identifier: "bug",
			wantTask:   task3,
		},
		{
			name:       "find by exact title prefix",
			identifier: "First",
			wantTask:   task1,
		},
		{
			name:       "find by longer title prefix",
			identifier: "Second Ta",
			wantTask:   task2,
		},
		{
			name:       "not found returns nil",
			identifier: "nonexistent",
			wantTask:   nil,
		},
		{
			name:       "short ID not found returns nil",
			identifier: "999",
			wantTask:   nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FindTaskByIdentifier(project, tt.identifier)
			if got != tt.wantTask {
				if tt.wantTask == nil {
					t.Errorf("FindTaskByIdentifier(%q) = %v, want nil", tt.identifier, got)
				} else if got == nil {
					t.Errorf("FindTaskByIdentifier(%q) = nil, want task %q", tt.identifier, tt.wantTask.Title)
				} else {
					t.Errorf("FindTaskByIdentifier(%q) = %q, want %q", tt.identifier, got.Title, tt.wantTask.Title)
				}
			}
		})
	}
}

func TestParseTaskTypeFlags(t *testing.T) {
	tests := []struct {
		name     string
		epic     bool
		story    bool
		bug      bool
		idea     bool
		note     bool
		typeFlag string
		want     models.TaskType
		wantErr  bool
	}{
		{
			name: "default is task",
			want: models.TaskTypeTask,
		},
		{
			name: "epic flag",
			epic: true,
			want: models.TaskTypeEpic,
		},
		{
			name:  "story flag",
			story: true,
			want:  models.TaskTypeStory,
		},
		{
			name: "bug flag",
			bug:  true,
			want: models.TaskTypeBug,
		},
		{
			name: "idea flag",
			idea: true,
			want: models.TaskTypeIdea,
		},
		{
			name: "note flag",
			note: true,
			want: models.TaskTypeNote,
		},
		{
			name:     "type flag epic",
			typeFlag: "epic",
			want:     models.TaskTypeEpic,
		},
		{
			name:     "type flag story",
			typeFlag: "story",
			want:     models.TaskTypeStory,
		},
		{
			name:     "type flag task",
			typeFlag: "task",
			want:     models.TaskTypeTask,
		},
		{
			name:     "type flag bug",
			typeFlag: "bug",
			want:     models.TaskTypeBug,
		},
		{
			name:     "type flag idea",
			typeFlag: "idea",
			want:     models.TaskTypeIdea,
		},
		{
			name:     "type flag note",
			typeFlag: "note",
			want:     models.TaskTypeNote,
		},
		{
			name:     "invalid type flag",
			typeFlag: "invalid",
			wantErr:  true,
		},
		{
			name:     "boolean flag takes precedence over type flag",
			epic:     true,
			typeFlag: "bug",
			want:     models.TaskTypeEpic,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseTaskTypeFlags(tt.epic, tt.story, tt.bug, tt.idea, tt.note, tt.typeFlag)

			if (err != nil) != tt.wantErr {
				t.Errorf("ParseTaskTypeFlags() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && got != tt.want {
				t.Errorf("ParseTaskTypeFlags() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFindTaskByIdentifierEmptyProject(t *testing.T) {
	project := testutil.NewTestProject("Empty Project")

	got := FindTaskByIdentifier(project, "1")
	if got != nil {
		t.Errorf("FindTaskByIdentifier on empty project should return nil, got %v", got)
	}
}

func TestFindTaskByIdentifierTitleMatchOrder(t *testing.T) {
	project := testutil.NewTestProject("Test Project")

	task1 := models.NewTask("uuid-1", 1, models.TaskTypeTask, "Apple Pie Recipe")
	task2 := models.NewTask("uuid-2", 2, models.TaskTypeTask, "Apple Crumble Recipe")
	project.Tasks = []*models.Task{task1, task2}

	// Should find the first matching task
	got := FindTaskByIdentifier(project, "apple")
	if got != task1 {
		t.Errorf("FindTaskByIdentifier should return first matching task, got %v", got.Title)
	}
}

func TestTasksToSelectorItems(t *testing.T) {
	t.Run("empty slice returns empty items", func(t *testing.T) {
		items := TasksToSelectorItems([]*models.Task{})
		if len(items) != 0 {
			t.Errorf("TasksToSelectorItems([]) length = %d, want 0", len(items))
		}
	})

	t.Run("tasks produce correct structure", func(t *testing.T) {
		task1 := models.NewTask("uuid-1", 1, models.TaskTypeTask, "First Task")
		task1.Status = models.StatusTodo
		task2 := models.NewTask("uuid-2", 42, models.TaskTypeBug, "Bug Report")
		task2.Status = models.StatusInProgress

		items := TasksToSelectorItems([]*models.Task{task1, task2})

		if len(items) != 2 {
			t.Fatalf("TasksToSelectorItems() length = %d, want 2", len(items))
		}

		// Check first item
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

		// Check second item short ID format
		if items[1].Columns[0] != "#42" {
			t.Errorf("item[1].Columns[0] = %q, want %q", items[1].Columns[0], "#42")
		}
	})

	t.Run("headers are consistent", func(t *testing.T) {
		task := models.NewTask("uuid-1", 1, models.TaskTypeTask, "Task")
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
