package models

import (
	"strings"
	"testing"
	"time"
)

func TestNewTask(t *testing.T) {
	task, err := NewTask("test-id", 1, TaskTypeTask, "Test Task")
	if err != nil {
		t.Fatal(err)
	}

	if task.ID != "test-id" {
		t.Errorf("expected ID 'test-id', got '%s'", task.ID)
	}
	if task.ShortID != 1 {
		t.Errorf("expected ShortID 1, got %d", task.ShortID)
	}
	if task.Type != TaskTypeTask {
		t.Errorf("expected Type TaskTypeTask, got %s", task.Type)
	}
	if task.Title != "Test Task" {
		t.Errorf("expected Title 'Test Task', got '%s'", task.Title)
	}
	if task.Status != StatusTodo {
		t.Errorf("expected Status StatusTodo, got %s", task.Status)
	}
	if task.Priority != 3 {
		t.Errorf("expected Priority 3, got %d", task.Priority)
	}
	if task.CreatedAt.IsZero() {
		t.Error("expected CreatedAt to be set")
	}
}

func TestValidateTaskTitle(t *testing.T) {
	// Valid titles
	if err := ValidateTaskTitle("Short title"); err != nil {
		t.Errorf("expected no error for short title, got: %v", err)
	}
	if err := ValidateTaskTitle(strings.Repeat("a", MaxTaskTitleLength)); err != nil {
		t.Errorf("expected no error for max-length title, got: %v", err)
	}
	if err := ValidateTaskTitle(""); err == nil {
		t.Error("expected error for empty title")
	}

	// Too long
	if err := ValidateTaskTitle(strings.Repeat("a", MaxTaskTitleLength+1)); err == nil {
		t.Error("expected error for title exceeding max length")
	}
	if err := ValidateTaskTitle(strings.Repeat("x", 1000)); err == nil {
		t.Error("expected error for 1000-char title")
	}

	// Markdown injection: rejected
	if err := ValidateTaskTitle("## heading at start"); err == nil {
		t.Error("expected error for title starting with ##")
	}
	if err := ValidateTaskTitle("first line\n## heading on second line"); err == nil {
		t.Error("expected error for ## at start of second line")
	}
	if err := ValidateTaskTitle("task with <!-- comment"); err == nil {
		t.Error("expected error for title containing <!--")
	}
	if err := ValidateTaskTitle("task with --> end"); err == nil {
		t.Error("expected error for title containing -->")
	}

	// Markdown injection: allowed (# alone or mid-line ## are fine)
	if err := ValidateTaskTitle("# single hash is ok"); err != nil {
		t.Errorf("expected no error for single # title, got: %v", err)
	}
	if err := ValidateTaskTitle("mid ## line is ok"); err != nil {
		t.Errorf("expected no error for mid-line ##, got: %v", err)
	}
	if err := ValidateTaskTitle("use -> arrows freely"); err != nil {
		t.Errorf("expected no error for -> arrows, got: %v", err)
	}
}

func TestTaskStatus(t *testing.T) {
	tests := []struct {
		status     TaskStatus
		isComplete bool
		isActive   bool
		icon       string
	}{
		{StatusTodo, false, true, "□"},
		{StatusInProgress, false, true, "▶"},
		{StatusDone, true, false, "✓"},
		{StatusBlocked, false, true, "⊘"},
		{StatusCanceled, true, false, "✕"},
	}

	for _, tc := range tests {
		t.Run(string(tc.status), func(t *testing.T) {
			if tc.status.IsComplete() != tc.isComplete {
				t.Errorf("%s.IsComplete() = %v, want %v", tc.status, tc.status.IsComplete(), tc.isComplete)
			}
			if tc.status.IsActive() != tc.isActive {
				t.Errorf("%s.IsActive() = %v, want %v", tc.status, tc.status.IsActive(), tc.isActive)
			}
			if tc.status.Icon() != tc.icon {
				t.Errorf("%s.Icon() = %s, want %s", tc.status, tc.status.Icon(), tc.icon)
			}
		})
	}
}

