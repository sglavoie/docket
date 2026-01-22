package storage

import (
	"strings"
	"testing"
	"time"

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

func TestParseTasksMarkdownWithSections_DefaultBacklog(t *testing.T) {
	content := `# Project: Test Project

## Task: My Task
<!-- id: task1, status: todo, priority: 3 -->
`
	result, err := ParseTasksMarkdownWithSections(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should default to Next when no sections specified
	if len(result.Sections) != 1 {
		t.Errorf("expected 1 default section, got %d", len(result.Sections))
	}
	if result.Sections[0] != models.SectionNext {
		t.Errorf("expected default section to be Next, got %s", result.Sections[0])
	}
}

func TestParseTasksMarkdownWithSections_ExplicitSections(t *testing.T) {
	content := `# Project: Test Project
<!-- sections: Backlog;Active;Review;Done -->

## Task: My Task
<!-- id: task1, status: todo, priority: 3 -->
`
	result, err := ParseTasksMarkdownWithSections(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expectedSections := []string{"Backlog", "Active", "Review", "Done"}
	if len(result.Sections) != len(expectedSections) {
		t.Errorf("expected %d sections, got %d", len(expectedSections), len(result.Sections))
	}

	for i, expected := range expectedSections {
		if i >= len(result.Sections) || result.Sections[i] != expected {
			t.Errorf("section %d: expected %s, got %s", i, expected, result.Sections[i])
		}
	}
}

func TestParseTasksMarkdownWithSections_SectionsAfterTaskIgnored(t *testing.T) {
	content := `# Project: Test Project

## Task: My Task
<!-- id: task1, status: todo, priority: 3 -->

<!-- sections: Backlog;Active;Done -->
`
	result, err := ParseTasksMarkdownWithSections(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Sections after first task should be ignored, defaulting to Next
	if len(result.Sections) != 1 {
		t.Errorf("expected 1 section (sections after task ignored), got %d", len(result.Sections))
	}
	if result.Sections[0] != models.SectionNext {
		t.Errorf("expected default Next, got %s", result.Sections[0])
	}
}

func TestParseTasksMarkdownWithSections_TaskWithSection(t *testing.T) {
	content := `# Project: Test Project
<!-- sections: Backlog;Active;Done -->

## Task: My Task
<!-- id: task1, status: todo, priority: 3, section: Active -->

Description here.
`
	result, err := ParseTasksMarkdownWithSections(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(result.Tasks))
	}

	if result.Tasks[0].Section != "Active" {
		t.Errorf("expected task section 'Active', got '%s'", result.Tasks[0].Section)
	}
}

func TestWriteTasksMarkdownWithSections(t *testing.T) {
	task := models.NewTask("task1", 1, models.TaskTypeTask, "Test Task")
	task.Section = "Active"
	tasks := []*models.Task{task}
	sections := []string{"Backlog", "Active", "Done"}

	content := WriteTasksMarkdownWithSections("Test Project", tasks, sections, nil)

	// Check project header
	if !strings.Contains(content, "# Project: Test Project") {
		t.Error("content should contain project header")
	}

	// Check sections metadata
	if !strings.Contains(content, "<!-- sections: Backlog;Active;Done -->") {
		t.Error("content should contain sections metadata")
	}

	// Check task section
	if !strings.Contains(content, "section: Active") {
		t.Error("content should contain task section")
	}
}

func TestWriteTasksMarkdownWithSections_EmptySections(t *testing.T) {
	task := models.NewTask("task1", 1, models.TaskTypeTask, "Test Task")
	tasks := []*models.Task{task}

	content := WriteTasksMarkdownWithSections("Test Project", tasks, nil, nil)

	// Should not contain sections line when nil
	if strings.Contains(content, "<!-- sections:") {
		t.Error("content should not contain sections metadata when sections is nil")
	}

	content = WriteTasksMarkdownWithSections("Test Project", tasks, []string{}, nil)

	// Should not contain sections line when empty
	if strings.Contains(content, "<!-- sections:") {
		t.Error("content should not contain sections metadata when sections is empty")
	}
}

func TestSectionsRoundTrip(t *testing.T) {
	task1 := models.NewTask("task1", 1, models.TaskTypeTask, "Task in Active")
	task1.Section = "Active"
	task2 := models.NewTask("task2", 2, models.TaskTypeTask, "Task in Backlog")
	task2.Section = "Backlog"
	task3 := models.NewTask("task3", 3, models.TaskTypeTask, "Task in Done")
	task3.Section = "Done"

	tasks := []*models.Task{task1, task2, task3}
	sections := []string{"Backlog", "Active", "Review", "Done"}

	// Write to markdown
	content := WriteTasksMarkdownWithSections("Round Trip Test", tasks, sections, nil)

	// Parse back
	result, err := ParseTasksMarkdownWithSections(content)
	if err != nil {
		t.Fatalf("failed to parse: %v", err)
	}

	// Check project name
	if result.ProjectName != "Round Trip Test" {
		t.Errorf("project name mismatch: got '%s'", result.ProjectName)
	}

	// Check sections
	if len(result.Sections) != len(sections) {
		t.Errorf("expected %d sections, got %d", len(sections), len(result.Sections))
	}
	for i, expected := range sections {
		if i >= len(result.Sections) || result.Sections[i] != expected {
			t.Errorf("section %d: expected %s, got %s", i, expected, result.Sections[i])
		}
	}

	// Check tasks
	if len(result.Tasks) != 3 {
		t.Fatalf("expected 3 tasks, got %d", len(result.Tasks))
	}

	if result.Tasks[0].Section != "Active" {
		t.Errorf("task1 section should be Active, got %s", result.Tasks[0].Section)
	}
	if result.Tasks[1].Section != "Backlog" {
		t.Errorf("task2 section should be Backlog, got %s", result.Tasks[1].Section)
	}
	if result.Tasks[2].Section != "Done" {
		t.Errorf("task3 section should be Done, got %s", result.Tasks[2].Section)
	}
}

func TestSectionColorsRoundTrip(t *testing.T) {
	task := models.NewTask("task1", 1, models.TaskTypeTask, "Test Task")
	task.Section = "Active"
	tasks := []*models.Task{task}
	sections := []string{"Backlog", "Active", "Done"}
	colors := map[string]string{
		"Active": "green",
		"Done":   "blue",
	}

	// Write to markdown with colors
	content := WriteTasksMarkdownWithSections("Color Test", tasks, sections, colors)

	// Check that colors are in the output
	if !strings.Contains(content, "Active:green") {
		t.Error("content should contain Active:green")
	}
	if !strings.Contains(content, "Done:blue") {
		t.Error("content should contain Done:blue")
	}
	// Backlog should not have a color (default gray is not written)
	if strings.Contains(content, "Backlog:") {
		t.Error("Backlog should not have a color suffix")
	}

	// Parse back
	result, err := ParseTasksMarkdownWithSections(content)
	if err != nil {
		t.Fatalf("failed to parse: %v", err)
	}

	// Check section colors
	if result.SectionColors["Active"] != "green" {
		t.Errorf("Active color should be green, got %s", result.SectionColors["Active"])
	}
	if result.SectionColors["Done"] != "blue" {
		t.Errorf("Done color should be blue, got %s", result.SectionColors["Done"])
	}
	if _, ok := result.SectionColors["Backlog"]; ok {
		t.Error("Backlog should not have a color in the map")
	}
}

func TestParseTasksMarkdown_Note(t *testing.T) {
	content := `# Project: Test Project

## Note: My Meeting Notes
<!-- id: note1, section: Backlog, created: 2024-01-15 10:30 -->

Discussion points from standup.
`
	_, tasks, err := ParseTasksMarkdown(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(tasks))
	}

	note := tasks[0]
	if note.ID != "note1" {
		t.Errorf("expected ID 'note1', got '%s'", note.ID)
	}
	if note.Type != models.TaskTypeNote {
		t.Errorf("expected Type 'note', got '%s'", note.Type)
	}
	if note.Title != "My Meeting Notes" {
		t.Errorf("expected Title 'My Meeting Notes', got '%s'", note.Title)
	}
	if note.Description != "Discussion points from standup." {
		t.Errorf("expected Description 'Discussion points from standup.', got '%s'", note.Description)
	}
}

func TestWriteTasksMarkdown_Note(t *testing.T) {
	note := models.NewTask("note-id", 1, models.TaskTypeNote, "My Note")
	note.Description = "Note content here."

	tasks := []*models.Task{note}
	content := WriteTasksMarkdown("Test", tasks)

	// Notes should not have status or priority in metadata
	if strings.Contains(content, "status:") {
		t.Error("Note should not have status in metadata")
	}
	if strings.Contains(content, "priority:") {
		t.Error("Note should not have priority in metadata")
	}
	// Should have the heading
	if !strings.Contains(content, "## Note: My Note") {
		t.Error("Note should have proper heading")
	}
	// Should have id
	if !strings.Contains(content, "id: note-id") {
		t.Error("Note should have id in metadata")
	}
}

func TestRoundTrip_Note(t *testing.T) {
	// Create a note
	note := models.NewTask("note-abc", 1, models.TaskTypeNote, "Test Note")
	note.Description = "This is a test note."
	note.Section = "Active"

	tasks := []*models.Task{note}
	content := WriteTasksMarkdown("Test Project", tasks)

	// Parse it back
	_, parsed, err := ParseTasksMarkdown(content)
	if err != nil {
		t.Fatalf("failed to parse: %v", err)
	}

	if len(parsed) != 1 {
		t.Fatalf("expected 1 task, got %d", len(parsed))
	}

	parsedNote := parsed[0]
	if parsedNote.Type != models.TaskTypeNote {
		t.Errorf("expected Type 'note', got '%s'", parsedNote.Type)
	}
	if parsedNote.ID != "note-abc" {
		t.Errorf("expected ID 'note-abc', got '%s'", parsedNote.ID)
	}
	if parsedNote.Title != "Test Note" {
		t.Errorf("expected Title 'Test Note', got '%s'", parsedNote.Title)
	}
}

func TestParseTasksMarkdown_WithComments(t *testing.T) {
	content := `# Project: Test Project

## Task: Task with Comments
<!-- id: task1, status: todo, priority: 3 -->

Task description here.

<!-- COMMENTS -->
<!-- comment: 2024-12-27 10:30 -->
First comment text.
<!-- comment: 2024-12-27 14:45 -->
Second comment text.
<!-- /COMMENTS -->
`
	_, tasks, err := ParseTasksMarkdown(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(tasks))
	}

	task := tasks[0]
	if task.Description != "Task description here." {
		t.Errorf("expected description 'Task description here.', got '%s'", task.Description)
	}

	if len(task.Comments) != 2 {
		t.Fatalf("expected 2 comments, got %d", len(task.Comments))
	}

	if task.Comments[0].Text != "First comment text." {
		t.Errorf("expected first comment 'First comment text.', got '%s'", task.Comments[0].Text)
	}
	if task.Comments[1].Text != "Second comment text." {
		t.Errorf("expected second comment 'Second comment text.', got '%s'", task.Comments[1].Text)
	}

	// Check timestamps
	expectedTime1, _ := time.ParseInLocation("2006-01-02 15:04", "2024-12-27 10:30", time.Local)
	if !task.Comments[0].CreatedAt.Equal(expectedTime1) {
		t.Errorf("expected first comment time %v, got %v", expectedTime1, task.Comments[0].CreatedAt)
	}
}

func TestWriteTasksMarkdown_WithComments(t *testing.T) {
	task := models.NewTask("task1", 1, models.TaskTypeTask, "Task with Comments")
	task.Comments = []*models.Comment{
		{Text: "First comment", CreatedAt: time.Date(2024, 12, 27, 10, 30, 0, 0, time.Local)},
		{Text: "Second comment", CreatedAt: time.Date(2024, 12, 27, 14, 45, 0, 0, time.Local)},
	}

	content := WriteTasksMarkdown("Test", []*models.Task{task})

	// Check comments block markers
	if !strings.Contains(content, "<!-- COMMENTS -->") {
		t.Error("expected markdown to contain '<!-- COMMENTS -->'")
	}
	if !strings.Contains(content, "<!-- /COMMENTS -->") {
		t.Error("expected markdown to contain '<!-- /COMMENTS -->'")
	}

	// Check comment headers and text
	if !strings.Contains(content, "<!-- comment: 2024-12-27 10:30 -->") {
		t.Error("expected markdown to contain first comment timestamp")
	}
	if !strings.Contains(content, "First comment") {
		t.Error("expected markdown to contain first comment text")
	}
	if !strings.Contains(content, "<!-- comment: 2024-12-27 14:45 -->") {
		t.Error("expected markdown to contain second comment timestamp")
	}
	if !strings.Contains(content, "Second comment") {
		t.Error("expected markdown to contain second comment text")
	}
}

func TestCommentsRoundTrip(t *testing.T) {
	original := models.NewTask("task1", 1, models.TaskTypeTask, "Task with Comments")
	original.Description = "Task description."
	original.Comments = []*models.Comment{
		{Text: "First comment", CreatedAt: time.Date(2024, 12, 27, 10, 30, 0, 0, time.Local)},
		{Text: "Second comment", CreatedAt: time.Date(2024, 12, 27, 14, 45, 0, 0, time.Local)},
	}

	content := WriteTasksMarkdown("Test", []*models.Task{original})

	_, tasks, err := ParseTasksMarkdown(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(tasks))
	}

	parsed := tasks[0]

	// Description should not contain comments block
	if strings.Contains(parsed.Description, "COMMENTS") {
		t.Error("description should not contain COMMENTS markers after parsing")
	}

	if len(parsed.Comments) != 2 {
		t.Fatalf("expected 2 comments after round trip, got %d", len(parsed.Comments))
	}

	if parsed.Comments[0].Text != "First comment" {
		t.Errorf("first comment text mismatch: expected 'First comment', got '%s'", parsed.Comments[0].Text)
	}
	if parsed.Comments[1].Text != "Second comment" {
		t.Errorf("second comment text mismatch: expected 'Second comment', got '%s'", parsed.Comments[1].Text)
	}

	// Check timestamps survived round trip
	if !parsed.Comments[0].CreatedAt.Equal(original.Comments[0].CreatedAt) {
		t.Errorf("first comment timestamp mismatch: expected %v, got %v",
			original.Comments[0].CreatedAt, parsed.Comments[0].CreatedAt)
	}
}

