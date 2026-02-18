package tui

import (
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/util"
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
			got := tt.task.IsOverdue()
			if got != tt.want {
				t.Errorf("IsOverdue() = %v, want %v", got, tt.want)
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
			got := tt.task.IsDueToday()
			if got != tt.want {
				t.Errorf("IsDueToday() = %v, want %v", got, tt.want)
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
			got := tt.task.IsDueThisWeek()
			if got != tt.want {
				t.Errorf("IsDueThisWeek() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFormatDueDate(t *testing.T) {
	now := time.Now()
	today := util.StartOfDay(now)
	yesterday := today.AddDate(0, 0, -1)
	tomorrow := today.AddDate(0, 0, 1)
	nextWeek := today.AddDate(0, 0, 7)
	nextYear := today.AddDate(1, 0, 0)

	tests := []struct {
		name        string
		task        *models.Task
		shouldEmpty bool
		contains    string
	}{
		{
			name: "no due date returns empty",
			task: &models.Task{
				DueDate: nil,
				Status:  models.StatusTodo,
			},
			shouldEmpty: true,
		},
		{
			name: "completed task returns empty",
			task: &models.Task{
				DueDate: &today,
				Status:  models.StatusDone,
			},
			shouldEmpty: true,
		},
		{
			name: "canceled task returns empty",
			task: &models.Task{
				DueDate: &today,
				Status:  models.StatusCanceled,
			},
			shouldEmpty: true,
		},
		{
			name: "due today contains Today",
			task: &models.Task{
				DueDate: &today,
				Status:  models.StatusTodo,
			},
			contains: "Today",
		},
		{
			name: "due tomorrow contains Tomorrow",
			task: &models.Task{
				DueDate: &tomorrow,
				Status:  models.StatusTodo,
			},
			contains: "Tomorrow",
		},
		{
			name: "overdue shows date",
			task: &models.Task{
				DueDate: &yesterday,
				Status:  models.StatusTodo,
			},
			contains: yesterday.Format("Jan 2"),
		},
		{
			name: "next week shows date",
			task: &models.Task{
				DueDate: &nextWeek,
				Status:  models.StatusInProgress,
			},
			contains: nextWeek.Format("Jan 2"),
		},
		{
			name: "next year includes year",
			task: &models.Task{
				DueDate: &nextYear,
				Status:  models.StatusTodo,
			},
			contains: nextYear.Format("'06"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatDueDate(tt.task)
			if tt.shouldEmpty {
				if result != "" {
					t.Errorf("formatDueDate() = %q, expected empty", result)
				}
				return
			}
			if result == "" {
				t.Errorf("formatDueDate() returned empty, expected to contain %q", tt.contains)
			}
		})
	}
}
