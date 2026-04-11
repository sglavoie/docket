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

func Test_getArchiveFilePath(t *testing.T) {
	project := &models.Project{
		Path: "/home/user/projects/my-project",
	}

	archivePath := getArchiveFilePath(project)
	expected := "/home/user/projects/my-project/completed.md"

	if archivePath != expected {
		t.Errorf("expected %s, got %s", expected, archivePath)
	}
}
