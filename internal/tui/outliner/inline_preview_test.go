package outliner

import (
	"strings"
	"testing"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/util"
)

// peekDocumentBody is the complete detail document for a row, independent of
// whichever page a render happens to be showing. A test asking whether Peek
// *holds* something reads this; a test asking what is on screen reads a page.
func peekDocumentBody(m *OutlinerModel, row OutlinerRow, width int) []string {
	return m.inlinePeekDocumentForRow(row, "  ", width, testPeekWindow()).Body
}

// peekFirstPage is the first window of a document built straight from options,
// which is what the dock draws and what the viewport reserves room for. Paging
// past it is inline_peek_paging_test.go's subject, not this file's. The window is
// the frame's answer, and these options carry no frame, so the shared test window
// is stamped on unless the caller named one of its own.
func peekFirstPage(opts inlinePeekOptions) []string {
	if !opts.Window.Displayed() {
		opts.Window = testPeekWindow()
	}
	return inlinePeekDocumentFor(opts).Lines(0)
}

// peekWindowText is the content a window carries, without the blank rows a short
// document is padded out to the window's height with and without the
// position row every window ends on. A test comparing Peek against exact text
// means this; a test about the dock's geometry means the row count instead.
func peekWindowText(lines []string) string {
	content := make([]string, 0, len(lines))
	for _, line := range lines[:min(len(lines), testPeekWindowRows)] {
		if strings.TrimSpace(ansi.Strip(line)) == "" {
			continue
		}
		content = append(content, ansi.Strip(line))
	}
	return strings.Join(content, "\n")
}

// TestInlinePeekNilAndEmptyTask pins the two degenerate inputs. A row that
// resolves no task draws nothing at all, which is what tells the scrolling keys
// there is no pane to consume them. A task carrying nothing still has a state —
// every task is in one — so the summary line is the floor of the document and
// there is no "nothing to say" sentence for a task to fall back to.
func TestInlinePeekNilAndEmptyTask(t *testing.T) {
	if lines := peekFirstPage(inlinePeekOptions{}); lines != nil {
		t.Fatalf("nil task lines = %v, want nil", lines)
	}

	lines := peekFirstPage(inlinePeekOptions{
		Task:   &models.Task{},
		Indent: "  ",
		Width:  40,
	})
	if got := peekWindowText(lines); got != "  Status: Backlog" {
		t.Fatalf("empty Peek = %q", got)
	}
	// The window is still the dock's fixed height: a task with one line to say
	// pads rather than shrinking the block the list gave up rows for.
	if len(lines) != testPeekWindowRows+inlinePeekPositionRows {
		t.Fatalf("empty Peek window = %d rows, want %d", len(lines), testPeekWindowRows+inlinePeekPositionRows)
	}
}

// TestInlinePeekPageIsBoundedWhileTheDocumentIsNot is the split the dock rests
// on: the document holds every detail row the task has, and the five-row budget
// decides only how many of them are on screen. The position row beneath them is
// always drawn, so the window is the same height whatever the document holds.
func TestInlinePeekPageIsBoundedWhileTheDocumentIsNot(t *testing.T) {
	task := &models.Task{
		ID:          "task",
		ShortID:     99,
		Title:       "Selected",
		Description: strings.Repeat("Unicode café 界 content ", 12),
		Comments: []*models.Comment{
			{Text: "Decision: retained as a comment"},
		},
		Checklist: []models.ChecklistItem{
			{Text: "First item", Done: true},
			{Text: "Second item"},
			{Text: "Third item"},
		},
	}

	const width = 64
	doc := inlinePeekDocumentFor(inlinePeekOptions{Task: task, Indent: "    ", Width: width, Window: testPeekWindow()})
	if !doc.Paged() {
		t.Fatalf("a description this long fit in one page:\n%s", ansi.Strip(strings.Join(doc.Body, "\n")))
	}
	if len(doc.Body) <= testPeekWindowRows {
		t.Fatalf("document body = %d rows, want more than the %d-row budget", len(doc.Body), testPeekWindowRows)
	}

	rendered := doc.Lines(0)
	plain := ansi.Strip(strings.Join(rendered, "\n"))
	if len(rendered) != testPeekWindowRows+inlinePeekPositionRows {
		t.Fatalf("first page = %d rows, want %d:\n%s", len(rendered), testPeekWindowRows+inlinePeekPositionRows, plain)
	}
	if got := ansi.Strip(rendered[0]); got != "    Selected" {
		t.Fatalf("page does not lead with the whole title: %q\n%s", got, plain)
	}
	if !strings.Contains(ansi.Strip(rendered[len(rendered)-1]), "0 above · ") {
		t.Fatalf("paged Peek did not reserve a counter row:\n%s", plain)
	}

	// The counts are in the document even though the description pushed them off
	// the first page. The checklist is a count and never its items: those are
	// outline rows of their own under the task.
	whole := ansi.Strip(strings.Join(doc.Body, "\n"))
	for _, want := range []string{"Comments: 1", "Checklist: 1/3 done"} {
		if !strings.Contains(whole, want) {
			t.Fatalf("document dropped %q:\n%s", want, whole)
		}
	}
	for _, banned := range []string{"#41", "#42", "#99", "First item", "Second item"} {
		if strings.Contains(whole, banned) {
			t.Fatalf("Peek repeated %q:\n%s", banned, whole)
		}
	}
	for lineNumber, line := range rendered {
		if got := lipgloss.Width(line); got > width {
			t.Fatalf("line %d width = %d, want <= %d: %q", lineNumber, got, width, ansi.Strip(line))
		}
	}
}

