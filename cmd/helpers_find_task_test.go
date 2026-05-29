package cmd

import (
	"errors"
	"strings"
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

// Regression: an empty identifier previously matched an arbitrary task because
// strings.HasPrefix(title, "") is always true. With a single task it would
// silently succeed; with multiple it produced a misleading "ambiguous" error.
// Empty input must yield ErrTaskNotFound so callers can surface a clear error.
func TestFindTaskByIdentifierEmptyIdentifier(t *testing.T) {
	project := testutil.NewTestProject("Test Project")

	task1, err := models.NewTask("uuid-1", 1, models.TaskTypeTask, "First Task")
	if err != nil {
		t.Fatal(err)
	}
	task2, err := models.NewTask("uuid-2", 2, models.TaskTypeTask, "Second Task")
	if err != nil {
		t.Fatal(err)
	}
	project.Tasks = []*models.Task{task1, task2}

	got, findErr := FindTaskByIdentifier(project, "")
	if findErr == nil {
		t.Fatalf("FindTaskByIdentifier(\"\") should return error, got task %v", got)
	}
	if !errors.Is(findErr, tterrors.ErrTaskNotFound) {
		t.Errorf("expected ErrTaskNotFound, got: %v", findErr)
	}

	// Also verify single-task case where empty input would have silently matched.
	soloProject := testutil.NewTestProject("Solo")
	solo, err := models.NewTask("uuid-solo", 7, models.TaskTypeTask, "Solo Task")
	if err != nil {
		t.Fatal(err)
	}
	soloProject.Tasks = []*models.Task{solo}
	if _, findErr := FindTaskByIdentifier(soloProject, ""); findErr == nil {
		t.Errorf("FindTaskByIdentifier(\"\") on single-task project should error")
	}
}

// Regression: when a query matches one title exactly and is also a prefix of
// other titles, the exact match must win. Previously the ambiguity check fired
// even though the user typed the full title verbatim, forcing them to fall
// back to short ID.
func TestFindTaskByIdentifierExactMatchPreferred(t *testing.T) {
	project := testutil.NewTestProject("Test Project")

	exact, err := models.NewTask("uuid-1", 1, models.TaskTypeTask, "Apple Pie")
	if err != nil {
		t.Fatal(err)
	}
	longer, err := models.NewTask("uuid-2", 2, models.TaskTypeTask, "Apple Pie Recipe")
	if err != nil {
		t.Fatal(err)
	}
	project.Tasks = []*models.Task{exact, longer}

	got, findErr := FindTaskByIdentifier(project, "Apple Pie")
	if findErr != nil {
		t.Fatalf("expected exact match to win, got error: %v", findErr)
	}
	if got != exact {
		t.Fatalf("expected exact match (#%d), got #%d", exact.ShortID, got.ShortID)
	}

	// Case-insensitive exact match should still win.
	got, findErr = FindTaskByIdentifier(project, "apple pie")
	if findErr != nil {
		t.Fatalf("expected case-insensitive exact match to win, got error: %v", findErr)
	}
	if got != exact {
		t.Fatalf("case-insensitive: expected exact match (#%d), got #%d", exact.ShortID, got.ShortID)
	}

	// Two tasks with identical titles should still report ambiguity.
	dup, err := models.NewTask("uuid-3", 3, models.TaskTypeTask, "Apple Pie")
	if err != nil {
		t.Fatal(err)
	}
	project.Tasks = append(project.Tasks, dup)
	if _, findErr := FindTaskByIdentifier(project, "Apple Pie"); findErr == nil {
		t.Error("expected ambiguous error when two tasks share an exact title")
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

// Regression: UUIDs are stored lowercase but users may paste uppercase
// (e.g., from external issue trackers). FindTaskByIdentifier must resolve
// case-insensitively when the query looks like a full UUID.
func TestFindTaskByIdentifierUUIDCaseInsensitive(t *testing.T) {
	project := testutil.NewTestProject("Test Project")

	task, err := models.NewTask("abc123-def456", 1, models.TaskTypeTask, "Some Task")
	if err != nil {
		t.Fatal(err)
	}
	project.Tasks = []*models.Task{task}

	got, findErr := FindTaskByIdentifier(project, "ABC123-DEF456")
	if findErr != nil {
		t.Fatalf("FindTaskByIdentifier should match uppercase UUID: %v", findErr)
	}
	if got != task {
		t.Fatalf("expected task, got %v", got)
	}
}

// Regression: shell quoting or paste may include surrounding whitespace.
// FindTaskByIdentifier should trim before matching.
func TestFindTaskByIdentifierWhitespaceTrimmed(t *testing.T) {
	project := testutil.NewTestProject("Test Project")

	task, err := models.NewTask("uuid-trim", 5, models.TaskTypeTask, "Trim Task")
	if err != nil {
		t.Fatal(err)
	}
	project.Tasks = []*models.Task{task}

	got, findErr := FindTaskByIdentifier(project, "  uuid-trim  ")
	if findErr != nil {
		t.Fatalf("FindTaskByIdentifier should match after trimming whitespace: %v", findErr)
	}
	if got != task {
		t.Fatalf("expected task, got %v", got)
	}

	// Whitespace-only input is effectively empty and must error.
	_, findErr = FindTaskByIdentifier(project, "   ")
	if findErr == nil {
		t.Fatal("whitespace-only identifier should return error")
	}
	if !errors.Is(findErr, tterrors.ErrTaskNotFound) {
		t.Fatalf("expected ErrTaskNotFound, got: %v", findErr)
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
		gotTask, gotProject, err := FindActionTaskByID(cfg, "123abc")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
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
		gotTask, gotProject, err := FindActionTaskByID(cfg, "123")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
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
		gotTask, gotProject, err := FindActionTaskByID(cfg, "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if gotTask != nil || gotProject != nil {
			t.Fatalf("FindActionTaskByID(\"\") should return (nil, nil); got %v / %v", gotTask, gotProject)
		}
	})

	// Regression: a UUID prefix that matches more than one task across loaded
	// projects must return ErrAmbiguousIdentifier rather than silently picking
	// the first match (potentially acting on the wrong task).
	t.Run("ambiguous UUID prefix returns ambiguity error", func(t *testing.T) {
		ambigProject := testutil.NewTestProject("Ambig Project")
		taskA, terr := models.NewTask("abc11111-0000-0000-0000-000000000000", 1, models.TaskTypeTask, "Alpha")
		if terr != nil {
			t.Fatal(terr)
		}
		taskB, terr := models.NewTask("abc22222-0000-0000-0000-000000000000", 2, models.TaskTypeTask, "Beta")
		if terr != nil {
			t.Fatal(terr)
		}
		ambigProject.Tasks = []*models.Task{taskA, taskB}
		ambigProject.NextShortID = 3

		ambigRepo := testutil.NewMockRepository()
		if err := ambigRepo.Save(ambigProject); err != nil {
			t.Fatal(err)
		}
		swap := storage.DefaultRepository
		storage.DefaultRepository = ambigRepo
		defer func() { storage.DefaultRepository = swap }()

		ambigCfg := &config.Config{Projects: []*models.Project{ambigProject}}
		gotTask, gotProject, err := FindActionTaskByID(ambigCfg, "abc")
		if err == nil {
			t.Fatalf("expected ambiguity error, got task=%v project=%v", gotTask, gotProject)
		}
		if !errors.Is(err, tterrors.ErrAmbiguousIdentifier) {
			t.Fatalf("expected ErrAmbiguousIdentifier, got: %v", err)
		}
		if gotTask != nil || gotProject != nil {
			t.Fatalf("ambiguous lookup should not return a task; got %v / %v", gotTask, gotProject)
		}
	})

	t.Run("ambiguous short stored IDs do not panic", func(t *testing.T) {
		ambigProject := testutil.NewTestProject("Short ID Project")
		taskA, terr := models.NewTask("abc", 1, models.TaskTypeTask, "Alpha")
		if terr != nil {
			t.Fatal(terr)
		}
		taskB, terr := models.NewTask("abd", 2, models.TaskTypeTask, "Beta")
		if terr != nil {
			t.Fatal(terr)
		}
		ambigProject.Tasks = []*models.Task{taskA, taskB}
		ambigProject.NextShortID = 3

		ambigRepo := testutil.NewMockRepository()
		if err := ambigRepo.Save(ambigProject); err != nil {
			t.Fatal(err)
		}
		swap := storage.DefaultRepository
		storage.DefaultRepository = ambigRepo
		defer func() { storage.DefaultRepository = swap }()

		ambigCfg := &config.Config{Projects: []*models.Project{ambigProject}}
		gotTask, gotProject, err := FindActionTaskByID(ambigCfg, "ab")
		if err == nil {
			t.Fatalf("expected ambiguity error, got task=%v project=%v", gotTask, gotProject)
		}
		if !errors.Is(err, tterrors.ErrAmbiguousIdentifier) {
			t.Fatalf("expected ErrAmbiguousIdentifier, got: %v", err)
		}
		if !strings.Contains(err.Error(), "(abc)") || !strings.Contains(err.Error(), "(abd)") {
			t.Fatalf("expected short IDs in ambiguity error, got: %v", err)
		}
	})

	// Regression: UUIDs are stored lowercase but users may paste uppercase
	// from external tools. Resolution must be case-insensitive.
	t.Run("uppercase UUID prefix matches lowercase stored ID", func(t *testing.T) {
		gotTask, _, err := FindActionTaskByID(cfg, "123ABC")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if gotTask == nil || gotTask.ID != prefixTask.ID {
			t.Fatalf("expected case-insensitive UUID prefix match, got %v", gotTask)
		}
	})

	// Regression: shell paste / quoting often leaves whitespace around the
	// argument. Trim before resolving rather than failing to match.
	t.Run("whitespace around query is trimmed", func(t *testing.T) {
		gotTask, _, err := FindActionTaskByID(cfg, "  123abc  ")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if gotTask == nil || gotTask.ID != prefixTask.ID {
			t.Fatalf("expected trimmed query to match, got %v", gotTask)
		}
	})
}
