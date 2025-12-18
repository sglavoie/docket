package storage

import (
	"strings"
	"testing"
	"time"

	"github.com/sglavoie/dev-helpers/go/tt/internal/models"
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
	if task.Status != models.StatusTodo {
		t.Errorf("expected Status 'todo', got '%s'", task.Status)
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
<!-- id: i1, status: cancelled, priority: 5 -->
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
<!-- id: t1, status: done, priority: 3, created: 2024-01-15, completed: 2024-01-20, due: 2024-02-01 -->
`
	_, tasks, err := ParseTasksMarkdown(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(tasks))
	}

	task := tasks[0]

	expectedCreated, _ := time.Parse("2006-01-02", "2024-01-15")
	if !task.CreatedAt.Equal(expectedCreated) {
		t.Errorf("expected CreatedAt %v, got %v", expectedCreated, task.CreatedAt)
	}

	if task.CompletedAt == nil {
		t.Error("expected CompletedAt to be set")
	} else {
		expectedCompleted, _ := time.Parse("2006-01-02", "2024-01-20")
		if !task.CompletedAt.Equal(expectedCompleted) {
			t.Errorf("expected CompletedAt %v, got %v", expectedCompleted, *task.CompletedAt)
		}
	}

	if task.DueDate == nil {
		t.Error("expected DueDate to be set")
	} else {
		expectedDue, _ := time.Parse("2006-01-02", "2024-02-01")
		if !task.DueDate.Equal(expectedDue) {
			t.Errorf("expected DueDate %v, got %v", expectedDue, *task.DueDate)
		}
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

func TestWriteTasksMarkdown(t *testing.T) {
	tasks := []*models.Task{
		{
			ID:        "epic1",
			ShortID:   1,
			Type:      models.TaskTypeEpic,
			Title:     "Test Epic",
			Status:    models.StatusInProgress,
			Priority:  1,
			Tags:      []string{"backend", "urgent"},
			CreatedAt: time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC),
		},
		{
			ID:        "task1",
			ShortID:   2,
			Type:      models.TaskTypeTask,
			Title:     "Test Task",
			Status:    models.StatusTodo,
			Priority:  3,
			CreatedAt: time.Date(2024, 1, 16, 0, 0, 0, 0, time.UTC),
		},
	}

	content := WriteTasksMarkdown("My Project", tasks)

	// Check project header
	if !strings.Contains(content, "# Project: My Project") {
		t.Error("content should contain project header")
	}

	// Check epic
	if !strings.Contains(content, "## Epic: Test Epic") {
		t.Error("content should contain epic heading")
	}
	if !strings.Contains(content, "id: epic1") {
		t.Error("content should contain epic ID")
	}
	if !strings.Contains(content, "status: in_progress") {
		t.Error("content should contain epic status")
	}
	if !strings.Contains(content, "priority: 1") {
		t.Error("content should contain epic priority")
	}
	if !strings.Contains(content, "tags: backend;urgent") {
		t.Error("content should contain epic tags")
	}

	// Check task
	if !strings.Contains(content, "## Task: Test Task") {
		t.Error("content should contain task heading")
	}
}

func TestWriteTasksMarkdown_Hierarchy(t *testing.T) {
	epicID := "epic1"
	storyID := "story1"

	tasks := []*models.Task{
		{
			ID:        "epic1",
			ShortID:   1,
			Type:      models.TaskTypeEpic,
			Title:     "Parent Epic",
			Status:    models.StatusInProgress,
			Priority:  1,
			CreatedAt: time.Now(),
		},
		{
			ID:        "story1",
			ShortID:   2,
			Type:      models.TaskTypeStory,
			Title:     "Child Story",
			Status:    models.StatusTodo,
			Priority:  2,
			ParentID:  &epicID,
			CreatedAt: time.Now(),
		},
		{
			ID:        "task1",
			ShortID:   3,
			Type:      models.TaskTypeTask,
			Title:     "Grandchild Task",
			Status:    models.StatusTodo,
			Priority:  3,
			ParentID:  &storyID,
			CreatedAt: time.Now(),
		},
	}

	content := WriteTasksMarkdown("Test", tasks)

	// Check heading levels
	if !strings.Contains(content, "## Epic:") {
		t.Error("epic should have ## heading")
	}
	if !strings.Contains(content, "### Story:") {
		t.Error("story should have ### heading (child of epic)")
	}
	if !strings.Contains(content, "#### Task:") {
		t.Error("task should have #### heading (grandchild)")
	}
}

func TestRoundTrip(t *testing.T) {
	// Create original tasks
	original := []*models.Task{
		{
			ID:          "e1",
			ShortID:     1,
			Type:        models.TaskTypeEpic,
			Title:       "Round Trip Epic",
			Description: "Epic description",
			Status:      models.StatusInProgress,
			Priority:    1,
			Tags:        []string{"test", "roundtrip"},
			CreatedAt:   time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
		},
		{
			ID:          "t1",
			ShortID:     2,
			Type:        models.TaskTypeTask,
			Title:       "Round Trip Task",
			Description: "Task description\nWith multiple lines",
			Status:      models.StatusDone,
			Priority:    2,
			CreatedAt:   time.Date(2024, 1, 16, 11, 45, 0, 0, time.UTC),
		},
	}

	// Write to markdown
	content := WriteTasksMarkdown("Round Trip Test", original)

	// Parse back
	projectName, parsed, err := ParseTasksMarkdown(content)
	if err != nil {
		t.Fatalf("failed to parse: %v", err)
	}

	if projectName != "Round Trip Test" {
		t.Errorf("project name mismatch: got '%s'", projectName)
	}

	if len(parsed) != len(original) {
		t.Fatalf("expected %d tasks, got %d", len(original), len(parsed))
	}

	// Compare key fields
	for i, orig := range original {
		p := parsed[i]
		if p.ID != orig.ID {
			t.Errorf("task %d: ID mismatch: %s vs %s", i, p.ID, orig.ID)
		}
		if p.Type != orig.Type {
			t.Errorf("task %d: Type mismatch: %s vs %s", i, p.Type, orig.Type)
		}
		if p.Title != orig.Title {
			t.Errorf("task %d: Title mismatch: %s vs %s", i, p.Title, orig.Title)
		}
		if p.Status != orig.Status {
			t.Errorf("task %d: Status mismatch: %s vs %s", i, p.Status, orig.Status)
		}
		if p.Priority != orig.Priority {
			t.Errorf("task %d: Priority mismatch: %d vs %d", i, p.Priority, orig.Priority)
		}
	}
}

func TestGetRecentTasks(t *testing.T) {
	now := time.Now()

	tasks := []*models.Task{
		{ID: "1", UpdatedAt: now.Add(-3 * time.Hour)},
		{ID: "2", UpdatedAt: now.Add(-1 * time.Hour)},
		{ID: "3", UpdatedAt: now.Add(-2 * time.Hour)},
		{ID: "4", UpdatedAt: now.Add(-5 * time.Minute)},
		{ID: "5", UpdatedAt: now.Add(-4 * time.Hour)},
	}

	recent := GetRecentTasks(tasks, 3)

	if len(recent) != 3 {
		t.Fatalf("expected 3 recent tasks, got %d", len(recent))
	}

	// Should be ordered by UpdatedAt descending
	expectedOrder := []string{"4", "2", "3"} // 5 min, 1 hour, 2 hours ago
	for i, id := range expectedOrder {
		if recent[i].ID != id {
			t.Errorf("position %d: expected ID %s, got %s", i, id, recent[i].ID)
		}
	}
}

func TestGetRecentTasks_LimitExceedsSize(t *testing.T) {
	tasks := []*models.Task{
		{ID: "1", UpdatedAt: time.Now()},
		{ID: "2", UpdatedAt: time.Now()},
	}

	recent := GetRecentTasks(tasks, 10)

	if len(recent) != 2 {
		t.Errorf("expected 2 tasks (all available), got %d", len(recent))
	}
}

func TestGetRecentTasks_Empty(t *testing.T) {
	recent := GetRecentTasks([]*models.Task{}, 5)
	if recent != nil {
		t.Errorf("expected nil for empty input, got %v", recent)
	}
}

func TestFormatRelativeTime(t *testing.T) {
	now := time.Now()

	tests := []struct {
		offset   time.Duration
		contains string
	}{
		{0, "just now"},
		{30 * time.Second, "just now"},
		{time.Minute, "1 minute ago"},
		{5 * time.Minute, "5 minutes ago"},
		{time.Hour, "1 hour ago"},
		{3 * time.Hour, "3 hours ago"},
		{24 * time.Hour, "yesterday"},
		{48 * time.Hour, "2 days ago"},
		{72 * time.Hour, "3 days ago"},
	}

	for _, tc := range tests {
		t.Run(tc.contains, func(t *testing.T) {
			result := FormatRelativeTime(now.Add(-tc.offset))
			if !strings.Contains(result, tc.contains) {
				t.Errorf("FormatRelativeTime(-%v) = %s, expected to contain %s", tc.offset, result, tc.contains)
			}
		})
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

func TestWriteTasksMarkdown_WithDependencies(t *testing.T) {
	task1 := models.NewTask("task1", 1, models.TaskTypeTask, "Blocked Task")
	task1.Status = models.StatusBlocked
	task1.DependsOn = []string{"blocker1", "blocker2"}

	task2 := models.NewTask("blocker1", 2, models.TaskTypeTask, "Blocker 1")
	task3 := models.NewTask("blocker2", 3, models.TaskTypeTask, "Blocker 2")

	content := WriteTasksMarkdown("Test", []*models.Task{task1, task2, task3})

	// Check that depends metadata is included
	if !strings.Contains(content, "depends: blocker1;blocker2") {
		t.Error("expected markdown to contain 'depends: blocker1;blocker2'")
	}
}

func TestDependenciesRoundTrip(t *testing.T) {
	original := models.NewTask("task1", 1, models.TaskTypeTask, "Task with deps")
	original.DependsOn = []string{"dep1", "dep2", "dep3"}

	content := WriteTasksMarkdown("Test", []*models.Task{original})

	_, tasks, err := ParseTasksMarkdown(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(tasks))
	}

	parsed := tasks[0]
	if len(parsed.DependsOn) != 3 {
		t.Errorf("expected 3 dependencies after round trip, got %d", len(parsed.DependsOn))
	}

	for i, dep := range original.DependsOn {
		if i >= len(parsed.DependsOn) || parsed.DependsOn[i] != dep {
			t.Errorf("dependency mismatch at index %d: expected %s", i, dep)
		}
	}
}
