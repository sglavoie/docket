package models

import (
	"testing"
	"time"
)

func TestIsDueToday(t *testing.T) {
	task, err := NewTask("t1", 1, TaskTypeTask, "Test")
	if err != nil {
		t.Fatal(err)
	}

	// No due date
	if task.IsDueToday() {
		t.Error("expected not due today when no due date")
	}

	// Due today
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 15, 30, 0, 0, time.Local)
	task.DueDate = &today
	if !task.IsDueToday() {
		t.Error("expected due today")
	}

	// Due tomorrow
	tomorrow := now.AddDate(0, 0, 1)
	task.DueDate = &tomorrow
	if task.IsDueToday() {
		t.Error("expected not due today when due tomorrow")
	}

	// Due yesterday
	yesterday := now.AddDate(0, 0, -1)
	task.DueDate = &yesterday
	if task.IsDueToday() {
		t.Error("expected not due today when due yesterday")
	}
}

func TestIsDueThisWeek(t *testing.T) {
	task, err := NewTask("t1", 1, TaskTypeTask, "Test")
	if err != nil {
		t.Fatal(err)
	}

	// No due date
	if task.IsDueThisWeek() {
		t.Error("expected not due this week when no due date")
	}

	now := time.Now()

	// Due today — should be this week
	today := time.Date(now.Year(), now.Month(), now.Day(), 12, 0, 0, 0, time.Local)
	task.DueDate = &today
	if !task.IsDueThisWeek() {
		t.Error("expected due this week for today")
	}

	// Due in 3 days — should be this week
	in3Days := now.AddDate(0, 0, 3)
	task.DueDate = &in3Days
	if !task.IsDueThisWeek() {
		t.Error("expected due this week for +3 days")
	}

	// Due in 6 days — should still be this week
	in6Days := now.AddDate(0, 0, 6)
	task.DueDate = &in6Days
	if !task.IsDueThisWeek() {
		t.Error("expected due this week for +6 days")
	}

	// Due in 7 days — should NOT be this week (exclusive)
	in7Days := now.AddDate(0, 0, 7)
	task.DueDate = &in7Days
	if task.IsDueThisWeek() {
		t.Error("expected not due this week for exactly +7 days")
	}

	// Due yesterday — should NOT be this week
	yesterday := now.AddDate(0, 0, -1)
	task.DueDate = &yesterday
	if task.IsDueThisWeek() {
		t.Error("expected not due this week for yesterday")
	}
}

func TestEnsureTypeAttrs(t *testing.T) {
	task, err := NewTask("t1", 1, TaskTypeBug, "Bug Test")
	if err != nil {
		t.Fatal(err)
	}

	// Initially nil
	if task.BugAttrs != nil {
		t.Error("expected BugAttrs nil initially")
	}

	task.EnsureBugAttrs()
	if task.BugAttrs == nil {
		t.Error("expected BugAttrs to be initialized")
	}

	// Second call should be idempotent
	task.BugAttrs.Severity = BugSeverityCritical
	task.EnsureBugAttrs()
	if task.BugAttrs.Severity != BugSeverityCritical {
		t.Error("EnsureBugAttrs should not reset existing attrs")
	}
}

func TestEnsureIdeaAttrs(t *testing.T) {
	task, err := NewTask("t1", 1, TaskTypeIdea, "Idea Test")
	if err != nil {
		t.Fatal(err)
	}

	task.EnsureIdeaAttrs()
	if task.IdeaAttrs == nil {
		t.Error("expected IdeaAttrs to be initialized")
	}

	task.IdeaAttrs.Value = IdeaValueHigh
	task.EnsureIdeaAttrs()
	if task.IdeaAttrs.Value != IdeaValueHigh {
		t.Error("EnsureIdeaAttrs should not reset existing attrs")
	}
}

func TestEnsureTaskAttrs(t *testing.T) {
	task, err := NewTask("t1", 1, TaskTypeTask, "Task Test")
	if err != nil {
		t.Fatal(err)
	}

	task.EnsureTaskAttrs()
	if task.TaskAttrs == nil {
		t.Error("expected TaskAttrs to be initialized")
	}

	task.TaskAttrs.Effort = EffortLarge
	task.EnsureTaskAttrs()
	if task.TaskAttrs.Effort != EffortLarge {
		t.Error("EnsureTaskAttrs should not reset existing attrs")
	}
}

func TestEnsureStoryAttrs(t *testing.T) {
	task, err := NewTask("t1", 1, TaskTypeStory, "Story Test")
	if err != nil {
		t.Fatal(err)
	}

	task.EnsureStoryAttrs()
	if task.StoryAttrs == nil {
		t.Error("expected StoryAttrs to be initialized")
	}

	task.StoryAttrs.StoryPoints = 13
	task.EnsureStoryAttrs()
	if task.StoryAttrs.StoryPoints != 13 {
		t.Error("EnsureStoryAttrs should not reset existing attrs")
	}
}

func TestEnsureEpicAttrs(t *testing.T) {
	task, err := NewTask("t1", 1, TaskTypeEpic, "Epic Test")
	if err != nil {
		t.Fatal(err)
	}

	task.EnsureEpicAttrs()
	if task.EpicAttrs == nil {
		t.Error("expected EpicAttrs to be initialized")
	}

	task.EpicAttrs.Scope = EpicScopeDefined
	task.EnsureEpicAttrs()
	if task.EpicAttrs.Scope != EpicScopeDefined {
		t.Error("EnsureEpicAttrs should not reset existing attrs")
	}
}

func TestParseTaskTypeInsensitive(t *testing.T) {
	tests := []struct {
		input    string
		expected TaskType
		valid    bool
	}{
		{"task", TaskTypeTask, true},
		{"TASK", TaskTypeTask, true},
		{"Task", TaskTypeTask, true},
		{"BUG", TaskTypeBug, true},
		{"Epic", TaskTypeEpic, true},
		{"STORY", TaskTypeStory, true},
		{"idea", TaskTypeIdea, true},
		{"invalid", TaskType("invalid"), false},
		{"", TaskType(""), false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, valid := ParseTaskTypeInsensitive(tt.input)
			if valid != tt.valid {
				t.Errorf("ParseTaskTypeInsensitive(%q) valid = %v, want %v", tt.input, valid, tt.valid)
			}
			if valid && got != tt.expected {
				t.Errorf("ParseTaskTypeInsensitive(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}