func TestParseTasksMarkdown_MultilineComments(t *testing.T) {
	content := `# Project: Test Project

## Task: Task with Multiline Comment
<!-- id: task1, status: todo, priority: 3 -->

<!-- COMMENTS -->
<!-- comment: 2024-12-27 10:30 -->
This is line one.
This is line two.
This is line three.
<!-- /COMMENTS -->
`
	_, tasks, err := ParseTasksMarkdown(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(tasks))
	}

	task := tasks[0]
	if len(task.Comments) != 1 {
		t.Fatalf("expected 1 comment, got %d", len(task.Comments))
	}

	expectedText := "This is line one.\nThis is line two.\nThis is line three."
	if task.Comments[0].Text != expectedText {
		t.Errorf("expected multiline comment text:\n%s\ngot:\n%s", expectedText, task.Comments[0].Text)
	}
}

func TestParseTasksMarkdown_CommentsWithEscapedChars(t *testing.T) {
	// When reading a file with escaped markers (written by writeComments),
	// the parser should unescape them back to the original text.
	content := `# Project: Test Project

## Task: Task with Special Chars
<!-- id: task1, status: todo, priority: 3 -->

<!-- COMMENTS -->
<!-- comment: 2024-12-27 10:30 -->
Comment with code: func() { return &lt;!-- test --&gt; }
<!-- /COMMENTS -->
`
	_, tasks, err := ParseTasksMarkdown(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(tasks))
	}

	task := tasks[0]
	if len(task.Comments) != 1 {
		t.Fatalf("expected 1 comment, got %d", len(task.Comments))
	}

	// The escaped markers should be unescaped back to original
	expected := "Comment with code: func() { return <!-- test --> }"
	if task.Comments[0].Text != expected {
		t.Errorf("expected unescaped markers:\n%s\ngot:\n%s", expected, task.Comments[0].Text)
	}
}

