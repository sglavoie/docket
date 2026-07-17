package format

import (
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/models"
)

// TestWriteMetadataV2 pins the canonical v2 serialization: the legacy lifecycle
// keys (status/section/stage/active) collapse into a single `state` field
// derived from the task, and the retired keys are no longer emitted. v2 is now
// the sole writer, so these cases replace the old v1 `status:`-based baseline.
func TestWriteMetadataV2(t *testing.T) {
	tests := []struct {
		name string
		task *models.Task
		want string
	}{
		{
			name: "basic task",
			task: &models.Task{
				ID:            "task-123",
				Type:          models.TaskTypeTask,
				WorkflowState: models.StateReady,
				Priority:      3,
			},
			want: "<!-- id: task-123, state: ready, priority: 3 -->",
		},
		{
			name: "task with tags",
			task: &models.Task{
				ID:            "task-456",
				Type:          models.TaskTypeTask,
				WorkflowState: models.StateDone,
				Priority:      1,
				Tags:          []string{"work", "urgent"},
			},
			want: "<!-- id: task-456, state: done, priority: 1, tags: work;urgent -->",
		},
		{
			// The GTD section collapses into the derived state; `section:` is
			// no longer written.
			name: "task with GTD section",
			task: &models.Task{
				ID:            "task-789",
				Type:          models.TaskTypeTask,
				WorkflowState: models.StateNext,
				Priority:      2,
			},
			want: "<!-- id: task-789, state: next, priority: 2 -->",
		},
		{
			name: "task with dates",
			task: &models.Task{
				ID:            "task-1",
				Type:          models.TaskTypeTask,
				WorkflowState: models.StateReady,
				Priority:      2,
				CreatedAt:     time.Date(2025, 1, 15, 10, 30, 0, 0, time.Local),
				UpdatedAt:     time.Date(2025, 1, 16, 14, 20, 0, 0, time.Local),
				DueDate:       timePtr(time.Date(2025, 2, 1, 0, 0, 0, 0, time.Local)),
			},
			want: "<!-- id: task-1, state: ready, priority: 2, created: 2025-01-15 10:30, updated: 2025-01-16 14:20, due: 2025-02-01 -->",
		},
		{
			name: "task with completed timestamp",
			task: &models.Task{
				ID:            "task-2",
				Type:          models.TaskTypeTask,
				WorkflowState: models.StateDone,
				Priority:      1,
				CreatedAt:     time.Date(2025, 1, 15, 10, 30, 0, 0, time.Local),
				CompletedAt:   timePtr(time.Date(2025, 1, 20, 9, 15, 0, 0, time.Local)),
			},
			want: "<!-- id: task-2, state: done, priority: 1, created: 2025-01-15 10:30, completed: 2025-01-20 09:15 -->",
		},
		{
			// Blocked status derives the `waiting` state.
			name: "task with dependencies",
			task: &models.Task{
				ID:            "task-3",
				Type:          models.TaskTypeTask,
				WorkflowState: models.StateWaiting,
				Priority:      2,
				DependsOn:     []string{"task-1", "task-2"},
			},
			want: "<!-- id: task-3, state: waiting, priority: 2, depends: task-1;task-2 -->",
		},
		{
			// `active` is retired and discarded, so only `pinned` survives among
			// the boolean flags.
			name: "task with boolean flags",
			task: &models.Task{
				ID:            "task-4",
				Type:          models.TaskTypeTask,
				WorkflowState: models.StateReady,
				Priority:      2,
				Pinned:        true,
			},
			want: "<!-- id: task-4, state: ready, priority: 2, pinned: true -->",
		},
		{
			name: "task with daily recurrence",
			task: &models.Task{
				ID:            "task-5",
				Type:          models.TaskTypeTask,
				WorkflowState: models.StateReady,
				Priority:      1,
				Recurrence: &models.RecurrencePattern{
					Interval: 1,
					Unit:     models.RecurrenceDaily,
				},
			},
			want: "<!-- id: task-5, state: ready, priority: 1, recur: 1d -->",
		},
		{
			name: "task with weekly recurrence and weekdays",
			task: &models.Task{
				ID:            "task-6",
				Type:          models.TaskTypeTask,
				WorkflowState: models.StateReady,
				Priority:      2,
				Recurrence: &models.RecurrencePattern{
					Interval: 2,
					Unit:     models.RecurrenceWeekly,
					Weekdays: []int{1, 3, 5},
				},
			},
			want: "<!-- id: task-6, state: ready, priority: 2, recur: 2w:mon,wed,fri -->",
		},
		{
			// Custom stages are retired: `stage:` is dropped and the state
			// derives as `ready`.
			name: "task with stage",
			task: &models.Task{
				ID:            "task-stage",
				Type:          models.TaskTypeTask,
				WorkflowState: models.StateReady,
				Priority:      2,
			},
			want: "<!-- id: task-stage, state: ready, priority: 2 -->",
		},
		{
			name: "task with updated same as created",
			task: &models.Task{
				ID:            "task-7",
				Type:          models.TaskTypeTask,
				WorkflowState: models.StateReady,
				Priority:      1,
				CreatedAt:     time.Date(2025, 1, 15, 10, 30, 0, 0, time.Local),
				UpdatedAt:     time.Date(2025, 1, 15, 10, 30, 0, 0, time.Local),
			},
			want: "<!-- id: task-7, state: ready, priority: 1, created: 2025-01-15 10:30 -->",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := WriteMetadataV2(tt.task)
			if got != tt.want {
				t.Errorf("WriteMetadataV2() = %q, want %q", got, tt.want)
			}
		})
	}
}
