package outliner

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/taskref"
)

// pagedPeekTask is a task whose detail cannot fit one page at any supported
// width: a long note, a long description, dates, attachments, comments, a
// context, and tags. Every paging claim below is made against it, so none of
// them is only true of a document that happens to be one row over.
func pagedPeekTask() *models.Task {
	task := &models.Task{
		ID:            "paged",
		ShortID:       1,
		Title:         "Selected",
		WorkflowState: models.StateBacklog,
		ResumeNote:    "stopped at the CSV column mapping and the retry budget",
		Description:   strings.Repeat("Wrapped description content ", 12),
		Context:       "platform-core",
		Attachments: []models.Attachment{
			{Filename: "design-spec.pdf"},
			{Filename: "migration-notes.md"},
		},
		Comments: []*models.Comment{{Text: "Decision"}},
		Tags:     []string{"backend", "security"},
	}
	return task
}

// pagedPeekOutliner puts that task under a cursor with Peek open, which is the
// only state the paging keys answer in.
func pagedPeekOutliner(t *testing.T, task *models.Task, width, height int) *OutlinerModel {
	t.Helper()
	project := renderFixtureProject(t, "project", "project", "Project", []*models.Task{task})
	outliner := scopeTestOutliner(project)
	outliner.SetSize(width, height)
	outliner.RebuildRows()
	outliner.MoveCursorToIdentity(taskref.TaskIdentity{ProjectID: project.ID, TaskID: task.ID})
	outliner.PeekMode = true
	// A frame has to have been drawn for LastViewport to hold the row width the
	// paging keys read, exactly as it has when a key arrives in production.
	outliner.View()
	return outliner
}

func cursorPeekDocument(t *testing.T, m *OutlinerModel) inlinePeekDocument {
	t.Helper()
	_, doc, ok := m.peekCursorDocument()
	if !ok {
		t.Fatal("cursor names no Peek document")
	}
	return doc
}

// TestInlinePeekPagesExposeEveryBodyLine is the completeness claim. Paging
// bought nothing unless every row of the document is on some page, so the union
// of the pages is checked against the body itself rather than against a spot
// check of what a reader hoped to find.
func TestInlinePeekPagesExposeEveryBodyLine(t *testing.T) {
	for _, width := range []int{40, 60, 80, 120} {
		doc := inlinePeekDocumentFor(inlinePeekOptions{Task: pagedPeekTask(), Indent: "  ", Width: width})
		if !doc.Paged() {
			t.Fatalf("width %d fit the fixture in one page", width)
		}

		seen := make([]bool, len(doc.Body))
		for page := range doc.PageCount() {
			start := doc.pageStart(page)
			for i := start; i < start+inlinePeekPagedRows; i++ {
				seen[i] = true
			}
			lines := doc.Lines(page)
			if len(lines) != len(doc.Header)+inlinePeekBodyRows {
				t.Fatalf("width %d page %d drew %d rows, want %d",
					width, page, len(lines), len(doc.Header)+inlinePeekBodyRows)
			}
			for number, line := range lines {
				if got := lipgloss.Width(line); got > width {
					t.Fatalf("width %d page %d row %d = %d columns: %q",
						width, page, number, got, ansi.Strip(line))
				}
			}
		}
		for i, shown := range seen {
			if !shown {
				t.Fatalf("width %d never showed body row %d: %q", width, i, ansi.Strip(doc.Body[i]))
			}
		}
	}
}

// TestInlinePeekPagesOverlapByOneContentRow locks the page turn's readability:
// the row that ended a page leads the next one, so a wrapped sentence is never
// split across a keystroke with nothing in common between the halves.
func TestInlinePeekPagesOverlapByOneContentRow(t *testing.T) {
	doc := inlinePeekDocumentFor(inlinePeekOptions{Task: pagedPeekTask(), Width: 60})
	if doc.PageCount() < 3 {
		t.Fatalf("fixture holds %d pages, want at least three to observe a middle boundary", doc.PageCount())
	}

	for page := 1; page < doc.PageCount(); page++ {
		previous := doc.pageStart(page-1) + inlinePeekPagedRows
		start := doc.pageStart(page)
		if start >= previous {
			t.Fatalf("page %d starts at %d, past the %d the previous page ended at", page, start, previous)
		}
		// The last page is pinned to the end of the body, so its overlap can be
		// larger than one row; every other boundary is exactly one.
		if page < doc.PageCount()-1 && previous-start != inlinePeekPageOverlap {
			t.Fatalf("page %d overlaps by %d rows, want %d", page, previous-start, inlinePeekPageOverlap)
		}
	}

	// The last page is full rather than short: paging never leaves blank rows
	// under the final scrap of detail.
	last := doc.pageStart(doc.PageCount() - 1)
	if last+inlinePeekPagedRows != len(doc.Body) {
		t.Fatalf("last page ends at %d, want the body's %d rows", last+inlinePeekPagedRows, len(doc.Body))
	}
}

