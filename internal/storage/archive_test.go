package storage

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/sglavoie/tt/internal/models"
)

func TestWriteArchivedTasks(t *testing.T) {
	t.Run("generates correct output format for single task", func(t *testing.T) {
		now := time.Now()
		task := &models.Task{
			ID:          "task-123",
			Title:       "Test task",
			Type:        models.TaskTypeTask,
			Status:      models.StatusDone,
			Priority:    2,
			Tags:        []string{"backend", "urgent"},
			Section:     "Backlog",
			CreatedAt:   now.Add(-24 * time.Hour),
			CompletedAt: &now,
			Description: "Task description here.",
		}

		output := WriteArchivedTasks("My Project", []*models.Task{task})

		if !strings.Contains(output, "## Archived:") {
			t.Error("output should contain archive header")
		}
		if !strings.Contains(output, "<!-- project: My Project, count: 1 -->") {
			t.Error("output should contain project metadata")
		}
		if !strings.Contains(output, "### Task: Test task") {
			t.Error("output should contain task heading")
		}
		if !strings.Contains(output, "id: task-123") {
			t.Error("output should contain task ID")
		}
		if !strings.Contains(output, "status: done") {
			t.Error("output should contain task status")
		}
		if !strings.Contains(output, "priority: 2") {
			t.Error("output should contain task priority")
		}
		if !strings.Contains(output, "tags: backend;urgent") {
			t.Error("output should contain task tags")
		}
		if !strings.Contains(output, "section: Backlog") {
			t.Error("output should contain task section")
		}
		if !strings.Contains(output, "Task description here.") {
			t.Error("output should contain task description")
		}
	})

	t.Run("returns empty string for empty task list", func(t *testing.T) {
		output := WriteArchivedTasks("Project", []*models.Task{})
		if output != "" {
			t.Errorf("expected empty string for empty tasks, got: %s", output)
		}
	})

	t.Run("generates correct output for multiple tasks", func(t *testing.T) {
		now := time.Now()
		tasks := []*models.Task{
			{
				ID:          "task-1",
				Title:       "First task",
				Type:        models.TaskTypeTask,
				Status:      models.StatusDone,
				Priority:    1,
				CompletedAt: &now,
			},
			{
				ID:          "task-2",
				Title:       "Second task",
				Type:        models.TaskTypeBug,
				Status:      models.StatusDone,
				Priority:    2,
				CompletedAt: &now,
			},
		}

		output := WriteArchivedTasks("Project", tasks)

		if !strings.Contains(output, "count: 2") {
			t.Error("output should show count of 2")
		}
		if !strings.Contains(output, "### Task: First task") {
			t.Error("output should contain first task")
		}
		if !strings.Contains(output, "### Bug: Second task") {
			t.Error("output should contain second task")
		}
	})

	t.Run("includes due date when present", func(t *testing.T) {
		now := time.Now()
		dueDate := now.Add(24 * time.Hour)
		task := &models.Task{
			ID:          "task-1",
			Title:       "Task with due date",
			Type:        models.TaskTypeTask,
			Status:      models.StatusDone,
			Priority:    3,
			DueDate:     &dueDate,
			CompletedAt: &now,
		}

		output := WriteArchivedTasks("Project", []*models.Task{task})

		if !strings.Contains(output, "due:") {
			t.Error("output should contain due date")
		}
	})
}

