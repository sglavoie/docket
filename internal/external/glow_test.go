package external

import (
	"strings"
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/models"
)

func TestFormatTaskAsMarkdown(t *testing.T) {
	now := time.Now()

	t.Run("basic task format", func(t *testing.T) {
		task, err := models.NewTask("uuid-1", 42, models.TaskTypeTask, "My Task")
		if err != nil {
			t.Fatal(err)
		}
		task.Status = models.StatusTodo
		task.Priority = 2
		task.CreatedAt = now

		result := FormatTaskAsMarkdown(task)

		if !strings.Contains(result, "# Task #42: My Task") {
			t.Error("output should contain header with type, short ID, and title")
		}
		if !strings.Contains(result, "| Status |") {
			t.Error("output should contain status row")
		}
		if !strings.Contains(result, "| Priority | 2 |") {
			t.Error("output should contain priority row")
		}
		if !strings.Contains(result, "| Created |") {
			t.Error("output should contain created date row")
		}
	})

	t.Run("task with description", func(t *testing.T) {
		task, err := models.NewTask("uuid-1", 1, models.TaskTypeTask, "Task with desc")
		if err != nil {
			t.Fatal(err)
		}
		task.Description = "This is a detailed description."

		result := FormatTaskAsMarkdown(task)

		if !strings.Contains(result, "## Description") {
			t.Error("output should contain description header")
		}
		if !strings.Contains(result, "This is a detailed description.") {
			t.Error("output should contain the description text")
		}
	})

	t.Run("task without description", func(t *testing.T) {
		task, err := models.NewTask("uuid-1", 1, models.TaskTypeTask, "No description task")
		if err != nil {
			t.Fatal(err)
		}
		task.Description = ""

		result := FormatTaskAsMarkdown(task)

		if strings.Contains(result, "## Description") {
			t.Error("output should not contain description section when description is empty")
		}
	})

	t.Run("task with tags", func(t *testing.T) {
		task, err := models.NewTask("uuid-1", 1, models.TaskTypeTask, "Tagged task")
		if err != nil {
			t.Fatal(err)
		}
		task.Tags = []string{"backend", "urgent"}

		result := FormatTaskAsMarkdown(task)

		if !strings.Contains(result, "| Tags | backend, urgent |") {
			t.Error("output should contain tags row with comma-separated values")
		}
	})

	t.Run("task without tags", func(t *testing.T) {
		task, err := models.NewTask("uuid-1", 1, models.TaskTypeTask, "Untagged task")
		if err != nil {
			t.Fatal(err)
		}
		task.Tags = nil

		result := FormatTaskAsMarkdown(task)

		if strings.Contains(result, "| Tags |") {
			t.Error("output should not contain tags row when no tags present")
		}
	})

	t.Run("task with due date", func(t *testing.T) {
		task, err := models.NewTask("uuid-1", 1, models.TaskTypeTask, "Task with due date")
		if err != nil {
			t.Fatal(err)
		}
		dueDate := time.Date(2025, 1, 15, 0, 0, 0, 0, time.UTC)
		task.DueDate = &dueDate

		result := FormatTaskAsMarkdown(task)

		if !strings.Contains(result, "| Due |") {
			t.Error("output should contain due date row")
		}
		if !strings.Contains(result, "2025-01-15") {
			t.Error("output should contain formatted due date")
		}
	})

	t.Run("completed task", func(t *testing.T) {
		task, err := models.NewTask("uuid-1", 1, models.TaskTypeTask, "Completed task")
		if err != nil {
			t.Fatal(err)
		}
		task.Status = models.StatusDone
		completedAt := now
		task.CompletedAt = &completedAt

		result := FormatTaskAsMarkdown(task)

		if !strings.Contains(result, "| Completed |") {
			t.Error("output should contain completed date row")
		}
	})

	t.Run("task with children shows subtasks section", func(t *testing.T) {
		parent, err := models.NewTask("parent-uuid", 1, models.TaskTypeStory, "Parent Story")
		if err != nil {
			t.Fatal(err)
		}
		child1, err := models.NewTask("child1-uuid", 2, models.TaskTypeTask, "Child Task 1")
		if err != nil {
			t.Fatal(err)
		}
		child2, err := models.NewTask("child2-uuid", 3, models.TaskTypeTask, "Child Task 2")
		if err != nil {
			t.Fatal(err)
		}
		child2.Status = models.StatusDone

		if err := parent.AddChild(child1); err != nil {
			t.Fatal(err)
		}
		if err := parent.AddChild(child2); err != nil {
			t.Fatal(err)
		}

		result := FormatTaskAsMarkdown(parent)

		if !strings.Contains(result, "## Subtasks") {
			t.Error("output should contain subtasks section")
		}
		if !strings.Contains(result, "Progress:") {
			t.Error("output should contain progress line")
		}
		if !strings.Contains(result, "1/2 completed") {
			t.Error("output should show 1/2 completed (1 done, 1 todo)")
		}
		if !strings.Contains(result, "[ ]") {
			t.Error("output should contain unchecked checkbox for incomplete child")
		}
		if !strings.Contains(result, "[x]") {
			t.Error("output should contain checked checkbox for completed child")
		}
	})

	t.Run("epic type header format", func(t *testing.T) {
		task, err := models.NewTask("uuid-1", 5, models.TaskTypeEpic, "My Epic")
		if err != nil {
			t.Fatal(err)
		}

		result := FormatTaskAsMarkdown(task)

		if !strings.Contains(result, "# Epic #5: My Epic") {
			t.Error("output should contain Epic heading prefix")
		}
	})

	t.Run("bug type header format", func(t *testing.T) {
		task, err := models.NewTask("uuid-1", 10, models.TaskTypeBug, "Critical Bug")
		if err != nil {
			t.Fatal(err)
		}

		result := FormatTaskAsMarkdown(task)

		if !strings.Contains(result, "# Bug #10: Critical Bug") {
			t.Error("output should contain Bug heading prefix")
		}
	})

}
