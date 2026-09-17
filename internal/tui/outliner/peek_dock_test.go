package outliner

import (
	"fmt"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/chrome"
)

// dockFixtureOutliner seats a one-line task beside the paged one, which is the
// pair every claim below is made against: the dock's whole promise is that
// crossing between them costs the list nothing, and a fixture whose tasks were
// the same length could not tell a fixed dock from a growing one.
func dockFixtureOutliner(t *testing.T, width, height int) *OutlinerModel {
	t.Helper()
	short := &models.Task{
		ID:            "short",
		ShortID:       1,
		Title:         "Short",
		WorkflowState: models.StateBacklog,
	}
	long := pagedPeekTask()
	long.Title = "Long"
	project := renderFixtureProject(t, "project", "project", "Project", []*models.Task{short, long})
	outliner := scopeTestOutliner(project)
	outliner.SetSize(width, height)
	outliner.RebuildRows()
	return outliner
}

// peekableRowIndexes names the rows that own a document and the first row that
// does not, so each claim can be made at both kinds of cursor without assuming
// which projection the fixture built.
func peekableRowIndexes(t *testing.T, m *OutlinerModel) (withDocument []int, withoutDocument int) {
	t.Helper()
	withoutDocument = -1
	for index, row := range m.Rows {
		if rowSupportsPeek(row) {
			withDocument = append(withDocument, index)
			continue
		}
		if withoutDocument < 0 {
			withoutDocument = index
		}
	}
	if len(withDocument) < 2 {
		t.Fatalf("fixture drew %d rows owning a document, want at least two", len(withDocument))
	}
	return withDocument, withoutDocument
}

// TestPeekDockIsTheSameHeightUnderEveryCursor is the session's central claim.
// The dock's height answers to the frame, so within one frame the rows it draws
// and the rows it costs the list are one number under a one-line task, under a
// document forty rows long, and under a row that owns no document at all. The
// number itself differs between the three terminals below, which is the other
// half of the claim: a resize moves everything once, crossing a row moves
// nothing.
func TestPeekDockIsTheSameHeightUnderEveryCursor(t *testing.T) {
	for _, size := range [][2]int{{60, 20}, {80, 24}, {120, 40}} {
		outliner := dockFixtureOutliner(t, size[0], size[1])
		outliner.PeekMode = true
		peekable, plain := peekableRowIndexes(t, outliner)
		cursors := append([]int{}, peekable...)
		if plain >= 0 {
			cursors = append(cursors, plain)
		}

		want := -1
		wantRows := outliner.PeekDockRows()
		if wantRows < peekDockMinRows {
			t.Fatalf("%dx%d reports a %d-row dock, want at least %d",
				size[0], size[1], wantRows, peekDockMinRows)
		}
		for _, cursor := range cursors {
			outliner.Cursor = cursor
			outliner.View()
			lines := outliner.PeekDockLines()
			if len(lines) != wantRows {
				t.Fatalf("%dx%d cursor %d drew %d dock rows, want %d",
					size[0], size[1], cursor, len(lines), wantRows)
			}
			if got := outliner.PeekDockRows(); got != wantRows {
				t.Fatalf("%dx%d cursor %d reports a %d-row dock, want %d",
					size[0], size[1], cursor, got, wantRows)
			}
			for number, line := range lines {
				if got := lipgloss.Width(line); got > size[0] {
					t.Fatalf("%dx%d cursor %d dock row %d = %d columns: %q",
						size[0], size[1], cursor, number, got, ansi.Strip(line))
				}
			}
			visible := outliner.LastViewport.VisibleRows
			if want < 0 {
				want = visible
				continue
			}
			if visible != want {
				t.Fatalf("%dx%d cursor %d left the list %d rows, want %d",
					size[0], size[1], cursor, visible, want)
			}
		}
	}
}