func TestTaskStatusTransitions(t *testing.T) {
	task, err := NewTask("test", 1, TaskTypeTask, "Test")
	if err != nil {
		t.Fatal(err)
	}

	// Start
	task.Start()
	if task.Status != StatusInProgress {
		t.Errorf("after Start(), Status = %s, want in_progress", task.Status)
	}

	// Block
	task.Block()
	if task.Status != StatusBlocked {
		t.Errorf("after Block(), Status = %s, want blocked", task.Status)
	}

	// Complete
	task.Complete()
	if task.Status != StatusDone {
		t.Errorf("after Complete(), Status = %s, want done", task.Status)
	}
	if task.CompletedAt == nil {
		t.Error("after Complete(), CompletedAt should be set")
	}

	// Reopen
	task.Reopen()
	if task.Status != StatusTodo {
		t.Errorf("after Reopen(), Status = %s, want todo", task.Status)
	}
	if task.CompletedAt != nil {
		t.Error("after Reopen(), CompletedAt should be nil")
	}

	// Cancel
	task.Cancel()
	if task.Status != StatusCanceled {
		t.Errorf("after Cancel(), Status = %s, want canceled", task.Status)
	}
	if task.CompletedAt == nil {
		t.Error("after Cancel(), CompletedAt should be set")
	}
}

func TestTaskTags(t *testing.T) {
	task, err := NewTask("test", 1, TaskTypeTask, "Test")
	if err != nil {
		t.Fatal(err)
	}

	// Add tag
	task.AddTag("backend")
	if !task.HasTag("backend") {
		t.Error("expected task to have tag 'backend'")
	}
	if len(task.Tags) != 1 {
		t.Errorf("expected 1 tag, got %d", len(task.Tags))
	}

	// Add duplicate tag (should not add)
	task.AddTag("backend")
	if len(task.Tags) != 1 {
		t.Errorf("expected 1 tag after duplicate add, got %d", len(task.Tags))
	}

	// Add another tag
	task.AddTag("urgent")
	if len(task.Tags) != 2 {
		t.Errorf("expected 2 tags, got %d", len(task.Tags))
	}

	// Remove tag
	task.RemoveTag("backend")
	if task.HasTag("backend") {
		t.Error("expected task to not have tag 'backend' after removal")
	}
	if len(task.Tags) != 1 {
		t.Errorf("expected 1 tag after removal, got %d", len(task.Tags))
	}

	// Remove non-existent tag (should not error)
	task.RemoveTag("nonexistent")
	if len(task.Tags) != 1 {
		t.Errorf("expected 1 tag after removing non-existent, got %d", len(task.Tags))
	}
}

func TestParseTaskStatus(t *testing.T) {
	tests := []struct {
		input string
		want  TaskStatus
		valid bool
	}{
		{"todo", StatusTodo, true},
		{"in_progress", StatusInProgress, true},
		{"done", StatusDone, true},
		{"blocked", StatusBlocked, true},
		{"canceled", StatusCanceled, true},
		{"invalid", TaskStatus("invalid"), false},
		{"", TaskStatus(""), false},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			got, valid := ParseTaskStatus(tc.input)
			if got != tc.want {
				t.Errorf("ParseTaskStatus(%s) = %s, want %s", tc.input, got, tc.want)
			}
			if valid != tc.valid {
				t.Errorf("ParseTaskStatus(%s) valid = %v, want %v", tc.input, valid, tc.valid)
			}
		})
	}
}

func TestTaskUpdatedAt(t *testing.T) {
	task, err := NewTask("test", 1, TaskTypeTask, "Test")
	if err != nil {
		t.Fatal(err)
	}
	originalUpdate := task.UpdatedAt

	// Wait a tiny bit to ensure time difference
	time.Sleep(time.Millisecond)

	task.Start()
	if !task.UpdatedAt.After(originalUpdate) {
		t.Error("UpdatedAt should be updated after Start()")
	}

	originalUpdate = task.UpdatedAt
	time.Sleep(time.Millisecond)

	task.AddTag("test")
	if !task.UpdatedAt.After(originalUpdate) {
		t.Error("UpdatedAt should be updated after AddTag()")
	}
}