// TestInlinePeekBodyKeepsItsContentOrder locks what replaced the old drop
// ladder. Nothing is shed to make room any more, so the body's only decision is
// order: identity first (title, parent), then what the task is and what it still
// owes, then the task's own text, then the supporting counts and the references,
// then the narrowing axes, and the instants it has already spent last. The
// checklist is a count and never its items.
func TestInlinePeekBodyKeepsItsContentOrder(t *testing.T) {
	stateChanged := time.Now().Add(-4 * time.Hour)
	due := time.Now().AddDate(0, 0, 4)
	checkIn := util.StartOfDay(time.Now().AddDate(0, 0, 2))
	lastReview := time.Now().Add(-25 * time.Hour)
	task := &models.Task{
		ID:             "task",
		Title:          "Rework the sync path",
		WorkflowState:  models.StateDoing,
		StateChangedAt: stateChanged,
		Priority:       2,
		Pinned:         true,
		Review:         true,
		ResumeNote:     "Left mid-refactor",
		Description:    strings.Repeat("Wrapped description content ", 12),
		Context:        "platform-core",
		CreatedAt:      time.Now().Add(-5 * 24 * time.Hour),
		UpdatedAt:      time.Now().Add(-time.Hour),
		DueDate:        &due,
		NextCheckIn:    &checkIn,
		LastReviewedAt: &lastReview,
		Attachments:    []models.Attachment{{Filename: "spec.pdf"}},
		Comments:       []*models.Comment{{Text: "Decision"}},
		Tags:           []string{"backend"},
		Checklist:      []models.ChecklistItem{{Text: "First item"}, {Text: "Second item", Done: true}},
	}

	doc := inlinePeekDocumentFor(inlinePeekOptions{
		Task: task, Breadcrumb: "Launch platform ›", Planning: "Today", Width: 100,
		Window:           testPeekWindow(),
		Dependencies:     []DependencyDetail{{Label: "Platform #7 Blocking work", State: "doing"}},
		Dependents:       []DependencyDetail{{Label: "Platform #9 Waiting work", State: "waiting"}},
		DependencyCensus: dependencyCensus{total: 1, blocked: 1},
	})
	body := ansi.Strip(strings.Join(doc.Body, "\n"))
	order := []string{
		"Rework the sync path",
		"Plan: Today · Status: Doing 4h · Priority: 2 medium · Pinned · Review",
		"Resume: Left mid-refactor",
		"Parent: Launch platform",
		"Due: " + chrome.FormatWeekdayDate(due, time.Now(), task.DueHasTime()) + " (" + chrome.DayGloss(due, time.Now()) + ")" +
			" · Check-in: " + chrome.FormatWeekdayDate(checkIn, time.Now(), false) + " (" + chrome.DayGloss(checkIn, time.Now()) + ")",
		"Repeat: off",
		"Description: Wrapped description content",
		"Checklist: 1/2 done",
		"Attachments: 1 · spec.pdf",
		"Comments: 1",
		"Dependencies: ",
		"Dependents: 1 · ",
		"Tags: #backend",
		"Created: 5d ago · Updated: 1h ago · Last review: 1d ago",
	}
	cursor := 0
	for _, want := range order {
		found := strings.Index(body[cursor:], want)
		if found < 0 {
			t.Fatalf("body is missing %q or has it out of order at %d:\n%s", want, cursor, body)
		}
		cursor += found + len(want)
	}
	for _, banned := range []string{"First item", "Second item"} {
		if strings.Contains(body, banned) {
			t.Fatalf("Peek still renders %q:\n%s", banned, body)
		}
	}
	if len(doc.Lines(0)) != testPeekWindowRows+inlinePeekPositionRows {
		t.Fatalf("page = %d rows, want %d", len(doc.Lines(0)), testPeekWindowRows+inlinePeekPositionRows)
	}
	// Every section is reachable by paging, which is the claim order has to rest
	// on: history moved to the end of a document longer than the window, so it is
	// only still readable if the last page holds it.
	last := ansi.Strip(strings.Join(doc.Lines(doc.MaxOffset()), "\n"))
	if !strings.Contains(last, "Created: 5d ago · Updated: 1h ago · Last review: 1d ago") {
		t.Fatalf("history is not on the document's last page:\n%s", last)
	}
}