func TestCommentsRoundTrip_WithHTMLMarkers(t *testing.T) {
	// This tests that comments containing HTML comment markers survive round trip
	original := models.NewTask("task1", 1, models.TaskTypeTask, "Task")
	original.Comments = []*models.Comment{
		{Text: "Code example: <!-- this is a comment -->", CreatedAt: time.Date(2024, 12, 27, 10, 30, 0, 0, time.Local)},
	}

	content := WriteTasksMarkdown("Test", []*models.Task{original})

	_, tasks, err := ParseTasksMarkdown(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(tasks))
	}

	if len(tasks[0].Comments) != 1 {
		t.Fatalf("expected 1 comment after round trip, got %d", len(tasks[0].Comments))
	}

	// The comment should survive round trip (escaping and unescaping)
	if tasks[0].Comments[0].Text != original.Comments[0].Text {
		t.Errorf("comment text mismatch after round trip:\nexpected: %s\ngot: %s",
			original.Comments[0].Text, tasks[0].Comments[0].Text)
	}
}

func TestCommentsRoundTrip_WithEndMarker(t *testing.T) {
	// Test the edge case where comment text contains the end marker
	original := models.NewTask("task1", 1, models.TaskTypeTask, "Task")
	original.Comments = []*models.Comment{
		{Text: "Here is an example: <!-- /COMMENTS --> which should be escaped", CreatedAt: time.Date(2024, 12, 27, 10, 30, 0, 0, time.Local)},
		{Text: "Second comment should still appear", CreatedAt: time.Date(2024, 12, 27, 11, 0, 0, 0, time.Local)},
	}

	content := WriteTasksMarkdown("Test", []*models.Task{original})

	_, tasks, err := ParseTasksMarkdown(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(tasks))
	}

	if len(tasks[0].Comments) != 2 {
		t.Fatalf("expected 2 comments after round trip, got %d", len(tasks[0].Comments))
	}

	// Both comments should survive round trip
	if tasks[0].Comments[0].Text != original.Comments[0].Text {
		t.Errorf("first comment text mismatch:\nexpected: %s\ngot: %s",
			original.Comments[0].Text, tasks[0].Comments[0].Text)
	}
	if tasks[0].Comments[1].Text != original.Comments[1].Text {
		t.Errorf("second comment text mismatch:\nexpected: %s\ngot: %s",
			original.Comments[1].Text, tasks[0].Comments[1].Text)
	}
}

