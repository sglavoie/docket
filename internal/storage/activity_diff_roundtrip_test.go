package storage

import (
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/models"
)

func TestActivityOrderPreservedAcrossWriteAndParse(t *testing.T) {
	ts1 := time.Date(2025, 1, 10, 8, 0, 0, 0, time.Local)
	ts2 := time.Date(2025, 1, 11, 9, 0, 0, 0, time.Local)
	ts3 := time.Date(2025, 1, 12, 10, 0, 0, 0, time.Local)

	task, err := models.NewTask("task-1", 1, models.TaskTypeTask, "Test Task")
	if err != nil {
		t.Fatal(err)
	}
	task.CreatedAt = ts1
	task.UpdatedAt = ts3
	task.Activity = []*models.ActivityEntry{
		{Timestamp: ts1, Field: "created"},
		{Timestamp: ts2, Field: "status", OldValue: "todo", NewValue: "in_progress"},
		{Timestamp: ts3, Field: "priority", OldValue: "3", NewValue: "1"},
	}

	md := WriteTasksMarkdown("Test Project", []*models.Task{task})
	_, parsedTasks, parseErr := ParseTasksMarkdown(md)
	if parseErr != nil {
		t.Fatalf("ParseTasksMarkdown failed: %v", parseErr)
	}
	if len(parsedTasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(parsedTasks))
	}

	parsed := parsedTasks[0]
	if len(parsed.Activity) != 3 {
		t.Fatalf("expected 3 activity entries, got %d", len(parsed.Activity))
	}

	wantFields := []string{"created", "state", "priority"}
	for i, want := range wantFields {
		if parsed.Activity[i].Field != want {
			t.Errorf("activity[%d].Field = %q, want %q", i, parsed.Activity[i].Field, want)
		}
	}

	for i := 1; i < len(parsed.Activity); i++ {
		if parsed.Activity[i].Timestamp.Before(parsed.Activity[i-1].Timestamp) {
			t.Errorf("activity not in chronological order at index %d: %v before %v",
				i, parsed.Activity[i].Timestamp, parsed.Activity[i-1].Timestamp)
		}
	}
}
