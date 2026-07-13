package format

import (
	"strings"
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/models"
)

func TestParseMetadata_PinnedAt(t *testing.T) {
	task := &models.Task{}
	ParseMetadata(task, "id: p1, status: todo, priority: 2, pinned: true, pinned_at: 2025-03-10 14:30:45")

	if !task.Pinned {
		t.Error("Pinned should be true")
	}
	if task.PinnedAt == nil {
		t.Fatal("PinnedAt should not be nil after parsing")
	}
	expected := "2025-03-10 14:30:45"
	got := task.PinnedAt.Format(pinnedAtFormat)
	if got != expected {
		t.Errorf("PinnedAt = %s, want %s", got, expected)
	}
}

func TestParseMetadata_PinnedAtMinutePrecisionFallback(t *testing.T) {
	task := &models.Task{}
	ParseMetadata(task, "id: p1, status: todo, priority: 2, pinned: true, pinned_at: 2025-03-10 14:30")

	if task.PinnedAt == nil {
		t.Fatal("PinnedAt should not be nil when parsing minute-precision format")
	}
	expected := "2025-03-10 14:30:00"
	got := task.PinnedAt.Format(pinnedAtFormat)
	if got != expected {
		t.Errorf("PinnedAt = %s, want %s", got, expected)
	}
}

func TestWriteMetadata_PinnedAt(t *testing.T) {
	pinnedTime := time.Date(2025, 3, 10, 14, 30, 45, 0, time.Local)
	task := &models.Task{
		ID:            "p1",
		WorkflowState: models.StateReady,
		Priority:      2,
		Pinned:        true,
		PinnedAt:      &pinnedTime,
		CreatedAt:     time.Date(2025, 1, 1, 0, 0, 0, 0, time.Local),
	}

	result := WriteMetadataV2(task)
	if !strings.Contains(result, "pinned: true") {
		t.Errorf("WriteMetadata should contain pinned field, got: %s", result)
	}
	if !strings.Contains(result, "pinned_at: 2025-03-10 14:30:45") {
		t.Errorf("WriteMetadata should contain pinned_at with seconds, got: %s", result)
	}
}

func TestWriteMetadata_NoPinnedAt(t *testing.T) {
	task := &models.Task{
		ID:            "p2",
		WorkflowState: models.StateReady,
		Priority:      2,
		CreatedAt:     time.Date(2025, 1, 1, 0, 0, 0, 0, time.Local),
	}

	result := WriteMetadataV2(task)
	if strings.Contains(result, "pinned_at") {
		t.Errorf("WriteMetadata should not contain pinned_at when nil, got: %s", result)
	}
}

func TestWriteMetadata_PinnedAtOmittedWhenNotPinned(t *testing.T) {
	// If a task has PinnedAt set but Pinned is false (e.g., from a manual file edit),
	// the writer should NOT emit pinned_at since pinned is not true.
	pinnedTime := time.Date(2025, 3, 10, 14, 30, 45, 0, time.Local)
	task := &models.Task{
		ID:            "p3",
		WorkflowState: models.StateReady,
		Priority:      2,
		Pinned:        false,
		PinnedAt:      &pinnedTime,
		CreatedAt:     time.Date(2025, 1, 1, 0, 0, 0, 0, time.Local),
	}

	result := WriteMetadataV2(task)
	if strings.Contains(result, "pinned") {
		t.Errorf("WriteMetadata should not contain pinned or pinned_at when Pinned is false, got: %s", result)
	}
}

func TestMetadata_PinnedAtRoundtrip(t *testing.T) {
	pinnedTime := time.Date(2025, 3, 10, 14, 30, 45, 0, time.Local)
	original := &models.Task{
		ID:            "p1",
		WorkflowState: models.StateReady,
		Priority:      2,
		Pinned:        true,
		PinnedAt:      &pinnedTime,
		CreatedAt:     time.Date(2025, 1, 1, 0, 0, 0, 0, time.Local),
	}

	meta := WriteMetadataV2(original)

	metaStr := strings.TrimPrefix(meta, "<!-- ")
	metaStr = strings.TrimSuffix(metaStr, " -->")

	parsed := &models.Task{}
	ParseMetadata(parsed, metaStr)

	if !parsed.Pinned {
		t.Error("Roundtrip: Pinned should be true")
	}
	if parsed.PinnedAt == nil {
		t.Fatal("Roundtrip: PinnedAt should not be nil")
	}
	if !parsed.PinnedAt.Equal(*original.PinnedAt) {
		t.Errorf("Roundtrip: PinnedAt = %s, want %s",
			parsed.PinnedAt.Format(pinnedAtFormat),
			original.PinnedAt.Format(pinnedAtFormat))
	}
}
