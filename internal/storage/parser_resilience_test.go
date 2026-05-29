package storage

import (
	"strings"
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

func TestParseTasksMarkdown_OrphanedParentPreserved(t *testing.T) {
	content := `# Project: Test

## Task: Child Task
<!-- id: child1, status: todo, priority: 3, parent: nonexistent-parent-id -->

Description of child.
`
	result, err := ParseTasksMarkdownWithSections(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(result.Tasks))
	}

	task := result.Tasks[0]
	if task.ParentID != nil {
		t.Error("ParentID should be nil for orphaned reference")
	}

	if len(result.Warnings) != 1 {
		t.Fatalf("expected 1 warning, got %d", len(result.Warnings))
	}
	if !strings.Contains(result.Warnings[0].Message, "clearing parent reference") {
		t.Errorf("warning should mention clearing parent reference, got: %s", result.Warnings[0].Message)
	}
}

func TestParseTasksMarkdown_OrphanedParentIgnored(t *testing.T) {
	content := `# Project: Test

## Task: Child Task
<!-- id: child1, status: todo, priority: 3, orphaned_parent: old-parent-id -->

Description of child.
`
	result, err := ParseTasksMarkdownWithSections(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(result.Tasks))
	}
	if result.Tasks[0].ID != "child1" {
		t.Errorf("task ID = %q, want %q", result.Tasks[0].ID, "child1")
	}
}

