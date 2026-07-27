package storage

import (
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
			ID:            "task-123",
			Title:         "Test task",
			Type:          models.TaskTypeTask,
			WorkflowState: models.StateDone,
			Priority:      2,
			Tags:          []string{"backend", "urgent"},
			CreatedAt:     now.Add(-24 * time.Hour),
			CompletedAt:   &now,
			Description:   "Task description here.",
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
		// The archive uses the canonical v2 metadata writer: the done status
		// serializes as state, and the retired section field is dropped.
		if !strings.Contains(output, "state: done") {
			t.Error("output should contain task state")
		}
		if !strings.Contains(output, "priority: 2") {
			t.Error("output should contain task priority")
		}
		if !strings.Contains(output, "tags: backend;urgent") {
			t.Error("output should contain task tags")
		}
		if strings.Contains(output, "section:") {
			t.Error("v2 archive output should not contain the retired section field")
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
				ID:            "task-1",
				Title:         "First task",
				Type:          models.TaskTypeTask,
				WorkflowState: models.StateDone,
				Priority:      1,
				CompletedAt:   &now,
			},
			{
				ID:            "task-2",
				Title:         "Second task",
				Type:          models.TaskTypeBug,
				WorkflowState: models.StateDone,
				Priority:      2,
				CompletedAt:   &now,
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
			ID:            "task-1",
			Title:         "Task with due date",
			Type:          models.TaskTypeTask,
			WorkflowState: models.StateDone,
			Priority:      3,
			DueDate:       &dueDate,
			CompletedAt:   &now,
		}

		output := WriteArchivedTasks("Project", []*models.Task{task})

		if !strings.Contains(output, "due:") {
			t.Error("output should contain due date")
		}
	})
}

// TestWriteArchivedTasks_DescriptionWithHeadingPattern verifies that archived
// tasks whose descriptions contain heading-like lines (e.g., "## Task: ...")
// survive a write-then-parse round-trip without creating phantom tasks.
func TestWriteArchivedTasks_DescriptionWithHeadingPattern(t *testing.T) {
	now := time.Now()
	completedAt := now
	task := &models.Task{
		ID:            uuid.New().String(),
		ShortID:       1,
		Title:         "Archived with heading desc",
		Type:          models.TaskTypeTask,
		WorkflowState: models.StateDone,
		Priority:      2,
		Tags:          []string{},
		DependsOn:     []string{},
		RelatedTo:     []string{},
		CreatedAt:     now.Add(-24 * time.Hour),
		UpdatedAt:     now,
		CompletedAt:   &completedAt,
		Description:   "Some notes\n## Task: Looks like a heading\nMore notes",
	}

	output := WriteArchivedTasks("Test Project", []*models.Task{task})

	result, err := ParseTasksMarkdownDocument(output)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	if len(result.Tasks) != 1 {
		t.Fatalf("expected 1 task, got %d (heading-like line in description created phantom task)", len(result.Tasks))
	}

	if result.Tasks[0].Description != task.Description {
		t.Errorf("description mismatch:\n  got:  %q\n  want: %q", result.Tasks[0].Description, task.Description)
	}
}