// TestInlinePeekCounterRowNamesWhatIsOffScreen holds the row the budget reserves.
// It is spent on exact counts rather than a page ordinal, because what a reader
// needs is how much detail is unread in each direction.
func TestInlinePeekCounterRowNamesWhatIsOffScreen(t *testing.T) {
	doc := inlinePeekDocumentFor(inlinePeekOptions{Task: pagedPeekTask(), Width: 80})
	total := len(doc.Body)

	for page := range doc.PageCount() {
		lines := doc.Lines(page)
		counter := ansi.Strip(lines[len(lines)-1])
		start := doc.pageStart(page)
		want := strings.TrimSpace(ansi.Strip(doc.counterLine(start, total-start-inlinePeekPagedRows)))
		if strings.TrimSpace(counter) != want {
			t.Fatalf("page %d counter = %q, want %q", page, counter, want)
		}
	}

	first := ansi.Strip(doc.Lines(0)[inlinePeekBodyRows-1])
	if !strings.HasPrefix(strings.TrimSpace(first), "0 above · ") {
		t.Fatalf("first page counter = %q, want nothing above it", first)
	}
	last := doc.Lines(doc.PageCount() - 1)
	if !strings.HasSuffix(strings.TrimSpace(ansi.Strip(last[len(last)-1])), "· 0 below") {
		t.Fatalf("last page counter = %q, want nothing below it", ansi.Strip(last[len(last)-1]))
	}

	// An unpaged document reserves nothing: the whole budget goes to content.
	short := inlinePeekDocumentFor(inlinePeekOptions{Task: &models.Task{Description: "Short"}, Width: 80})
	if short.Paged() {
		t.Fatal("a one-line document claimed to be paged")
	}
	if got := ansi.Strip(strings.Join(short.Lines(0), "\n")); strings.Contains(got, "below") {
		t.Fatalf("an unpaged document drew a counter row: %q", got)
	}
}

// TestPeekPagingIsOfferedOnlyWhereItCanMove is the offer contract. Ctrl+F and
// Ctrl+B are advertised and dispatched off one fact, so a key that cannot move
// the page is never on the footer and never accepted.
func TestPeekPagingIsOfferedOnlyWhereItCanMove(t *testing.T) {
	outliner := pagedPeekOutliner(t, pagedPeekTask(), 80, 24)

	peek := outliner.PeekCommandContext()
	if !peek.Visible || !peek.CanPageDown || peek.CanPageUp {
		t.Fatalf("first page context = %+v, want down only", peek)
	}
	if !outliner.CanPagePeek(1) || outliner.CanPagePeek(-1) {
		t.Fatalf("first page can page down = %v, up = %v", outliner.CanPagePeek(1), outliner.CanPagePeek(-1))
	}
	if labels := outliner.peekPagingHintLabels(); len(labels) != 1 || !strings.HasSuffix(labels[0], " more") {
		t.Fatalf("first page hints = %v, want the forward turn alone", labels)
	}

	doc := cursorPeekDocument(t, outliner)
	for range doc.PageCount() - 1 {
		if !outliner.PagePeek(1) {
			t.Fatal("a page turn the context offered refused to move")
		}
	}
	peek = outliner.PeekCommandContext()
	if !peek.Visible || peek.CanPageDown || !peek.CanPageUp {
		t.Fatalf("last page context = %+v, want up only", peek)
	}
	if outliner.PagePeek(1) {
		t.Fatal("paging past the last page moved")
	}
	if labels := outliner.peekPagingHintLabels(); len(labels) != 1 || !strings.HasSuffix(labels[0], " back") {
		t.Fatalf("last page hints = %v, want the back turn alone", labels)
	}

	// Peek closed is no offer at all, whatever the document holds.
	outliner.PeekMode = false
	if peek := outliner.PeekCommandContext(); peek.Visible || peek.CanPageDown || peek.CanPageUp {
		t.Fatalf("closed Peek context = %+v, want the zero value", peek)
	}
	if outliner.PagePeek(1) || outliner.PagePeek(-1) {
		t.Fatal("a closed Peek paged")
	}
	if labels := outliner.peekPagingHintLabels(); labels != nil {
		t.Fatalf("closed Peek hints = %v, want none", labels)
	}
}

// TestPeekPagingIsRefusedWhereThereIsNothingToPage covers the two states
// PeekMode alone does not settle: a document short enough to fit, and a cursor
// on a row Peek draws nothing under.
func TestPeekPagingIsRefusedWhereThereIsNothingToPage(t *testing.T) {
	short := &models.Task{ID: "short", ShortID: 1, Title: "Short", WorkflowState: models.StateBacklog}
	outliner := pagedPeekOutliner(t, short, 80, 24)
	if peek := outliner.PeekCommandContext(); !peek.Visible || peek.CanPageDown || peek.CanPageUp {
		t.Fatalf("unpaged context = %+v, want visible and immovable", peek)
	}
	if outliner.PagePeek(1) {
		t.Fatal("an unpaged Peek moved")
	}

	// A header row keeps PeekMode on and shows no Peek, so the offer has to come
	// off the footer even though the mode is still open.
	for index, row := range outliner.Rows {
		if !row.IsTaskRow() {
			outliner.Cursor = index
			break
		}
	}
	if outliner.Rows[outliner.Cursor].IsTaskRow() {
		t.Skip("fixture produced no header row to park the cursor on")
	}
	if peek := outliner.PeekCommandContext(); peek.Visible {
		t.Fatalf("a header row claimed a visible Peek: %+v", peek)
	}
}

