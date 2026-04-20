package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/viper"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

func TestRunUndo(t *testing.T) {
	t.Run("restores from undo history", func(t *testing.T) {
		tmpDir := t.TempDir()
		projectPath := filepath.Join(tmpDir, "test-project")
		if err := os.MkdirAll(projectPath, 0755); err != nil {
			t.Fatalf("failed to create project dir: %v", err)
		}

		originalContent := `# Project: Test Project
<!-- next_short_id: 2 -->

## Task: Original task
<!-- id: task-1, status: todo, priority: 3 -->
`
		currentContent := `# Project: Test Project
<!-- next_short_id: 3 -->

## Task: Modified task
<!-- id: task-1, status: done, priority: 1 -->

## Task: New task
<!-- id: task-2, status: todo, priority: 3 -->
`
		tasksPath := filepath.Join(projectPath, "tasks.md")

		// Write original content first so we can record a snapshot
		if err := os.WriteFile(tasksPath, []byte(originalContent), 0644); err != nil {
			t.Fatalf("failed to write tasks file: %v", err)
		}

		project := &models.Project{
			ID:   "proj-1",
			Name: "Test Project",
			Slug: "test-project",
			Path: projectPath,
		}

		// Record undo snapshot (captures original state), then write current content
		if err := storage.RecordUndoSnapshot(project, "test", "modified task", 10, nil); err != nil {
			t.Fatalf("failed to record undo snapshot: %v", err)
		}
		if err := os.WriteFile(tasksPath, []byte(currentContent), 0644); err != nil {
			t.Fatalf("failed to write modified tasks file: %v", err)
		}

		if err := storage.LoadProject(project); err != nil {
			t.Fatalf("failed to load project: %v", err)
		}

		cfg := &config.Config{
			DataDir:        tmpDir,
			CurrentProject: project.ID,
			Projects:       []*models.Project{project},
		}

		configPath := filepath.Join(tmpDir, "config.json")
		manager := config.NewManager(configPath)
		if err := manager.Save(cfg); err != nil {
			t.Fatalf("failed to save config: %v", err)
		}

		viper.Set("config", configPath)
		defer viper.Set("config", "")

		oldRepo := storage.DefaultRepository
		storage.DefaultRepository = storage.NewFileSystemRepository()
		defer func() { storage.DefaultRepository = oldRepo }()

		tasksBefore := len(project.Tasks)
		if tasksBefore != 2 {
			t.Fatalf("expected 2 tasks before undo, got %d", tasksBefore)
		}

		err := runUndo(nil, nil)
		if err != nil {
			t.Fatalf("runUndo failed: %v", err)
		}

		restoredContent, err := os.ReadFile(tasksPath)
		if err != nil {
			t.Fatalf("failed to read restored tasks file: %v", err)
		}

		if !strings.Contains(string(restoredContent), "Original task") {
			t.Error("restored content should contain 'Original task'")
		}
		if strings.Contains(string(restoredContent), "New task") {
			t.Error("restored content should not contain 'New task'")
		}
	})

	t.Run("returns error when no backup exists", func(t *testing.T) {
		tmpDir := t.TempDir()
		projectPath := filepath.Join(tmpDir, "test-project")
		if err := os.MkdirAll(projectPath, 0755); err != nil {
			t.Fatalf("failed to create project dir: %v", err)
		}

		tasksContent := `# Project: Test Project
<!-- next_short_id: 2 -->

## Task: Some task
<!-- id: task-1, status: todo, priority: 3 -->
`
		tasksPath := filepath.Join(projectPath, "tasks.md")
		if err := os.WriteFile(tasksPath, []byte(tasksContent), 0644); err != nil {
			t.Fatalf("failed to write tasks file: %v", err)
		}

		project := &models.Project{
			ID:   "proj-1",
			Name: "Test Project",
			Slug: "test-project",
			Path: projectPath,
		}

		cfg := &config.Config{
			DataDir:        tmpDir,
			CurrentProject: project.ID,
			Projects:       []*models.Project{project},
		}

		configPath := filepath.Join(tmpDir, "config.json")
		manager := config.NewManager(configPath)
		if err := manager.Save(cfg); err != nil {
			t.Fatalf("failed to save config: %v", err)
		}

		viper.Set("config", configPath)
		defer viper.Set("config", "")

		oldRepo := storage.DefaultRepository
		storage.DefaultRepository = storage.NewFileSystemRepository()
		defer func() { storage.DefaultRepository = oldRepo }()

		err := runUndo(nil, nil)
		if err == nil {
			t.Fatal("expected error when no backup exists")
		}

		if !strings.Contains(err.Error(), "no undo history available") {
			t.Errorf("error should mention 'no undo history available', got: %v", err)
		}
	})

	t.Run("undo reapplies side effects and saves", func(t *testing.T) {
		tmpDir := t.TempDir()
		projectPath := filepath.Join(tmpDir, "test-project")
		if err := os.MkdirAll(projectPath, 0755); err != nil {
			t.Fatalf("failed to create project dir: %v", err)
		}

		// Snapshot content: completed task appears before uncompleted task.
		// After side effects (ReorderRootTasksByCompletion), the uncompleted
		// task should be moved before the completed one.
		snapshotContent := `# Project: Test Project

## Task: Completed task
<!-- id: task-1, status: done, priority: 3, created: 2025-01-01 10:00, completed: 2025-01-02 10:00 -->

## Task: Uncompleted task
<!-- id: task-2, status: todo, priority: 3, created: 2025-01-01 10:00 -->
`
		currentContent := `# Project: Test Project

## Task: Uncompleted task
<!-- id: task-2, status: todo, priority: 3, created: 2025-01-01 10:00 -->
`
		tasksPath := filepath.Join(projectPath, "tasks.md")

		// Write snapshot content first so we can record it
		if err := os.WriteFile(tasksPath, []byte(snapshotContent), 0644); err != nil {
			t.Fatalf("failed to write tasks file: %v", err)
		}

		project := &models.Project{
			ID:   "proj-1",
			Name: "Test Project",
			Slug: "test-project",
			Path: projectPath,
		}

		if err := storage.RecordUndoSnapshot(project, "test", "", 10, nil); err != nil {
			t.Fatalf("failed to record undo snapshot: %v", err)
		}
		if err := os.WriteFile(tasksPath, []byte(currentContent), 0644); err != nil {
			t.Fatalf("failed to write modified tasks file: %v", err)
		}
		if err := storage.LoadProject(project); err != nil {
			t.Fatalf("failed to load project: %v", err)
		}

		cfg := &config.Config{
			DataDir:        tmpDir,
			CurrentProject: project.ID,
			Projects:       []*models.Project{project},
		}
		configPath := filepath.Join(tmpDir, "config.json")
		manager := config.NewManager(configPath)
		if err := manager.Save(cfg); err != nil {
			t.Fatalf("failed to save config: %v", err)
		}

		viper.Set("config", configPath)
		defer viper.Set("config", "")

		oldRepo := storage.DefaultRepository
		storage.DefaultRepository = storage.NewFileSystemRepository()
		defer func() { storage.DefaultRepository = oldRepo }()

		if err := runUndo(nil, nil); err != nil {
			t.Fatalf("runUndo failed: %v", err)
		}

		// Read the saved file and verify reorder happened:
		// uncompleted task should appear before completed task.
		restoredContent, err := os.ReadFile(tasksPath)
		if err != nil {
			t.Fatalf("failed to read restored tasks file: %v", err)
		}
		content := string(restoredContent)
		uncompletedIdx := strings.Index(content, "Uncompleted task")
		completedIdx := strings.Index(content, "Completed task")
		if uncompletedIdx == -1 || completedIdx == -1 {
			t.Fatalf("expected both tasks in restored content, got:\n%s", content)
		}
		if uncompletedIdx > completedIdx {
			t.Errorf("expected uncompleted task before completed task after side effects reapplied; got uncompleted at %d, completed at %d", uncompletedIdx, completedIdx)
		}
	})

	t.Run("undo does not silently redo when undo stack is empty", func(t *testing.T) {
		tmpDir := t.TempDir()
		projectPath := filepath.Join(tmpDir, "test-project")
		if err := os.MkdirAll(projectPath, 0755); err != nil {
			t.Fatalf("failed to create project dir: %v", err)
		}

		tasksContent := `# Project: Test Project
<!-- next_short_id: 2 -->

## Task: Some task
<!-- id: task-1, status: todo, priority: 3 -->
`
		tasksPath := filepath.Join(projectPath, "tasks.md")
		if err := os.WriteFile(tasksPath, []byte(tasksContent), 0644); err != nil {
			t.Fatalf("failed to write tasks file: %v", err)
		}

		project := &models.Project{
			ID:   "proj-1",
			Name: "Test Project",
			Slug: "test-project",
			Path: projectPath,
		}

		// Record a snapshot, mutate, then undo to create redo entries
		if err := storage.RecordUndoSnapshot(project, "test", "", 10, nil); err != nil {
			t.Fatalf("failed to record snapshot: %v", err)
		}
		if err := os.WriteFile(tasksPath, []byte(tasksContent), 0644); err != nil {
			t.Fatalf("failed to write tasks file: %v", err)
		}

		cfg := &config.Config{
			DataDir:        tmpDir,
			CurrentProject: project.ID,
			Projects:       []*models.Project{project},
		}

		configPath := filepath.Join(tmpDir, "config.json")
		manager := config.NewManager(configPath)
		if err := manager.Save(cfg); err != nil {
			t.Fatalf("failed to save config: %v", err)
		}

		viper.Set("config", configPath)
		defer viper.Set("config", "")

		oldRepo := storage.DefaultRepository
		storage.DefaultRepository = storage.NewFileSystemRepository()
		defer func() { storage.DefaultRepository = oldRepo }()

		// First undo succeeds (consumes the undo entry, creates redo)
		if err := runUndo(nil, nil); err != nil {
			t.Fatalf("first undo failed: %v", err)
		}

		// Second undo should fail, not silently redo
		err := runUndo(nil, nil)
		if err == nil {
			t.Fatal("expected error when undo stack is empty, got nil")
		}
		if !strings.Contains(err.Error(), "no undo history available") {
			t.Errorf("expected 'no undo history available' error, got: %v", err)
		}
	})
}
