package cmd

import (
	"testing"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/testutil"
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
			name:     "invalid type flag",
			typeFlag: "invalid",
			wantErr:  true,
		},
		{
			name:     "conflicting boolean flag and type flag",
			epic:     true,
			typeFlag: "bug",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseTaskTypeFlags(tt.epic, tt.story, tt.bug, tt.idea, tt.typeFlag)

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

	task1, err := models.NewTask("uuid-1", 1, models.TaskTypeTask, "Apple Pie Recipe")
	if err != nil {
		t.Fatal(err)
	}
	task2, err := models.NewTask("uuid-2", 2, models.TaskTypeTask, "Apple Crumble Recipe")
	if err != nil {
		t.Fatal(err)
	}
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
		task1, err := models.NewTask("uuid-1", 1, models.TaskTypeTask, "First Task")
		if err != nil {
			t.Fatal(err)
		}
		task1.Status = models.StatusTodo
		task2, err := models.NewTask("uuid-2", 42, models.TaskTypeBug, "Bug Report")
		if err != nil {
			t.Fatal(err)
		}
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

func TestSaveProject(t *testing.T) {
	t.Run("basic save without options", func(t *testing.T) {
		deps, project := testutil.SetupTestDepsWithProject(t, "Test Project")
		defer deps.Cleanup()

		// Set up storage to use the mock repository
		oldStorageRepo := storage.DefaultRepository
		storage.DefaultRepository = deps.Repository
		defer func() { storage.DefaultRepository = oldStorageRepo }()

		err := SaveProject(project, nil)
		testutil.AssertNoError(t, err)

		// Verify project was saved
		if deps.Repository.Projects[project.ID] == nil {
			t.Error("SaveProject() did not save project to repository")
		}
	})

	t.Run("save with rules but no config returns error", func(t *testing.T) {
		deps, project := testutil.SetupTestDepsWithProject(t, "Test Project")
		defer deps.Cleanup()

		// Set up storage to use the mock repository
		oldStorageRepo := storage.DefaultRepository
		storage.DefaultRepository = deps.Repository
		defer func() { storage.DefaultRepository = oldStorageRepo }()

		opts := &SaveOptions{
			EvaluateRules: true,
			Config:        nil, // Missing required config
		}

		err := SaveProject(project, opts)
		if err == nil {
			t.Error("SaveProject() should error when Config is nil but EvaluateRules is true")
		}
	})

	t.Run("save with rules but no config manager returns error", func(t *testing.T) {
		deps, project := testutil.SetupTestDepsWithProject(t, "Test Project")
		defer deps.Cleanup()

		// Set up storage to use the mock repository
		oldStorageRepo := storage.DefaultRepository
		storage.DefaultRepository = deps.Repository
		defer func() { storage.DefaultRepository = oldStorageRepo }()

		opts := &SaveOptions{
			EvaluateRules: true,
			Config:        deps.Config,
			ConfigManager: nil, // Missing required manager
		}

		err := SaveProject(project, opts)
		if err == nil {
			t.Error("SaveProject() should error when ConfigManager is nil but EvaluateRules is true")
		}
	})

	t.Run("save with rules evaluates successfully", func(t *testing.T) {
		deps, project := testutil.SetupTestDepsWithProject(t, "Test Project")
		defer deps.Cleanup()

		manager := config.NewManager(deps.ConfigPath)

		// Set up storage to use the mock repository
		oldStorageRepo := storage.DefaultRepository
		storage.DefaultRepository = deps.Repository
		defer func() { storage.DefaultRepository = oldStorageRepo }()

		opts := &SaveOptions{
			EvaluateRules: true,
			Config:        deps.Config,
			ConfigManager: manager,
		}

		err := SaveProject(project, opts)
		testutil.AssertNoError(t, err)

		// Verify project was saved
		if deps.Repository.Projects[project.ID] == nil {
			t.Error("SaveProject() did not save project to repository")
		}
	})

	// Note: Error handling tests for backup/save failures are not included
	// because storage.BackupProject and storage.SaveProject don't go through
	// the repository pattern - they use filesystem functions directly.
	// This would require refactoring the storage layer to fully support mocking.
}

func TestCaptureTaskStates(t *testing.T) {
	t.Run("captures task states", func(t *testing.T) {
		task1 := testutil.NewTestTaskWithStatus("Task 1", models.StatusTodo)
		task1.Priority = 2
		task1.Tags = []string{"urgent", "backend"}

		task2 := testutil.NewTestTaskWithStatus("Task 2", models.StatusInProgress)
		task2.Section = "In Progress"

		tasks := []*models.Task{task1, task2}
		states := CaptureTaskStates(tasks)

		if len(states) != 2 {
			t.Fatalf("CaptureTaskStates() captured %d states, want 2", len(states))
		}

		// Check task1 state
		state1 := states[task1.ID]
		if state1 == nil {
			t.Fatal("task1 state not captured")
		}
		if state1.Status != task1.Status {
			t.Errorf("state1.Status = %s, want %s", state1.Status, task1.Status)
		}
		if state1.Priority != task1.Priority {
			t.Errorf("state1.Priority = %d, want %d", state1.Priority, task1.Priority)
		}
		if len(state1.Tags) != 2 {
			t.Errorf("state1.Tags length = %d, want 2", len(state1.Tags))
		}

		// Verify it's a copy, not the same instance
		if state1 == task1 {
			t.Error("CaptureTaskStates() should create copies, not reference originals")
		}

		// Check task2 state
		state2 := states[task2.ID]
		if state2 == nil {
			t.Fatal("task2 state not captured")
		}
		if state2.Section != task2.Section {
			t.Errorf("state2.Section = %s, want %s", state2.Section, task2.Section)
		}
	})

	t.Run("handles empty task list", func(t *testing.T) {
		states := CaptureTaskStates([]*models.Task{})
		if len(states) != 0 {
			t.Errorf("CaptureTaskStates([]) should return empty map, got %d states", len(states))
		}
	})

	t.Run("modifying original tasks does not affect captured states", func(t *testing.T) {
		task := testutil.NewTestTaskWithStatus("Task", models.StatusTodo)
		task.Priority = 3
		task.Tags = []string{"original"}

		states := CaptureTaskStates([]*models.Task{task})
		capturedState := states[task.ID]

		// Modify original task
		task.Priority = 5
		task.Tags = []string{"modified"}

		// Captured state should be unchanged
		if capturedState.Priority != 3 {
			t.Errorf("captured state Priority = %d, want 3 (should not change)", capturedState.Priority)
		}
		if len(capturedState.Tags) != 1 || capturedState.Tags[0] != "original" {
			t.Errorf("captured state Tags = %v, want [original] (should not change)", capturedState.Tags)
		}
	})
}

func TestBuildEvaluationContext(t *testing.T) {
	t.Run("builds context with previous states", func(t *testing.T) {
		task1 := testutil.NewTestTaskWithStatus("Task 1", models.StatusTodo)
		task2 := testutil.NewTestTaskWithStatus("Task 2", models.StatusInProgress)

		prevTask1 := testutil.NewTestTaskWithStatus("Task 1", models.StatusInProgress)
		prevTask1.ID = task1.ID

		prevStates := map[string]*models.Task{
			task1.ID: prevTask1,
			// task2 has no previous state (new task)
		}

		tasks := []*models.Task{task1, task2}
		ctx := BuildEvaluationContext(tasks, prevStates, "TestProject", "proj-123")

		if ctx == nil {
			t.Fatal("BuildEvaluationContext() returned nil")
		}

		if ctx.ProjectName != "TestProject" {
			t.Errorf("ctx.ProjectName = %s, want TestProject", ctx.ProjectName)
		}
		if ctx.ProjectID != "proj-123" {
			t.Errorf("ctx.ProjectID = %s, want proj-123", ctx.ProjectID)
		}

		if len(ctx.Changes) != 2 {
			t.Fatalf("ctx.Changes length = %d, want 2", len(ctx.Changes))
		}

		// Check task1 change context
		change1 := ctx.Changes[0]
		if change1.Task.ID != task1.ID {
			t.Errorf("change1.Task.ID = %s, want %s", change1.Task.ID, task1.ID)
		}
		if change1.Previous == nil {
			t.Error("change1.Previous should not be nil (has previous state)")
		} else if change1.Previous.Status != models.StatusInProgress {
			t.Errorf("change1.Previous.Status = %s, want %s", change1.Previous.Status, models.StatusInProgress)
		}

		// Check task2 change context (new task)
		change2 := ctx.Changes[1]
		if change2.Task.ID != task2.ID {
			t.Errorf("change2.Task.ID = %s, want %s", change2.Task.ID, task2.ID)
		}
		if change2.Previous != nil {
			t.Error("change2.Previous should be nil (new task)")
		}

		// Check AllTasks
		if len(ctx.AllTasks) != 2 {
			t.Errorf("ctx.AllTasks length = %d, want 2", len(ctx.AllTasks))
		}

		// Check TaskMap
		if len(ctx.TaskMap) != 2 {
			t.Errorf("ctx.TaskMap length = %d, want 2", len(ctx.TaskMap))
		}
		if ctx.TaskMap[task1.ID] != task1 {
			t.Error("ctx.TaskMap[task1.ID] does not reference task1")
		}
	})

	t.Run("handles empty previous states", func(t *testing.T) {
		task := testutil.NewTestTaskWithStatus("Task", models.StatusTodo)
		tasks := []*models.Task{task}

		ctx := BuildEvaluationContext(tasks, map[string]*models.Task{}, "", "")

		if len(ctx.Changes) != 1 {
			t.Fatalf("ctx.Changes length = %d, want 1", len(ctx.Changes))
		}

		if ctx.Changes[0].Previous != nil {
			t.Error("ctx.Changes[0].Previous should be nil when no previous state")
		}
	})
}
