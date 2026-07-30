package format

import (
	"strings"
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/models"
)

// TestRoundTrip_PlainAttachmentNamedLikeCodeOrURL guards against a plain
// attachment whose filename literally starts with "code:" or "url:" being
// promoted to a code reference / URL on the next load. The writer must
// disambiguate it (e.g. with a "file:" prefix) so the parser preserves the
// original IsURL/IsCodeRef classification.
func TestRoundTrip_PlainAttachmentNamedLikeCodeOrURL(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	task := &models.Task{
		ID:            "t1",
		WorkflowState: models.StateReady,
		Priority:      2,
		Attachments: []models.Attachment{
			{Filename: "code:weird.txt", AddedAt: now},
			{Filename: "url:also-weird.txt", AddedAt: now},
			{Filename: "file:literal.txt", AddedAt: now},
		},
	}

	parsed := &models.Task{}
	warnings := parseWrittenMetadata(task, parsed)
	if len(warnings) != 0 {
		t.Errorf("unexpected warnings on round-trip: %v", warnings)
	}

	if len(parsed.Attachments) != 3 {
		t.Fatalf("got %d attachments, want 3", len(parsed.Attachments))
	}

	for i, want := range task.Attachments {
		got := parsed.Attachments[i]
		if got.Filename != want.Filename {
			t.Errorf("att[%d].Filename = %q, want %q", i, got.Filename, want.Filename)
		}
		if got.IsURL {
			t.Errorf("att[%d].IsURL = true, want false", i)
		}
		if got.IsCodeRef {
			t.Errorf("att[%d].IsCodeRef = true, want false", i)
		}
	}
}

// TestRoundTrip_EscapeNewlinesIncludesCR ensures that escapeNewlines /
// unescapeNewlines round-trip CRLF symmetrically. Before the fix, only "\n"
// and "\\" were escaped, so carriage returns in a bug.workaround / epic.metrics
// / story.persona value would corrupt the metadata line on save.
func TestRoundTrip_EscapeNewlinesIncludesCR(t *testing.T) {
	cases := []string{
		"line1\r\nline2",
		"only\rcarriage",
		"trailing\r",
		"mixed\r\n\\backslash\nand more",
	}
	for _, in := range cases {
		got := unescapeNewlines(escapeNewlines(in))
		if got != in {
			t.Errorf("round-trip mismatch:\n want: %q\n got:  %q", in, got)
		}
		if strings.ContainsAny(escapeNewlines(in), "\r\n") {
			t.Errorf("escaped form still contains raw CR/LF: %q", escapeNewlines(in))
		}
	}
}

// TestParseTimeField_AcceptsSeconds covers the case where a user (or an
// external tool) edits a created/updated/completed timestamp to include
// seconds. The minute-precision-only parser silently zeroed those values
// before this fix.
func TestParseTimeField_AcceptsSeconds(t *testing.T) {
	value := "2024-05-17 09:08:07"
	got, ok := ParseTimeField(value)
	if !ok {
		t.Fatalf("ParseTimeField(%q) returned ok=false", value)
	}
	if got.Year() != 2024 || got.Month() != 5 || got.Day() != 17 || got.Hour() != 9 || got.Minute() != 8 || got.Second() != 7 {
		t.Errorf("ParseTimeField(%q) = %v, want 2024-05-17 09:08:07", value, got)
	}
}

// TestParseMetadata_CreatedWithSeconds ensures the created/updated/completed
// codecs route through the seconds-tolerant parser end-to-end.
func TestParseMetadata_CreatedWithSeconds(t *testing.T) {
	metaStr := "id: t1, status: todo, priority: 2, created: 2024-05-17 09:08:07, updated: 2024-05-17 09:08:07"
	task := &models.Task{}
	warnings := ParseMetadata(task, metaStr)
	for _, w := range warnings {
		if w.Field == "created" || w.Field == "updated" {
			t.Errorf("unexpected warning for %s: %s", w.Field, w.Message)
		}
	}
	if task.CreatedAt.IsZero() {
		t.Errorf("CreatedAt is zero; seconds-form was rejected")
	}
	if task.CreatedAt.Second() != 7 {
		t.Errorf("CreatedAt.Second() = %d, want 7", task.CreatedAt.Second())
	}
}

// TestParseAttachments_NegativeEpochClampedToZero guards against a negative
// epoch silently producing an attachment dated before 1970. We clamp to 0 and
// emit a validation warning so the data anomaly is at least visible.
func TestParseAttachments_NegativeEpochClampedToZero(t *testing.T) {
	var warnings []ParseWarning
	got := parseAttachments("notes.txt@-123", &warnings)
	if len(got) != 1 {
		t.Fatalf("got %d attachments, want 1", len(got))
	}
	if got[0].AddedAt.Unix() != 0 {
		t.Errorf("AddedAt.Unix() = %d, want 0 (clamped)", got[0].AddedAt.Unix())
	}
	found := false
	for _, w := range warnings {
		if w.Field == "attachments" && strings.Contains(w.Message, "negative timestamp") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected a negative-timestamp warning, got: %v", warnings)
	}
}
