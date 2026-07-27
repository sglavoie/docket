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
	if task.State() != StateReady {
		t.Errorf("expected State StateReady, got %s", task.State())
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

	// Newlines: rejected (would cause data loss on markdown round-trip)
	if err := ValidateTaskTitle("first line\nsecond line"); err == nil {
		t.Error("expected error for title containing newline")
	}
	if err := ValidateTaskTitle("first line\rsecond line"); err == nil {
		t.Error("expected error for title containing carriage return")
	}
	if err := ValidateTaskTitle("first line\r\nsecond line"); err == nil {
		t.Error("expected error for title containing CRLF")
	}
	if err := ValidateTaskTitle("first line\n## heading on second line"); err == nil {
		t.Error("expected error for title containing newline (with ##)")
	}

	// Markdown injection: rejected
	if err := ValidateTaskTitle("## heading at start"); err == nil {
		t.Error("expected error for title starting with ##")
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

func TestValidateTaskTitle_MultiByteCountsAsRunes(t *testing.T) {
	// A title of MaxTaskTitleLength runes using multi-byte characters
	// must be accepted: each "猫" is 3 bytes in UTF-8, so the byte length
	// is well above MaxTaskTitleLength, but the rune count equals it.
	maxRunes := strings.Repeat("猫", MaxTaskTitleLength)
	if got := len(maxRunes); got <= MaxTaskTitleLength {
		t.Fatalf("test precondition: byte length %d should exceed MaxTaskTitleLength %d", got, MaxTaskTitleLength)
	}
	if err := ValidateTaskTitle(maxRunes); err != nil {
		t.Errorf("expected %d-rune multi-byte title to be valid, got error: %v", MaxTaskTitleLength, err)
	}

	// One rune over the limit must be rejected.
	tooMany := strings.Repeat("猫", MaxTaskTitleLength+1)
	if err := ValidateTaskTitle(tooMany); err == nil {
		t.Errorf("expected %d-rune multi-byte title to be rejected", MaxTaskTitleLength+1)
	}
}

func TestWorkflowStateClassificationAndIcons(t *testing.T) {
	tests := []struct {
		state      WorkflowState
		isTerminal bool
		icon       string
	}{
		{StateReady, false, "□"},
		{StateInProgress, false, "▶"},
		{StateDone, true, "✓"},
		{StateWaiting, false, "⊘"},
		{StateCanceled, true, "✕"},
	}

	for _, tc := range tests {
		t.Run(string(tc.state), func(t *testing.T) {
			if tc.state.IsTerminal() != tc.isTerminal {
				t.Errorf("%s.IsTerminal() = %v, want %v", tc.state, tc.state.IsTerminal(), tc.isTerminal)
			}
			if tc.state.Icon() != tc.icon {
				t.Errorf("%s.Icon() = %s, want %s", tc.state, tc.state.Icon(), tc.icon)
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
	if task.State() != StateInProgress {
		t.Errorf("after Start(), State = %s, want in_progress", task.State())
	}

	// Block
	task.Block()
	if task.State() != StateWaiting {
		t.Errorf("after Block(), State = %s, want waiting", task.State())
	}

	// Complete
	task.Complete()
	if task.State() != StateDone {
		t.Errorf("after Complete(), State = %s, want done", task.State())
	}
	if task.CompletedAt == nil {
		t.Error("after Complete(), CompletedAt should be set")
	}

	// Reopen returns the task to the Inbox for re-processing.
	task.Reopen()
	if task.State() != StateInbox {
		t.Errorf("after Reopen(), State = %s, want inbox", task.State())
	}
	if task.CompletedAt != nil {
		t.Error("after Reopen(), CompletedAt should be nil")
	}

	// Cancel
	task.Cancel()
	if task.State() != StateCanceled {
		t.Errorf("after Cancel(), State = %s, want canceled", task.State())
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

func TestParseWorkflowStateFromCanonicalStrings(t *testing.T) {
	tests := []struct {
		input string
		want  WorkflowState
		valid bool
	}{
		{"ready", StateReady, true},
		{"in_progress", StateInProgress, true},
		{"done", StateDone, true},
		{"waiting", StateWaiting, true},
		{"canceled", StateCanceled, true},
		{"invalid", WorkflowState("invalid"), false},
		{"", WorkflowState(""), false},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			got, valid := ParseWorkflowState(tc.input)
			if got != tc.want {
				t.Errorf("ParseWorkflowState(%s) = %s, want %s", tc.input, got, tc.want)
			}
			if valid != tc.valid {
				t.Errorf("ParseWorkflowState(%s) valid = %v, want %v", tc.input, valid, tc.valid)
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
