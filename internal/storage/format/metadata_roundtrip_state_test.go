package format

import (
	"strings"
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/util"
)

func TestRoundTrip_SnoozedUntil(t *testing.T) {
	snoozed := time.Date(2026, 6, 15, 0, 0, 0, 0, time.Local)
	task := &models.Task{
		ID:           "snooze-1",
		Type:         models.TaskTypeTask,
		Status:       models.StatusTodo,
		Priority:     2,
		SnoozedUntil: &snoozed,
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
		ID:       "pin-1",
		Type:     models.TaskTypeTask,
		Status:   models.StatusTodo,
		Priority: 1,
		Pinned:   true,
		PinnedAt: &pinnedAt,
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

func TestRoundTrip_ParkedNoteAndParkedAt(t *testing.T) {
	parkedAt := time.Date(2026, 7, 8, 10, 15, 0, 0, time.Local)
	note := `Blocked, waiting on café path C:\tmp`
	task := &models.Task{
		ID:         "park-1",
		Type:       models.TaskTypeTask,
		Status:     models.StatusTodo,
		Priority:   2,
		ParkedNote: note,
		ParkedAt:   &parkedAt,
	}

	metaStr := WriteMetadata(task)
	if !strings.Contains(metaStr, `parked: Blocked\, waiting on café path C:\\tmp`) {
		t.Errorf("written metadata did not escape parked note, got %q", metaStr)
	}

	parsed := &models.Task{}
	warnings := parseWrittenMetadata(task, parsed)

	if len(warnings) != 0 {
		t.Errorf("got %d warnings, want 0: %v", len(warnings), warnings)
	}
	if parsed.ParkedNote != note {
		t.Errorf("ParkedNote = %q, want %q", parsed.ParkedNote, note)
	}
	if parsed.ParkedAt == nil {
		t.Fatal("ParkedAt is nil after round-trip")
	}
	if parsed.ParkedAt.Format(util.DateTimeFormat) != parkedAt.Format(util.DateTimeFormat) {
		t.Errorf("ParkedAt = %q, want %q", parsed.ParkedAt.Format(util.DateTimeFormat), parkedAt.Format(util.DateTimeFormat))
	}
}

func TestParse_ParkedAtMalformedEmitsWarning(t *testing.T) {
	metaStr := "id: park-1, status: todo, priority: 3, parked: waiting, parked_at: not-a-date"

	task := &models.Task{}
	warnings := ParseMetadata(task, metaStr)

	if task.ParkedAt != nil {
		t.Errorf("ParkedAt = %v, want nil after malformed parse", task.ParkedAt)
	}
	if !hasWarningField(warnings, "parked_at") {
		t.Errorf("expected a warning with Field=parked_at, got: %v", warnings)
	}
}
