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
// and description are both long enough to want more rows than the cap will give
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

	plain := ansi.Strip(strings.Join(inlinePeekLines(inlinePeekOptions{
		Task: task, Indent: "  ", Width: 80,
	}), "\n"))

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
	if got := ansi.Strip(strings.Join(inlinePeekLines(inlinePeekOptions{
		Task: task, Indent: "  ", Width: 80,
	}), "\n")); strings.Contains(got, "Resume:") {
		t.Fatalf("an absent note still drew a line:\n%s", got)
	}
}

// TestInlinePeekResumeNoteWrapsWithinTwoLines holds the note's own share of the
// budget at the widths the product supports. The longest storable note is used
// at every width, so the bound is proved against the worst case rather than a
// convenient one, and the cut announces itself.
func TestInlinePeekResumeNoteWrapsWithinTwoLines(t *testing.T) {
	note := strings.TrimSpace(strings.Repeat("resume ", models.MaxResumeNoteLength/7))
	if len(note) > models.MaxResumeNoteLength {
		note = strings.TrimSpace(note[:models.MaxResumeNoteLength])
	}
	if err := models.ValidateResumeNote(note); err != nil {
		t.Fatalf("the fixture note is not storable: %v", err)
	}

	for _, width := range []int{40, 60, 80, 120} {
		task := &models.Task{ID: "task", Title: "Selected", ResumeNote: note}
		lines := inlinePeekLines(inlinePeekOptions{Task: task, Indent: "  ", Width: width})
		if len(lines) > inlinePeekResumeRows {
			t.Fatalf("width %d spent %d rows on the note, want at most %d:\n%s",
				width, len(lines), inlinePeekResumeRows, ansi.Strip(strings.Join(lines, "\n")))
		}
		plain := ansi.Strip(strings.Join(lines, "\n"))
		if !strings.HasPrefix(plain, "  Resume: ") {
			t.Fatalf("width %d resume line = %q", width, plain)
		}
		if len(lines) == inlinePeekResumeRows && !strings.HasSuffix(plain, "…") {
			t.Fatalf("width %d truncated the note without saying so:\n%s", width, plain)
		}
		for number, line := range lines {
			if got := lipgloss.Width(line); got > width {
				t.Fatalf("width %d line %d overflowed to %d: %q", width, number, got, ansi.Strip(line))
			}
		}
	}
}

// TestInlinePeekResumeNoteSpendsTheCapRatherThanRaisingIt is the budgeting
// claim. The note is content the row never showed, not content the row had to
// clip, so it competes with the description below it instead of growing the
// block the viewport already reserved.
func TestInlinePeekResumeNoteSpendsTheCapRatherThanRaisingIt(t *testing.T) {
	task := resumeNotePeekTask("stopped at the CSV column mapping")
	task.CreatedAt = time.Now().Add(-5 * 24 * time.Hour)
	task.UpdatedAt = time.Now().Add(-time.Hour)
	task.Tags = []string{"backend"}

	const width = 80
	without := inlinePeekLines(inlinePeekOptions{Task: &models.Task{
		ID: task.ID, Title: task.Title, Description: task.Description,
		CreatedAt: task.CreatedAt, UpdatedAt: task.UpdatedAt, Tags: task.Tags,
	}, Width: width})
	with := inlinePeekLines(inlinePeekOptions{Task: task, Width: width})

	if len(without) != inlinePeekMaxLines || len(with) != inlinePeekMaxLines {
		t.Fatalf("Peek height = %d without a note and %d with one, want %d both ways",
			len(without), len(with), inlinePeekMaxLines)
	}
	plain := ansi.Strip(strings.Join(with, "\n"))
	if !strings.HasPrefix(plain, "Resume: stopped at the CSV column mapping") {
		t.Fatalf("the note is not the first Peek row:\n%s", plain)
	}
	// The row it costs comes off the tail, which is the same drop order every
	// other detail already obeys.
	if !strings.Contains(plain, "Description:") {
		t.Fatalf("the note displaced the description entirely:\n%s", plain)
	}
	if strings.Contains(plain, "Tags: #backend") {
		t.Fatalf("the note was added without spending the cap:\n%s", plain)
	}
}

// TestInlinePeekResumeNotePreservesTheRaisedCapExceptions holds the two lines
// that do raise the cap against the one that does not: a clipped title and a
// promoted root's ancestor chain still buy their own rows with the note beside
// them.
func TestInlinePeekResumeNotePreservesTheRaisedCapExceptions(t *testing.T) {
	const overflow = "the remainder the row had no width for"
	task := resumeNotePeekTask("stopped at the CSV column mapping")
	task.CreatedAt = time.Now().Add(-5 * 24 * time.Hour)
	task.UpdatedAt = time.Now().Add(-time.Hour)

	continuation := inlinePeekLines(inlinePeekOptions{
		Task: task, TitleOverflow: overflow, Width: 80,
	})
	if len(continuation) != inlinePeekMaxLines+1 {
		t.Fatalf("continuation Peek height = %d, want %d", len(continuation), inlinePeekMaxLines+1)
	}
	plain := ansi.Strip(strings.Join(continuation, "\n"))
	if !strings.HasPrefix(plain, overflow) {
		t.Fatalf("the note displaced the title continuation:\n%s", plain)
	}
	if !strings.Contains(plain, "Resume: stopped at the CSV column mapping") {
		t.Fatalf("the continuation displaced the note:\n%s", plain)
	}

	ancestor := inlinePeekLines(inlinePeekOptions{
		Task: task, Breadcrumb: "Launch platform › User authentication ›", Width: 80,
	})
	if len(ancestor) != inlinePeekMaxLines+1 {
		t.Fatalf("ancestor Peek height = %d, want %d", len(ancestor), inlinePeekMaxLines+1)
	}
	plain = ansi.Strip(strings.Join(ancestor, "\n"))
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
		indent := strings.Repeat(" ", lipgloss.Width(row.Prefix)+4)
		drawn := model.InlinePeekLinesForRow(row, indent, width, chrome.WorstCaseColumnLayout(width))
		if got := model.inlinePeekLineCount(row, width); got != len(drawn) {
			t.Fatalf("width %d measured %d Peek lines and drew %d", width, got, len(drawn))
		}
		if !strings.Contains(ansi.Strip(strings.Join(drawn, "\n")), "Resume: ") {
			t.Fatalf("width %d drew no resume line:\n%s", width, ansi.Strip(strings.Join(drawn, "\n")))
		}
	}
}
