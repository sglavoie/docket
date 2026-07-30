package format

import (
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/models"
)

func TestRoundTrip_SnoozedUntil(t *testing.T) {
	snoozed := time.Date(2026, 6, 15, 0, 0, 0, 0, time.Local)
	task := &models.Task{
		ID:            "snooze-1",
		WorkflowState: models.StateReady,
		Priority:      2,
		SnoozedUntil:  &snoozed,
	}

	parsed := &models.Task{}
	warnings := parseWrittenMetadata(task, parsed)

	if len(warnings) != 0 {
		t.Errorf("got %d warnings, want 0: %v", len(warnings), warnings)
	}
	if parsed.SnoozedUntil == nil {
		t.Fatal("SnoozedUntil is nil after round-trip")
	}

	wantDate := snoozed.Format("2006-01-02")
	gotDate := parsed.SnoozedUntil.Format("2006-01-02")
	if gotDate != wantDate {
		t.Errorf("SnoozedUntil date = %q, want %q", gotDate, wantDate)
	}
}

func TestRoundTrip_PinnedAt(t *testing.T) {
	pinnedAt := time.Date(2026, 3, 10, 14, 30, 45, 0, time.Local)
	task := &models.Task{
		ID:            "pin-1",
		WorkflowState: models.StateReady,
		Priority:      1,
		Pinned:        true,
		PinnedAt:      &pinnedAt,
	}

	parsed := &models.Task{}
	warnings := parseWrittenMetadata(task, parsed)

	if len(warnings) != 0 {
		t.Errorf("got %d warnings, want 0: %v", len(warnings), warnings)
	}
	if parsed.PinnedAt == nil {
		t.Fatal("PinnedAt is nil after round-trip")
	}

	wantStr := pinnedAt.Format("2006-01-02 15:04:05")
	gotStr := parsed.PinnedAt.Format("2006-01-02 15:04:05")
	if gotStr != wantStr {
		t.Errorf("PinnedAt = %q, want %q", gotStr, wantStr)
	}
}