// TestPeekDockCostsTheListOneFixedReservation locks what opening Peek spends.
// The dock comes off the list once, and it is the same subtraction whatever the
// cursor rests on, so the list moves when Peek opens and never again while it is
// open.
//
// It is the dock plus the one row that holds it still. That row is the floor
// session 4 put under the shell's measured footer excess while the dock is open:
// the dock is the one thing whose guidance this view's cursor decides, since the
// paging keys are advertised only over a document that can move, so a list
// following the footer honestly would slide the dock from one row to the next as
// the cursor crossed a short task. One row is paid either way.
func TestPeekDockCostsTheListOneFixedReservation(t *testing.T) {
	outliner := dockFixtureOutliner(t, 80, 24)
	peekable, _ := peekableRowIndexes(t, outliner)
	outliner.Cursor = peekable[0]

	outliner.View()
	closed := outliner.LastViewport.VisibleRows
	if outliner.PeekDockVisible() {
		t.Fatal("the dock is open before b was pressed")
	}

	outliner.PeekMode = true
	outliner.View()
	open := outliner.LastViewport.VisibleRows
	dock := outliner.PeekDockRows()
	if want := closed - dock - peekDockFooterWrapRows; open != want {
		t.Fatalf("an open dock left the list %d rows, want %d (%d - %d - %d wrap row)",
			open, want, closed, dock, peekDockFooterWrapRows)
	}
}

// TestPeekDockReservationRidesTheMeasuredFooter is the measured half. With the
// dock closed the list pays whatever the shell measured, exactly once. With it
// open the reservation has a floor rather than a constant: a footer that costs
// no more than that floor moves nothing, and one that costs more is still
// charged in full, so the frame is never over-drawn.
func TestPeekDockReservationRidesTheMeasuredFooter(t *testing.T) {
	oneRow := chrome.ShellFrameMetrics{FeedbackRows: 1, ControlRows: 1}
	if got := oneRow.ExcessRows(); got != 1 {
		t.Fatalf("a feedback row beside a control row is an excess of %d, want 1", got)
	}
	twoRows := chrome.ShellFrameMetrics{FeedbackRows: 1, ControlRows: 2}
	if got := twoRows.ExcessRows(); got != 2 {
		t.Fatalf("a feedback row over wrapped controls is an excess of %d, want 2", got)
	}

	for _, test := range []struct {
		name string
		peek bool
		// lost is what each metric costs the list relative to the resting budget.
		lostAtOne int
		lostAtTwo int
	}{
		// Closed, nothing stands between the list and the footer, so both
		// readings are the measurement itself.
		{name: "dock closed", lostAtOne: 1, lostAtTwo: 2},
		// Open, the first row is already reserved, so only the second is new.
		{name: "dock open", peek: true, lostAtOne: 0, lostAtTwo: 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			outliner := dockFixtureOutliner(t, 80, 24)
			peekable, _ := peekableRowIndexes(t, outliner)
			outliner.Cursor = peekable[0]
			outliner.PeekMode = test.peek

			outliner.View()
			baseline := outliner.LastViewport.VisibleRows

			outliner.SetShellFrameMetrics(oneRow)
			outliner.View()
			if got, want := outliner.LastViewport.VisibleRows, baseline-test.lostAtOne; got != want {
				t.Fatalf("a footer one row taller left the list %d rows, want %d", got, want)
			}

			outliner.SetShellFrameMetrics(twoRows)
			outliner.View()
			if got, want := outliner.LastViewport.VisibleRows, baseline-test.lostAtTwo; got != want {
				t.Fatalf("a footer two rows taller left the list %d rows, want %d", got, want)
			}

			// The budget is not a size, so handing it back restores the list
			// without the scroll window having been re-anchored on the way.
			outliner.SetShellFrameMetrics(chrome.ShellFrameMetrics{})
			outliner.View()
			if got := outliner.LastViewport.VisibleRows; got != baseline {
				t.Fatalf("clearing the budget left the list %d rows, want %d", got, baseline)
			}
		})
	}
}

// TestPeekDockNamesARowThatOwnsNoDocument is the placeholder claim. A section
// heading, a note, or a checklist item keeps the dock open and keeps it this
// tall, so crossing one moves nothing on screen and the pane can still be closed
// from where the cursor landed.
func TestPeekDockNamesARowThatOwnsNoDocument(t *testing.T) {
	outliner := dockFixtureOutliner(t, 80, 24)
	_, plain := peekableRowIndexes(t, outliner)
	if plain < 0 {
		t.Skip("the fixture drew no row without a document")
	}
	outliner.PeekMode = true
	outliner.Cursor = plain
	outliner.View()

	lines := outliner.PeekDockLines()
	if want := outliner.PeekDockRows(); len(lines) != want {
		t.Fatalf("a row with no document drew %d dock rows, want %d", len(lines), want)
	}
	body := ansi.Strip(strings.Join(lines, "\n"))
	if !strings.Contains(body, peekDockPlaceholderText) {
		t.Fatalf("the dock over a row with no document reads %q, want the placeholder", body)
	}
	if peek := outliner.PeekCommandContext(); !peek.Visible || peek.CanPageDown || peek.CanPageUp {
		t.Fatalf("placeholder peek context = %+v, want visible with nothing to page", peek)
	}
}

