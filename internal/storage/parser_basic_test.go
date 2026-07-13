package storage

import (
	"strings"
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

func TestParseTasksMarkdown_Empty(t *testing.T) {
	content := `# Project: Test Project

`
	projectName, tasks, err := ParseTasksMarkdown(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if projectName != "Test Project" {
		t.Errorf("expected project name 'Test Project', got '%s'", projectName)
	}

	if len(tasks) != 0 {
		t.Errorf("expected 0 tasks, got %d", len(tasks))
	}
}

func TestParseTasksMarkdown_SingleTask(t *testing.T) {
	content := `# Project: Test Project

## Task: My First Task
<!-- id: abc123, status: todo, priority: 2 -->

This is the description.
`
	projectName, tasks, err := ParseTasksMarkdown(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if projectName != "Test Project" {
		t.Errorf("expected project name 'Test Project', got '%s'", projectName)
	}

	if len(tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(tasks))
	}

	task := tasks[0]
	if task.ID != "abc123" {
		t.Errorf("expected ID 'abc123', got '%s'", task.ID)
	}
	if task.Title != "My First Task" {
		t.Errorf("expected Title 'My First Task', got '%s'", task.Title)
	}
	if task.Type != models.TaskTypeTask {
		t.Errorf("expected Type 'task', got '%s'", task.Type)
	}
	if task.State() != models.StateReady {
		t.Errorf("expected State 'ready', got '%s'", task.State())
	}
	if task.Priority != 2 {
		t.Errorf("expected Priority 2, got %d", task.Priority)
	}
	if task.Description != "This is the description." {
		t.Errorf("expected Description 'This is the description.', got '%s'", task.Description)
	}
}

func TestParseTasksMarkdown_AllTypes(t *testing.T) {
	content := `# Project: Test

## Epic: An Epic
<!-- id: e1, status: in_progress, priority: 1 -->

## Story: A Story
<!-- id: s1, status: todo, priority: 2 -->

## Task: A Task
<!-- id: t1, status: done, priority: 3 -->

## Bug: A Bug
<!-- id: b1, status: blocked, priority: 1 -->

## Idea: An Idea
<!-- id: i1, status: canceled, priority: 5 -->
`
	_, tasks, err := ParseTasksMarkdown(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(tasks) != 5 {
		t.Fatalf("expected 5 tasks, got %d", len(tasks))
	}

	expectedTypes := []models.TaskType{
		models.TaskTypeEpic,
		models.TaskTypeStory,
		models.TaskTypeTask,
		models.TaskTypeBug,
		models.TaskTypeIdea,
	}

	for i, task := range tasks {
		if task.Type != expectedTypes[i] {
			t.Errorf("task %d: expected Type '%s', got '%s'", i, expectedTypes[i], task.Type)
		}
	}
}

func TestParseTasksMarkdown_Hierarchy(t *testing.T) {
	content := `# Project: Test

## Epic: Parent Epic
<!-- id: epic1, status: in_progress, priority: 1 -->

### Story: Child Story
<!-- id: story1, status: todo, priority: 2 -->

#### Task: Grandchild Task
<!-- id: task1, status: todo, priority: 3 -->

## Task: Root Task
<!-- id: task2, status: todo, priority: 3 -->
`
	_, tasks, err := ParseTasksMarkdown(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(tasks) != 4 {
		t.Fatalf("expected 4 tasks, got %d", len(tasks))
	}

	// Check hierarchy is set up
	epic := tasks[0]
	story := tasks[1]
	task1 := tasks[2]
	task2 := tasks[3]

	// Story should be child of Epic
	if story.ParentID == nil || *story.ParentID != "epic1" {
		t.Error("story should have epic1 as parent")
	}

	// Task1 should be child of Story
	if task1.ParentID == nil || *task1.ParentID != "story1" {
		t.Error("task1 should have story1 as parent")
	}

	// Root task should have no parent
	if task2.ParentID != nil {
		t.Error("task2 should have no parent")
	}

	// Epic should have no parent
	if epic.ParentID != nil {
		t.Error("epic should have no parent")
	}
}

func TestParseTasksMarkdown_MultilineDescription(t *testing.T) {
	content := `# Project: Test

## Task: Multiline Task
<!-- id: t1, status: todo, priority: 3 -->

This is the first line.

This is a second paragraph.

- List item 1
- List item 2

## Task: Next Task
<!-- id: t2, status: todo, priority: 3 -->
`
	_, tasks, err := ParseTasksMarkdown(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(tasks) != 2 {
		t.Fatalf("expected 2 tasks, got %d", len(tasks))
	}

	desc := tasks[0].Description
	if !strings.Contains(desc, "first line") {
		t.Error("description should contain 'first line'")
	}
	if !strings.Contains(desc, "second paragraph") {
		t.Error("description should contain 'second paragraph'")
	}
	if !strings.Contains(desc, "List item 1") {
		t.Error("description should contain list items")
	}
}