func TestCommentsRoundTrip_WithEndMarkerOnItsOwnLine(t *testing.T) {
	// Test the edge case where comment text is exactly the end marker on its own line
	original := models.NewTask("task1", 1, models.TaskTypeTask, "Task")
	original.Comments = []*models.Comment{
		{Text: "Line 1\n<!-- /COMMENTS -->\nLine 3", CreatedAt: time.Date(2024, 12, 27, 10, 30, 0, 0, time.Local)},
		{Text: "Second comment", CreatedAt: time.Date(2024, 12, 27, 11, 0, 0, 0, time.Local)},
	}

	content := WriteTasksMarkdown("Test", []*models.Task{original})

	_, tasks, err := ParseTasksMarkdown(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(tasks))
	}

	if len(tasks[0].Comments) != 2 {
		t.Fatalf("expected 2 comments after round trip, got %d", len(tasks[0].Comments))
	}

	// The comment with the marker should survive round trip
	if tasks[0].Comments[0].Text != original.Comments[0].Text {
		t.Errorf("first comment text mismatch:\nexpected: %s\ngot: %s",
			original.Comments[0].Text, tasks[0].Comments[0].Text)
	}
}

func TestParseTasksMarkdown_WithRecurrence(t *testing.T) {
	content := `# Project: Test Project

## Task: Daily Standup
<!-- id: task1, status: todo, priority: 2, recur: 1d -->

Daily standup meeting.

## Task: Weekly Review
<!-- id: task2, status: todo, priority: 2, recur: 1w -->

Weekly review.

## Task: MWF Meeting
<!-- id: task3, status: todo, priority: 2, recur: 1w:mon,wed,fri -->

Meeting on Mon, Wed, Fri.
`
	_, tasks, err := ParseTasksMarkdown(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(tasks) != 3 {
		t.Fatalf("expected 3 tasks, got %d", len(tasks))
	}

	// Daily task
	if tasks[0].Recurrence == nil {
		t.Error("task1 should have recurrence")
	} else {
		if tasks[0].Recurrence.Interval != 1 {
			t.Errorf("task1 recurrence interval = %d, want 1", tasks[0].Recurrence.Interval)
		}
		if tasks[0].Recurrence.Unit != models.RecurrenceDaily {
			t.Errorf("task1 recurrence unit = %s, want day", tasks[0].Recurrence.Unit)
		}
	}

	// Weekly task
	if tasks[1].Recurrence == nil {
		t.Error("task2 should have recurrence")
	} else {
		if tasks[1].Recurrence.Interval != 1 {
			t.Errorf("task2 recurrence interval = %d, want 1", tasks[1].Recurrence.Interval)
		}
		if tasks[1].Recurrence.Unit != models.RecurrenceWeekly {
			t.Errorf("task2 recurrence unit = %s, want week", tasks[1].Recurrence.Unit)
		}
	}

	// Weekly with weekdays
	if tasks[2].Recurrence == nil {
		t.Error("task3 should have recurrence")
	} else {
		if tasks[2].Recurrence.Interval != 1 {
			t.Errorf("task3 recurrence interval = %d, want 1", tasks[2].Recurrence.Interval)
		}
		if tasks[2].Recurrence.Unit != models.RecurrenceWeekly {
			t.Errorf("task3 recurrence unit = %s, want week", tasks[2].Recurrence.Unit)
		}
		expectedDays := []int{1, 3, 5}
		if len(tasks[2].Recurrence.Weekdays) != len(expectedDays) {
			t.Errorf("task3 weekdays = %v, want %v", tasks[2].Recurrence.Weekdays, expectedDays)
		} else {
			for i, d := range expectedDays {
				if tasks[2].Recurrence.Weekdays[i] != d {
					t.Errorf("task3 weekdays[%d] = %d, want %d", i, tasks[2].Recurrence.Weekdays[i], d)
				}
			}
		}
	}
}

func TestRecurrenceRoundTrip(t *testing.T) {
	tests := []struct {
		name       string
		recurrence *models.RecurrencePattern
	}{
		{"daily", &models.RecurrencePattern{Interval: 1, Unit: models.RecurrenceDaily}},
		{"every 3 days", &models.RecurrencePattern{Interval: 3, Unit: models.RecurrenceDaily}},
		{"weekly", &models.RecurrencePattern{Interval: 1, Unit: models.RecurrenceWeekly}},
		{"biweekly", &models.RecurrencePattern{Interval: 2, Unit: models.RecurrenceWeekly}},
		{"monthly", &models.RecurrencePattern{Interval: 1, Unit: models.RecurrenceMonthly}},
		{"quarterly", &models.RecurrencePattern{Interval: 3, Unit: models.RecurrenceMonthly}},
		{"yearly", &models.RecurrencePattern{Interval: 1, Unit: models.RecurrenceYearly}},
		{"every 2 years", &models.RecurrencePattern{Interval: 2, Unit: models.RecurrenceYearly}},
		{"weekly mon", &models.RecurrencePattern{Interval: 1, Unit: models.RecurrenceWeekly, Weekdays: []int{1}}},
		{"weekly mon,wed,fri", &models.RecurrencePattern{Interval: 1, Unit: models.RecurrenceWeekly, Weekdays: []int{1, 3, 5}}},
		{"biweekly tue,thu", &models.RecurrencePattern{Interval: 2, Unit: models.RecurrenceWeekly, Weekdays: []int{2, 4}}},
		{"weekly all days", &models.RecurrencePattern{Interval: 1, Unit: models.RecurrenceWeekly, Weekdays: []int{0, 1, 2, 3, 4, 5, 6}}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dueDate := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
			original := models.NewTask("task1", 1, models.TaskTypeTask, "Recurring Task")
			original.DueDate = &dueDate
			original.Recurrence = tc.recurrence

			content := WriteTasksMarkdown("Test", []*models.Task{original})

			_, tasks, err := ParseTasksMarkdown(content)
			if err != nil {
				t.Fatalf("failed to parse: %v", err)
			}

			if len(tasks) != 1 {
				t.Fatalf("expected 1 task, got %d", len(tasks))
			}

			parsed := tasks[0]
			if parsed.Recurrence == nil {
				t.Fatal("parsed task should have recurrence")
			}

			if parsed.Recurrence.Interval != tc.recurrence.Interval {
				t.Errorf("interval mismatch: got %d, want %d", parsed.Recurrence.Interval, tc.recurrence.Interval)
			}
			if parsed.Recurrence.Unit != tc.recurrence.Unit {
				t.Errorf("unit mismatch: got %s, want %s", parsed.Recurrence.Unit, tc.recurrence.Unit)
			}
			if len(parsed.Recurrence.Weekdays) != len(tc.recurrence.Weekdays) {
				t.Errorf("weekdays length mismatch: got %v, want %v", parsed.Recurrence.Weekdays, tc.recurrence.Weekdays)
			} else {
				for i, wd := range tc.recurrence.Weekdays {
					if parsed.Recurrence.Weekdays[i] != wd {
						t.Errorf("weekday[%d] mismatch: got %d, want %d", i, parsed.Recurrence.Weekdays[i], wd)
					}
				}
			}
		})
	}
}

func TestWriteTasksMarkdown_WithRecurrence(t *testing.T) {
	dueDate := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)

	task1 := models.NewTask("task1", 1, models.TaskTypeTask, "Daily Task")
	task1.DueDate = &dueDate
	task1.Recurrence = &models.RecurrencePattern{Interval: 1, Unit: models.RecurrenceDaily}

	task2 := models.NewTask("task2", 2, models.TaskTypeTask, "Weekly MWF Task")
	task2.DueDate = &dueDate
	task2.Recurrence = &models.RecurrencePattern{Interval: 1, Unit: models.RecurrenceWeekly, Weekdays: []int{1, 3, 5}}

	content := WriteTasksMarkdown("Test", []*models.Task{task1, task2})

	if !strings.Contains(content, "recur: 1d") {
		t.Error("expected markdown to contain 'recur: 1d'")
	}
	if !strings.Contains(content, "recur: 1w:mon,wed,fri") {
		t.Error("expected markdown to contain 'recur: 1w:mon,wed,fri'")
	}
}
