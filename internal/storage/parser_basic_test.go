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

	// ParentID should be cleared
	if task.ParentID != nil {
		t.Error("ParentID should be nil for orphaned reference")
	}

	// Warning should mention clearing parent reference
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

	// Should parse without error, orphaned_parent key is simply ignored
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

	// Parser should not crash; it should produce tasks even without metadata
	if len(tasks) < 1 {
		t.Fatal("expected at least 1 task, got 0")
	}

	// The task without metadata should still exist (with defaults)
	// and the normal task should parse correctly
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
		name    string
		content string
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
		},
		{
			name: "colon with missing title prefix",
			content: `# Project: Test

## : Missing Title
<!-- id: t1, status: todo, priority: 2 -->

## Task: Valid Task
<!-- id: t2, status: todo, priority: 3 -->
`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Should not panic
			_, tasks, err := ParseTasksMarkdown(tc.content)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			// Should parse at least the valid tasks without crashing
			if tasks == nil {
				t.Fatal("tasks should not be nil")
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

	// Task with only ID should have defaults for status and priority
	t1 := tasks[0]
	if t1.ID != "t1" {
		t.Errorf("expected ID 't1', got %q", t1.ID)
	}
	if t1.Title != "Only ID" {
		t.Errorf("expected title 'Only ID', got %q", t1.Title)
	}

	// Task with ID and status but no priority
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

	// Bad priority should fall back to 0 and produce a warning
	t1 := result.Tasks[0]
	if t1.ID != "t1" {
		t.Errorf("expected ID 't1', got %q", t1.ID)
	}
	if t1.Status != models.StatusTodo {
		t.Errorf("expected status todo, got %q", t1.Status)
	}

	// Invalid status should keep the zero-value default
	t2 := result.Tasks[1]
	if t2.ID != "t2" {
		t.Errorf("expected ID 't2', got %q", t2.ID)
	}
	if t2.Priority != 2 {
		t.Errorf("expected priority 2, got %d", t2.Priority)
	}

	// Should have warnings for corrupted values
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

	// Should not panic; should parse at least the normal task
	if len(tasks) < 1 {
		t.Fatal("expected at least 1 task")
	}

	// The normal task should be parseable
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

	// First task had no metadata - should get a generated UUID
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
	// Should have a warning about the generated ID
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

	// Second task should be normal
	normal := result.Tasks[1]
	if normal.ID != "t2" {
		t.Errorf("expected ID 't2', got %q", normal.ID)
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

func TestParseTasksMarkdown_BrokenMetadataComment(t *testing.T) {
	tests := []struct {
		name    string
		content string
		desc    string // what to verify
	}{
		{
			name: "unclosed HTML comment",
			content: `# Project: Test

## Task: Task With Unclosed Comment
<!-- id: t1, status: todo, priority: 2

Some description that follows.

## Task: Normal Task
<!-- id: t2, status: todo, priority: 3 -->
`,
			desc: "unclosed comment should not crash parser",
		},
		{
			name: "truncated comment opener",
			content: `# Project: Test

## Task: Task With Bad Opener
<!- id: t1, status: todo, priority: 2 -->

## Task: Normal Task
<!-- id: t2, status: todo, priority: 3 -->
`,
			desc: "truncated opener should not crash parser",
		},
		{
			name: "nested HTML comments",
			content: `# Project: Test

## Task: Nested Comments
<!-- id: t1, status: todo, priority: 2 -->

Description with <!-- nested --> comment markers.

## Task: After Nested
<!-- id: t2, status: done, priority: 1 -->
`,
			desc: "nested comment markers in description",
		},
		{
			name: "only closing comment tag",
			content: `# Project: Test

## Task: Just Closing
--> id: t1, status: todo

## Task: Normal After
<!-- id: t2, status: todo, priority: 3 -->
`,
			desc: "only closing tag should not crash parser",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, tasks, err := ParseTasksMarkdown(tc.content)
			if err != nil {
				t.Fatalf("parser should not return error for %s: %v", tc.desc, err)
			}

			// Should always parse at least one task without panicking
			if len(tasks) < 1 {
				t.Fatalf("expected at least 1 task for %s, got 0", tc.desc)
			}

			// The "Normal Task" (t2) should always be parseable
			var foundNormal bool
			for _, task := range tasks {
				if task.ID == "t2" {
					foundNormal = true
				}
			}
			if !foundNormal {
				t.Errorf("expected to find task t2 in %s", tc.desc)
			}
		})
	}
}

func TestParseTasksMarkdown_LineExceedingScannerBuffer(t *testing.T) {
	// A line longer than scannerMaxBufSize (1MB) should return a parse error
	// instead of silently truncating the file.
	longLine := strings.Repeat("x", 2*1024*1024) // 2MB line
	content := "# Project: Test\n\n## Task: Normal\n<!-- id: t1, status: todo, priority: 3 -->\n\n" + longLine + "\n"

	_, err := ParseTasksMarkdownWithSections(content)
	if err == nil {
		t.Fatal("expected error for line exceeding scanner buffer, got nil")
	}
	if !strings.Contains(err.Error(), "parse") {
		t.Errorf("expected parse error, got: %v", err)
	}
}

func TestParseTasksMarkdown_CommentMarkerInDescription(t *testing.T) {
	// Verify that <!-- in task descriptions doesn't corrupt parsing
	content := `# Project: Test

## Task: Has Comment Markers
<!-- id: t1, status: todo, priority: 2 -->

This description has <!-- an HTML comment --> embedded in it.
And another <!-- partial marker without closing.
Plus a real code example: <!-- TODO: fix this -->

## Task: After Comment Markers
<!-- id: t2, status: in_progress, priority: 1 -->

Normal description.
`
	result, err := ParseTasksMarkdownWithSections(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Tasks) != 2 {
		t.Fatalf("expected 2 tasks, got %d", len(result.Tasks))
	}

	// First task should have its description with the comment markers preserved
	t1 := result.Tasks[0]
	if t1.ID != "t1" {
		t.Errorf("expected first task ID 't1', got %q", t1.ID)
	}
	if !strings.Contains(t1.Description, "<!-- an HTML comment -->") {
		t.Error("expected description to preserve comment markers")
	}

	// Second task should parse normally
	t2 := result.Tasks[1]
	if t2.ID != "t2" {
		t.Errorf("expected second task ID 't2', got %q", t2.ID)
	}
	if t2.Status != models.StatusInProgress {
		t.Errorf("expected status in_progress, got %q", t2.Status)
	}
}

func TestRoundTripEdgeCaseTitleChars(t *testing.T) {
	// Titles with special chars that are NOT rejected by ValidateTaskTitle
	edgeTitles := []string{
		"Use #hashtag in title",
		"Arrow -> points right",
		"Single # sign",
		"Backslash \\ character",
		"Ampersand & symbol",
		"Asterisk * wildcard",
		"[brackets] around text",
		"(parens) around text",
		`Backtick ` + "`code`" + ` reference`,
	}

	for _, title := range edgeTitles {
		t.Run(title, func(t *testing.T) {
			tasks := []*models.Task{
				{
					ID:      "task1",
					ShortID: 1,
					Type:    models.TaskTypeTask,
					Title:   title,
					Status:  models.StatusTodo,
					Tags:    []string{},
				},
			}

			// Write to markdown
			content := WriteTasksMarkdown("Edge Case Project", tasks)

			// Parse back
			_, parsed, err := ParseTasksMarkdown(content)
			if err != nil {
				t.Fatalf("parse error for title %q: %v", title, err)
			}
			if len(parsed) != 1 {
				t.Fatalf("expected 1 task, got %d", len(parsed))
			}
			if parsed[0].Title != title {
				t.Errorf("title mismatch: got %q, want %q", parsed[0].Title, title)
			}
		})
	}
}

func TestParseDeprecatedNoteType(t *testing.T) {
	content := `# Project: Test

## Note: My Meeting Notes
<!-- id: note-1, status: todo, priority: 3 -->

Some notes here.
`
	result, err := ParseTasksMarkdownWithSections(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(result.Tasks))
	}

	task := result.Tasks[0]
	// "Note" is a deprecated type that should be migrated to "Task"
	if task.Type != models.TaskTypeTask {
		t.Errorf("expected type 'task', got %q", task.Type)
	}
	if task.Title != "My Meeting Notes" {
		t.Errorf("expected title 'My Meeting Notes', got %q", task.Title)
	}

	// Should produce a deprecation warning (not a validation warning)
	var found bool
	for _, w := range result.Warnings {
		if w.Field == "type" && strings.Contains(w.Message, "deprecated type 'note'") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected deprecation warning for 'note' type, got warnings: %v", result.Warnings)
	}
}

func TestParseDeprecatedActionType(t *testing.T) {
	content := `# Project: Test

## Action: Review PR
<!-- id: act-1, status: todo, priority: 2 -->

Review the pull request.
`
	result, err := ParseTasksMarkdownWithSections(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(result.Tasks))
	}

	task := result.Tasks[0]
	// "Action" is a deprecated type that should be migrated to "Task"
	if task.Type != models.TaskTypeTask {
		t.Errorf("expected type 'task', got %q", task.Type)
	}
	if task.Title != "Review PR" {
		t.Errorf("expected title 'Review PR', got %q", task.Title)
	}

	// Should produce a deprecation warning
	var found bool
	for _, w := range result.Warnings {
		if w.Field == "type" && strings.Contains(w.Message, "deprecated type 'action'") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected deprecation warning for 'action' type, got warnings: %v", result.Warnings)
	}
}