// TestPeekDockHeightIsAShareOfTheFrame locks the responsive height at the four
// reviewed terminals, the brief's eleven rows at 100x35 included. The height is
// a third of what this view was handed, bounded at both ends, so a taller
// terminal reads more of a document without a shorter one losing its list.
func TestPeekDockHeightIsAShareOfTheFrame(t *testing.T) {
	for _, test := range []struct {
		width, height int
		want          int
	}{
		// The floor binds at the two short terminals: a third of 18 and of 22 rows
		// is under seven, so both take the minimum readable dock.
		{width: 60, height: 20, want: peekDockPreferredMinRows},
		{width: 80, height: 24, want: peekDockPreferredMinRows},
		// A third of the 33 rows this view is handed at 100x35, which is the
		// brief's eleven: nine content lines, the divider, and the position row.
		{width: 100, height: 35, want: 11},
		// The ceiling binds at the tallest, so the list keeps the rest of it.
		{width: 120, height: 40, want: peekDockPreferredMaxRows},
	} {
		t.Run(fmt.Sprintf("%dx%d", test.width, test.height), func(t *testing.T) {
			// The sizes are terminals and the shell hands a view two rows fewer.
			m := dockFixtureOutliner(t, test.width, test.height-2)
			m.PeekMode = true
			peekable, _ := peekableRowIndexes(t, m)
			m.Cursor = peekable[0]
			m.View()

			if got := m.PeekDockRows(); got != test.want {
				t.Fatalf("dock = %d rows, want %d", got, test.want)
			}
			lines := m.PeekDockLines()
			if len(lines) != test.want {
				t.Fatalf("dock drew %d rows, want %d", len(lines), test.want)
			}
			geometry := m.peekDockGeometry()
			if got := geometry.BodyRows(); got != test.want-peekDockDividerRows-inlinePeekPositionRows {
				t.Fatalf("dock body = %d rows, want %d", got, test.want-2)
			}
			// The window and the movement distances come off that one body, so a
			// taller dock pages further without any key declaring a distance.
			if got, want := geometry.HalfPageRows(), max(geometry.BodyRows()/2, 1); got != want {
				t.Fatalf("half page = %d rows, want %d", got, want)
			}
			if got, want := geometry.FullPageRows(), max(geometry.BodyRows()-1, 1); got != want {
				t.Fatalf("full page = %d rows, want %d", got, want)
			}
		})
	}
}

// TestPeekDockYieldsToTheFrameUnderPressure is the constrained half. The dock
// takes the room left after the chrome this view has already committed to and
// after the one task row the list may never lose, so a frame with nowhere to put
// a full dock gets a shorter one rather than a list with no rows — and a frame
// with nowhere to put even the three-row minimum gets no dock at all rather than
// a window with no content row or one that cannot say what it is hiding.
func TestPeekDockYieldsToTheFrameUnderPressure(t *testing.T) {
	for _, test := range []struct {
		name   string
		height int
		// shortened says the dock is drawn under its preferred height; absent means
		// it is not drawn at all.
		shortened bool
	}{
		{name: "resting", height: 33, shortened: false},
		{name: "squeezed", height: 8, shortened: true},
		{name: "no room", height: 4},
	} {
		t.Run(test.name, func(t *testing.T) {
			m := dockFixtureOutliner(t, 80, test.height)
			m.PeekMode = true
			peekable, _ := peekableRowIndexes(t, m)
			m.Cursor = peekable[0]
			m.View()

			rows := m.PeekDockRows()
			switch {
			case !test.shortened && test.height > 20:
				if rows != m.peekDockPreferredRows() {
					t.Fatalf("a resting frame drew a %d-row dock, want its preferred %d",
						rows, m.peekDockPreferredRows())
				}
			case test.shortened:
				if rows < peekDockMinRows || rows >= m.peekDockPreferredRows() {
					t.Fatalf("a squeezed frame drew a %d-row dock, want between %d and %d",
						rows, peekDockMinRows, m.peekDockPreferredRows())
				}
			default:
				if rows != 0 || m.PeekDockVisible() {
					t.Fatalf("a frame with no room drew a %d-row dock, want none", rows)
				}
			}
			if len(m.PeekDockLines()) != rows {
				t.Fatalf("the dock reports %d rows and drew %d", rows, len(m.PeekDockLines()))
			}
			// Whatever the dock took, the list keeps a row: the list is the view and
			// the dock is a detail beside it.
			if got := m.LastViewport.VisibleRows; got < peekDockListFloorRows {
				t.Fatalf("the list kept %d rows, want at least %d", got, peekDockListFloorRows)
			}
		})
	}
}

