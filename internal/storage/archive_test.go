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

		if _, statErr := os.Stat(archivePath); !os.IsNotExist(statErr) {
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

func TestLoadArchivedTasks(t *testing.T) {
	t.Run("returns empty slice when no archive file exists", func(t *testing.T) {
		tmpDir := t.TempDir()
		project := &models.Project{Path: tmpDir}

		tasks := LoadArchivedTasks(project)

		if len(tasks) != 0 {
			t.Errorf("expected 0 tasks, got %d", len(tasks))
		}
	})

	t.Run("loads tasks from single archive file", func(t *testing.T) {
		tmpDir := t.TempDir()
		project := &models.Project{Path: tmpDir}

		completedAt := time.Date(2025, 6, 10, 14, 30, 0, 0, time.Local)
		content := `## Archived: 2025-06-10 15:00
<!-- project: TestProj, count: 2 -->

### Task: First archived task
<!-- id: arch-1, status: done, priority: 2, created: 2025-06-01 10:00, completed: 2025-06-10 14:30 -->

Some description.

### Bug: Second archived task
<!-- id: arch-2, status: canceled, priority: 1, created: 2025-06-02 11:00, completed: 2025-06-10 14:30 -->
`
		archivePath := filepath.Join(tmpDir, "completed.md")
		if err := os.WriteFile(archivePath, []byte(content), 0644); err != nil {
			t.Fatalf("failed to write archive: %v", err)
		}

		tasks := LoadArchivedTasks(project)

		if len(tasks) != 2 {
			t.Fatalf("expected 2 tasks, got %d", len(tasks))
		}
		if tasks[0].ID != "arch-1" {
			t.Errorf("expected first task ID arch-1, got %s", tasks[0].ID)
		}
		if tasks[0].Status != models.StatusDone {
			t.Errorf("expected status done, got %s", tasks[0].Status)
		}
		if tasks[0].CompletedAt == nil || !tasks[0].CompletedAt.Equal(completedAt) {
			t.Errorf("expected CompletedAt %v, got %v", completedAt, tasks[0].CompletedAt)
		}
		if tasks[1].ID != "arch-2" {
			t.Errorf("expected second task ID arch-2, got %s", tasks[1].ID)
		}
		if tasks[1].Status != models.StatusCanceled {
			t.Errorf("expected status canceled, got %s", tasks[1].Status)
		}
	})

	t.Run("loads tasks from multiple rotated archive files", func(t *testing.T) {
		tmpDir := t.TempDir()
		project := &models.Project{Path: tmpDir}

		mainContent := `### Task: Main task
<!-- id: main-1, status: done, priority: 3, completed: 2025-06-10 14:30 -->
`
		rotated1Content := `### Task: Rotated 1 task
<!-- id: rot1-1, status: done, priority: 2, completed: 2025-05-15 10:00 -->
`
		rotated2Content := `### Bug: Rotated 2 bug
<!-- id: rot2-1, status: done, priority: 1, completed: 2025-04-20 09:00 -->
`
		if err := os.WriteFile(filepath.Join(tmpDir, "completed.md"), []byte(mainContent), 0644); err != nil {
			t.Fatalf("failed to write main archive: %v", err)
		}
		if err := os.WriteFile(filepath.Join(tmpDir, "completed.1.md"), []byte(rotated1Content), 0644); err != nil {
			t.Fatalf("failed to write rotated 1: %v", err)
		}
		if err := os.WriteFile(filepath.Join(tmpDir, "completed.2.md"), []byte(rotated2Content), 0644); err != nil {
			t.Fatalf("failed to write rotated 2: %v", err)
		}

		tasks := LoadArchivedTasks(project)

		if len(tasks) != 3 {
			t.Fatalf("expected 3 tasks from all archive files, got %d", len(tasks))
		}

		ids := make(map[string]bool)
		for _, task := range tasks {
			ids[task.ID] = true
		}
		for _, expected := range []string{"main-1", "rot1-1", "rot2-1"} {
			if !ids[expected] {
				t.Errorf("expected task %s to be loaded from archives", expected)
			}
		}
	})

	t.Run("skips malformed archive files without failing", func(t *testing.T) {
		tmpDir := t.TempDir()
		project := &models.Project{Path: tmpDir}

		goodContent := `### Task: Good task
<!-- id: good-1, status: done, priority: 3, completed: 2025-06-10 14:30 -->
`
		if err := os.WriteFile(filepath.Join(tmpDir, "completed.md"), []byte(goodContent), 0644); err != nil {
			t.Fatalf("failed to write archive: %v", err)
		}

		tasks := LoadArchivedTasks(project)

		if len(tasks) != 1 {
			t.Errorf("expected 1 task from good archive, got %d", len(tasks))
		}
	})

	t.Run("activity logs round-trip through archive write and load", func(t *testing.T) {
		tmpDir := t.TempDir()
		project := &models.Project{Path: tmpDir, Name: "TestProj"}

		now := time.Date(2025, 6, 10, 14, 30, 0, 0, time.Local)
		startedAt := time.Date(2025, 6, 5, 10, 0, 0, 0, time.Local)
		blockedAt := time.Date(2025, 6, 7, 9, 0, 0, 0, time.Local)
		unblockedAt := time.Date(2025, 6, 8, 11, 0, 0, 0, time.Local)

		task := &models.Task{
			ID:          "act-task-1",
			Title:       "Task with activity",
			Type:        models.TaskTypeTask,
			Status:      models.StatusDone,
			Priority:    2,
			CreatedAt:   startedAt,
			CompletedAt: &now,
			Activity: []*models.ActivityEntry{
				{Timestamp: startedAt, Field: "status", OldValue: "todo", NewValue: "in_progress"},
				{Timestamp: blockedAt, Field: "status", OldValue: "in_progress", NewValue: "blocked"},
				{Timestamp: unblockedAt, Field: "status", OldValue: "blocked", NewValue: "in_progress"},
			},
		}

		// Write archived tasks (this now includes activity)
		content := WriteArchivedTasks(project.Name, []*models.Task{task})
		archivePath := filepath.Join(tmpDir, "completed.md")
		if err := os.WriteFile(archivePath, []byte(content), 0644); err != nil {
			t.Fatalf("failed to write archive: %v", err)
		}

		// Load them back
		tasks := LoadArchivedTasks(project)

		if len(tasks) != 1 {
			t.Fatalf("expected 1 task, got %d", len(tasks))
		}

		loaded := tasks[0]
		if len(loaded.Activity) != 3 {
			t.Fatalf("expected 3 activity entries, got %d", len(loaded.Activity))
		}

		// Verify the activity entries round-tripped correctly
		if loaded.Activity[0].Field != "status" || loaded.Activity[0].NewValue != "in_progress" {
			t.Errorf("first activity entry mismatch: %+v", loaded.Activity[0])
		}
		if loaded.Activity[1].Field != "status" || loaded.Activity[1].NewValue != "blocked" {
			t.Errorf("second activity entry mismatch: %+v", loaded.Activity[1])
		}
		if loaded.Activity[2].Field != "status" || loaded.Activity[2].NewValue != "in_progress" {
			t.Errorf("third activity entry mismatch: %+v", loaded.Activity[2])
		}
	})

	t.Run("loads tasks even with gaps in rotated file numbering", func(t *testing.T) {
		tmpDir := t.TempDir()
		project := &models.Project{Path: tmpDir}

		// Create completed.1.md and completed.3.md (skip 2)
		content1 := `### Task: Rotated 1
<!-- id: rot1, status: done, priority: 3, completed: 2025-06-10 14:30 -->
`
		content3 := `### Task: Rotated 3
<!-- id: rot3, status: done, priority: 3, completed: 2025-06-10 14:30 -->
`
		if err := os.WriteFile(filepath.Join(tmpDir, "completed.1.md"), []byte(content1), 0644); err != nil {
			t.Fatalf("failed to write completed.1.md: %v", err)
		}
		if err := os.WriteFile(filepath.Join(tmpDir, "completed.3.md"), []byte(content3), 0644); err != nil {
			t.Fatalf("failed to write completed.3.md: %v", err)
		}

		tasks := LoadArchivedTasks(project)

		if len(tasks) != 2 {
			t.Fatalf("expected 2 tasks from gapped archives, got %d", len(tasks))
		}

		ids := make(map[string]bool)
		for _, task := range tasks {
			ids[task.ID] = true
		}
		if !ids["rot1"] || !ids["rot3"] {
			t.Errorf("expected both rot1 and rot3, got %v", ids)
		}
	})

	t.Run("comments round-trip through archive write and load", func(t *testing.T) {
		tmpDir := t.TempDir()
		project := &models.Project{Path: tmpDir, Name: "TestProj"}

		now := time.Date(2025, 6, 10, 14, 30, 0, 0, time.Local)
		commentTime := time.Date(2025, 6, 8, 9, 0, 0, 0, time.Local)

		task := &models.Task{
			ID:          "cmt-task-1",
			Title:       "Task with comments",
			Type:        models.TaskTypeTask,
			Status:      models.StatusDone,
			Priority:    2,
			CompletedAt: &now,
			Comments: []*models.Comment{
				{Text: "This is a test comment", CreatedAt: commentTime},
			},
		}

		content := WriteArchivedTasks(project.Name, []*models.Task{task})
		archivePath := filepath.Join(tmpDir, "completed.md")
		if err := os.WriteFile(archivePath, []byte(content), 0644); err != nil {
			t.Fatalf("failed to write archive: %v", err)
		}

		tasks := LoadArchivedTasks(project)

		if len(tasks) != 1 {
			t.Fatalf("expected 1 task, got %d", len(tasks))
		}
		if len(tasks[0].Comments) != 1 {
			t.Fatalf("expected 1 comment, got %d", len(tasks[0].Comments))
		}
		if tasks[0].Comments[0].Text != "This is a test comment" {
			t.Errorf("comment text mismatch: %q", tasks[0].Comments[0].Text)
		}
	})
}

func TestArchivePreservesAllMetadataFields(t *testing.T) {
	tmpDir := t.TempDir()
	project := &models.Project{Path: tmpDir, Name: "TestProj"}

	now := time.Date(2025, 6, 10, 14, 30, 0, 0, time.Local)
	snoozed := time.Date(2025, 7, 1, 0, 0, 0, 0, time.Local)

	task := &models.Task{
		ID:            "full-task-1",
		Title:         "Task with all fields",
		Type:          models.TaskTypeBug,
		Status:        models.StatusDone,
		Priority:      1,
		Tags:          []string{"backend", "critical"},
		Section:       "Next",
		CustomSection: "Sprint-1",
		CreatedAt:     now.Add(-48 * time.Hour),
		UpdatedAt:     now.Add(-1 * time.Hour),
		CompletedAt:   &now,
		DependsOn:     []string{"dep-1", "dep-2"},
		RelatedTo:     []string{"rel-1"},
		SnoozedUntil:  &snoozed,
		Stage:         "Review",
		Source:        "github",
		URL:           "https://github.com/example/issue/42",
		Checklist: []models.ChecklistItem{
			{Text: "Step one", Done: true},
			{Text: "Step two", Done: false},
		},
		ScoreDimensions: map[string]int{
			"impact": 4,
			"effort": 2,
		},
		BugAttrs: &models.BugAttrs{
			Severity:     models.BugSeverityCritical,
			Reproducible: models.BugReproducibleYes,
			Workaround:   "Restart the service",
		},
		Attachments: []models.Attachment{
			{
				Filename:     "screenshot.png",
				OriginalPath: "/home/user/screenshot.png",
				AddedAt:      now.Add(-24 * time.Hour),
			},
			{
				Filename:     "https://example.com/doc",
				IsURL:        true,
				OriginalPath: "https://example.com/doc",
				AddedAt:      now.Add(-12 * time.Hour),
			},
			{
				Filename:     "main.go",
				IsCodeRef:    true,
				LineNumber:   42,
				OriginalPath: "/home/user/project/main.go",
				AddedAt:      now.Add(-6 * time.Hour),
			},
		},
	}

	content := WriteArchivedTasks(project.Name, []*models.Task{task})
	archivePath := filepath.Join(tmpDir, "completed.md")
	if err := os.WriteFile(archivePath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write archive: %v", err)
	}

	tasks := LoadArchivedTasks(project)
	if len(tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(tasks))
	}
	loaded := tasks[0]

	// Fields that were previously lost by the old writeArchivedMetadata
	if loaded.CustomSection != "Sprint-1" {
		t.Errorf("custom_section: got %q, want %q", loaded.CustomSection, "Sprint-1")
	}
	if len(loaded.DependsOn) != 2 || loaded.DependsOn[0] != "dep-1" || loaded.DependsOn[1] != "dep-2" {
		t.Errorf("depends_on: got %v, want [dep-1 dep-2]", loaded.DependsOn)
	}
	if len(loaded.RelatedTo) != 1 || loaded.RelatedTo[0] != "rel-1" {
		t.Errorf("related_to: got %v, want [rel-1]", loaded.RelatedTo)
	}
	if loaded.SnoozedUntil == nil {
		t.Error("snoozed_until: should not be nil")
	}
	if loaded.Stage != "Review" {
		t.Errorf("stage: got %q, want %q", loaded.Stage, "Review")
	}
	if loaded.Source != "github" {
		t.Errorf("source: got %q, want %q", loaded.Source, "github")
	}
	if loaded.URL != "https://github.com/example/issue/42" {
		t.Errorf("url: got %q, want %q", loaded.URL, "https://github.com/example/issue/42")
	}
	if len(loaded.Checklist) != 2 {
		t.Fatalf("checklist: got %d items, want 2", len(loaded.Checklist))
	}
	if !loaded.Checklist[0].Done || loaded.Checklist[0].Text != "Step one" {
		t.Errorf("checklist[0]: got %+v, want done=true text='Step one'", loaded.Checklist[0])
	}
	if loaded.Checklist[1].Done || loaded.Checklist[1].Text != "Step two" {
		t.Errorf("checklist[1]: got %+v, want done=false text='Step two'", loaded.Checklist[1])
	}
	if len(loaded.ScoreDimensions) != 2 {
		t.Fatalf("score_dimensions: got %d, want 2", len(loaded.ScoreDimensions))
	}
	if loaded.ScoreDimensions["impact"] != 4 || loaded.ScoreDimensions["effort"] != 2 {
		t.Errorf("score_dimensions: got %v, want impact=4 effort=2", loaded.ScoreDimensions)
	}
	if loaded.BugAttrs == nil {
		t.Fatal("bug_attrs: should not be nil")
	}
	if loaded.BugAttrs.Severity != models.BugSeverityCritical {
		t.Errorf("bug.severity: got %q, want %q", loaded.BugAttrs.Severity, models.BugSeverityCritical)
	}
	if loaded.BugAttrs.Reproducible != models.BugReproducibleYes {
		t.Errorf("bug.reproducible: got %q, want %q", loaded.BugAttrs.Reproducible, models.BugReproducibleYes)
	}
	if loaded.BugAttrs.Workaround != "Restart the service" {
		t.Errorf("bug.workaround: got %q, want %q", loaded.BugAttrs.Workaround, "Restart the service")
	}

	// Verify attachment fields including code refs and original paths
	if len(loaded.Attachments) != 3 {
		t.Fatalf("attachments: got %d, want 3", len(loaded.Attachments))
	}
	// File attachment with original path
	if loaded.Attachments[0].OriginalPath != "/home/user/screenshot.png" {
		t.Errorf("attachment[0].OriginalPath: got %q, want %q", loaded.Attachments[0].OriginalPath, "/home/user/screenshot.png")
	}
	// URL attachment
	if !loaded.Attachments[1].IsURL {
		t.Error("attachment[1]: should be URL")
	}
	// Code ref attachment with line number
	if !loaded.Attachments[2].IsCodeRef {
		t.Error("attachment[2]: should be code ref")
	}
	if loaded.Attachments[2].LineNumber != 42 {
		t.Errorf("attachment[2].LineNumber: got %d, want 42", loaded.Attachments[2].LineNumber)
	}
	if loaded.Attachments[2].OriginalPath != "/home/user/project/main.go" {
		t.Errorf("attachment[2].OriginalPath: got %q, want %q", loaded.Attachments[2].OriginalPath, "/home/user/project/main.go")
	}
}

