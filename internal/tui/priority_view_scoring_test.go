package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/models"
)

func TestComputeUrgencyScore(t *testing.T) {
	project := newTestPriorityProject(t)
	m := NewPriorityViewModel(project, nil, nil)

	tests := []struct {
		name    string
		task    *models.Task
		wantMin int
		wantMax int
	}{
		{
			name: "priority 1 base score",
			task: func() *models.Task {
				task, err := models.NewTask("1", 1, models.TaskTypeTask, "High priority")
				if err != nil {
					t.Fatal(err)
				}
				task.Priority = 1
				return task
			}(),
			wantMin: 5000,
			wantMax: 5000,
		},
		{
			name: "priority 5 base score",
			task: func() *models.Task {
				task, err := models.NewTask("2", 2, models.TaskTypeTask, "Low priority")
				if err != nil {
					t.Fatal(err)
				}
				task.Priority = 5
				return task
			}(),
			wantMin: 1000,
			wantMax: 1000,
		},
		{
			name: "priority 3 base score",
			task: func() *models.Task {
				task, err := models.NewTask("3", 3, models.TaskTypeTask, "Medium priority")
				if err != nil {
					t.Fatal(err)
				}
				task.Priority = 3
				return task
			}(),
			wantMin: 3000,
			wantMax: 3000,
		},
		{
			name: "in progress bonus",
			task: func() *models.Task {
				task, err := models.NewTask("4", 4, models.TaskTypeTask, "In progress task")
				if err != nil {
					t.Fatal(err)
				}
				task.Priority = 3
				task.Status = models.StatusInProgress
				return task
			}(),
			wantMin: 3500,
			wantMax: 3500,
		},
		{
			name: "active flag bonus",
			task: func() *models.Task {
				task, err := models.NewTask("5", 5, models.TaskTypeTask, "Active task")
				if err != nil {
					t.Fatal(err)
				}
				task.Priority = 3
				task.Active = true
				return task
			}(),
			wantMin: 3200,
			wantMax: 3200,
		},
		{
			name: "combined bonuses",
			task: func() *models.Task {
				task, err := models.NewTask("6", 6, models.TaskTypeTask, "Active in progress")
				if err != nil {
					t.Fatal(err)
				}
				task.Priority = 1
				task.Status = models.StatusInProgress
				task.Active = true
				return task
			}(),
			wantMin: 5700,
			wantMax: 5700,
		},
		{
			name: "bug type bonus",
			task: func() *models.Task {
				task, err := models.NewTask("7", 7, models.TaskTypeBug, "Bug task")
				if err != nil {
					t.Fatal(err)
				}
				task.Priority = 3
				return task
			}(),
			wantMin: 3100,
			wantMax: 3100,
		},
		{
			name: "bug outranks task at same priority",
			task: func() *models.Task {
				task, err := models.NewTask("8", 8, models.TaskTypeBug, "Bug at priority 3")
				if err != nil {
					t.Fatal(err)
				}
				task.Priority = 3
				return task
			}(),
			wantMin: 3100,
			wantMax: 3100,
		},
		{
			name: "overdue adds points",
			task: func() *models.Task {
				task := newTestTaskWithDueDate(t, "Overdue task", -2)
				task.Priority = 3
				return task
			}(),
			wantMin: 3100,
			wantMax: 3100,
		},
		{
			name: "future due subtracts points",
			task: func() *models.Task {
				task := newTestTaskWithDueDate(t, "Future task", 2)
				task.Priority = 3
				return task
			}(),
			wantMin: 2900,
			wantMax: 2900,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := m.computeUrgencyScore(tt.task)
			if got < tt.wantMin || got > tt.wantMax {
				t.Errorf("computeUrgencyScore() = %d, want between %d and %d", got, tt.wantMin, tt.wantMax)
			}
		})
	}
}

func TestCategorizeByDueDate(t *testing.T) {
	project := newTestPriorityProject(t)
	m := NewPriorityViewModel(project, nil, nil)

	tests := []struct {
		name       string
		daysOffset *int
		want       PriorityGroup
	}{
		{"overdue yesterday", intPtr(-1), PriorityGroupOverdue},
		{"overdue 10 days ago", intPtr(-10), PriorityGroupOverdue},
		{"due today", intPtr(0), PriorityGroupToday},
		{"due tomorrow", intPtr(1), PriorityGroupThisWeek},
		{"due in 3 days", intPtr(3), PriorityGroupThisWeek},
		{"due in 6 days", intPtr(6), PriorityGroupThisWeek},
		{"due in 7 days", intPtr(7), PriorityGroupLater},
		{"due in 14 days", intPtr(14), PriorityGroupLater},
		{"no due date", nil, PriorityGroupBacklog},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			task, err := models.NewTask("1", 1, models.TaskTypeTask, "Test")
			if err != nil {
				t.Fatal(err)
			}
			if tt.daysOffset != nil {
				due := time.Now().AddDate(0, 0, *tt.daysOffset)
				task.DueDate = &due
			}
			got := m.categorizeByDueDate(task)
			if got != tt.want {
				t.Errorf("categorizeByDueDate() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFormatRelativeDue(t *testing.T) {
	tests := []struct {
		name       string
		daysOffset *int
		wantMatch  string
	}{
		{"nil due date", nil, ""},
		{"yesterday", intPtr(-1), "yesterday"},
		{"2 days ago", intPtr(-2), "days ago"},
		{"10 days ago", intPtr(-10), "days ago"},
		{"today", intPtr(0), "today"},
		{"tomorrow", intPtr(1), "tomorrow"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			task, err := models.NewTask("1", 1, models.TaskTypeTask, "Test")
			if err != nil {
				t.Fatal(err)
			}
			if tt.daysOffset != nil {
				due := time.Now().AddDate(0, 0, *tt.daysOffset)
				task.DueDate = &due
			}

			got := FormatRelativeDue(task.DueDate, false)
			if tt.wantMatch == "" {
				if got != "" {
					t.Errorf("FormatRelativeDue() = %q, want empty", got)
				}
				return
			}
			if got != tt.wantMatch && !strings.Contains(got, tt.wantMatch) {
				t.Errorf("FormatRelativeDue() = %q, want to contain %q", got, tt.wantMatch)
			}
		})
	}
}