// TestPeekDockHeightIgnoresTheDocumentAndTheCursorOffset is the purity claim
// read from the other side: the geometry is a function of the frame, so the dock
// over the fixture's one-line task, over its forty-row one, and over the same
// document scrolled halfway all measure identically.
func TestPeekDockHeightIgnoresTheDocumentAndTheCursorOffset(t *testing.T) {
	m := dockFixtureOutliner(t, 100, 33)
	m.PeekMode = true
	peekable, _ := peekableRowIndexes(t, m)
	m.Cursor = peekable[0]
	m.View()
	want := m.PeekDockRows()

	for _, cursor := range peekable {
		m.Cursor = cursor
		m.View()
		if got := m.PeekDockRows(); got != want {
			t.Fatalf("cursor %d measured a %d-row dock, want %d", cursor, got, want)
		}
		for range 3 {
			m.HalfPagePeek(1)
			m.View()
			if got := m.PeekDockRows(); got != want {
				t.Fatalf("cursor %d measured a %d-row dock after paging, want %d", cursor, got, want)
			}
		}
	}
}

// dockGeometryReviewSizes are the four terminals the refinement is reviewed at,
// spelled as terminals: the shell hands every view the terminal height less its
// footer, so the render below is sized to two rows fewer than each of these.
var dockGeometryReviewSizes = [][2]int{{60, 20}, {80, 24}, {100, 35}, {120, 40}}

// dockScreenRows reads the dock's coordinates back off a rendered frame rather
// than off a constant: the row its divider landed on and the row its position
// line landed on. The view's block is stacked from the frame's first row, so
// these indices are the rows a reader counts down the screen. It returns the
// frame too, because every claim below is made about the same rendering these
// numbers came out of.
func dockScreenRows(t *testing.T, m *OutlinerModel, label string) (frame []string, divider, position int) {
	t.Helper()
	frame = strings.Split(ansi.Strip(m.View().Content), "\n")
	for index := range frame {
		frame[index] = strings.TrimRight(frame[index], " ")
	}
	position = -1
	for index, line := range frame {
		if emptyProjectPeekCounter.MatchString(strings.TrimSpace(line)) {
			position = index
		}
	}
	if position < 0 {
		t.Fatalf("%s: the frame draws no Peek position row:\n%s", label, strings.Join(frame, "\n"))
	}
	rows := m.PeekDockRows()
	divider = position - rows + 1
	if divider < 0 {
		t.Fatalf("%s: the position row is on screen row %d, too high to hold a %d-row dock:\n%s",
			label, position, rows, strings.Join(frame, "\n"))
	}
	if rule := strings.TrimSpace(frame[divider]); rule == "" || strings.Trim(rule, "─") != "" {
		t.Fatalf("%s: screen row %d is %q, want the dock's divider %d rows above its position row:\n%s",
			label, divider, frame[divider], rows-1, strings.Join(frame, "\n"))
	}
	return frame, divider, position
}

// assertDockIsDrawnWhole holds every row the dock says it has to the row of the
// frame it claims to be on. It is the difference between a dock as tall as the
// frame sized it and one that was cut down to whatever a projection left it: a
// shortened dock still draws a divider and still draws whatever rows fit, so a
// coordinate pair alone would not notice the missing ones.
func assertDockIsDrawnWhole(t *testing.T, m *OutlinerModel, frame []string, divider int, label string) {
	t.Helper()
	dock := m.PeekDockLines()
	if want := m.PeekDockRows(); len(dock) != want {
		t.Fatalf("%s: the dock reports %d rows, want %d", label, len(dock), want)
	}
	for offset, line := range dock {
		row := divider + offset
		if row >= len(frame) {
			t.Fatalf("%s: dock row %d is off the end of a %d-row frame", label, offset, len(frame))
		}
		if want := strings.TrimRight(ansi.Strip(line), " "); frame[row] != want {
			t.Fatalf("%s: screen row %d is %q, want dock row %d %q:\n%s",
				label, row, frame[row], offset, want, strings.Join(frame, "\n"))
		}
	}
}

