package storage

import (
	"testing"
	"time"
)

func TestParseTasksMarkdown_Tags(t *testing.T) {
	content := `# Project: Test

## Task: Tagged Task
<!-- id: t1, status: todo, priority: 3, tags: backend;urgent;api -->

Description here.
`
	_, tasks, err := ParseTasksMarkdown(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(tasks))
	}

	task := tasks[0]
	expectedTags := []string{"backend", "urgent", "api"}

	if len(task.Tags) != len(expectedTags) {
		t.Errorf("expected %d tags, got %d", len(expectedTags), len(task.Tags))
	}

	for i, tag := range expectedTags {
		if i >= len(task.Tags) || task.Tags[i] != tag {
			t.Errorf("expected tag '%s' at position %d", tag, i)
		}
	}
}

func TestParseTasksMarkdown_Dates(t *testing.T) {
	content := `# Project: Test

## Task: Dated Task
<!-- id: t1, status: done, priority: 3, created: 2024-01-15 14:30, completed: 2024-01-20 09:45, due: 2024-02-01 -->
`
	_, tasks, err := ParseTasksMarkdown(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(tasks))
	}

	task := tasks[0]

	expectedCreated, _ := time.ParseInLocation("2006-01-02 15:04", "2024-01-15 14:30", time.Local)
	if !task.CreatedAt.Equal(expectedCreated) {
		t.Errorf("expected CreatedAt %v, got %v", expectedCreated, task.CreatedAt)
	}

	if task.CompletedAt == nil {
		t.Error("expected CompletedAt to be set")
	} else {
		expectedCompleted, _ := time.ParseInLocation("2006-01-02 15:04", "2024-01-20 09:45", time.Local)
		if !task.CompletedAt.Equal(expectedCompleted) {
			t.Errorf("expected CompletedAt %v, got %v", expectedCompleted, *task.CompletedAt)
		}
	}

	if task.DueDate == nil {
		t.Error("expected DueDate to be set")
	} else {
		expectedDue, _ := time.ParseInLocation("2006-01-02", "2024-02-01", time.Local)
		if !task.DueDate.Equal(expectedDue) {
			t.Errorf("expected DueDate %v, got %v", expectedDue, *task.DueDate)
		}
	}
}

func TestParseTasksMarkdown_DatesBackwardsCompatibility(t *testing.T) {
	// Test that date-only format (without time) still parses correctly
	content := `# Project: Test

## Task: Legacy Dated Task
<!-- id: t1, status: done, priority: 3, created: 2024-01-15, completed: 2024-01-20 -->
`
	_, tasks, err := ParseTasksMarkdown(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(tasks))
	}

	task := tasks[0]

	expectedCreated, _ := time.ParseInLocation("2006-01-02", "2024-01-15", time.Local)
	if !task.CreatedAt.Equal(expectedCreated) {
		t.Errorf("expected CreatedAt %v, got %v", expectedCreated, task.CreatedAt)
	}

	if task.CompletedAt == nil {
		t.Error("expected CompletedAt to be set")
	} else {
		expectedCompleted, _ := time.ParseInLocation("2006-01-02", "2024-01-20", time.Local)
		if !task.CompletedAt.Equal(expectedCompleted) {
			t.Errorf("expected CompletedAt %v, got %v", expectedCompleted, *task.CompletedAt)
		}
	}
}

func TestParseTasksMarkdown_WithDependencies(t *testing.T) {
	content := `# Project: Test Project

## Task: Blocked Task
<!-- id: task1, status: blocked, priority: 2, depends: blocker1;blocker2 -->

This task is blocked.

## Task: Blocker 1
<!-- id: blocker1, status: todo, priority: 1 -->

First blocker.

## Task: Blocker 2
<!-- id: blocker2, status: done, priority: 1 -->

Second blocker.
`
	_, tasks, err := ParseTasksMarkdown(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(tasks) != 3 {
		t.Fatalf("expected 3 tasks, got %d", len(tasks))
	}

	blockedTask := tasks[0]
	if len(blockedTask.DependsOn) != 2 {
		t.Errorf("expected 2 dependencies, got %d", len(blockedTask.DependsOn))
	}

	if blockedTask.DependsOn[0] != "blocker1" {
		t.Errorf("expected first dependency 'blocker1', got '%s'", blockedTask.DependsOn[0])
	}
	if blockedTask.DependsOn[1] != "blocker2" {
		t.Errorf("expected second dependency 'blocker2', got '%s'", blockedTask.DependsOn[1])
	}

	// Other tasks should have no dependencies
	if len(tasks[1].DependsOn) != 0 {
		t.Errorf("expected 0 dependencies for blocker1, got %d", len(tasks[1].DependsOn))
	}
}
