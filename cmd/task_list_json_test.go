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
	task.Dimmed = true
	task.ParkedNote = "resume after API review"
	parkedAt := time.Date(2026, 7, 9, 12, 30, 0, 0, time.UTC)
	task.ParkedAt = &parkedAt
	parentID := "parent-uuid"
	task.ParentID = &parentID

	tags := task.Tags
	if tags == nil {
		tags = []string{}
	}
	entry := taskJSONEntry{
		ID:         task.ID,
		ShortID:    task.ShortID,
		Type:       task.Type.String(),
		Title:      task.Title,
		Status:     models.LegacyStatusForState(task.State()).String(),
		Priority:   task.Priority,
		ParentID:   task.ParentID,
		Tags:       tags,
		Section:    models.SectionForState(task.State()),
		Pinned:     task.Pinned,
		Queued:     true,
		Dimmed:     task.Dimmed,
		Parked:     task.IsParked(),
		ParkedNote: task.ParkedNote,
		ParkedAt:   task.ParkedAt,
		CreatedAt:  task.CreatedAt,
		UpdatedAt:  task.UpdatedAt,
		DueDate:    task.DueDate,
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
	if v, ok := parsed["queued"]; !ok || v != true {
		t.Errorf("expected queued=true in JSON, got %v", v)
	}
	if _, ok := parsed["active"]; ok {
		t.Error("did not expect legacy active field in JSON")
	}
	if v, ok := parsed["dimmed"]; !ok || v != true {
		t.Errorf("expected dimmed=true in JSON, got %v", v)
	}
	if v, ok := parsed["parked"]; !ok || v != true {
		t.Errorf("expected parked=true in JSON, got %v", v)
	}
	if v, ok := parsed["parked_note"]; !ok || v != "resume after API review" {
		t.Errorf("expected parked_note in JSON, got %v", v)
	}
	if _, ok := parsed["parked_at"]; !ok {
		t.Error("expected parked_at in JSON output")
	}
	legacyTaskSectionField := "custom" + "_section"
	if _, ok := parsed[legacyTaskSectionField]; ok {
		t.Error("did not expect legacy section field in JSON")
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
		Status:   models.LegacyStatusForState(task.State()).String(),
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
		Status:       models.LegacyStatusForState(task.State()).String(),
		Priority:     task.Priority,
		Tags:         tags,
		Stage:        "Design",
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
		Status:   models.LegacyStatusForState(task.State()).String(),
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
	if v, ok := parsed["parked"]; !ok || v != false {
		t.Errorf("expected parked=false in JSON, got %v", v)
	}
	if _, ok := parsed["parked_note"]; ok {
		t.Error("expected parked_note to be omitted when empty")
	}
	if _, ok := parsed["parked_at"]; ok {
		t.Error("expected parked_at to be omitted when nil")
	}
}

func TestTaskListJSONEntries_IncludesParkedMetadata(t *testing.T) {
	parkedAt := time.Date(2026, 7, 9, 12, 30, 0, 0, time.UTC)
	parked := testutil.NewTestTask("Parked task", models.TaskTypeTask)
	parked.ParkedNote = "resume after API review"
	parked.ParkedAt = &parkedAt
	plain := testutil.NewTestTask("Plain task", models.TaskTypeTask)

	entries := taskListJSONEntries([]*models.Task{parked, plain})

	if len(entries) != 2 {
		t.Fatalf("got %d entries, want 2", len(entries))
	}
	if !entries[0].Parked {
		t.Fatal("parked task JSON entry should have parked=true")
	}
	if entries[0].ParkedNote != "resume after API review" {
		t.Errorf("ParkedNote = %q, want resume note", entries[0].ParkedNote)
	}
	if entries[0].ParkedAt == nil || !entries[0].ParkedAt.Equal(parkedAt) {
		t.Errorf("ParkedAt = %v, want %v", entries[0].ParkedAt, parkedAt)
	}
	if entries[1].Parked {
		t.Error("plain task JSON entry should have parked=false")
	}
	if entries[1].ParkedNote != "" {
		t.Errorf("plain task ParkedNote = %q, want empty", entries[1].ParkedNote)
	}
	if entries[1].ParkedAt != nil {
		t.Errorf("plain task ParkedAt = %v, want nil", entries[1].ParkedAt)
	}
}

// TestTaskJSONQueuedUsesWorkflowState locks FR-008-002 for the CLI task JSON:
// the queued flag is StateNext membership, not presence in the Focus Queue
// order index. An unindexed StateNext task reports queued=true; a stale indexed
// non-Next task reports queued=false.
func TestTaskJSONQueuedUsesWorkflowState(t *testing.T) {
	next := testutil.NewTestTask("On deck", models.TaskTypeTask)
	next.ApplyWorkflowState(models.StateNext)
	stale := testutil.NewTestTask("Left Next", models.TaskTypeTask)
	stale.ApplyWorkflowState(models.StateReady)

	entries := taskListJSONEntries([]*models.Task{next, stale})

	if len(entries) != 2 {
		t.Fatalf("got %d entries, want 2", len(entries))
	}
	if !entries[0].Queued {
		t.Error("unindexed StateNext task should report queued=true")
	}
	if entries[1].Queued {
		t.Error("stale non-Next task should report queued=false even if it lingers in the order index")
	}
}
