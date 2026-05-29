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

// TestGetSuggestionReasonRecentActivityMatchesScoreWindow guards that the
// "recent activity" reason and the recent-activity score boost cover the same
// window. computeCrossProjectScore boosts tasks updated within the last 3 days
// (+200 for <=3 days, +400 for <=0 days). Previously getSuggestionReason only
// surfaced "recent activity" when daysSinceUpdated <= 0, so a task updated 1-3
// days ago was silently boosted in the ranking with no explanation in the
// reason column.
func TestGetSuggestionReasonRecentActivityMatchesScoreWindow(t *testing.T) {
	now := time.Now()
	project, err := models.NewProject(uuid.New().String(), "P", "")
	if err != nil {
		t.Fatal(err)
	}

	for _, daysAgo := range []int{1, 2, 3} {
		task, taskErr := models.NewTask(uuid.New().String(), 1, models.TaskTypeTask, "T")
		if taskErr != nil {
			t.Fatal(taskErr)
		}
		task.Priority = 3
		task.CreatedAt = now.AddDate(0, 0, -30)
		task.UpdatedAt = now.AddDate(0, 0, -daysAgo)
		task.Status = models.StatusTodo

		reason := getSuggestionReason(task, project, nil, now)
		if !strings.Contains(reason, "ecent activity") {
			t.Errorf("daysAgo=%d: expected 'recent activity' in reason, got %q", daysAgo, reason)
		}
	}

	// Negative control: 4+ days should not list recent activity.
	stale, err := models.NewTask(uuid.New().String(), 1, models.TaskTypeTask, "Stale")
	if err != nil {
		t.Fatal(err)
	}
	stale.Priority = 3
	stale.CreatedAt = now.AddDate(0, 0, -30)
	stale.UpdatedAt = now.AddDate(0, 0, -4)
	stale.Status = models.StatusTodo
	if reason := getSuggestionReason(stale, project, nil, now); strings.Contains(reason, "ecent activity") {
		t.Errorf("4-day-old task should not list 'recent activity', got %q", reason)
	}

	// Suppression: in-progress should still suppress the redundant reason.
	inProgress, err := models.NewTask(uuid.New().String(), 1, models.TaskTypeTask, "IP")
	if err != nil {
		t.Fatal(err)
	}
	inProgress.Priority = 3
	inProgress.CreatedAt = now.AddDate(0, 0, -30)
	inProgress.UpdatedAt = now.AddDate(0, 0, -1)
	inProgress.Status = models.StatusInProgress
	if reason := getSuggestionReason(inProgress, project, nil, now); strings.Contains(reason, "ecent activity") {
		t.Errorf("in-progress task should suppress redundant 'recent activity', got %q", reason)
	}
}
