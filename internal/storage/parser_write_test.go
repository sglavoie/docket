package storage

import (
	"strings"
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/models"
)

func TestWriteTasksMarkdown(t *testing.T) {
	tasks := []*models.Task{
		{
			ID:            "epic1",
			ShortID:       1,
			Type:          models.TaskTypeEpic,
			Title:         "Test Epic",
			WorkflowState: models.StateInProgress,
			Priority:      1,
			Tags:          []string{"backend", "urgent"},
			CreatedAt:     time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC),
		},
		{
			ID:            "task1",
			ShortID:       2,
			Type:          models.TaskTypeTask,
			Title:         "Test Task",
			WorkflowState: models.StateReady,
			Priority:      3,
			CreatedAt:     time.Date(2024, 1, 16, 0, 0, 0, 0, time.UTC),
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
	if !strings.Contains(content, "state: in_progress") {
		t.Error("content should contain epic state")
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
			ID:            "epic1",
			ShortID:       1,
			Type:          models.TaskTypeEpic,
			Title:         "Parent Epic",
			WorkflowState: models.StateInProgress,
			Priority:      1,
			CreatedAt:     time.Now(),
		},
		{
			ID:            "story1",
			ShortID:       2,
			Type:          models.TaskTypeStory,
			Title:         "Child Story",
			WorkflowState: models.StateReady,
			Priority:      2,
			ParentID:      &epicID,
			CreatedAt:     time.Now(),
		},
		{
			ID:            "task1",
			ShortID:       3,
			Type:          models.TaskTypeTask,
			Title:         "Grandchild Task",
			WorkflowState: models.StateReady,
			Priority:      3,
			ParentID:      &storyID,
			CreatedAt:     time.Now(),
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

func TestWriteTasksMarkdown_WithDependencies(t *testing.T) {
	task1, err := models.NewTask("task1", 1, models.TaskTypeTask, "Blocked Task")
	if err != nil {
		t.Fatal(err)
	}
	task1.WorkflowState = models.StateWaiting
	task1.DependsOn = []string{"blocker1", "blocker2"}

	task2, err := models.NewTask("blocker1", 2, models.TaskTypeTask, "Blocker 1")
	if err != nil {
		t.Fatal(err)
	}
	task3, err := models.NewTask("blocker2", 3, models.TaskTypeTask, "Blocker 2")
	if err != nil {
		t.Fatal(err)
	}

	content := WriteTasksMarkdown("Test", []*models.Task{task1, task2, task3})

	// Check that depends metadata is included
	if !strings.Contains(content, "depends: blocker1;blocker2") {
		t.Error("expected markdown to contain 'depends: blocker1;blocker2'")
	}
}

func TestRoundTrip(t *testing.T) {
	// Create original tasks
	original := []*models.Task{
		{
			ID:            "e1",
			ShortID:       1,
			Type:          models.TaskTypeEpic,
			Title:         "Round Trip Epic",
			Description:   "Epic description",
			WorkflowState: models.StateInProgress,
			Priority:      1,
			Tags:          []string{"test", "roundtrip"},
			CreatedAt:     time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
		},
		{
			ID:            "t1",
			ShortID:       2,
			Type:          models.TaskTypeTask,
			Title:         "Round Trip Task",
			Description:   "Task description\nWith multiple lines",
			WorkflowState: models.StateDone,
			Priority:      2,
			CreatedAt:     time.Date(2024, 1, 16, 11, 45, 0, 0, time.UTC),
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
		if p.State() != orig.State() {
			t.Errorf("task %d: State mismatch: %s vs %s", i, p.State(), orig.State())
		}
		if p.Priority != orig.Priority {
			t.Errorf("task %d: Priority mismatch: %d vs %d", i, p.Priority, orig.Priority)
		}
	}
}

func TestDependenciesRoundTrip(t *testing.T) {
	original, err := models.NewTask("task1", 1, models.TaskTypeTask, "Task with deps")
	if err != nil {
		t.Fatal(err)
	}
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

func TestParentIDRoundTrip(t *testing.T) {
	parentID := "epic1"
	storyID := "story1"

	tasks := []*models.Task{
		{
			ID:            "epic1",
			ShortID:       1,
			Type:          models.TaskTypeEpic,
			Title:         "Parent Epic",
			WorkflowState: models.StateInProgress,
			Priority:      1,
			CreatedAt:     time.Now(),
		},
		{
			ID:            "story1",
			ShortID:       2,
			Type:          models.TaskTypeStory,
			Title:         "Child Story",
			WorkflowState: models.StateReady,
			Priority:      2,
			ParentID:      &parentID,
			CreatedAt:     time.Now(),
		},
		{
			ID:            "task1",
			ShortID:       3,
			Type:          models.TaskTypeTask,
			Title:         "Grandchild Task",
			WorkflowState: models.StateReady,
			Priority:      3,
			ParentID:      &storyID,
			CreatedAt:     time.Now(),
		},
	}

	content := WriteTasksMarkdown("Test", tasks)

	// Verify parent: metadata is written
	if !strings.Contains(content, "parent: epic1") {
		t.Error("expected metadata to contain 'parent: epic1' for story")
	}
	if !strings.Contains(content, "parent: story1") {
		t.Error("expected metadata to contain 'parent: story1' for task")
	}

	// Parse back and verify parent IDs are preserved
	_, parsed, err := ParseTasksMarkdown(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(parsed) != 3 {
		t.Fatalf("expected 3 tasks, got %d", len(parsed))
	}

	story := parsed[1]
	if story.ParentID == nil || *story.ParentID != "epic1" {
		t.Error("story should have epic1 as parent after round trip")
	}

	task := parsed[2]
	if task.ParentID == nil || *task.ParentID != "story1" {
		t.Error("task should have story1 as parent after round trip")
	}
}

func TestDuplicateTaskIDsDetected(t *testing.T) {
	content := `# Project: Test

## Task: First Task
<!-- id: dup-id, status: todo, priority: 1 -->

First description.

## Task: Second Task
<!-- id: dup-id, status: in_progress, priority: 2 -->

Second description.
`
	result, err := ParseTasksMarkdownDocument(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Tasks) != 2 {
		t.Fatalf("expected 2 tasks, got %d", len(result.Tasks))
	}

	// Both tasks should exist with unique IDs
	if result.Tasks[0].ID == result.Tasks[1].ID {
		t.Errorf("duplicate task IDs should be reassigned, but both have %q", result.Tasks[0].ID)
	}

	// The first task keeps the original ID
	if result.Tasks[0].ID != "dup-id" {
		t.Errorf("first task should keep original ID 'dup-id', got %q", result.Tasks[0].ID)
	}

	// A warning should be emitted for the duplicate
	foundDupWarning := false
	for _, w := range result.Warnings {
		if w.Field == "id" && strings.Contains(w.Message, "duplicate") {
			foundDupWarning = true
			break
		}
	}
	if !foundDupWarning {
		t.Error("expected a warning about duplicate task ID")
	}
}

func TestHeadingLevelCappedAtSix(t *testing.T) {
	// Create a deeply nested structure using explicit parent IDs
	// that would exceed heading level 6 if uncapped
	epicID := "epic1"
	storyID := "story1"

	tasks := []*models.Task{
		{
			ID:            "epic1",
			ShortID:       1,
			Type:          models.TaskTypeEpic,
			Title:         "Root Epic",
			WorkflowState: models.StateReady,
			Priority:      1,
			CreatedAt:     time.Now(),
		},
		{
			ID:            "story1",
			ShortID:       2,
			Type:          models.TaskTypeStory,
			Title:         "Child Story",
			WorkflowState: models.StateReady,
			Priority:      2,
			ParentID:      &epicID,
			CreatedAt:     time.Now(),
		},
		{
			ID:            "task1",
			ShortID:       3,
			Type:          models.TaskTypeTask,
			Title:         "Deep Task",
			WorkflowState: models.StateReady,
			Priority:      3,
			ParentID:      &storyID,
			CreatedAt:     time.Now(),
		},
	}

	content := WriteTasksMarkdown("Test", tasks)

	// All headings should use at most 6 # characters
	for line := range strings.SplitSeq(content, "\n") {
		if strings.HasPrefix(line, "#") {
			hashes := 0
			for _, c := range line {
				if c == '#' {
					hashes++
				} else {
					break
				}
			}
			if hashes > 6 {
				t.Errorf("heading has %d # characters (max 6): %s", hashes, line)
			}
		}
	}

	// Verify the content round-trips correctly
	_, parsed, err := ParseTasksMarkdown(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(parsed) != len(tasks) {
		t.Fatalf("expected %d tasks after round trip, got %d", len(tasks), len(parsed))
	}

	// Verify parent relationships are preserved via parent: metadata
	for _, p := range parsed {
		original := findTaskByID(tasks, p.ID)
		if original == nil {
			t.Errorf("task %s not found in original", p.ID)
			continue
		}
		origParent := ""
		if original.ParentID != nil {
			origParent = *original.ParentID
		}
		parsedParent := ""
		if p.ParentID != nil {
			parsedParent = *p.ParentID
		}
		if origParent != parsedParent {
			t.Errorf("task %s: parent mismatch: original=%q parsed=%q", p.ID, origParent, parsedParent)
		}
	}
}