func TestParseTasksMarkdown_MissingMetadataComment(t *testing.T) {
	content := `# Project: Test

## Task: No Metadata Task

Some description here.

## Task: Normal Task
<!-- id: t1, status: todo, priority: 2 -->

Normal description.
`
	_, tasks, err := ParseTasksMarkdown(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(tasks) < 1 {
		t.Fatal("expected at least 1 task, got 0")
	}

	var foundNormal bool
	for _, task := range tasks {
		if task.ID == "t1" {
			foundNormal = true
			if task.Title != "Normal Task" {
				t.Errorf("expected title 'Normal Task', got %q", task.Title)
			}
		}
	}
	if !foundNormal {
		t.Error("expected to find task with id 't1'")
	}
}

func TestParseTasksMarkdown_BrokenHeadingFormat(t *testing.T) {
	tests := []struct {
		name      string
		content   string
		wantIDs   []string
		wantTitle map[string]string
	}{
		{
			name: "missing title after type colon",
			content: `# Project: Test

## Task: Normal Before
<!-- id: t0, status: todo, priority: 2 -->

## Task:
<!-- id: t1, status: todo, priority: 2 -->

## Task: Normal After
<!-- id: t2, status: todo, priority: 3 -->
`,
			wantIDs: []string{"t0", "t1", "t2"},
			wantTitle: map[string]string{
				"t0": "Normal Before",
				"t1": "",
				"t2": "Normal After",
			},
		},
		{
			name: "colon with missing title prefix",
			content: `# Project: Test

## : Missing Title
<!-- id: t1, status: todo, priority: 2 -->

## Task: Valid Task
<!-- id: t2, status: todo, priority: 3 -->
`,
			wantIDs: []string{"t2"},
			wantTitle: map[string]string{
				"t2": "Valid Task",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, err := ParseTasksMarkdownWithSections(tc.content)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result.Tasks == nil {
				t.Fatal("tasks should not be nil")
			}
			if len(result.Tasks) != len(tc.wantIDs) {
				t.Fatalf("expected %d tasks, got %d", len(tc.wantIDs), len(result.Tasks))
			}
			byID := make(map[string]*models.Task, len(result.Tasks))
			for _, task := range result.Tasks {
				byID[task.ID] = task
			}
			for _, id := range tc.wantIDs {
				task, ok := byID[id]
				if !ok {
					t.Fatalf("expected task ID %q in parsed tasks", id)
				}
				if task.Title != tc.wantTitle[id] {
					t.Errorf("task %s title = %q, want %q", id, task.Title, tc.wantTitle[id])
				}
			}
		})
	}
}

func TestParseTasksMarkdown_IncompleteMetadata(t *testing.T) {
	content := `# Project: Test

## Task: Only ID
<!-- id: t1 -->

Description for task with only ID.

## Task: ID and Status
<!-- id: t2, status: in_progress -->

Description here.
`
	_, tasks, err := ParseTasksMarkdown(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(tasks) != 2 {
		t.Fatalf("expected 2 tasks, got %d", len(tasks))
	}

	t1 := tasks[0]
	if t1.ID != "t1" {
		t.Errorf("expected ID 't1', got %q", t1.ID)
	}
	if t1.Title != "Only ID" {
		t.Errorf("expected title 'Only ID', got %q", t1.Title)
	}

	t2 := tasks[1]
	if t2.ID != "t2" {
		t.Errorf("expected ID 't2', got %q", t2.ID)
	}
	if t2.Status != models.StatusInProgress {
		t.Errorf("expected status in_progress, got %q", t2.Status)
	}
}

func TestParseTasksMarkdown_CorruptedMetadataValues(t *testing.T) {
	content := `# Project: Test

## Task: Bad Priority
<!-- id: t1, status: todo, priority: abc -->

## Task: Invalid Status
<!-- id: t2, status: invalid_status, priority: 2 -->

## Task: Both Bad
<!-- id: t3, status: nonexistent, priority: xyz -->
`
	result, err := ParseTasksMarkdownWithSections(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Tasks) != 3 {
		t.Fatalf("expected 3 tasks, got %d", len(result.Tasks))
	}

	t1 := result.Tasks[0]
	if t1.ID != "t1" {
		t.Errorf("expected ID 't1', got %q", t1.ID)
	}
	if t1.Status != models.StatusTodo {
		t.Errorf("expected status todo, got %q", t1.Status)
	}

	t2 := result.Tasks[1]
	if t2.ID != "t2" {
		t.Errorf("expected ID 't2', got %q", t2.ID)
	}
	if t2.Priority != 2 {
		t.Errorf("expected priority 2, got %d", t2.Priority)
	}

	if len(result.Warnings) == 0 {
		t.Error("expected warnings for corrupted metadata values")
	}
}

func TestParseTasksMarkdown_EmptyMetadataComment(t *testing.T) {
	content := `# Project: Test

## Task: Empty Meta
<!-- -->

Description for empty meta task.

## Task: Normal Task
<!-- id: t2, status: todo, priority: 3 -->

Normal description.
`
	_, tasks, err := ParseTasksMarkdown(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(tasks) < 1 {
		t.Fatal("expected at least 1 task")
	}

	var foundNormal bool
	for _, task := range tasks {
		if task.ID == "t2" {
			foundNormal = true
			if task.Title != "Normal Task" {
				t.Errorf("expected title 'Normal Task', got %q", task.Title)
			}
			if task.Priority != 3 {
				t.Errorf("expected priority 3, got %d", task.Priority)
			}
		}
	}
	if !foundNormal {
		t.Error("expected to find task with id 't2'")
	}
}

func TestParseTasksMarkdown_HeadingWithoutMetadata(t *testing.T) {
	content := `# Project: Test

## Task: No Metadata Task

Description without any metadata comment.

## Task: Normal Task
<!-- id: t2, status: todo, priority: 3 -->

Normal description.
`
	result, err := ParseTasksMarkdownWithSections(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Tasks) != 2 {
		t.Fatalf("expected 2 tasks, got %d", len(result.Tasks))
	}

	noMeta := result.Tasks[0]
	if noMeta.ID == "" {
		t.Error("task without metadata should have a generated UUID, got empty ID")
	}
	if noMeta.Title != "No Metadata Task" {
		t.Errorf("expected title 'No Metadata Task', got %q", noMeta.Title)
	}
	if noMeta.CreatedAt.IsZero() {
		t.Error("task without metadata should have CreatedAt set")
	}
	if noMeta.UpdatedAt.IsZero() {
		t.Error("task without metadata should have UpdatedAt set")
	}

	foundWarning := false
	for _, w := range result.Warnings {
		if w.Field == "id" && w.TaskID == noMeta.ID {
			foundWarning = true
			break
		}
	}
	if !foundWarning {
		t.Error("expected a warning about the generated ID")
	}

	normal := result.Tasks[1]
	if normal.ID != "t2" {
		t.Errorf("expected ID 't2', got %q", normal.ID)
	}
}