// TestPeekPagingResetsOnEveryAxisItIsKeyedTo is the session-only page's whole
// lifetime. The stored page answers for one task, one detail, one reference
// population, and one layout; each of those changing falls back to the first
// page, and nothing else does.
func TestPeekPagingResetsOnEveryAxisItIsKeyedTo(t *testing.T) {
	task := pagedPeekTask()
	other := &models.Task{
		ID: "other", ShortID: 2, Title: "Other", WorkflowState: models.StateBacklog,
		Description: strings.Repeat("Another long description body ", 12),
	}
	project := renderFixtureProject(t, "project", "project", "Project", []*models.Task{task, other})
	outliner := scopeTestOutliner(project)
	outliner.SetSize(80, 24)
	outliner.RebuildRows()
	outliner.MoveCursorToIdentity(taskref.TaskIdentity{ProjectID: project.ID, TaskID: task.ID})
	outliner.PeekMode = true
	outliner.View()

	page := func() int {
		row, doc, ok := outliner.peekCursorDocument()
		if !ok {
			t.Fatal("cursor names no Peek document")
		}
		return outliner.peekPageForRow(row, doc)
	}
	turn := func() {
		if !outliner.PagePeek(1) {
			t.Fatal("the fixture would not page")
		}
	}

	// Redrawing preserves a manual turn: a render is not a reason to lose a page.
	turn()
	outliner.View()
	if page() != 1 {
		t.Fatalf("a redraw reset the page to %d", page())
	}

	// Moving to another task and back does too — the page belongs to the task,
	// so the other row reads its own first page and this one keeps its second.
	outliner.MoveCursorToIdentity(taskref.TaskIdentity{ProjectID: project.ID, TaskID: other.ID})
	if page() != 0 {
		t.Fatalf("another task inherited page %d", page())
	}
	outliner.MoveCursorToIdentity(taskref.TaskIdentity{ProjectID: project.ID, TaskID: task.ID})
	if page() != 1 {
		t.Fatalf("returning to the task lost its page: %d", page())
	}

	// Editing the detail resets: the page pointed into a document that no longer
	// exists, and the second page of a different document is not where the
	// reader was.
	task.Description += " and a newly appended sentence"
	if page() != 0 {
		t.Fatalf("an edited description kept page %d", page())
	}
	turn()

	// A resize resets for the same reason, since the wrap decides what a row is.
	outliner.SetSize(60, 24)
	outliner.View()
	if page() != 0 {
		t.Fatalf("a resize kept page %d", page())
	}
}

// TestPeekPagingKeepsOneMeasurementAndOneRender is the invariant the whole split
// exists to serve: whatever page Peek is on, the height the viewport reserved
// and the height the renderer drew are the same number, because both come from
// the same document.
func TestPeekPagingKeepsOneMeasurementAndOneRender(t *testing.T) {
	for _, size := range []struct{ width, height int }{{40, 12}, {60, 20}, {80, 24}, {120, 40}} {
		outliner := pagedPeekOutliner(t, pagedPeekTask(), size.width, size.height)
		row := outliner.Rows[outliner.Cursor]

		for page := 0; ; page++ {
			indent := inlinePeekIndentForRow(row)
			width := outliner.peekRenderWidth()
			drawn := outliner.InlinePeekLinesForRow(row, indent, width, chrome.WorstCaseColumnLayout(width))
			if got := outliner.inlinePeekLineCount(row, size.width-1); got != len(drawn) {
				t.Fatalf("%dx%d page %d measured %d rows and drew %d",
					size.width, size.height, page, got, len(drawn))
			}
			for number, line := range drawn {
				if got := lipgloss.Width(line); got > width {
					t.Fatalf("%dx%d page %d row %d = %d columns, want <= %d",
						size.width, size.height, page, number, got, width)
				}
			}
			if !outliner.PagePeek(1) {
				break
			}
		}

		// The whole view still fits the terminal on the last page, which is the
		// page a reserved-height mistake would overflow on.
		content := outliner.View().Content
		if got := lipgloss.Height(content); got > size.height {
			t.Fatalf("%dx%d view height = %d", size.width, size.height, got)
		}
		for number, line := range strings.Split(content, "\n") {
			if got := lipgloss.Width(line); got > size.width {
				t.Fatalf("%dx%d line %d = %d columns", size.width, size.height, number, got)
			}
		}
	}
}
