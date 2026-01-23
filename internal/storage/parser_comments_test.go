package storage

import (
	"strings"
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/models"
)

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
