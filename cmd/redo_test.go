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

func TestRunRedo(t *testing.T) {
	t.Run("redo reapplies side effects and saves", func(t *testing.T) {
		tmpDir := t.TempDir()
		projectPath := filepath.Join(tmpDir, "test-project")
		if err := os.MkdirAll(projectPath, 0755); err != nil {
			t.Fatalf("failed to create project dir: %v", err)
		}

		// Original content: properly ordered (uncompleted before completed).
		originalContent := `# Project: Test Project

## Task: Uncompleted task
<!-- id: task-2, status: todo, priority: 3, created: 2025-01-01 10:00 -->

## Task: Completed task
<!-- id: task-1, status: done, priority: 3, created: 2025-01-01 10:00, completed: 2025-01-02 10:00 -->
`
		// Modified content: completed task appears first (bad order).
		// After redo restores this and side effects run, reorder should fix it.
		modifiedContent := `# Project: Test Project

## Task: Completed task
<!-- id: task-1, status: done, priority: 3, created: 2025-01-01 10:00, completed: 2025-01-02 10:00 -->

## Task: Active task
<!-- id: task-3, status: todo, priority: 3, created: 2025-01-01 10:00 -->
`
		tasksPath := filepath.Join(projectPath, "tasks.md")

		// Step 1: Write original content, record undo snapshot
		if err := os.WriteFile(tasksPath, []byte(originalContent), 0644); err != nil {
			t.Fatalf("failed to write original tasks file: %v", err)
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

		// Step 2: Write modified content (simulates a mutation)
		if err := os.WriteFile(tasksPath, []byte(modifiedContent), 0644); err != nil {
			t.Fatalf("failed to write modified tasks file: %v", err)
		}

		// Step 3: Perform undo (restores original, creates redo entry for modified)
		if _, err := storage.PerformUndo(project); err != nil {
			t.Fatalf("PerformUndo failed: %v", err)
		}

		// Step 4: Set up config for CLI command
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

		// Step 5: Run redo (restores modified content + side effects)
		if err := runRedo(nil, nil); err != nil {
			t.Fatalf("runRedo failed: %v", err)
		}

		// Read the saved file — side effects should have reordered
		restoredContent, err := os.ReadFile(tasksPath)
		if err != nil {
			t.Fatalf("failed to read restored tasks file: %v", err)
		}
		content := string(restoredContent)

		// Both tasks from the redo state should be present
		if !strings.Contains(content, "Active task") {
			t.Error("redo content should contain 'Active task'")
		}
		if !strings.Contains(content, "Completed task") {
			t.Error("redo content should contain 'Completed task'")
		}

		// After side effects: uncompleted task should appear before completed
		activeIdx := strings.Index(content, "Active task")
		completedIdx := strings.Index(content, "Completed task")
		if activeIdx > completedIdx {
			t.Errorf("expected active task before completed task after side effects; active at %d, completed at %d", activeIdx, completedIdx)
		}
	})

	t.Run("returns error when no redo history", func(t *testing.T) {
		tmpDir := t.TempDir()
		projectPath := filepath.Join(tmpDir, "test-project")
		if err := os.MkdirAll(projectPath, 0755); err != nil {
			t.Fatalf("failed to create project dir: %v", err)
		}

		tasksContent := `# Project: Test Project

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

		err := runRedo(nil, nil)
		if err == nil {
			t.Fatal("expected error when no redo history exists")
		}
		if !strings.Contains(err.Error(), "no redo history available") {
			t.Errorf("error should mention 'no redo history available', got: %v", err)
		}
	})
}
