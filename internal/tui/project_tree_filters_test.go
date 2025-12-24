package tui

import (
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/models"
)

func TestIsOverdue(t *testing.T) {
	now := time.Now()
	yesterday := now.AddDate(0, 0, -1)
	tomorrow := now.AddDate(0, 0, 1)

	tests := []struct {
		name string
		task *models.Task
		want bool
	}{
		{
			name: "overdue task returns true",
			task: &models.Task{
				DueDate: &yesterday,
				Status:  models.StatusTodo,
			},
			want: true,
		},
		{
			name: "completed overdue task returns false",
			task: &models.Task{
				DueDate: &yesterday,
				Status:  models.StatusDone,
			},
			want: false,
		},
		{
			name: "future due date returns false",
			task: &models.Task{
				DueDate: &tomorrow,
				Status:  models.StatusTodo,
			},
			want: false,
		},
		{
			name: "nil due date returns false",
			task: &models.Task{
				DueDate: nil,
				Status:  models.StatusTodo,
			},
			want: false,
		},
		{
			name: "due today returns false",
			task: &models.Task{
				DueDate: &now,
				Status:  models.StatusTodo,
			},
			want: false,
		},
		{
			name: "in progress overdue returns true",
			task: &models.Task{
				DueDate: &yesterday,
				Status:  models.StatusInProgress,
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isOverdue(tt.task)
			if got != tt.want {
				t.Errorf("isOverdue() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsDueToday(t *testing.T) {
	now := time.Now()
	yesterday := now.AddDate(0, 0, -1)
	tomorrow := now.AddDate(0, 0, 1)

	tests := []struct {
		name string
		task *models.Task
		want bool
	}{
		{
			name: "due today returns true",
			task: &models.Task{
				DueDate: &now,
			},
			want: true,
		},
		{
			name: "due yesterday returns false",
			task: &models.Task{
				DueDate: &yesterday,
			},
			want: false,
		},
		{
			name: "due tomorrow returns false",
			task: &models.Task{
				DueDate: &tomorrow,
			},
			want: false,
		},
		{
			name: "nil due date returns false",
			task: &models.Task{
				DueDate: nil,
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isDueToday(tt.task)
			if got != tt.want {
				t.Errorf("isDueToday() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsDueThisWeek(t *testing.T) {
	now := time.Now()
	yesterday := now.AddDate(0, 0, -1)
	in3Days := now.AddDate(0, 0, 3)
	in6Days := now.AddDate(0, 0, 6)
	in7Days := now.AddDate(0, 0, 7)
	in8Days := now.AddDate(0, 0, 8)

	tests := []struct {
		name string
		task *models.Task
		want bool
	}{
		{
			name: "due in 3 days returns true",
			task: &models.Task{
				DueDate: &in3Days,
			},
			want: true,
		},
		{
			name: "due in 6 days returns true",
			task: &models.Task{
				DueDate: &in6Days,
			},
			want: true,
		},
		{
			name: "due in 7 days returns false",
			task: &models.Task{
				DueDate: &in7Days,
			},
			want: false,
		},
		{
			name: "due in 8 days returns false",
			task: &models.Task{
				DueDate: &in8Days,
			},
			want: false,
		},
		{
			name: "overdue returns false",
			task: &models.Task{
				DueDate: &yesterday,
			},
			want: false,
		},
		{
			name: "due today returns true",
			task: &models.Task{
				DueDate: &now,
			},
			want: true,
		},
		{
			name: "nil due date returns false",
			task: &models.Task{
				DueDate: nil,
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isDueThisWeek(tt.task)
			if got != tt.want {
				t.Errorf("isDueThisWeek() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCycleFilter(t *testing.T) {
	t.Run("string filters", func(t *testing.T) {
		values := []string{"a", "b", "c"}

		tests := []struct {
			name    string
			current string
			want    string
		}{
			{"empty to first", "", "a"},
			{"first to second", "a", "b"},
			{"second to third", "b", "c"},
			{"last wraps to empty", "c", ""},
			{"unknown wraps to empty", "x", ""},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				got := cycleFilter(tt.current, values)
				if got != tt.want {
					t.Errorf("cycleFilter(%q, values) = %q, want %q", tt.current, got, tt.want)
				}
			})
		}
	})

	t.Run("int filters", func(t *testing.T) {
		values := []int{1, 2, 3}

		tests := []struct {
			name    string
			current int
			want    int
		}{
			{"zero to first", 0, 1},
			{"first to second", 1, 2},
			{"last wraps to zero", 3, 0},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				got := cycleFilter(tt.current, values)
				if got != tt.want {
					t.Errorf("cycleFilter(%d, values) = %d, want %d", tt.current, got, tt.want)
				}
			})
		}
	})

	t.Run("empty values", func(t *testing.T) {
		got := cycleFilter("a", []string{})
		if got != "" {
			t.Errorf("cycleFilter with empty values = %q, want empty", got)
		}
	})
}
