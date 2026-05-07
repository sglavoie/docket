package tui

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

func TestGetStatusIndexCoversAllStatuses(t *testing.T) {
	tests := []struct {
		status models.TaskStatus
		want   int
	}{
		{models.StatusTodo, 0},
		{models.StatusInProgress, 1},
		{models.StatusReview, 2},
		{models.StatusStandby, 3},
		{models.StatusMaybe, 4},
		{models.StatusDone, 5},
		{models.StatusBlocked, 6},
		{models.StatusCanceled, 7},
	}

	for _, tt := range tests {
		got := getStatusIndex(tt.status)
		if got != tt.want {
			t.Errorf("getStatusIndex(%q) = %d, want %d", tt.status, got, tt.want)
		}
	}
}

func TestGetStatusIndexUniqueValues(t *testing.T) {
	all := models.AllTaskStatuses()
	seen := make(map[int]models.TaskStatus)
	for _, s := range all {
		idx := getStatusIndex(s)
		if prev, ok := seen[idx]; ok {
			t.Errorf("getStatusIndex(%q) = %d collides with %q", s, idx, prev)
		}
		seen[idx] = s
	}
}