func TestRotateArchiveIfNeeded(t *testing.T) {
	t.Run("does nothing when no archive exists", func(t *testing.T) {
		tmpDir := t.TempDir()

		err := rotateArchiveIfNeeded(tmpDir, 100)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		archivePath := filepath.Join(tmpDir, "completed.md")
		if _, err := os.Stat(archivePath); !os.IsNotExist(err) {
			t.Error("should not create archive file when none exists")
		}
	})

	t.Run("does nothing when archive is under threshold", func(t *testing.T) {
		tmpDir := t.TempDir()
		archivePath := filepath.Join(tmpDir, "completed.md")

		content := "# Small archive content"
		if err := os.WriteFile(archivePath, []byte(content), 0644); err != nil {
			t.Fatalf("failed to write archive: %v", err)
		}

		err := rotateArchiveIfNeeded(tmpDir, 1024)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if _, err := os.Stat(archivePath); os.IsNotExist(err) {
			t.Error("archive file should still exist")
		}

		rotatedPath := filepath.Join(tmpDir, "completed.1.md")
		if _, err := os.Stat(rotatedPath); !os.IsNotExist(err) {
			t.Error("should not rotate when under threshold")
		}
	})

	t.Run("rotates archive when over threshold", func(t *testing.T) {
		tmpDir := t.TempDir()
		archivePath := filepath.Join(tmpDir, "completed.md")

		content := strings.Repeat("x", 200)
		if err := os.WriteFile(archivePath, []byte(content), 0644); err != nil {
			t.Fatalf("failed to write archive: %v", err)
		}

		err := rotateArchiveIfNeeded(tmpDir, 100)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if _, err := os.Stat(archivePath); !os.IsNotExist(err) {
			t.Error("original archive should be removed after rotation")
		}

		rotatedPath := filepath.Join(tmpDir, "completed.1.md")
		rotatedContent, err := os.ReadFile(rotatedPath)
		if err != nil {
			t.Fatalf("failed to read rotated archive: %v", err)
		}
		if string(rotatedContent) != content {
			t.Error("rotated archive content should match original")
		}
	})

	t.Run("shifts existing rotated files when rotating", func(t *testing.T) {
		tmpDir := t.TempDir()
		archivePath := filepath.Join(tmpDir, "completed.md")
		rotated1 := filepath.Join(tmpDir, "completed.1.md")
		rotated2 := filepath.Join(tmpDir, "completed.2.md")

		if err := os.WriteFile(archivePath, []byte(strings.Repeat("a", 200)), 0644); err != nil {
			t.Fatalf("failed to write archive: %v", err)
		}
		if err := os.WriteFile(rotated1, []byte("rotated1-content"), 0644); err != nil {
			t.Fatalf("failed to write rotated.1: %v", err)
		}
		if err := os.WriteFile(rotated2, []byte("rotated2-content"), 0644); err != nil {
			t.Fatalf("failed to write rotated.2: %v", err)
		}

		err := rotateArchiveIfNeeded(tmpDir, 100)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		content1, _ := os.ReadFile(rotated1)
		if !strings.HasPrefix(string(content1), "aaa") {
			t.Error("completed.1.md should contain the original archive content")
		}

		content2, _ := os.ReadFile(rotated2)
		if string(content2) != "rotated1-content" {
			t.Error("completed.2.md should contain previous completed.1.md content")
		}

		rotated3 := filepath.Join(tmpDir, "completed.3.md")
		content3, _ := os.ReadFile(rotated3)
		if string(content3) != "rotated2-content" {
			t.Error("completed.3.md should contain previous completed.2.md content")
		}
	})
}

