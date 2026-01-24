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
	t.Run("restores backup when backup exists", func(t *testing.T) {
		tmpDir := t.TempDir()
		projectPath := filepath.Join(tmpDir, "test-project")
		if err := os.MkdirAll(projectPath, 0755); err != nil {
			t.Fatalf("failed to create project dir: %v", err)
		}

		currentContent := `# Project: Test Project
<!-- next_short_id: 3 -->

## Task: Modified task
<!-- id: task-1, status: done, priority: 1 -->

## Task: New task
<!-- id: task-2, status: todo, priority: 3 -->
`
		backupContent := `# Project: Test Project
<!-- next_short_id: 2 -->

## Task: Original task
<!-- id: task-1, status: todo, priority: 3 -->
`
		tasksPath := filepath.Join(projectPath, "tasks.md")
		backupPath := filepath.Join(projectPath, ".tasks.md.bak")

		if err := os.WriteFile(tasksPath, []byte(currentContent), 0644); err != nil {
			t.Fatalf("failed to write tasks file: %v", err)
		}
		if err := os.WriteFile(backupPath, []byte(backupContent), 0644); err != nil {
			t.Fatalf("failed to write backup file: %v", err)
		}

		project := &models.Project{
			ID:   "proj-1",
			Name: "Test Project",
			Slug: "test-project",
			Path: projectPath,
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

		if !strings.Contains(err.Error(), "no backup available") {
			t.Errorf("error should mention 'no backup available', got: %v", err)
		}
	})

	t.Run("backup swap allows redo via second undo", func(t *testing.T) {
		tmpDir := t.TempDir()
		projectPath := filepath.Join(tmpDir, "test-project")
		if err := os.MkdirAll(projectPath, 0755); err != nil {
			t.Fatalf("failed to create project dir: %v", err)
		}

		stateA := `# Project: Test Project
<!-- next_short_id: 2 -->

## Task: State A task
<!-- id: task-1, status: todo, priority: 3 -->
`
		stateB := `# Project: Test Project
<!-- next_short_id: 2 -->

## Task: State B task
<!-- id: task-1, status: done, priority: 1 -->
`
		tasksPath := filepath.Join(projectPath, "tasks.md")
		backupPath := filepath.Join(projectPath, ".tasks.md.bak")

		if err := os.WriteFile(tasksPath, []byte(stateB), 0644); err != nil {
			t.Fatalf("failed to write tasks file: %v", err)
		}
		if err := os.WriteFile(backupPath, []byte(stateA), 0644); err != nil {
			t.Fatalf("failed to write backup file: %v", err)
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

		if err := runUndo(nil, nil); err != nil {
			t.Fatalf("first undo failed: %v", err)
		}

		afterFirstUndo, _ := os.ReadFile(tasksPath)
		if !strings.Contains(string(afterFirstUndo), "State A task") {
			t.Error("after first undo, should have State A content")
		}

		if err := runUndo(nil, nil); err != nil {
			t.Fatalf("second undo (redo) failed: %v", err)
		}

		afterSecondUndo, _ := os.ReadFile(tasksPath)
		if !strings.Contains(string(afterSecondUndo), "State B task") {
			t.Error("after second undo, should have State B content (redo)")
		}
	})
}
