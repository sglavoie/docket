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
