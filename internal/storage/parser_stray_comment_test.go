package storage

import (
	"strings"
	"testing"
)

// TestParse_StrayHTMLCommentAfterHeading guards against the parser silently
// swallowing the real metadata line when a non-metadata HTML comment appears
// as the first post-heading line. Before the fix, the comment lit the
// `metadataParsed` latch unconditionally, so the legitimate metadata line on
// the next row was treated as description content — losing the task's ID,
// timestamps, and other metadata on every load.
func TestParse_StrayHTMLCommentAfterHeading(t *testing.T) {
	doc := strings.Join([]string{
		"# Project: Demo",
		"",
		"## Task: With stray comment",
		"<!-- editorial note: review this one -->",
		"<!-- id: real-uuid-001, status: todo, priority: 2, created: 2024-01-02 10:00 -->",
		"Body text here.",
	}, "\n") + "\n"

	_, tasks, err := ParseTasksMarkdown(doc)
	if err != nil {
		t.Fatalf("ParseTasksMarkdown: %v", err)
	}
	if len(tasks) != 1 {
		t.Fatalf("got %d tasks, want 1", len(tasks))
	}
	task := tasks[0]
	if task.ID != "real-uuid-001" {
		t.Errorf("task.ID = %q, want %q (metadata was swallowed by stray comment)", task.ID, "real-uuid-001")
	}
	if task.Priority != 2 {
		t.Errorf("task.Priority = %d, want 2", task.Priority)
	}
	if task.CreatedAt.IsZero() {
		t.Error("task.CreatedAt is zero; metadata line was not parsed")
	}
	// The stray comment should still appear in the description verbatim so we
	// don't silently discard the user's content.
	if !strings.Contains(task.Description, "editorial note") {
		t.Errorf("description does not retain the stray comment: %q", task.Description)
	}
}
