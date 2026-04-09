package storage

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

// TestParse_MetadataCommentInsideCodeBlock verifies that a metadata-like
// HTML comment inside a fenced code block in the description does not get
// parsed as task metadata (because the task's ID is already set from the
// real metadata comment).
func TestParse_MetadataCommentInsideCodeBlock(t *testing.T) {
	content := "# Project: Test\n\n## Task: Code Example\n<!-- id: real-id, status: todo, priority: 3 -->\n\nHere is how task metadata looks:\n\n```markdown\n## Task: Fake Task\n<!-- id: fake-id, status: done, priority: 1 -->\n```\n\n## Task: After Code Block\n<!-- id: after-id, status: in_progress, priority: 2 -->\n"

	_, tasks, err := ParseTasksMarkdown(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(tasks) < 2 {
		t.Fatalf("expected at least 2 tasks, got %d", len(tasks))
	}

	var realTask, afterTask *models.Task
	for _, task := range tasks {
		switch task.ID {
		case "real-id":
			realTask = task
		case "after-id":
			afterTask = task
		}
	}

	if realTask == nil {
		t.Error("expected to find task with id 'real-id'")
	}
	if afterTask == nil {
		t.Error("expected to find task with id 'after-id'")
	}
	if afterTask != nil && afterTask.Status != models.StatusInProgress {
		t.Errorf("after-task status = %q, want 'in_progress'", afterTask.Status)
	}
}
