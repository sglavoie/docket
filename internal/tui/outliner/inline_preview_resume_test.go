package outliner

import (
	"strings"
	"testing"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/chrome"
)

// resumeNotePeekTask is the shape every case below varies: one task whose note
// and description are both long enough to want more rows than a page will give
// them, so the order between them is observable rather than incidental.
func resumeNotePeekTask(note string) *models.Task {
	return &models.Task{
		ID:          "task",
		Title:       "Selected",
		ResumeNote:  note,
		Description: strings.Repeat("Wrapped description content ", 12),
	}
}

// TestInlinePeekLeadsTheBodyWithTheResumeNote is the ordering claim. Peek is
// opened to ask where the work was left, so the note that answers it precedes
// the description that answers an older question.
func TestInlinePeekLeadsTheBodyWithTheResumeNote(t *testing.T) {
	task := resumeNotePeekTask("stopped at the CSV column mapping")

	plain := ansi.Strip(strings.Join(inlinePeekDocumentFor(inlinePeekOptions{
		Task: task, Indent: "  ", Width: 80,
	}).Body, "\n"))

	resume := strings.Index(plain, "Resume: stopped at the CSV column mapping")
	description := strings.Index(plain, "Description:")
	if resume < 0 {
		t.Fatalf("Peek rendered no resume line:\n%s", plain)
	}
	if description < 0 {
		t.Fatalf("Peek dropped the description entirely:\n%s", plain)
	}
	if resume > description {
		t.Fatalf("the resume line follows the description:\n%s", plain)
	}
	// A task with no note has no line at all: absence is the only spelling of
	// "no resume note", so an empty label would be a row spent on nothing.
	task.ResumeNote = ""
	if got := ansi.Strip(strings.Join(inlinePeekDocumentFor(inlinePeekOptions{
		Task: task, Indent: "  ", Width: 80,
	}).Body, "\n")); strings.Contains(got, "Resume:") {
		t.Fatalf("an absent note still drew a line:\n%s", got)
	}
}

// TestInlinePeekResumeNoteIsReadableWhole replaced the old two-row bound. The
// longest storable note is used at every supported width, so the claim is proved
// against the worst case: the note wraps over as many document rows as it needs,
// nothing is elided, and the page that shows it is still five rows tall.
func TestInlinePeekResumeNoteIsReadableWhole(t *testing.T) {
	note := strings.TrimSpace(strings.Repeat("resume ", models.MaxResumeNoteLength/7))
	if len(note) > models.MaxResumeNoteLength {
		note = strings.TrimSpace(note[:models.MaxResumeNoteLength])
	}
	if err := models.ValidateResumeNote(note); err != nil {
		t.Fatalf("the fixture note is not storable: %v", err)
	}

	for _, width := range []int{40, 60, 80, 120} {
		task := &models.Task{ID: "task", Title: "Selected", ResumeNote: note}
		doc := inlinePeekDocumentFor(inlinePeekOptions{Task: task, Indent: "  ", Width: width})
		plain := ansi.Strip(strings.Join(doc.Body, "\n"))
		if strings.Contains(plain, "…") {
			t.Fatalf("width %d elided the note instead of wrapping it:\n%s", width, plain)
		}
		if got := strings.Join(strings.Fields(plain), " "); got != "Resume: "+note {
			t.Fatalf("width %d did not carry the whole note:\n%s", width, got)
		}
		if page := doc.Lines(0); len(page) > inlinePeekBodyRows {
			t.Fatalf("width %d drew %d rows, want at most the %d-row budget:\n%s",
				width, len(page), inlinePeekBodyRows, ansi.Strip(strings.Join(page, "\n")))
		}
		for number, line := range doc.Body {
			if got := lipgloss.Width(line); got > width {
				t.Fatalf("width %d line %d overflowed to %d: %q", width, number, got, ansi.Strip(line))
			}
		}
	}
}

