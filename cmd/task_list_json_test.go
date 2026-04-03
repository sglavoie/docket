package cmd

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/testutil"
)

func TestTaskJSONEntry_IncludesFlagFields(t *testing.T) {
	task := testutil.NewTestTask("Flagged task", models.TaskTypeTask)
	task.Pinned = true
	task.Active = true
	task.Dimmed = true
	task.CustomSection = "sprint-1"
	parentID := "parent-uuid"
	task.ParentID = &parentID

	tags := task.Tags
	if tags == nil {
		tags = []string{}
	}
	entry := taskJSONEntry{
		ID:            task.ID,
		ShortID:       task.ShortID,
		Type:          task.Type.String(),
		Title:         task.Title,
		Status:        task.Status.String(),
		Priority:      task.Priority,
		ParentID:      task.ParentID,
		Tags:          tags,
		Section:       task.Section,
		CustomSection: task.CustomSection,
		Pinned:        task.Pinned,
		Active:        task.Active,
		Dimmed:        task.Dimmed,
		CreatedAt:     task.CreatedAt,
		UpdatedAt:     task.UpdatedAt,
		DueDate:       task.DueDate,
	}

	data, err := json.Marshal(entry)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if v, ok := parsed["pinned"]; !ok || v != true {
		t.Errorf("expected pinned=true in JSON, got %v", v)
	}
	if v, ok := parsed["active"]; !ok || v != true {
		t.Errorf("expected active=true in JSON, got %v", v)
	}
	if v, ok := parsed["dimmed"]; !ok || v != true {
		t.Errorf("expected dimmed=true in JSON, got %v", v)
	}
	if v, ok := parsed["custom_section"]; !ok || v != "sprint-1" {
		t.Errorf("expected custom_section=sprint-1 in JSON, got %v", v)
	}
	if v, ok := parsed["parent_id"]; !ok || v != "parent-uuid" {
		t.Errorf("expected parent_id=parent-uuid in JSON, got %v", v)
	}
}

func TestTaskJSONEntry_NilParentIDSerializesAsNull(t *testing.T) {
	task := testutil.NewTestTask("Root task", models.TaskTypeTask)

	entry := taskJSONEntry{
		ID:       task.ID,
		ShortID:  task.ShortID,
		Type:     task.Type.String(),
		Title:    task.Title,
		Status:   task.Status.String(),
		Priority: task.Priority,
		ParentID: nil,
		Tags:     []string{},
	}

	data, err := json.Marshal(entry)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if _, ok := parsed["parent_id"]; !ok {
		t.Error("expected parent_id key to be present even when nil")
	}
	if parsed["parent_id"] != nil {
		t.Errorf("expected parent_id=null for root task, got %v", parsed["parent_id"])
	}
}

func TestTaskJSONEntry_IncludesStageAndSnoozedUntil(t *testing.T) {
	snoozeTime := time.Now().Add(24 * time.Hour)
	task := testutil.NewTestTask("Staged task", models.TaskTypeTask)
	task.Stage = "Design"
	task.SnoozedUntil = &snoozeTime

	tags := task.Tags
	if tags == nil {
		tags = []string{}
	}
	entry := taskJSONEntry{
		ID:           task.ID,
		ShortID:      task.ShortID,
		Type:         task.Type.String(),
		Title:        task.Title,
		Status:       task.Status.String(),
		Priority:     task.Priority,
		Tags:         tags,
		Stage:        task.Stage,
		SnoozedUntil: task.SnoozedUntil,
		CreatedAt:    task.CreatedAt,
		UpdatedAt:    task.UpdatedAt,
	}

	data, err := json.Marshal(entry)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if v, ok := parsed["stage"]; !ok || v != "Design" {
		t.Errorf("expected stage=Design in JSON, got %v", v)
	}
	if _, ok := parsed["snoozed_until"]; !ok {
		t.Error("expected snoozed_until in JSON output")
	}
}

func TestTaskJSONEntry_OmitsEmptyStageAndSnoozedUntil(t *testing.T) {
	task := testutil.NewTestTask("Plain task", models.TaskTypeTask)

	entry := taskJSONEntry{
		ID:       task.ID,
		ShortID:  task.ShortID,
		Type:     task.Type.String(),
		Title:    task.Title,
		Status:   task.Status.String(),
		Priority: task.Priority,
		Tags:     []string{},
	}

	data, err := json.Marshal(entry)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if _, ok := parsed["stage"]; ok {
		t.Error("expected stage to be omitted when empty")
	}
	if _, ok := parsed["snoozed_until"]; ok {
		t.Error("expected snoozed_until to be omitted when nil")
	}
}
