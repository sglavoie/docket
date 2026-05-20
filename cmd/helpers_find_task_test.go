package cmd

import (
	"errors"
	"testing"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/testutil"
	"github.com/sglavoie/tt/internal/tterrors"
)

func TestFindTaskByIdentifier(t *testing.T) {
	project := testutil.NewTestProject("Test Project")

	task1, err := models.NewTask("uuid-1", 1, models.TaskTypeTask, "First Task")
	if err != nil {
		t.Fatal(err)
	}
	task2, err := models.NewTask("uuid-2", 42, models.TaskTypeTask, "Second Task")
	if err != nil {
		t.Fatal(err)
	}
	task3, err := models.NewTask("uuid-3", 100, models.TaskTypeBug, "Bug Report")
	if err != nil {
		t.Fatal(err)
	}
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
			got, err := FindTaskByIdentifier(project, tt.identifier)
			if tt.wantTask == nil {
				if err == nil {
					t.Errorf("FindTaskByIdentifier(%q) = %v, want error", tt.identifier, got)
				}
			} else {
				if err != nil {
					t.Errorf("FindTaskByIdentifier(%q) returned unexpected error: %v", tt.identifier, err)
				} else if got != tt.wantTask {
					t.Errorf("FindTaskByIdentifier(%q) = %q, want %q", tt.identifier, got.Title, tt.wantTask.Title)
				}
			}
		})
	}
}

func TestFindTaskByIdentifierEmptyProject(t *testing.T) {
	project := testutil.NewTestProject("Empty Project")

	got, err := FindTaskByIdentifier(project, "1")
	if err == nil {
		t.Errorf("FindTaskByIdentifier on empty project should return error, got %v", got)
	}
}

func TestFindTaskByIdentifierTitleMatchAmbiguous(t *testing.T) {
	project := testutil.NewTestProject("Test Project")

	task1, err := models.NewTask("uuid-1", 1, models.TaskTypeTask, "Apple Pie Recipe")
	if err != nil {
		t.Fatal(err)
	}
	task2, err := models.NewTask("uuid-2", 2, models.TaskTypeTask, "Apple Crumble Recipe")
	if err != nil {
		t.Fatal(err)
	}
	project.Tasks = []*models.Task{task1, task2}

	_, findErr := FindTaskByIdentifier(project, "apple")
	if findErr == nil {
		t.Error("FindTaskByIdentifier should return error for ambiguous prefix")
	}
	if !errors.Is(findErr, tterrors.ErrAmbiguousIdentifier) {
		t.Errorf("expected ErrAmbiguousIdentifier, got: %v", findErr)
	}

	got, findErr := FindTaskByIdentifier(project, "Apple Pie")
	if findErr != nil {
		t.Errorf("FindTaskByIdentifier returned unexpected error: %v", findErr)
	}
	if got != task1 {
		t.Errorf("FindTaskByIdentifier should return task1, got %v", got)
	}
}

func TestFindActionTaskByID(t *testing.T) {
	project := testutil.NewTestProject("Test Project")

	prefixTask, err := models.NewTask("123abcde-0000-0000-0000-000000000000", 42, models.TaskTypeTask, "Prefix Task")
	if err != nil {
		t.Fatal(err)
	}
	shortIDTask, err := models.NewTask("other-uuid", 123, models.TaskTypeTask, "Short ID Task")
	if err != nil {
		t.Fatal(err)
	}
	project.Tasks = []*models.Task{prefixTask, shortIDTask}
	project.NextShortID = 124

	repo := testutil.NewMockRepository()
	if err := repo.Save(project); err != nil {
		t.Fatal(err)
	}

	oldRepo := storage.DefaultRepository
	storage.DefaultRepository = repo
	defer func() { storage.DefaultRepository = oldRepo }()

	cfg := &config.Config{
		Projects: []*models.Project{project},
	}

	t.Run("mixed numeric prefix matches UUID prefix", func(t *testing.T) {
		gotTask, gotProject := FindActionTaskByID(cfg, "123abc")
		if gotTask == nil {
			t.Fatal("expected task match for UUID prefix")
		}
		if gotTask.ID != prefixTask.ID {
			t.Fatalf("FindActionTaskByID returned %q, want %q", gotTask.ID, prefixTask.ID)
		}
		if gotProject != project {
			t.Fatal("expected matching project to be returned")
		}
	})

	t.Run("pure numeric query still matches short ID", func(t *testing.T) {
		gotTask, gotProject := FindActionTaskByID(cfg, "123")
		if gotTask == nil {
			t.Fatal("expected task match for short ID")
		}
		if gotTask.ID != shortIDTask.ID {
			t.Fatalf("FindActionTaskByID returned %q, want %q", gotTask.ID, shortIDTask.ID)
		}
		if gotProject != project {
			t.Fatal("expected matching project to be returned")
		}
	})

	// Regression: an empty query previously matched the first task in the
	// first loadable project because HasPrefix("", "") is true. Guard against
	// returning arbitrary tasks for what is effectively a missing argument.
	t.Run("empty query returns no match", func(t *testing.T) {
		gotTask, gotProject := FindActionTaskByID(cfg, "")
		if gotTask != nil || gotProject != nil {
			t.Fatalf("FindActionTaskByID(\"\") should return (nil, nil); got %v / %v", gotTask, gotProject)
		}
	})
}
