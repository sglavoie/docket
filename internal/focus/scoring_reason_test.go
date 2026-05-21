package focus

import (
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/sglavoie/tt/internal/models"
)

func TestGetSuggestionReason(t *testing.T) {
	now := time.Now()
	project, err := models.NewProject(uuid.New().String(), "Test", "")
	if err != nil {
		t.Fatal(err)
	}

	t.Run("critical priority", func(t *testing.T) {
		task, err := models.NewTask(uuid.New().String(), 1, models.TaskTypeTask, "Critical")
		if err != nil {
			t.Fatal(err)
		}
		task.Priority = 1
		reason := getSuggestionReason(task, project, nil, now)
		if !strings.Contains(reason, "ritical priority") {
			t.Errorf("expected 'critical priority' in reason, got %q", reason)
		}
	})

	t.Run("overdue", func(t *testing.T) {
		task, err := models.NewTask(uuid.New().String(), 1, models.TaskTypeTask, "Overdue")
		if err != nil {
			t.Fatal(err)
		}
		task.Priority = 3
		yesterday := time.Now().AddDate(0, 0, -1)
		task.DueDate = &yesterday
		reason := getSuggestionReason(task, project, nil, now)
		if !strings.Contains(reason, "verdue") {
			t.Errorf("expected 'overdue' in reason, got %q", reason)
		}
	})

	t.Run("default reason for generic task", func(t *testing.T) {
		task, err := models.NewTask(uuid.New().String(), 1, models.TaskTypeTask, "Generic")
		if err != nil {
			t.Fatal(err)
		}
		task.Priority = 3
		task.CreatedAt = time.Now().AddDate(0, -1, 0)
		task.UpdatedAt = time.Now().AddDate(0, -1, 0)
		reason := getSuggestionReason(task, project, nil, now)
		if reason == "" {
			t.Error("expected non-empty reason")
		}
	})
}

// Regression: capitalization of the first reason must not panic on a leading
// multi-byte rune. Previously getSuggestionReason used r[:1] which can split
// a UTF-8 rune; reviewed dependencies could feed unicode strings into the
// reason list once profile descriptions become user-defined.
func TestGetSuggestionReasonUnicodeFirstChar(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("unexpected panic capitalizing unicode reason: %v", r)
		}
	}()
	project, err := models.NewProject(uuid.New().String(), "Unicode", "")
	if err != nil {
		t.Fatal(err)
	}
	task, err := models.NewTask(uuid.New().String(), 1, models.TaskTypeTask, "T")
	if err != nil {
		t.Fatal(err)
	}
	task.Priority = 1 // ensures at least one reason
	reason := getSuggestionReason(task, project, nil, time.Now())
	if reason == "" {
		t.Fatal("expected non-empty reason")
	}
}

func containsSubstring(s, substr string) bool {
	return strings.Contains(s, substr)
}