// TestInlinePeekResumeNoteSpendsThePageRatherThanRaisingIt is the budgeting
// claim, restated for a paged Peek. The note is content the row never showed,
// not content the row had to clip, so it goes in the body: it lengthens the
// document a page can move through and never the block the viewport reserves.
func TestInlinePeekResumeNoteSpendsThePageRatherThanRaisingIt(t *testing.T) {
	task := resumeNotePeekTask("stopped at the CSV column mapping")
	task.CreatedAt = time.Now().Add(-5 * 24 * time.Hour)
	task.UpdatedAt = time.Now().Add(-time.Hour)
	task.Tags = []string{"backend"}

	const width = 80
	without := inlinePeekDocumentFor(inlinePeekOptions{Task: &models.Task{
		ID: task.ID, Title: task.Title, Description: task.Description,
		CreatedAt: task.CreatedAt, UpdatedAt: task.UpdatedAt, Tags: task.Tags,
	}, Width: width})
	with := inlinePeekDocumentFor(inlinePeekOptions{Task: task, Width: width})

	if len(without.Lines(0)) != inlinePeekBodyRows || len(with.Lines(0)) != inlinePeekBodyRows {
		t.Fatalf("Peek height = %d without a note and %d with one, want %d both ways",
			len(without.Lines(0)), len(with.Lines(0)), inlinePeekBodyRows)
	}
	if len(with.Body) <= len(without.Body) {
		t.Fatalf("the note added no document rows: %d with, %d without", len(with.Body), len(without.Body))
	}
	plain := ansi.Strip(strings.Join(with.Body, "\n"))
	if !strings.HasPrefix(plain, "Resume: stopped at the CSV column mapping") {
		t.Fatalf("the note is not the first Peek row:\n%s", plain)
	}
	// Nothing comes off the tail to pay for it any more: the detail below the
	// note is a page turn away rather than gone.
	for _, want := range []string{"Description:", "Tags: #backend"} {
		if !strings.Contains(plain, want) {
			t.Fatalf("the note displaced %q:\n%s", want, plain)
		}
	}
}

// TestInlinePeekResumeNotePreservesTheHeaderExceptions holds the two lines that
// sit outside the body against the note that sits inside it: a clipped title and
// a promoted root's ancestor chain still head the pane with the note beneath
// them, and neither pages away.
func TestInlinePeekResumeNotePreservesTheHeaderExceptions(t *testing.T) {
	const overflow = "the remainder the row had no width for"
	task := resumeNotePeekTask("stopped at the CSV column mapping")
	task.CreatedAt = time.Now().Add(-5 * 24 * time.Hour)
	task.UpdatedAt = time.Now().Add(-time.Hour)

	continuation := inlinePeekDocumentFor(inlinePeekOptions{
		Task: task, TitleOverflow: overflow, Width: 80,
	})
	if got := len(continuation.Lines(0)); got != len(continuation.Header)+inlinePeekBodyRows {
		t.Fatalf("continuation Peek height = %d, want %d", got, len(continuation.Header)+inlinePeekBodyRows)
	}
	plain := ansi.Strip(strings.Join(continuation.Lines(0), "\n"))
	if !strings.HasPrefix(plain, overflow) {
		t.Fatalf("the note displaced the title continuation:\n%s", plain)
	}
	if !strings.Contains(plain, "Resume: stopped at the CSV column mapping") {
		t.Fatalf("the continuation displaced the note:\n%s", plain)
	}

	ancestor := inlinePeekDocumentFor(inlinePeekOptions{
		Task: task, Breadcrumb: "Launch platform › User authentication ›", Width: 80,
	})
	if got := len(ancestor.Lines(0)); got != len(ancestor.Header)+inlinePeekBodyRows {
		t.Fatalf("ancestor Peek height = %d, want %d", got, len(ancestor.Header)+inlinePeekBodyRows)
	}
	plain = ansi.Strip(strings.Join(ancestor.Lines(0), "\n"))
	if !strings.HasPrefix(plain, "Parent: Launch platform › User authentication") {
		t.Fatalf("the note displaced the ancestor chain:\n%s", plain)
	}
	if !strings.Contains(plain, "Resume: stopped at the CSV column mapping") {
		t.Fatalf("the ancestor chain displaced the note:\n%s", plain)
	}
}

// TestInlinePeekResumeNoteIsMeasuredByTheSameAuthorityThatDrawsIt keeps the one
// rendering-and-measurement authority the viewport depends on: the note may not
// add a row the height reservation never counted.
func TestInlinePeekResumeNoteIsMeasuredByTheSameAuthorityThatDrawsIt(t *testing.T) {
	note := strings.TrimSpace(strings.Repeat("resume ", 30))
	task := resumeNotePeekTask(note)
	project := &models.Project{ID: "project", Slug: "project", Name: "Project", Tasks: []*models.Task{task}}
	project.BuildTaskTree()
	model := NewOutlinerModel(project, nil, newTestConfig())
	row := OutlinerRow{Task: task, ProjectID: project.ID, Prefix: "│   ├── "}

	for _, width := range []int{40, 60, 80, 120} {
		indent := inlinePeekIndentForRow(row)
		drawn := model.InlinePeekLinesForRow(row, indent, width, chrome.WorstCaseColumnLayout(width))
		if got := model.inlinePeekLineCount(row, width); got != len(drawn) {
			t.Fatalf("width %d measured %d Peek lines and drew %d", width, got, len(drawn))
		}
		if !strings.Contains(ansi.Strip(strings.Join(drawn, "\n")), "Resume: ") {
			t.Fatalf("width %d drew no resume line:\n%s", width, ansi.Strip(strings.Join(drawn, "\n")))
		}
	}
}