func TestRemoveCompletedTasks(t *testing.T) {
	t.Run("removes completed tasks from project", func(t *testing.T) {
		now := time.Now()
		project := &models.Project{
			ID:   "proj-1",
			Name: "Test",
			Tasks: []*models.Task{
				{ID: "1", Title: "Todo task", Status: models.StatusTodo},
				{ID: "2", Title: "Done task", Status: models.StatusDone, CompletedAt: &now},
				{ID: "3", Title: "In progress task", Status: models.StatusInProgress},
				{ID: "4", Title: "Another done task", Status: models.StatusDone, CompletedAt: &now},
			},
		}

		removeCompletedTasks(project)

		if len(project.Tasks) != 2 {
			t.Errorf("expected 2 tasks after removal, got %d", len(project.Tasks))
		}

		for _, task := range project.Tasks {
			if task.Status == models.StatusDone {
				t.Error("completed tasks should be removed")
			}
		}
	})

	t.Run("does nothing when no completed tasks", func(t *testing.T) {
		project := &models.Project{
			ID:   "proj-1",
			Name: "Test",
			Tasks: []*models.Task{
				{ID: "1", Title: "Todo task", Status: models.StatusTodo},
				{ID: "2", Title: "In progress task", Status: models.StatusInProgress},
			},
		}

		removeCompletedTasks(project)

		if len(project.Tasks) != 2 {
			t.Errorf("expected 2 tasks unchanged, got %d", len(project.Tasks))
		}
	})

	t.Run("handles empty task list", func(t *testing.T) {
		project := &models.Project{
			ID:    "proj-1",
			Name:  "Test",
			Tasks: []*models.Task{},
		}

		removeCompletedTasks(project)

		if len(project.Tasks) != 0 {
			t.Error("expected 0 tasks for empty project")
		}
	})
}

