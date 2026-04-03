package format

import (
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/models"
)

func TestWriteMetadata(t *testing.T) {
	tests := []struct {
		name string
		task *models.Task
		want string
	}{
		{
			name: "basic task",
			task: &models.Task{
				ID:       "task-123",
				Type:     models.TaskTypeTask,
				Status:   models.StatusTodo,
				Priority: 3,
			},
			want: "<!-- id: task-123, status: todo, priority: 3 -->",
		},
		{
			name: "task with tags",
			task: &models.Task{
				ID:       "task-456",
				Type:     models.TaskTypeTask,
				Status:   models.StatusDone,
				Priority: 1,
				Tags:     []string{"work", "urgent"},
			},
			want: "<!-- id: task-456, status: done, priority: 1, tags: work;urgent -->",
		},
		{
			name: "task with GTD section",
			task: &models.Task{
				ID:       "task-789",
				Type:     models.TaskTypeTask,
				Status:   models.StatusTodo,
				Priority: 2,
				Section:  "Next",
			},
			want: "<!-- id: task-789, status: todo, priority: 2, section: Next -->",
		},
		{
			name: "task with custom section",
			task: &models.Task{
				ID:            "task-790",
				Type:          models.TaskTypeTask,
				Status:        models.StatusTodo,
				Priority:      2,
				CustomSection: "Backend",
			},
			want: "<!-- id: task-790, status: todo, priority: 2, custom_section: Backend -->",
		},
		{
			name: "task with dates",
			task: &models.Task{
				ID:        "task-1",
				Type:      models.TaskTypeTask,
				Status:    models.StatusTodo,
				Priority:  2,
				CreatedAt: time.Date(2025, 1, 15, 10, 30, 0, 0, time.Local),
				UpdatedAt: time.Date(2025, 1, 16, 14, 20, 0, 0, time.Local),
				DueDate:   timePtr(time.Date(2025, 2, 1, 0, 0, 0, 0, time.Local)),
			},
			want: "<!-- id: task-1, status: todo, priority: 2, created: 2025-01-15 10:30, updated: 2025-01-16 14:20, due: 2025-02-01 -->",
		},
		{
			name: "task with completed timestamp",
			task: &models.Task{
				ID:          "task-2",
				Type:        models.TaskTypeTask,
				Status:      models.StatusDone,
				Priority:    1,
				CreatedAt:   time.Date(2025, 1, 15, 10, 30, 0, 0, time.Local),
				CompletedAt: timePtr(time.Date(2025, 1, 20, 9, 15, 0, 0, time.Local)),
			},
			want: "<!-- id: task-2, status: done, priority: 1, created: 2025-01-15 10:30, completed: 2025-01-20 09:15 -->",
		},
		{
			name: "task with dependencies",
			task: &models.Task{
				ID:        "task-3",
				Type:      models.TaskTypeTask,
				Status:    models.StatusBlocked,
				Priority:  2,
				DependsOn: []string{"task-1", "task-2"},
			},
			want: "<!-- id: task-3, status: blocked, priority: 2, depends: task-1;task-2 -->",
		},
		{
			name: "task with boolean flags",
			task: &models.Task{
				ID:       "task-4",
				Type:     models.TaskTypeTask,
				Status:   models.StatusTodo,
				Priority: 2,
				Pinned:   true,
				Active:   true,
			},
			want: "<!-- id: task-4, status: todo, priority: 2, pinned: true, active: true -->",
		},
		{
			name: "task with daily recurrence",
			task: &models.Task{
				ID:       "task-5",
				Type:     models.TaskTypeTask,
				Status:   models.StatusTodo,
				Priority: 1,
				Recurrence: &models.RecurrencePattern{
					Interval: 1,
					Unit:     models.RecurrenceDaily,
				},
			},
			want: "<!-- id: task-5, status: todo, priority: 1, recur: 1d -->",
		},
		{
			name: "task with weekly recurrence and weekdays",
			task: &models.Task{
				ID:       "task-6",
				Type:     models.TaskTypeTask,
				Status:   models.StatusTodo,
				Priority: 2,
				Recurrence: &models.RecurrencePattern{
					Interval: 2,
					Unit:     models.RecurrenceWeekly,
					Weekdays: []int{1, 3, 5},
				},
			},
			want: "<!-- id: task-6, status: todo, priority: 2, recur: 2w:mon,wed,fri -->",
		},
		{
			name: "task with stage",
			task: &models.Task{
				ID:       "task-stage",
				Type:     models.TaskTypeTask,
				Status:   models.StatusTodo,
				Priority: 2,
				Stage:    "Implement",
			},
			want: "<!-- id: task-stage, status: todo, priority: 2, stage: Implement -->",
		},
		{
			name: "task with updated same as created",
			task: &models.Task{
				ID:        "task-7",
				Type:      models.TaskTypeTask,
				Status:    models.StatusTodo,
				Priority:  1,
				CreatedAt: time.Date(2025, 1, 15, 10, 30, 0, 0, time.Local),
				UpdatedAt: time.Date(2025, 1, 15, 10, 30, 0, 0, time.Local),
			},
			want: "<!-- id: task-7, status: todo, priority: 1, created: 2025-01-15 10:30 -->",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := WriteMetadata(tt.task)
			if got != tt.want {
				t.Errorf("WriteMetadata() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCustomSectionRoundTrip(t *testing.T) {
	task := &models.Task{
		ID:            "cs-1",
		Type:          models.TaskTypeTask,
		Status:        models.StatusTodo,
		Priority:      2,
		Section:       "Next",
		CustomSection: "Backend",
	}

	metaStr := WriteMetadata(task)
	parsed := &models.Task{}
	content := metaStr[5 : len(metaStr)-4]
	ParseMetadata(parsed, content)

	if parsed.Section != "Next" {
		t.Errorf("Section = %q, want %q", parsed.Section, "Next")
	}
	if parsed.CustomSection != "Backend" {
		t.Errorf("CustomSection = %q, want %q", parsed.CustomSection, "Backend")
	}
}