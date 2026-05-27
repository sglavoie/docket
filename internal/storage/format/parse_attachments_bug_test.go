package format

import (
	"strings"
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/models"
)

// TestRoundTrip_CodeRefPortLikePathNotMisparsed guards against a code-ref
// attachment whose OriginalPath contains a colon followed by digits (e.g.,
// "localhost:8080") being silently corrupted on parse.
//
// Before the fix, the parser unconditionally treated the trailing ":N" as a
// line number, dropping it from OriginalPath and stamping LineNumber=N — even
// when LineNumber was originally 0 and the colon was a legitimate part of the
// path (e.g., host:port reference, Windows drive letter "C:/foo/file:42").
//
// The fix requires the path before the colon to look like a file reference
// (contain a "/" or ".") before extracting a trailing ":N" as a line number.
func TestRoundTrip_CodeRefPortLikePathNotMisparsed(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	task := &models.Task{
		ID:       "code-port",
		Type:     models.TaskTypeTask,
		Status:   models.StatusTodo,
		Priority: 3,
		Attachments: []models.Attachment{
			{
				Filename:     "localhost:8080",
				IsCodeRef:    true,
				LineNumber:   0,
				OriginalPath: "localhost:8080",
				AddedAt:      now,
			},
		},
	}

	parsed := &models.Task{}
	warnings := parseWrittenMetadata(task, parsed)
	for _, w := range warnings {
		if w.Field == "attachments" {
			t.Errorf("unexpected attachments warning on round-trip: %v", w)
		}
	}

	if len(parsed.Attachments) != 1 {
		t.Fatalf("got %d attachments, want 1", len(parsed.Attachments))
	}
	got := parsed.Attachments[0]
	if got.OriginalPath != "localhost:8080" {
		t.Errorf("OriginalPath = %q, want %q (port-like suffix silently extracted as line number)", got.OriginalPath, "localhost:8080")
	}
	if got.LineNumber != 0 {
		t.Errorf("LineNumber = %d, want 0 (phantom line number extracted from port suffix)", got.LineNumber)
	}
	if got.Filename != "localhost:8080" {
		t.Errorf("Filename = %q, want %q", got.Filename, "localhost:8080")
	}
}

// TestParseAttachments_WindowsStyleCodeRefPathNoSpuriousWarning ensures a
// Windows-style code-ref path ("C:/Users/foo.go") does not emit a bogus
// "invalid line number" warning. The colon after the drive letter would
// otherwise be picked up as a malformed line-number suffix.
func TestParseAttachments_WindowsStyleCodeRefPathNoSpuriousWarning(t *testing.T) {
	var warnings []ParseWarning
	// Construct a code attachment serialization: code:<filename>||<originalPath>@<epoch>
	// The OriginalPath "C:/Users/foo.go" has a colon at position 1 (drive
	// letter) but the suffix after the LAST colon is "/Users/foo.go" which is
	// clearly not an integer. The parser must not emit a warning here.
	got := parseAttachments("code:foo.go||C:/Users/foo.go@1700000000", &warnings)

	if len(got) != 1 {
		t.Fatalf("got %d attachments, want 1", len(got))
	}
	if got[0].OriginalPath != "C:/Users/foo.go" {
		t.Errorf("OriginalPath = %q, want %q", got[0].OriginalPath, "C:/Users/foo.go")
	}
	if got[0].LineNumber != 0 {
		t.Errorf("LineNumber = %d, want 0", got[0].LineNumber)
	}
	for _, w := range warnings {
		if w.Field == "attachments" && strings.Contains(w.Message, "invalid line number") {
			t.Errorf("unexpected spurious 'invalid line number' warning for Windows-style path: %v", w)
		}
	}
}

// TestRoundTrip_CodeRefPathContainingDotAndLineNumber confirms the heuristic
// still correctly extracts line numbers for legitimate file paths (containing
// "/" or "."). This is the happy path that must keep working.
func TestRoundTrip_CodeRefPathContainingDotAndLineNumber(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	task := &models.Task{
		ID:       "code-ok",
		Type:     models.TaskTypeTask,
		Status:   models.StatusTodo,
		Priority: 3,
		Attachments: []models.Attachment{
			{
				Filename:     "main.go",
				IsCodeRef:    true,
				LineNumber:   42,
				OriginalPath: "/path/to/main.go",
				AddedAt:      now,
			},
		},
	}

	parsed := &models.Task{}
	parseWrittenMetadata(task, parsed)

	if len(parsed.Attachments) != 1 {
		t.Fatalf("got %d attachments, want 1", len(parsed.Attachments))
	}
	got := parsed.Attachments[0]
	if got.LineNumber != 42 {
		t.Errorf("LineNumber = %d, want 42", got.LineNumber)
	}
	if got.OriginalPath != "/path/to/main.go" {
		t.Errorf("OriginalPath = %q, want %q", got.OriginalPath, "/path/to/main.go")
	}
}
