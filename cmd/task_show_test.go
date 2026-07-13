package cmd

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/models"
)

func TestTaskShowJSONIncludesPinnedAt(t *testing.T) {
	pinnedAt := time.Date(2024, 6, 15, 10, 30, 0, 0, time.UTC)
	task := &models.Task{
		ID:            "test-id",
		ShortID:       1,
		Type:          models.TaskTypeTask,
		Title:         "Test Task",
		WorkflowState: models.StateReady,
		Priority:      3,
		Pinned:        true,
		PinnedAt:      &pinnedAt,
		Tags:          []string{},
		DependsOn:     []string{},
		RelatedTo:     []string{},
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	// Build the JSON entry the same way displayTaskShowJSON does
	entry := taskShowJSONEntry{
		ID:              task.ID,
		ShortID:         task.ShortID,
		Type:            task.Type.String(),
		Title:           task.Title,
		Status:          models.LegacyStatusForState(task.State()).String(),
		Priority:        task.Priority,
		Tags:            task.Tags,
		DependsOn:       task.DependsOn,
		RelatedTo:       task.RelatedTo,
		Checklist:       []models.ChecklistItem{},
		Attachments:     []models.Attachment{},
		Comments:        []*models.Comment{},
		Activity:        []*models.ActivityEntry{},
		ScoreDimensions: map[string]int{},
		ChildIDs:        []string{},
		Pinned:          task.Pinned,
		PinnedAt:        task.PinnedAt,
		CreatedAt:       task.CreatedAt,
		UpdatedAt:       task.UpdatedAt,
	}

	data, err := json.Marshal(entry)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	// Verify pinned_at is present in the JSON output
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if _, ok := raw["pinned_at"]; !ok {
		t.Error("pinned_at field missing from JSON output")
	}
}

func TestTaskShowJSONIncludesActivity(t *testing.T) {
	task := &models.Task{
		ID:            "test-id",
		ShortID:       1,
		Type:          models.TaskTypeTask,
		Title:         "Test Task",
		WorkflowState: models.StateInProgress,
		Priority:      3,
		Activity: []*models.ActivityEntry{
			{
				Timestamp: time.Date(2024, 6, 15, 10, 0, 0, 0, time.UTC),
				Field:     "status",
				OldValue:  "todo",
				NewValue:  "in_progress",
			},
		},
		Tags:      []string{},
		DependsOn: []string{},
		RelatedTo: []string{},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	entry := taskShowJSONEntry{
		ID:              task.ID,
		ShortID:         task.ShortID,
		Type:            task.Type.String(),
		Title:           task.Title,
		Status:          models.LegacyStatusForState(task.State()).String(),
		Priority:        task.Priority,
		Tags:            task.Tags,
		DependsOn:       task.DependsOn,
		RelatedTo:       task.RelatedTo,
		Checklist:       []models.ChecklistItem{},
		Attachments:     []models.Attachment{},
		Comments:        []*models.Comment{},
		Activity:        task.Activity,
		ScoreDimensions: map[string]int{},
		ChildIDs:        []string{},
		CreatedAt:       task.CreatedAt,
		UpdatedAt:       task.UpdatedAt,
	}

	data, err := json.Marshal(entry)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	activityRaw, ok := raw["activity"]
	if !ok {
		t.Fatal("activity field missing from JSON output")
	}

	activities, ok := activityRaw.([]any)
	if !ok {
		t.Fatalf("activity field is not an array: %T", activityRaw)
	}
	if len(activities) != 1 {
		t.Errorf("expected 1 activity entry, got %d", len(activities))
	}
}
