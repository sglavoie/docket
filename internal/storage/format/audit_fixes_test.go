package format

import (
	"strings"
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/models"
)

// TestParse_PinnedAtMalformedEmitsWarning asserts that a malformed pinned_at
// value emits a validation warning instead of being silently dropped.
//
// Before the fix the parse closure swallowed the time-parse error with no
// warning and left PinnedAt = nil; the writer then skipped pinned_at on the
// next save, deleting the user's pin timestamp without trace.
func TestParse_PinnedAtMalformedEmitsWarning(t *testing.T) {
	metaStr := "id: pin-1, status: todo, priority: 3, pinned: true, pinned_at: not-a-date"

	task := &models.Task{}
	warnings := ParseMetadata(task, metaStr)

	if task.PinnedAt != nil {
		t.Errorf("PinnedAt = %v, want nil after malformed parse", task.PinnedAt)
	}
	if !hasWarningField(warnings, "pinned_at") {
		t.Errorf("expected a warning with Field=pinned_at, got: %v", warnings)
	}
}

// TestParse_PinnedAtValidStillRoundTrips guards the happy-path: adding a
// warning for malformed values must not regress the well-formed case.
func TestParse_PinnedAtValidStillRoundTrips(t *testing.T) {
	at := time.Date(2024, 6, 15, 12, 30, 45, 0, time.Local)
	task := &models.Task{
		ID:       "pin-ok",
		Type:     models.TaskTypeTask,
		Status:   models.StatusTodo,
		Priority: 3,
		Pinned:   true,
		PinnedAt: &at,
	}

	parsed := &models.Task{}
	warnings := parseWrittenMetadata(task, parsed)

	for _, w := range warnings {
		if w.Field == "pinned_at" {
			t.Errorf("unexpected pinned_at warning on well-formed value: %v", w)
		}
	}
	if parsed.PinnedAt == nil || !parsed.PinnedAt.Equal(at) {
		t.Errorf("PinnedAt = %v, want %v", parsed.PinnedAt, at)
	}
}

// TestRoundTrip_CodeRefLineNumberPreservedWithoutOriginalPath asserts that a
// code-ref attachment with a non-zero LineNumber survives a write→parse cycle
// even when OriginalPath is empty.
//
// Before the fix, writeAttachmentsMetadata only emitted the line number inside
// the OriginalPath segment ("||path:N"), so an empty OriginalPath caused the
// line number to be silently dropped on save.
func TestRoundTrip_CodeRefLineNumberPreservedWithoutOriginalPath(t *testing.T) {
	now := time.Now()
	task := &models.Task{
		ID:       "code-ref-no-path",
		Type:     models.TaskTypeTask,
		Status:   models.StatusTodo,
		Priority: 3,
		Attachments: []models.Attachment{
			{
				Filename:     "main.go",
				IsCodeRef:    true,
				LineNumber:   17,
				OriginalPath: "",
				AddedAt:      now,
			},
		},
	}

	// Sanity-check the written metadata: line number must be present.
	metaStr := WriteMetadata(task)
	if !strings.Contains(metaStr, ":17") {
		t.Errorf("written metadata %q does not preserve line number :17", metaStr)
	}

	parsed := &models.Task{}
	if _, err := parseWriteRoundTrip(task, parsed); err != nil {
		t.Fatalf("round-trip failed: %v", err)
	}

	if len(parsed.Attachments) != 1 {
		t.Fatalf("got %d attachments, want 1", len(parsed.Attachments))
	}
	got := parsed.Attachments[0]
	if !got.IsCodeRef {
		t.Errorf("IsCodeRef = false, want true")
	}
	if got.LineNumber != 17 {
		t.Errorf("LineNumber = %d, want 17 (silently dropped before fix)", got.LineNumber)
	}
	if got.Filename != "main.go" {
		t.Errorf("Filename = %q, want %q", got.Filename, "main.go")
	}
}

// parseWriteRoundTrip writes the task metadata, parses it back, and returns
// any warnings. Mirrors parseWrittenMetadata but exposes warnings for callers
// that care about the second leg.
func parseWriteRoundTrip(src, dst *models.Task) ([]ParseWarning, error) {
	warnings := parseWrittenMetadata(src, dst)
	return warnings, nil
}

func TestParse_NonGTDSectionWithLegacySectionFieldEmitsWarning(t *testing.T) {
	metaStr := "id: dup-section, status: todo, priority: 3, section: legacy_value, " + legacyTaskSectionField + ": kept"

	task := &models.Task{ShortID: 7}
	warnings := ParseMetadata(task, metaStr)

	if task.Section != "" {
		t.Errorf("Section = %q, want empty after correction", task.Section)
	}

	found := false
	for _, w := range warnings {
		if w.Field == "section" && w.Type == WarningDataCorrection {
			found = true
			if !strings.Contains(w.Message, "legacy_value") {
				t.Errorf("warning message %q does not mention discarded section value", w.Message)
			}
			break
		}
	}
	if !found {
		t.Errorf("expected a DataCorrection warning on Field=section, got: %v", warnings)
	}
}