// TestPeekDockHoldsItsPositionOnAnEmptyProjection is the empty branch of the
// fixed dock, and it is the branch a reader is most likely to meet: an empty
// project is exactly where Peek's project detail is worth opening. The empty
// state draws no list, so it used to hang the dock off its explanation and shorten
// it to what was left, which put Peek four rows down a frame where a populated
// projection docks it at the bottom — the moving target docking was meant to
// retire, read at its worst because nothing else on screen anchors the eye.
//
// The two projections are two scopes of one model at one size, so the frames
// differ in the render path alone and the coordinates are the review's own
// question: at this terminal, with this footer, where is the dock?
func TestPeekDockHoldsItsPositionOnAnEmptyProjection(t *testing.T) {
	for _, size := range dockGeometryReviewSizes {
		width, height := size[0], size[1]
		t.Run(fmt.Sprintf("%dx%d", width, height), func(t *testing.T) {
			populated := focusedHeaderProject("alpha")
			empty := emptyPeekProject("quiet")
			m := scopeTestOutliner(populated, empty)
			m.SetSize(width, height-2)
			m.PeekMode = true

			m.PushProjectScope(populated.ID)
			m.RebuildRows()
			if !m.HasContentRows() {
				t.Fatal("the populated scope drew no content rows")
			}
			cursorToTaskRow(t, m, "alpha-backlog")
			populatedFrame, wantDivider, wantPosition := dockScreenRows(t, m, "populated")
			assertDockIsDrawnWhole(t, m, populatedFrame, wantDivider, "populated")

			m.PushProjectScope(empty.ID)
			m.RebuildRows()
			if m.HasContentRows() {
				t.Fatal("the empty project's scope holds content rows, so it is not the empty state")
			}
			if !moveCursorToProjectHeader(m, empty.ID, 0) {
				t.Fatal("the empty projection built no header for its project")
			}
			emptyFrame, divider, position := dockScreenRows(t, m, "empty")
			if divider != wantDivider || position != wantPosition {
				t.Fatalf("the empty projection docks Peek at rows %d..%d while the populated one docks it at %d..%d\n%s\n---\n%s",
					divider, position, wantDivider, wantPosition,
					strings.Join(emptyFrame, "\n"), strings.Join(populatedFrame, "\n"))
			}
			assertDockIsDrawnWhole(t, m, emptyFrame, divider, "empty")

			// The dock holding its row costs the empty state nothing it owed: the
			// header naming the project and the sentence explaining the emptiness are
			// both still above it, in that order, at every reviewed size.
			message := -1
			for index, line := range emptyFrame[:divider] {
				if strings.Contains(line, m.OutlinerEmptyMessage()) {
					message = index
				}
			}
			if message <= 1 {
				t.Fatalf("the empty state's explanation is not drawn above the dock (row %d)\n%s",
					message, strings.Join(emptyFrame, "\n"))
			}
			if strings.TrimSpace(emptyFrame[message-1]) == "" {
				t.Fatalf("the project header above the explanation is blank\n%s", strings.Join(emptyFrame, "\n"))
			}
		})
	}
}

// TestPeekDockKeepsTheListRowsStillAcrossCursorMoves is the same promise read
// off the frame rather than off the measurement: the rendered list block and the
// screen row the divider lands on are identical under the short task and under
// the long one, which is what a reader sees as the outline not jumping.
func TestPeekDockKeepsTheListRowsStillAcrossCursorMoves(t *testing.T) {
	outliner := dockFixtureOutliner(t, 80, 24)
	outliner.PeekMode = true
	peekable, _ := peekableRowIndexes(t, outliner)

	dividerRow := -1
	for _, cursor := range peekable {
		outliner.Cursor = cursor
		frame := outliner.RenderOutlinerTreeContent(80)
		dock := outliner.PeekDockLines()
		lines := strings.Split(frame, "\n")
		index := -1
		for number, line := range lines {
			if line == dock[0] {
				index = number
				break
			}
		}
		if index < 0 {
			t.Fatalf("cursor %d drew no dock divider in its frame", cursor)
		}
		if dividerRow < 0 {
			dividerRow = index
			continue
		}
		if index != dividerRow {
			t.Fatalf("cursor %d put the dock on screen row %d, want %d", cursor, index, dividerRow)
		}
	}
}