func TestArchiveTasks(t *testing.T) {
	t.Run("archives completed tasks and removes them from project", func(t *testing.T) {
		tmpDir := t.TempDir()
		projectPath := filepath.Join(tmpDir, "test-project")
		if err := os.MkdirAll(projectPath, 0755); err != nil {
			t.Fatalf("failed to create project dir: %v", err)
		}

		now := time.Now()
		project := &models.Project{
			ID:   uuid.New().String(),
			Name: "Test Project",
			Slug: "test-project",
			Path: projectPath,
			Tasks: []*models.Task{
				{ID: "1", ShortID: 1, Title: "Todo task", Type: models.TaskTypeTask, Status: models.StatusTodo, Priority: 3},
				{ID: "2", ShortID: 2, Title: "Done task", Type: models.TaskTypeTask, Status: models.StatusDone, Priority: 3, CompletedAt: &now},
				{ID: "3", ShortID: 3, Title: "Another done", Type: models.TaskTypeBug, Status: models.StatusDone, Priority: 2, CompletedAt: &now},
			},
			NextShortID: 4,
		}

		count, err := ArchiveTasks(project)
		if err != nil {
			t.Fatalf("ArchiveTasks failed: %v", err)
		}

		if count != 2 {
			t.Errorf("expected 2 tasks archived, got %d", count)
		}

		if len(project.Tasks) != 1 {
			t.Errorf("expected 1 task remaining, got %d", len(project.Tasks))
		}

		if project.Tasks[0].ID != "1" {
			t.Error("remaining task should be the todo task")
		}

		archivePath := filepath.Join(projectPath, "completed.md")
		archiveContent, err := os.ReadFile(archivePath)
		if err != nil {
			t.Fatalf("failed to read archive: %v", err)
		}

		content := string(archiveContent)
		if !strings.Contains(content, "Done task") {
			t.Error("archive should contain 'Done task'")
		}
		if !strings.Contains(content, "Another done") {
			t.Error("archive should contain 'Another done'")
		}
	})

	t.Run("returns zero and does nothing when no completed tasks", func(t *testing.T) {
		tmpDir := t.TempDir()
		projectPath := filepath.Join(tmpDir, "test-project")
		if err := os.MkdirAll(projectPath, 0755); err != nil {
			t.Fatalf("failed to create project dir: %v", err)
		}

		project := &models.Project{
			ID:   uuid.New().String(),
			Name: "Test Project",
			Slug: "test-project",
			Path: projectPath,
			Tasks: []*models.Task{
				{ID: "1", ShortID: 1, Title: "Todo task", Type: models.TaskTypeTask, Status: models.StatusTodo, Priority: 3},
				{ID: "2", ShortID: 2, Title: "In progress", Type: models.TaskTypeTask, Status: models.StatusInProgress, Priority: 3},
			},
			NextShortID: 3,
		}

		count, err := ArchiveTasks(project)
		if err != nil {
			t.Fatalf("ArchiveTasks failed: %v", err)
		}

		if count != 0 {
			t.Errorf("expected 0 tasks archived, got %d", count)
		}

		if len(project.Tasks) != 2 {
			t.Errorf("expected 2 tasks unchanged, got %d", len(project.Tasks))
		}

		archivePath := filepath.Join(projectPath, "completed.md")
		if _, err := os.Stat(archivePath); !os.IsNotExist(err) {
			t.Error("archive file should not be created when no tasks to archive")
		}
	})

	t.Run("appends to existing archive file", func(t *testing.T) {
		tmpDir := t.TempDir()
		projectPath := filepath.Join(tmpDir, "test-project")
		if err := os.MkdirAll(projectPath, 0755); err != nil {
			t.Fatalf("failed to create project dir: %v", err)
		}

		archivePath := filepath.Join(projectPath, "completed.md")
		existingContent := "# Existing Archive\n\nPrevious content here.\n"
		if err := os.WriteFile(archivePath, []byte(existingContent), 0644); err != nil {
			t.Fatalf("failed to write existing archive: %v", err)
		}

		now := time.Now()
		project := &models.Project{
			ID:   uuid.New().String(),
			Name: "Test Project",
			Slug: "test-project",
			Path: projectPath,
			Tasks: []*models.Task{
				{ID: "1", ShortID: 1, Title: "New done task", Type: models.TaskTypeTask, Status: models.StatusDone, Priority: 3, CompletedAt: &now},
			},
			NextShortID: 2,
		}

		_, err := ArchiveTasks(project)
		if err != nil {
			t.Fatalf("ArchiveTasks failed: %v", err)
		}

		archiveContent, _ := os.ReadFile(archivePath)
		content := string(archiveContent)

		if !strings.HasPrefix(content, existingContent) {
			t.Error("archive should preserve existing content")
		}
		if !strings.Contains(content, "New done task") {
			t.Error("archive should contain newly archived task")
		}
	})

	t.Run("rotates archive when threshold exceeded", func(t *testing.T) {
		tmpDir := t.TempDir()
		projectPath := filepath.Join(tmpDir, "test-project")
		if err := os.MkdirAll(projectPath, 0755); err != nil {
			t.Fatalf("failed to create project dir: %v", err)
		}

		archivePath := filepath.Join(projectPath, "completed.md")
		largeContent := strings.Repeat("x", 6*1024*1024)
		if err := os.WriteFile(archivePath, []byte(largeContent), 0644); err != nil {
			t.Fatalf("failed to write large archive: %v", err)
		}

		now := time.Now()
		project := &models.Project{
			ID:   uuid.New().String(),
			Name: "Test Project",
			Slug: "test-project",
			Path: projectPath,
			Tasks: []*models.Task{
				{ID: "1", ShortID: 1, Title: "Task to archive", Type: models.TaskTypeTask, Status: models.StatusDone, Priority: 3, CompletedAt: &now},
			},
			NextShortID: 2,
		}

		_, err := ArchiveTasks(project)
		if err != nil {
			t.Fatalf("ArchiveTasks failed: %v", err)
		}

		rotatedPath := filepath.Join(projectPath, "completed.1.md")
		if _, err := os.Stat(rotatedPath); os.IsNotExist(err) {
			t.Error("large archive should have been rotated to completed.1.md")
		}

		newArchiveContent, _ := os.ReadFile(archivePath)
		if strings.Contains(string(newArchiveContent), strings.Repeat("x", 100)) {
			t.Error("new archive should not contain old large content")
		}
		if !strings.Contains(string(newArchiveContent), "Task to archive") {
			t.Error("new archive should contain newly archived task")
		}
	})
}

func TestGetArchiveFilePath(t *testing.T) {
	project := &models.Project{
		Path: "/home/user/projects/my-project",
	}

	archivePath := GetArchiveFilePath(project)
	expected := "/home/user/projects/my-project/completed.md"

	if archivePath != expected {
		t.Errorf("expected %s, got %s", expected, archivePath)
	}
}
