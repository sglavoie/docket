package format

import (
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/models"
)

func TestParseWriteMetadata_RoundTrip(t *testing.T) {
	tests := []struct {
		name string
		task *models.Task
	}{
		{
			name: "complete task",
			task: &models.Task{
				ID:            "task-round-1",
				Type:          models.TaskTypeTask,
				WorkflowState: models.StateInProgress,
				Priority:      2,
				Tags:          []string{"work", "urgent"},
				Pinned:        true,
				DependsOn:     []string{"dep1", "dep2"},
				CreatedAt:     time.Date(2025, 1, 15, 10, 30, 0, 0, time.Local),
				UpdatedAt:     time.Date(2025, 1, 16, 14, 20, 0, 0, time.Local),
				DueDate:       timePtr(time.Date(2025, 2, 1, 0, 0, 0, 0, time.Local)),
				Recurrence: &models.RecurrencePattern{
					Interval: 1,
					Unit:     models.RecurrenceWeekly,
					Weekdays: []int{1, 3, 5},
				},
			},
		},
		{
			name: "minimal task",
			task: &models.Task{
				ID:            "task-round-2",
				Type:          models.TaskTypeTask,
				WorkflowState: models.StateReady,
				Priority:      3,
			},
		},
		{
			name: "task with in-progress state",
			task: &models.Task{
				ID:            "task-round-3",
				Type:          models.TaskTypeTask,
				WorkflowState: models.StateInProgress,
				Priority:      2,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			metaStr := WriteMetadataV2(tt.task)

			if len(metaStr) < 8 || metaStr[:4] != "<!--" || metaStr[len(metaStr)-3:] != "-->" {
				t.Fatalf("Invalid metadata format: %q", metaStr)
			}
			content := metaStr[4 : len(metaStr)-3]
			content = content[1 : len(content)-1]

			parsed := &models.Task{}
			ParseMetadata(parsed, content)

			assertTaskFieldsAfterV2RoundTrip(t, parsed, tt.task)
		})
	}
}