func TestClearOrphanedParentRefs(t *testing.T) {
	t.Run("clears parent refs pointing to removed tasks", func(t *testing.T) {
		parentID := "parent-1"
		project := &models.Project{
			ID:   "proj-1",
			Name: "Test",
			Tasks: []*models.Task{
				{ID: "child-1", Title: "Child 1", ParentID: &parentID, Status: models.StatusTodo},
				{ID: "child-2", Title: "Child 2", ParentID: &parentID, Status: models.StatusInProgress},
				{ID: "unrelated", Title: "Unrelated", Status: models.StatusTodo},
			},
		}
		removedIDs := map[string]bool{"parent-1": true}

		clearOrphanedParentRefs(project, removedIDs)

		for _, task := range project.Tasks {
			if task.ParentID != nil {
				t.Errorf("task %s should have nil ParentID after parent was removed, got %s", task.ID, *task.ParentID)
			}
		}
	})

	t.Run("clears deps and related refs pointing to removed tasks", func(t *testing.T) {
		project := &models.Project{
			ID:   "proj-1",
			Name: "Test",
			Tasks: []*models.Task{
				{ID: "t1", Title: "Task 1", DependsOn: []string{"removed-1", "remaining-1"}, RelatedTo: []string{"removed-1"}, Status: models.StatusTodo},
				{ID: "t2", Title: "Task 2", DependsOn: []string{"remaining-1"}, Status: models.StatusTodo},
			},
		}
		removedIDs := map[string]bool{"removed-1": true}

		clearOrphanedParentRefs(project, removedIDs)

		if len(project.Tasks[0].DependsOn) != 1 || project.Tasks[0].DependsOn[0] != "remaining-1" {
			t.Errorf("expected DependsOn=[remaining-1], got %v", project.Tasks[0].DependsOn)
		}
		if len(project.Tasks[0].RelatedTo) != 0 {
			t.Errorf("expected RelatedTo=[], got %v", project.Tasks[0].RelatedTo)
		}
		if len(project.Tasks[1].DependsOn) != 1 || project.Tasks[1].DependsOn[0] != "remaining-1" {
			t.Errorf("expected DependsOn=[remaining-1], got %v", project.Tasks[1].DependsOn)
		}
	})

	t.Run("preserves refs to non-removed tasks", func(t *testing.T) {
		keepParentID := "keep-parent"
		project := &models.Project{
			ID:   "proj-1",
			Name: "Test",
			Tasks: []*models.Task{
				{ID: "child-1", Title: "Child", ParentID: &keepParentID, Status: models.StatusTodo},
			},
		}
		removedIDs := map[string]bool{"other-parent": true}

		clearOrphanedParentRefs(project, removedIDs)

		if project.Tasks[0].ParentID == nil || *project.Tasks[0].ParentID != "keep-parent" {
			t.Error("ParentID pointing to non-removed task should be preserved")
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
