package storage

import (
	"strings"
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/models"
)

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

func TestGetRecentTasks_NonPositiveLimit(t *testing.T) {
	tasks := []*models.Task{{ID: "1", UpdatedAt: time.Now()}}

	for _, limit := range []int{0, -1} {
		recent := GetRecentTasks(tasks, limit)
		if recent != nil {
			t.Errorf("limit %d: expected nil, got %v", limit, recent)
		}
	}
}

func TestGetRecentTasks_IgnoresNilTasks(t *testing.T) {
	now := time.Now()
	tasks := []*models.Task{
		nil,
		{ID: "1", UpdatedAt: now.Add(-time.Hour)},
		{ID: "2", UpdatedAt: now},
	}

	recent := GetRecentTasks(tasks, 3)
	if len(recent) != 2 {
		t.Fatalf("expected 2 non-nil tasks, got %d", len(recent))
	}
	if recent[0].ID != "2" || recent[1].ID != "1" {
		t.Fatalf("unexpected order: got IDs %q, %q", recent[0].ID, recent[1].ID)
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
