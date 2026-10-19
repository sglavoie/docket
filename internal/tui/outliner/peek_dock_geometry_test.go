package outliner

import (
	"fmt"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

// dockGeometryReviewSizes are the four terminals the refinement is reviewed at,
// spelled as terminals: the shell hands every view the terminal height less its
// footer, so the render below is sized to two rows fewer than each of these.
var dockGeometryReviewSizes = [][2]int{{60, 20}, {80, 24}, {100, 35}, {120, 40}}

// dockScreenRows reads the dock's coordinates back off a rendered frame rather
// than off a constant: the row its divider landed on and the row its position
// line landed on. The divider is found as the dock's own first line, the last
// rule of its kind on the frame, because a document that fits leaves the
// position row blank and so cannot be searched for. The view's block is stacked
// from the frame's first row, so these indices are the rows a reader counts down
// the screen. It returns the frame too, because every claim below is made about
// the same rendering these numbers came out of.
func dockScreenRows(t *testing.T, m *OutlinerModel, label string) (frame []string, divider, position int) {
	t.Helper()
	frame = strings.Split(ansi.Strip(m.View().Content), "\n")
	for index := range frame {
		frame[index] = strings.TrimRight(frame[index], " ")
	}
	dock := m.PeekDockLines()
	if len(dock) == 0 {
		t.Fatalf("%s: the model draws no Peek dock", label)
	}
	rule := strings.TrimRight(ansi.Strip(dock[0]), " ")
	if strings.TrimSpace(rule) == "" || strings.Trim(strings.TrimSpace(rule), "─") != "" {
		t.Fatalf("%s: the dock's first row is %q, want its divider", label, rule)
	}
	divider = -1
	for index, line := range frame {
		if line == rule {
			divider = index
		}
	}
	if divider < 0 {
		t.Fatalf("%s: the frame draws no Peek divider:\n%s", label, strings.Join(frame, "\n"))
	}
	position = divider + m.PeekDockRows() - 1
	if position >= len(frame) {
		t.Fatalf("%s: the divider is on screen row %d, too low to hold a %d-row dock:\n%s",
			label, divider, m.PeekDockRows(), strings.Join(frame, "\n"))
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
				if strings.Contains(line, emptyMessageLead(m.OutlinerEmptyMessage())) {
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

// emptyMessageLead is the explanation's first sentence, which is what a frame
// can be searched for: the whole explanation wraps at narrow widths, so its full
// text is never one row.
func emptyMessageLead(message string) string {
	if lead, _, ok := strings.Cut(message, ". "); ok {
		return lead + "."
	}
	return message
}
