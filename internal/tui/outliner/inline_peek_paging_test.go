package outliner

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/taskref"
	"github.com/sglavoie/tt/internal/tui/theme"
)

// pagedPeekTask is a task whose detail cannot fit one window at any supported
// width: a long note, a long description, dates, attachments, comments, a
// context, and tags. Every scrolling claim below is made against it, so none of
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

// unicodePeekTask is the same shape spelled in wide graphemes and combining
// marks, which is where a window measured in runes rather than in columns would
// overflow the pane it was clamped to.
func unicodePeekTask() *models.Task {
	return &models.Task{
		ID:            "unicode",
		ShortID:       2,
		Title:         "受け入れ基準の確認",
		WorkflowState: models.StateBacklog,
		ResumeNote:    "途中まで確認済み — 残りは請求書の丸め処理とロケール別の桁区切り",
		Description:   strings.Repeat("行の折り返しを確かめるための日本語の本文。 ", 10),
		Context:       "国際化",
		Attachments:   []models.Attachment{{Filename: "仕様書-final.pdf"}},
		Comments:      []*models.Comment{{Text: "決定事項"}},
		Tags:          []string{"ｉ18ｎ", "équipe"},
	}
}

// pagedPeekOutliner puts that task under a cursor with Peek open, which is the
// only state the scrolling keys answer in.
func pagedPeekOutliner(t *testing.T, task *models.Task, width, height int) *OutlinerModel {
	t.Helper()
	project := renderFixtureProject(t, "project", "project", "Project", []*models.Task{task})
	outliner := scopeTestOutliner(project)
	outliner.SetSize(width, height)
	outliner.RebuildRows()
	outliner.MoveCursorToIdentity(taskref.TaskIdentity{ProjectID: project.ID, TaskID: task.ID})
	outliner.PeekMode = true
	// A frame is drawn for the same reason production draws one before a key
	// arrives: the dock is on screen from here on, and its window is what the
	// scrolling keys move.
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

func peekOffset(t *testing.T, m *OutlinerModel) int {
	t.Helper()
	row, doc, ok := m.peekCursorDocument()
	if !ok {
		t.Fatal("cursor names no Peek document")
	}
	return m.peekOffsetForRow(row, doc)
}

// TestInlinePeekWindowsExposeEveryBodyLine is the completeness claim, and it is
// made against the mixed sequence rather than one distance: a reader who
// alternates the two keys must still be able to reach every row, and the window
// must stay inside its budget and its width at every stop along the way.
func TestInlinePeekWindowsExposeEveryBodyLine(t *testing.T) {
	window := testPeekWindow()
	for _, width := range []int{40, 60, 80, 120} {
		doc := inlinePeekDocumentFor(inlinePeekOptions{Task: pagedPeekTask(), Indent: "  ", Width: width, Window: window})
		if !doc.Paged() {
			t.Fatalf("width %d fit the fixture in one window", width)
		}

		seen := make([]bool, len(doc.Body))
		offset, step := 0, window.HalfPageRows()
		for {
			for i := offset; i < min(offset+window.BodyRows(), len(seen)); i++ {
				seen[i] = true
			}
			lines := doc.Lines(offset)
			if len(lines) != window.WindowRows() {
				t.Fatalf("width %d offset %d drew %d rows, want %d",
					width, offset, len(lines), window.WindowRows())
			}
			for number, line := range lines {
				if got := lipgloss.Width(line); got > width {
					t.Fatalf("width %d offset %d row %d = %d columns: %q",
						width, offset, number, got, ansi.Strip(line))
				}
			}
			next := doc.ClampOffset(offset + step)
			if next == offset {
				break
			}
			// Alternating the two distances is the sequence the keys make available,
			// and a window that only lands on page boundaries would skip rows under it.
			offset, step = next, window.HalfPageRows()+window.FullPageRows()-step
		}
		if offset != doc.MaxOffset() {
			t.Fatalf("width %d stopped at offset %d, want the document end %d", width, offset, doc.MaxOffset())
		}
		for i, shown := range seen {
			if !shown {
				t.Fatalf("width %d never showed body row %d: %q", width, i, ansi.Strip(doc.Body[i]))
			}
		}
	}
}

// TestInlinePeekWindowMovesByContentRows locks the two distances and the clamp
// that ends them. Both are derived from the window rather than declared beside
// it — half of it, and all but one row of it — and both are counted off wherever
// the window already sits, so the two keys compose instead of each snapping to
// its own grid.
func TestInlinePeekWindowMovesByContentRows(t *testing.T) {
	window := testPeekWindow()
	half, full := window.HalfPageRows(), window.FullPageRows()
	doc := inlinePeekDocumentFor(inlinePeekOptions{Task: pagedPeekTask(), Width: 60, Window: window})
	if doc.MaxOffset() < half+full {
		t.Fatalf("fixture ends at offset %d, too short to observe a mixed sequence", doc.MaxOffset())
	}

	if got := doc.ClampOffset(0 + half); got != half {
		t.Fatalf("a half page from the top = %d, want %d", got, half)
	}
	mixed := doc.ClampOffset(half + full)
	if want := half + full; mixed != want {
		t.Fatalf("a full page after a half page = %d, want %d", mixed, want)
	}
	if got := doc.ClampOffset(mixed - half); got != full {
		t.Fatalf("a half page back = %d, want %d", got, full)
	}

	// A full page keeps one content row on screen across the move, which is the
	// overlap that makes the jump readable. That holds at every window height the
	// frame can hand the dock, down to the one-row minimum where the two
	// distances collapse onto the same single row.
	for body := 1; body <= peekDockPreferredMaxRows; body++ {
		geometry := peekDockGeometry{bodyRows: body}
		if overlap := body - geometry.FullPageRows(); overlap != 1 && body != 1 {
			t.Fatalf("a full page of %d rows in a %d-row window leaves %d rows of overlap, want 1",
				geometry.FullPageRows(), body, overlap)
		}
		if geometry.FullPageRows() < 1 || geometry.HalfPageRows() < 1 {
			t.Fatalf("a %d-row window moves %d and %d rows, want at least one each",
				body, geometry.HalfPageRows(), geometry.FullPageRows())
		}
	}

	// Both ends clamp, and the last window is full rather than short: scrolling
	// never leaves blank rows under the final scrap of detail.
	if got := doc.ClampOffset(-full); got != 0 {
		t.Fatalf("scrolling above the top = %d, want 0", got)
	}
	if got := doc.ClampOffset(len(doc.Body)); got != doc.MaxOffset() {
		t.Fatalf("scrolling past the end = %d, want %d", got, doc.MaxOffset())
	}
	if doc.MaxOffset()+window.BodyRows() != len(doc.Body) {
		t.Fatalf("the last window ends at %d, want the body's %d rows",
			doc.MaxOffset()+window.BodyRows(), len(doc.Body))
	}
}

// TestInlinePeekPositionRowNamesWhatIsOffScreen holds the row the dock reserves
// under the body. It is spent on exact counts rather than an ordinal, because
// what a reader needs is how much detail is unread in each direction — and once
// a window can stop between two pages, an ordinal cannot be spelled at all. It
// is always drawn, an unpaged document included: a row that came and went would
// move the frame under the cursor for no reason the reader chose.
func TestInlinePeekPositionRowNamesWhatIsOffScreen(t *testing.T) {
	window := testPeekWindow()
	doc := inlinePeekDocumentFor(inlinePeekOptions{Task: pagedPeekTask(), Width: 80, Window: window})
	total := len(doc.Body)

	for offset := 0; offset <= doc.MaxOffset(); offset++ {
		lines := doc.Lines(offset)
		position := ansi.Strip(lines[len(lines)-1])
		want := strings.TrimSpace(ansi.Strip(doc.positionLine(offset, total-offset-window.BodyRows())))
		if strings.TrimSpace(position) != want {
			t.Fatalf("offset %d position row = %q, want %q", offset, position, want)
		}
	}

	first := ansi.Strip(doc.Lines(0)[window.WindowRows()-1])
	if !strings.HasPrefix(strings.TrimSpace(first), "0 above · ") {
		t.Fatalf("top position row = %q, want nothing above it", first)
	}
	last := doc.Lines(doc.MaxOffset())
	if !strings.HasSuffix(strings.TrimSpace(ansi.Strip(last[len(last)-1])), "· 0 below") {
		t.Fatalf("end position row = %q, want nothing below it", ansi.Strip(last[len(last)-1]))
	}

	// An unpaged document keeps the same window: the body is padded rather than
	// the dock shortened, and the position row stays reserved but blank, since
	// nothing is off screen to count.
	short := inlinePeekDocumentFor(inlinePeekOptions{Task: &models.Task{Description: "Short"}, Width: 80, Window: window})
	if short.Paged() {
		t.Fatal("a one-line document claimed to be paged")
	}
	shortLines := short.Lines(0)
	if len(shortLines) != window.WindowRows() {
		t.Fatalf("an unpaged document drew %d rows, want %d", len(shortLines), window.WindowRows())
	}
	if got := strings.TrimSpace(ansi.Strip(shortLines[len(shortLines)-1])); got != "" {
		t.Fatalf("an unpaged position row = %q, want it blank", got)
	}
}

// TestPeekScrollingReachesEveryLineThroughMixedKeys is the acceptance sequence
// pressed through the model rather than computed on the document: half pages and
// full pages interleaved, from the top to the end and back, with the cursor
// asserted still on the row it started on.
func TestPeekScrollingReachesEveryLineThroughMixedKeys(t *testing.T) {
	outliner := pagedPeekOutliner(t, pagedPeekTask(), 80, 24)
	cursor := outliner.Cursor
	doc := cursorPeekDocument(t, outliner)
	end := doc.MaxOffset()

	scroll := func(half bool, direction int) bool {
		if half {
			return outliner.HalfPagePeek(direction)
		}
		return outliner.PagePeek(direction)
	}

	seen := map[int]bool{0: true}
	for half := true; scroll(half, 1); half = !half {
		seen[peekOffset(t, outliner)] = true
	}
	if got := peekOffset(t, outliner); got != end {
		t.Fatalf("mixed scrolling stopped at offset %d, want the document end %d", got, end)
	}
	if len(seen) < 3 {
		t.Fatalf("mixed scrolling visited %d offsets, want the intermediate stops too", len(seen))
	}

	// Back the same way, starting on the other distance so the return trip lands
	// on different offsets than the outward one did.
	for half := false; scroll(half, -1); half = !half { //nolint:revive // Reaching the top is the point of the loop.
	}
	if got := peekOffset(t, outliner); got != 0 {
		t.Fatalf("scrolling back stopped at offset %d, want the top", got)
	}
	if outliner.Cursor != cursor {
		t.Fatalf("scrolling moved the cursor to row %d, want %d", outliner.Cursor, cursor)
	}
}

// TestPeekScrollingIsOfferedOnlyWhereItCanMove is the availability half of the
// contract: the footer names a movement only in the direction the window has one.
func TestPeekScrollingIsOfferedOnlyWhereItCanMove(t *testing.T) {
	outliner := pagedPeekOutliner(t, pagedPeekTask(), 80, 24)

	peek := outliner.PeekCommandContext()
	if !peek.Visible || !peek.CanPageDown || peek.CanPageUp {
		t.Fatalf("top context = %+v, want down only", peek)
	}
	if !outliner.CanScrollPeek(1) || outliner.CanScrollPeek(-1) {
		t.Fatalf("top can scroll down = %v, up = %v", outliner.CanScrollPeek(1), outliner.CanScrollPeek(-1))
	}
	hints := outliner.peekScrollHints()
	if len(hints) != 1 {
		t.Fatalf("top hints = %v, want the half-page forward movement alone", hintEntries(hints))
	}
	for _, hint := range hints {
		if strings.HasSuffix(hint.Label, "back") || strings.HasSuffix(hint.Label, "page up") {
			t.Fatalf("top offered a backward movement: %v", hintEntries(hints))
		}
	}

	for outliner.PagePeek(1) { //nolint:revive // Reaching the end is the point of the loop.
	}
	peek = outliner.PeekCommandContext()
	if !peek.Visible || peek.CanPageDown || !peek.CanPageUp {
		t.Fatalf("end context = %+v, want up only", peek)
	}
	if outliner.PagePeek(1) || outliner.HalfPagePeek(1) {
		t.Fatal("scrolling past the end moved")
	}
	hints = outliner.peekScrollHints()
	if len(hints) != 1 {
		t.Fatalf("end hints = %v, want the half-page backward movement alone", hintEntries(hints))
	}
	for _, hint := range hints {
		if strings.HasSuffix(hint.Label, "more") || strings.HasSuffix(hint.Label, "page down") {
			t.Fatalf("the end offered a forward movement: %v", hintEntries(hints))
		}
	}

	// Peek closed is no offer at all, whatever the document holds.
	outliner.PeekMode = false
	if peek := outliner.PeekCommandContext(); peek.Visible || peek.CanPageDown || peek.CanPageUp {
		t.Fatalf("closed Peek context = %+v, want the zero value", peek)
	}
	if outliner.PagePeek(1) || outliner.HalfPagePeek(-1) {
		t.Fatal("a closed Peek scrolled")
	}
	if hints := outliner.peekScrollHints(); hints != nil {
		t.Fatalf("closed Peek hints = %v, want none", hintEntries(hints))
	}
}

func hintEntries(hints []chrome.GuidanceHint) []string {
	entries := make([]string, 0, len(hints))
	for _, hint := range hints {
		entries = append(entries, hint.Key+" "+hint.Label)
	}
	return entries
}

// TestPeekScrollingIsRefusedWhereThereIsNothingToScroll covers the two states
// PeekMode alone does not settle: a document short enough to fit, and a cursor
// on a row Peek draws nothing under. A short document is still visible, so the
// keys are still consumed there; what it has is nowhere to go.
func TestPeekScrollingIsRefusedWhereThereIsNothingToScroll(t *testing.T) {
	short := &models.Task{ID: "short", ShortID: 1, Title: "Short", WorkflowState: models.StateBacklog}
	outliner := pagedPeekOutliner(t, short, 80, 24)
	if peek := outliner.PeekCommandContext(); !peek.Visible || peek.CanPageDown || peek.CanPageUp {
		t.Fatalf("unpaged context = %+v, want visible and immovable", peek)
	}
	if outliner.PagePeek(1) || outliner.HalfPagePeek(1) || outliner.HalfPagePeek(-1) {
		t.Fatal("an unpaged Peek moved")
	}
	if hints := outliner.peekScrollHints(); len(hints) != 0 {
		t.Fatalf("an unpaged Peek offered %v", hintEntries(hints))
	}

	// A row that owns no document keeps the dock open and draws the placeholder,
	// so the keys are still consumed there while the offer comes off the footer.
	for index, row := range outliner.Rows {
		if !rowSupportsPeek(row) {
			outliner.Cursor = index
			break
		}
	}
	if rowSupportsPeek(outliner.Rows[outliner.Cursor]) {
		t.Skip("fixture produced no unsupported row to park the cursor on")
	}
	peek := outliner.PeekCommandContext()
	if !peek.Visible || peek.CanPageDown || peek.CanPageUp {
		t.Fatalf("placeholder context = %+v, want visible and immovable", peek)
	}
	dock := outliner.PeekDockLines()
	if want := outliner.PeekDockRows(); len(dock) != want {
		t.Fatalf("placeholder dock drew %d rows, want %d", len(dock), want)
	}
	if got := ansi.Strip(strings.Join(dock, "\n")); !strings.Contains(got, peekDockPlaceholderText) {
		t.Fatalf("placeholder dock = %q, want the stand-in text", got)
	}
	if hints := outliner.peekScrollHints(); len(hints) != 0 {
		t.Fatalf("a placeholder offered %v", hintEntries(hints))
	}
}

// TestPeekScrollingResetsOnEveryAxisItIsKeyedTo is the session-only offset's
// whole lifetime. The stored offset answers for one owner, one detail, one
// reference population, and one layout; each of those changing falls back to the
// top of the document, and nothing else does.
func TestPeekScrollingResetsOnEveryAxisItIsKeyedTo(t *testing.T) {
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

	half := outliner.peekDockGeometry().HalfPageRows()
	scroll := func() {
		if !outliner.HalfPagePeek(1) {
			t.Fatal("the fixture would not scroll")
		}
	}

	// Redrawing preserves a manual scroll: a render is not a reason to lose a
	// reading position.
	scroll()
	outliner.View()
	if got := peekOffset(t, outliner); got != half {
		t.Fatalf("a redraw reset the offset to %d", got)
	}

	// Moving to another task and back does too — the offset belongs to the owner,
	// so the other row reads its own top and this one keeps where it was.
	outliner.MoveCursorToIdentity(taskref.TaskIdentity{ProjectID: project.ID, TaskID: other.ID})
	if got := peekOffset(t, outliner); got != 0 {
		t.Fatalf("another task inherited offset %d", got)
	}
	outliner.MoveCursorToIdentity(taskref.TaskIdentity{ProjectID: project.ID, TaskID: task.ID})
	if got := peekOffset(t, outliner); got != half {
		t.Fatalf("returning to the task lost its offset: %d", got)
	}

	// Editing the detail resets: the offset pointed into a document that no longer
	// exists, and the same row of a different document is not where the reader was.
	outliner.Rows[outliner.Cursor].Task.Description += " and a newly appended sentence"
	if got := peekOffset(t, outliner); got != 0 {
		t.Fatalf("an edited description kept offset %d", got)
	}
	scroll()

	// A resize resets for the same reason, since the wrap decides what a row is.
	outliner.SetSize(60, 24)
	outliner.View()
	if got := peekOffset(t, outliner); got != 0 {
		t.Fatalf("a resize kept offset %d", got)
	}
}

// TestPeekScrollingKeepsOneMeasurementAndOneRender is the invariant the whole
// split exists to serve: wherever the window sits, the height the viewport
// reserved and the height the renderer drew are the same number, because both
// come from the same document. It runs over Unicode content and both themes for
// the same reason it runs over four sizes — a wide grapheme, a combining mark, or
// a palette that paints differently must not change what a row costs. Every
// width is asserted against the stripped text as well, which is the same row
// under --no-color: a fit that depended on the escapes being invisible would be
// an overflow there.
func TestPeekScrollingKeepsOneMeasurementAndOneRender(t *testing.T) {
	for _, themeName := range []string{"dark", "light"} {
		theme.ApplyTheme(theme.ThemeConfig{Name: themeName})
		for _, task := range []*models.Task{pagedPeekTask(), unicodePeekTask()} {
			for _, size := range []struct{ width, height int }{{40, 12}, {60, 20}, {80, 24}, {120, 40}} {
				outliner := pagedPeekOutliner(t, task, size.width, size.height)

				for step := 0; ; step++ {
					width := size.width
					drawn := outliner.PeekDockLines()
					if got := outliner.PeekDockRows(); got != len(drawn) {
						t.Fatalf("%s %s %dx%d step %d measured %d rows and drew %d",
							themeName, task.ID, size.width, size.height, step, got, len(drawn))
					}
					for number, line := range drawn {
						for label, measured := range map[string]int{
							"styled":   lipgloss.Width(line),
							"no-color": lipgloss.Width(ansi.Strip(line)),
						} {
							if measured > width {
								t.Fatalf("%s %s %dx%d step %d row %d %s = %d columns, want <= %d",
									themeName, task.ID, size.width, size.height, step, number, label, measured, width)
							}
						}
					}
					if step%2 == 0 {
						if !outliner.HalfPagePeek(1) {
							break
						}
						continue
					}
					if !outliner.PagePeek(1) {
						break
					}
				}

				// The whole view still fits the terminal at the end of the document,
				// which is where a reserved-height mistake would overflow.
				content := outliner.View().Content
				if got := lipgloss.Height(content); got > size.height {
					t.Fatalf("%s %s %dx%d view height = %d", themeName, task.ID, size.width, size.height, got)
				}
				for number, line := range strings.Split(content, "\n") {
					if got := lipgloss.Width(line); got > size.width {
						t.Fatalf("%s %s %dx%d line %d = %d columns",
							themeName, task.ID, size.width, size.height, number, got)
					}
				}
			}
		}
	}
	theme.ApplyTheme(theme.ThemeConfig{})
}
