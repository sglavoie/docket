package outliner

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/models"
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
// The dock is a fixed block, so the rows it draws and the rows it costs the list
// are one constant under a one-line task, under a document forty rows long, and
// under a row that owns no document at all.
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
		for _, cursor := range cursors {
			outliner.Cursor = cursor
			outliner.View()
			lines := outliner.PeekDockLines()
			if len(lines) != PeekDockRows {
				t.Fatalf("%dx%d cursor %d drew %d dock rows, want %d",
					size[0], size[1], cursor, len(lines), PeekDockRows)
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
// The dock plus its guard row comes off the list once, and it is the same
// subtraction whatever the cursor rests on, so the list moves when Peek opens
// and never again while it is open.
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
	if want := closed - PeekDockRows - 1; open != want {
		t.Fatalf("an open dock left the list %d rows, want %d (%d - %d - 1 guard row)",
			open, want, closed, PeekDockRows)
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
	if len(lines) != PeekDockRows {
		t.Fatalf("a row with no document drew %d dock rows, want %d", len(lines), PeekDockRows)
	}
	body := ansi.Strip(strings.Join(lines, "\n"))
	if !strings.Contains(body, peekDockPlaceholderText) {
		t.Fatalf("the dock over a row with no document reads %q, want the placeholder", body)
	}
	if peek := outliner.PeekCommandContext(); !peek.Visible || peek.CanPageDown || peek.CanPageUp {
		t.Fatalf("placeholder peek context = %+v, want visible with nothing to page", peek)
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
