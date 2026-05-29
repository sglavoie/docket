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

// TestRoundTrip_AttachmentSemicolonPreserved guards against the data-loss bug
// where a literal `;` in a filename, URL query string, or code-ref path
// previously split a single attachment into multiple corrupted entries because
// the writer joined attachments with `;` without escaping. Each subcase round-
// trips a task with a semicolon-bearing attachment and verifies the value
// survives unchanged.
func TestRoundTrip_AttachmentSemicolonPreserved(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)

	cases := []struct {
		name string
		att  models.Attachment
	}{
		{
			name: "plain_filename_with_semicolon",
			att:  models.Attachment{Filename: "a;b.pdf", AddedAt: now},
		},
		{
			name: "url_with_semicolon_in_query",
			att: models.Attachment{
				Filename:     "https://example.com/x?a=1;b=2",
				IsURL:        true,
				OriginalPath: "https://example.com/x?a=1;b=2",
				AddedAt:      now,
			},
		},
		{
			name: "code_ref_with_semicolon_and_line_number",
			att: models.Attachment{
				Filename:     "weird;file.go",
				IsCodeRef:    true,
				LineNumber:   42,
				OriginalPath: "/path/weird;file.go",
				AddedAt:      now,
			},
		},
		{
			name: "plain_filename_with_backslash_and_semicolon",
			att: models.Attachment{
				Filename: `mix\and;match.txt`,
				AddedAt:  now,
			},
		},
		{
			name: "plain_filename_with_metadata_key_after_comma",
			att: models.Attachment{
				Filename: "report, stage: review.pdf",
				AddedAt:  now,
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			task := &models.Task{
				ID:          "att-" + tc.name,
				Type:        models.TaskTypeTask,
				Status:      models.StatusTodo,
				Priority:    3,
				Attachments: []models.Attachment{tc.att},
			}

			parsed := &models.Task{}
			parseWrittenMetadata(task, parsed)

			if len(parsed.Attachments) != 1 {
				t.Fatalf("got %d attachments, want 1 (entry split on literal ';')", len(parsed.Attachments))
			}
			got := parsed.Attachments[0]
			if got.Filename != tc.att.Filename {
				t.Errorf("Filename = %q, want %q", got.Filename, tc.att.Filename)
			}
			if got.OriginalPath != tc.att.OriginalPath && tc.att.OriginalPath != "" {
				t.Errorf("OriginalPath = %q, want %q", got.OriginalPath, tc.att.OriginalPath)
			}
			if got.LineNumber != tc.att.LineNumber {
				t.Errorf("LineNumber = %d, want %d", got.LineNumber, tc.att.LineNumber)
			}
			if got.IsURL != tc.att.IsURL {
				t.Errorf("IsURL = %v, want %v", got.IsURL, tc.att.IsURL)
			}
			if got.IsCodeRef != tc.att.IsCodeRef {
				t.Errorf("IsCodeRef = %v, want %v", got.IsCodeRef, tc.att.IsCodeRef)
			}
		})
	}
}

// TestParseAttachments_BackwardCompatUnescapedFile confirms that pre-existing
// tasks.md files written before the `;` escape was added still parse correctly
// — i.e. an attachment value with no `\` characters keeps its raw form.
func TestParseAttachments_BackwardCompatUnescapedFile(t *testing.T) {
	var warnings []ParseWarning
	got := parseAttachments("file1.pdf@1700000000;file2.pdf@1700000001", &warnings)

	if len(got) != 2 {
		t.Fatalf("got %d attachments, want 2", len(got))
	}
	if got[0].Filename != "file1.pdf" || got[1].Filename != "file2.pdf" {
		t.Errorf("filenames: %q %q", got[0].Filename, got[1].Filename)
	}
	if got[0].AddedAt.Unix() != 1_700_000_000 || got[1].AddedAt.Unix() != 1_700_000_001 {
		t.Errorf("timestamps: %d %d", got[0].AddedAt.Unix(), got[1].AddedAt.Unix())
	}
}
