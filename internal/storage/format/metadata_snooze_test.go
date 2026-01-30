package format

import (
	"strings"
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/util"
)

func TestParseMetadata_Snoozed(t *testing.T) {
	task := &models.Task{}
	ParseMetadata(task, "id: abc123, status: todo, priority: 3, snoozed: 2025-06-15")

	if task.SnoozedUntil == nil {
		t.Fatal("SnoozedUntil should not be nil after parsing")
	}
	expected := "2025-06-15"
	got := task.SnoozedUntil.Format(util.DateFormat)
	if got != expected {
		t.Errorf("SnoozedUntil = %s, want %s", got, expected)
	}
}

func TestWriteMetadata_Snoozed(t *testing.T) {
	snoozeDate := time.Date(2025, 6, 15, 0, 0, 0, 0, time.Local)
	task := &models.Task{
		ID:           "abc123",
		Status:       models.StatusTodo,
		Priority:     3,
		SnoozedUntil: &snoozeDate,
		CreatedAt:    time.Date(2025, 1, 1, 0, 0, 0, 0, time.Local),
	}

	result := WriteMetadata(task)
	if !strings.Contains(result, "snoozed: 2025-06-15") {
		t.Errorf("WriteMetadata should contain snoozed field, got: %s", result)
	}
}

func TestWriteMetadata_NoSnooze(t *testing.T) {
	task := &models.Task{
		ID:        "abc123",
		Status:    models.StatusTodo,
		Priority:  3,
		CreatedAt: time.Date(2025, 1, 1, 0, 0, 0, 0, time.Local),
	}

	result := WriteMetadata(task)
	if strings.Contains(result, "snoozed") {
		t.Errorf("WriteMetadata should not contain snoozed field when nil, got: %s", result)
	}
}

func TestMetadata_SnoozeRoundtrip(t *testing.T) {
	snoozeDate := time.Date(2025, 6, 15, 0, 0, 0, 0, time.Local)
	original := &models.Task{
		ID:           "abc123",
		Status:       models.StatusTodo,
		Priority:     3,
		SnoozedUntil: &snoozeDate,
		CreatedAt:    time.Date(2025, 1, 1, 0, 0, 0, 0, time.Local),
	}

	// Write metadata
	meta := WriteMetadata(original)

	// Strip <!-- and -->
	metaStr := strings.TrimPrefix(meta, "<!-- ")
	metaStr = strings.TrimSuffix(metaStr, " -->")

	// Parse back
	parsed := &models.Task{}
	ParseMetadata(parsed, metaStr)

	if parsed.SnoozedUntil == nil {
		t.Fatal("Roundtrip: SnoozedUntil should not be nil")
	}
	if parsed.SnoozedUntil.Format(util.DateFormat) != original.SnoozedUntil.Format(util.DateFormat) {
		t.Errorf("Roundtrip: SnoozedUntil = %s, want %s",
			parsed.SnoozedUntil.Format(util.DateFormat),
			original.SnoozedUntil.Format(util.DateFormat))
	}
}
