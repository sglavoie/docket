package format

import (
	"strings"
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

func TestRoundTrip_RelatedTo(t *testing.T) {
	task := &models.Task{
		ID:            "rel-1",
		Type:          models.TaskTypeTask,
		WorkflowState: models.StateReady,
		Priority:      2,
		RelatedTo:     []string{"id1", "id2"},
	}

	parsed := &models.Task{}
	warnings := parseWrittenMetadata(task, parsed)

	if len(warnings) != 0 {
		t.Errorf("got %d warnings, want 0: %v", len(warnings), warnings)
	}
	if !stringSlicesEqual(parsed.RelatedTo, []string{"id1", "id2"}) {
		t.Errorf("RelatedTo = %v, want %v", parsed.RelatedTo, []string{"id1", "id2"})
	}
}

func TestRoundTrip_ParentID(t *testing.T) {
	parentID := "parent-abc-123"
	task := &models.Task{
		ID:            "child-xyz-456",
		Type:          models.TaskTypeTask,
		WorkflowState: models.StateReady,
		Priority:      2,
		ParentID:      &parentID,
	}

	metaStr := WriteMetadataV2(task)
	if !strings.Contains(metaStr, "parent: parent-abc-123") {
		t.Errorf("WriteMetadata should include parent field, got: %s", metaStr)
	}

	parsed := &models.Task{}
	warnings := parseWrittenMetadata(task, parsed)

	if len(warnings) != 0 {
		t.Errorf("got %d warnings, want 0: %v", len(warnings), warnings)
	}
	if parsed.ParentID == nil {
		t.Fatal("ParentID is nil after round-trip")
	}
	if *parsed.ParentID != parentID {
		t.Errorf("ParentID = %q, want %q", *parsed.ParentID, parentID)
	}
}

func TestRoundTrip_NilParentID(t *testing.T) {
	task := &models.Task{
		ID:            "root-task-1",
		Type:          models.TaskTypeTask,
		WorkflowState: models.StateReady,
		Priority:      1,
	}

	metaStr := WriteMetadataV2(task)
	if strings.Contains(metaStr, "parent:") {
		t.Errorf("WriteMetadata should not include parent field for nil ParentID, got: %s", metaStr)
	}

	parsed := &models.Task{}
	warnings := parseWrittenMetadata(task, parsed)

	if len(warnings) != 0 {
		t.Errorf("got %d warnings, want 0: %v", len(warnings), warnings)
	}
	if parsed.ParentID != nil {
		t.Errorf("ParentID should be nil, got %q", *parsed.ParentID)
	}
}

func TestRoundTrip_DependsOnCrossProject(t *testing.T) {
	task := &models.Task{
		ID:            "local-task-1",
		Type:          models.TaskTypeTask,
		WorkflowState: models.StateReady,
		Priority:      2,
		DependsOn: []string{
			"local-dep-abc",
			"projID:task-xyz",
			"other-proj:another-123",
		},
	}

	parsed := &models.Task{}
	warnings := parseWrittenMetadata(task, parsed)

	if len(warnings) != 0 {
		t.Errorf("got %d warnings, want 0: %v", len(warnings), warnings)
	}

	want := []string{"local-dep-abc", "projID:task-xyz", "other-proj:another-123"}
	if !stringSlicesEqual(parsed.DependsOn, want) {
		t.Errorf("DependsOn = %v, want %v", parsed.DependsOn, want)
	}
}

func TestRoundTrip_KitchenSinkWithParentID(t *testing.T) {
	parentID := "parent-epic-001"
	task := &models.Task{
		ID:            "full-task-1",
		Type:          models.TaskTypeTask,
		WorkflowState: models.StateInProgress,
		Priority:      1,
		ParentID:      &parentID,
		Tags:          []string{"alpha", "beta"},
		DependsOn:     []string{"dep-local", "proj2:dep-cross"},
		RelatedTo:     []string{"rel-1"},
		Recurrence: &models.RecurrencePattern{
			Interval: 3,
			Unit:     models.RecurrenceMonthly,
		},
		Source: "jira",
		URL:    "https://jira.example.com/browse/PROJ-1",
		ScoreDimensions: map[string]int{
			"impact": 5,
		},
	}

	parsed := &models.Task{}
	warnings := parseWrittenMetadata(task, parsed)

	if len(warnings) != 0 {
		t.Errorf("got %d warnings, want 0: %v", len(warnings), warnings)
	}
	if parsed.ParentID == nil {
		t.Fatal("ParentID is nil after round-trip")
	}
	if *parsed.ParentID != parentID {
		t.Errorf("ParentID = %q, want %q", *parsed.ParentID, parentID)
	}
	if !stringSlicesEqual(parsed.DependsOn, task.DependsOn) {
		t.Errorf("DependsOn = %v, want %v", parsed.DependsOn, task.DependsOn)
	}
	if !stringSlicesEqual(parsed.RelatedTo, task.RelatedTo) {
		t.Errorf("RelatedTo = %v, want %v", parsed.RelatedTo, task.RelatedTo)
	}
	if parsed.Recurrence == nil {
		t.Fatal("Recurrence is nil after round-trip")
	}
	if parsed.Recurrence.Unit != models.RecurrenceMonthly {
		t.Errorf("Recurrence.Unit = %q, want %q", parsed.Recurrence.Unit, models.RecurrenceMonthly)
	}
	if parsed.Recurrence.Interval != 3 {
		t.Errorf("Recurrence.Interval = %d, want 3", parsed.Recurrence.Interval)
	}
}
