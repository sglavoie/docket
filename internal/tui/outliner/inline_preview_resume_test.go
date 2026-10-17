package outliner

import (
	"strings"
	"testing"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/models"
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

func TestCompactPeekKeepsAResumePassageAheadOfMetadata(t *testing.T) {
	for _, title := range []string{
		"Restore the overdue billing reconciliation export before the audit closes",
		strings.Repeat("A long task title ", 20),
	} {
		task := resumeNotePeekTask("Stopped after the ledger export; the reconciliation diff is still open.")
		task.Title = title
		opts := inlinePeekOptions{Task: task, Indent: "  ", Width: 60, Window: peekDockGeometry{bodyRows: 4}, Compact: true}
		doc := inlinePeekDocumentFor(opts)
		page := strings.Join(strings.Fields(ansi.Strip(strings.Join(doc.Lines(0), " "))), " ")
		if !strings.Contains(page, task.ResumeNote) {
			t.Fatalf("compact Peek lost the restart instruction: %s", page)
		}
		whole := strings.Join(strings.Fields(ansi.Strip(strings.Join(doc.Body, " "))), " ")
		if !strings.Contains(whole, strings.TrimSpace(task.Title)) || !strings.Contains(whole, "Status: Backlog") {
			t.Fatalf("compact Peek dropped detail: %s", whole)
		}
		opts.Compact = false
		expanded := inlinePeekDocumentFor(opts)
		if !strings.HasPrefix(expanded.sections[0].key, "title:") {
			t.Fatal("expanded document no longer leads with its full title")
		}
		// A reader on the second resume line stays on that passage when the
		// supporting sections move around it, and when returning to compact.
		for offset, line := range doc.Body {
			if !strings.Contains(ansi.Strip(line), "reconciliation diff") {
				continue
			}
			anchor := doc.readingAnchor(offset)
			expandedOffset := expanded.offsetForAnchor(anchor)
			if !strings.Contains(ansi.Strip(expanded.Body[expandedOffset]), "reconciliation diff") {
				t.Fatal("expansion lost the resume passage")
			}
			if got := doc.offsetForAnchor(expanded.readingAnchor(expandedOffset)); got != offset {
				t.Fatalf("compacting moved the passage to %d from %d", got, offset)
			}
		}
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
		doc := inlinePeekDocumentFor(inlinePeekOptions{Task: task, Indent: "  ", Width: width, Window: testPeekWindow()})
		plain := ansi.Strip(strings.Join(doc.Body, "\n"))
		if strings.Contains(plain, "…") {
			t.Fatalf("width %d elided the note instead of wrapping it:\n%s", width, plain)
		}
		if got := strings.Join(strings.Fields(plain), " "); got != "Selected Status: Backlog Resume: "+note {
			t.Fatalf("width %d did not carry the whole note:\n%s", width, got)
		}
		if page := doc.Lines(0); len(page) != testPeekWindowRows+inlinePeekPositionRows {
			t.Fatalf("width %d drew %d rows, want the %d-row window:\n%s",
				width, len(page), testPeekWindowRows+inlinePeekPositionRows, ansi.Strip(strings.Join(page, "\n")))
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
	}, Width: width, Window: testPeekWindow()})
	with := inlinePeekDocumentFor(inlinePeekOptions{Task: task, Width: width, Window: testPeekWindow()})

	if len(without.Lines(0)) != testPeekWindowRows+inlinePeekPositionRows || len(with.Lines(0)) != testPeekWindowRows+inlinePeekPositionRows {
		t.Fatalf("Peek height = %d without a note and %d with one, want %d both ways",
			len(without.Lines(0)), len(with.Lines(0)), testPeekWindowRows+inlinePeekPositionRows)
	}
	if len(with.Body) <= len(without.Body) {
		t.Fatalf("the note added no document rows: %d with, %d without", len(with.Body), len(without.Body))
	}
	plain := ansi.Strip(strings.Join(with.Body, "\n"))
	resume := strings.Index(plain, "Resume: stopped at the CSV column mapping")
	if resume < 0 {
		t.Fatalf("the note is missing from the document:\n%s", plain)
	}
	if description := strings.Index(plain, "Description:"); resume > description {
		t.Fatalf("the note follows the description:\n%s", plain)
	}
	// Nothing comes off the tail to pay for it any more: the detail below the
	// note is a page turn away rather than gone.
	for _, want := range []string{"Description:", "Tags: #backend"} {
		if !strings.Contains(plain, want) {
			t.Fatalf("the note displaced %q:\n%s", want, plain)
		}
	}
}

// Status and resume lead supporting metadata so restarting work does not require
// paging past a long ancestor chain.
func TestInlinePeekResumeNotePrecedesTheAncestorChain(t *testing.T) {
	task := resumeNotePeekTask("stopped at the CSV column mapping")
	task.Title = "Selected task whose title is long enough to want more than one row of a narrow pane"
	task.CreatedAt = time.Now().Add(-5 * 24 * time.Hour)
	task.UpdatedAt = time.Now().Add(-time.Hour)

	doc := inlinePeekDocumentFor(inlinePeekOptions{
		Task: task, Breadcrumb: "Launch platform › User authentication ›", Width: 80,
		Window: testPeekWindow(),
	})
	if got := len(doc.Lines(0)); got != testPeekWindowRows+inlinePeekPositionRows {
		t.Fatalf("Peek height = %d, want %d", got, testPeekWindowRows+inlinePeekPositionRows)
	}
	plain := ansi.Strip(strings.Join(doc.Body, "\n"))
	if !strings.HasPrefix(plain, "Selected task whose title") {
		t.Fatalf("the title does not lead the document:\n%s", plain)
	}
	parent := strings.Index(plain, "Parent: Launch platform › User authentication")
	resume := strings.Index(plain, "Resume: stopped at the CSV column mapping")
	if parent < 0 || resume < 0 {
		t.Fatalf("the document lost the ancestor chain or the note:\n%s", plain)
	}
	if parent < resume {
		t.Fatalf("the note follows the ancestor chain:\n%s", plain)
	}
}

// TestInlinePeekResumeNoteIsInTheRowsDocumentAtEveryWidth reaches the note the
// way the dock does, through the row rather than through a document the test
// built itself, so the indent and the width the dock supplies are the ones under
// test. The note is in the document and fits the terminal at every width; how
// much of that document is on screen is the dock's fixed window, measured where
// the dock is measured.
func TestInlinePeekResumeNoteIsInTheRowsDocumentAtEveryWidth(t *testing.T) {
	note := strings.TrimSpace(strings.Repeat("resume ", 30))
	task := resumeNotePeekTask(note)
	project := &models.Project{ID: "project", Slug: "project", Name: "Project", Tasks: []*models.Task{task}}
	project.BuildTaskTree()
	model := NewOutlinerModel(project, nil, newTestConfig())
	row := OutlinerRow{Task: task, ProjectID: project.ID, Prefix: "│   ├── "}

	for _, width := range []int{40, 60, 80, 120} {
		model.SetSize(width, 24)
		body := model.PeekDocumentBodyForRow(row)
		plain := ansi.Strip(strings.Join(body, "\n"))
		if !strings.Contains(strings.Join(strings.Fields(plain), " "), "Resume: "+note) {
			t.Fatalf("width %d lost the note from the row's document:\n%s", width, plain)
		}
		for number, line := range body {
			if got := lipgloss.Width(line); got > width {
				t.Fatalf("width %d line %d overflowed to %d: %q", width, number, got, ansi.Strip(line))
			}
		}
	}
}
