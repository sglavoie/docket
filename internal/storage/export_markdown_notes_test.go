package storage

import (
	"strings"
	"testing"
	"time"

	"github.com/sglavoie/docket/internal/models"
)

func TestWriteNotesSectionClean(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name        string
		notes       []*models.Note
		wantExact   string
		contains    string
		notContains string
	}{
		{
			name:      "empty slice returns empty string",
			notes:     []*models.Note{},
			wantExact: "",
		},
		{
			name: "single note retains body",
			notes: []*models.Note{
				{Title: "Project ideas", Body: "body text", ModifiedAt: now},
			},
			contains:    "\n---\n\n# Notes\n\n## Project ideas\n\nbody text\n",
			notContains: "tags:",
		},
		{
			name: "single note",
			notes: []*models.Note{
				{Title: "No Tags", Body: "content", ModifiedAt: now},
			},
			contains:    "## No Tags",
			notContains: "tags:",
		},
		{
			name: "single note empty body",
			notes: []*models.Note{
				{Title: "Empty Body", Body: "", ModifiedAt: now},
			},
			contains: "## Empty Body\n",
		},
		{
			name: "parse error note heading suffix",
			notes: []*models.Note{
				{Title: "Broken", ParseError: "invalid frontmatter: missing delimiter", Body: "recovered", ModifiedAt: now},
			},
			contains: "## Broken [parse error]\n",
		},
		{
			name: "multiple notes ordering preserved with blank line between",
			notes: []*models.Note{
				{Title: "First", Body: "body one", ModifiedAt: now},
				{Title: "Second", Body: "body two", ModifiedAt: now.Add(-time.Hour)},
			},
			contains: "## First",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := writeNotesSectionClean(tc.notes)

			if tc.wantExact != "" || len(tc.notes) == 0 {
				if got != tc.wantExact {
					t.Errorf("expected exact %q, got %q", tc.wantExact, got)
				}
				return
			}
			if tc.contains != "" && !strings.Contains(got, tc.contains) {
				t.Errorf("expected output to contain %q, got: %q", tc.contains, got)
			}
			if tc.notContains != "" && strings.Contains(got, tc.notContains) {
				t.Errorf("expected output NOT to contain %q, got: %q", tc.notContains, got)
			}
		})
	}
}

func TestWriteNotesSectionClean_MultipleNotesOrdering(t *testing.T) {
	now := time.Now()
	notes := []*models.Note{
		{Title: "Alpha", Body: "a", ModifiedAt: now},
		{Title: "Beta", Body: "b", ModifiedAt: now.Add(-time.Hour)},
	}
	got := writeNotesSectionClean(notes)

	alphaPos := strings.Index(got, "## Alpha")
	betaPos := strings.Index(got, "## Beta")
	if alphaPos < 0 || betaPos < 0 {
		t.Fatalf("expected both notes in output: %q", got)
	}
	if alphaPos > betaPos {
		t.Errorf("expected Alpha before Beta (caller ordering preserved): alpha=%d beta=%d", alphaPos, betaPos)
	}
	if !strings.Contains(got, "a\n\n## Beta") {
		t.Errorf("expected blank line between notes, got: %q", got)
	}
}
