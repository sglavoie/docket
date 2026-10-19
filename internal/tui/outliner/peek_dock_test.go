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
// reviewed terminals. The height is
// a quarter of what this view was handed, bounded at both ends, so a taller
// terminal reads more of a document without a shorter one losing its list.
func TestPeekDockHeightIsAShareOfTheFrame(t *testing.T) {
	for _, test := range []struct {
		width, height int
		want          int
	}{
		// The floor binds at the two short terminals: a quarter of 18 and of 22 rows
		// is under six, so both take the minimum readable dock.
		{width: 60, height: 20, want: peekDockPreferredMinRows},
		{width: 80, height: 24, want: peekDockPreferredMinRows},
		// A quarter of 33 rows: six content lines plus divider and position row.
		{width: 100, height: 35, want: 8},
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
