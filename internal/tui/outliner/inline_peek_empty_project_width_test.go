package outliner

import (
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/docket/internal/models"
)

// emptyProjectPeekSentinel ends the fixture's comment. It is one unbreakable
// token, so no wrap can split it across two rows and the assertion below names
// the document's last row rather than whichever row a walk happened to stop on.
const emptyProjectPeekSentinel = "final-sentinel-row"

// emptyPeekProject is the state this regression is about: a project with no
// tasks at all, so its projection is the empty state — the one render path that
// never reaches ComputeOutlinerViewport — carrying one comment long enough that
// the rows it wraps to at 60 columns are not the rows it wraps to at 120.
func emptyPeekProject(id string) *models.Project {
	project := primaryRowsProject(id)
	project.Name = "Empty Platform"
	project.Comments = []*models.ProjectComment{
		peekProjectComment(0, strings.Repeat(
			"The vendor contract renewal still needs the finance sign-off before the migration window closes. ",
			5,
		)+emptyProjectPeekSentinel),
	}
	return project
}

// emptyProjectPeekOutliner parks the cursor on a task-empty project's focused
// header with Peek open. populated decides whether a projection holding rows was
// drawn first, which is what leaves LastViewport naming another frame's width
// while the empty state below it draws at the full one.
func emptyProjectPeekOutliner(t *testing.T, width, height int, populated bool) *OutlinerModel {
	t.Helper()
	empty := emptyPeekProject("empty")
	outliner := scopeTestOutliner(focusedHeaderProject("alpha"), empty)
	outliner.SetSize(width, height)
	if populated {
		outliner.View()
		if !outliner.HasContentRows() {
			t.Fatal("the populated projection drew no content rows, so LastViewport holds no width")
		}
	}

	outliner.PushProjectScope(empty.ID)
	outliner.RebuildRows()
	if outliner.HasContentRows() {
		t.Fatal("the scoped projection holds content rows, so it is not the empty state")
	}
	if !moveCursorToProjectHeader(outliner, empty.ID, 0) {
		t.Fatal("the empty projection built no header for its project")
	}
	if row := outliner.Rows[outliner.Cursor]; !row.IsFocusedProjectHeader {
		t.Fatalf("cursor row = %+v, want the focused project header", row)
	}
	outliner.PeekMode = true
	return outliner
}

// emptyProjectPeekFrame is the frame a reader is looking at: the real View path,
// stripped of color and of the padding renderFullWidth pays every line. The
// assertions read the window out of here rather than measuring a width, because
// a document assembled against another width does not draw the rows it claims
// are on screen.
func emptyProjectPeekFrame(t *testing.T, m *OutlinerModel, width int, label string) []string {
	t.Helper()
	rows := strings.Split(ansi.Strip(m.View().Content), "\n")
	for i, row := range rows {
		if got := len([]rune(row)); got > width {
			t.Fatalf("%s: frame row %d is %d columns, want <= %d: %q", label, i, got, width, row)
		}
		rows[i] = strings.TrimRight(row, " ")
	}
	return rows
}

// emptyProjectPeekCounter reads the counter row Peek spends one of its five rows
// on. It is the render's own statement of how much detail is still unread, which
// is what makes "the walk reached the end" a claim about the screen rather than
// about an offset only the keys can see.
var emptyProjectPeekCounter = regexp.MustCompile(`^(\d+) above · (\d+) below$`)

// assertEmptyProjectPeekIsDrawn holds one stop of a walk to the frame: every row
// of the window the scrolling keys act on must be a row the render drew. It
// returns the counts the rendered counter row named.
func assertEmptyProjectPeekIsDrawn(t *testing.T, m *OutlinerModel, width int, label string) (above, below int) {
	t.Helper()
	row, doc, ok := m.peekCursorDocument()
	if !ok {
		t.Fatalf("%s: the cursor names no Peek document", label)
	}
	frame := emptyProjectPeekFrame(t, m, width, label)
	for number, line := range doc.Lines(m.peekOffsetForRow(row, doc)) {
		want := strings.TrimRight(ansi.Strip(line), " ")
		if !slices.Contains(frame, want) {
			t.Fatalf("%s: window row %d is not on screen: %q\nframe:\n%s",
				label, number, want, strings.Join(frame, "\n"))
		}
	}

	for _, line := range frame {
		match := emptyProjectPeekCounter.FindStringSubmatch(strings.TrimSpace(line))
		if match == nil {
			continue
		}
		above, _ = strconv.Atoi(match[1])
		below, _ = strconv.Atoi(match[2])
		return above, below
	}
	t.Fatalf("%s: the frame drew no Peek counter row:\n%s", label, strings.Join(frame, "\n"))
	return 0, 0
}

// walkEmptyProjectPeek pages to one end of the document, alternating the two
// distances so a reader who mixes them is covered rather than one who commits to
// a single key. Every stop is held to the frame, and the stop that cannot move
// must be the end the render is showing.
func walkEmptyProjectPeek(t *testing.T, m *OutlinerModel, width, direction int, label string) {
	t.Helper()
	const maxSteps = 200
	for step := 0; ; step++ {
		if step > maxSteps {
			t.Fatalf("%s: the walk did not settle in %d steps", label, maxSteps)
		}
		above, below := assertEmptyProjectPeekIsDrawn(t, m, width, fmt.Sprintf("%s step %d", label, step))

		// One move per stop, alternating the distances. Calling both would spend
		// five content rows on an odd step and read the counter a step behind the
		// window, which turns "the walk reached the end" into an accident of how
		// many rows the document happened to hold.
		move := m.HalfPagePeek
		if step%2 == 1 {
			move = m.PagePeek
		}
		if move(direction) {
			continue
		}
		remaining := below
		if direction < 0 {
			remaining = above
		}
		if remaining != 0 {
			t.Fatalf("%s: scrolling stopped with %d rows still unread in direction %d",
				label, remaining, direction)
		}
		return
	}
}

// TestEmptyProjectPeekScrollUsesRenderedWidth is the empty projection's width
// contract. renderOutlinerEmptyState returns before ComputeOutlinerViewport, so
// LastViewport names some other frame — a wider populated one, or none at all —
// while the header and its Peek are drawn at the full current width. A document
// the keys built against that stale width has a different number of body rows
// than the document on screen, which either stops the walk before the last
// comment row or lets the offset run past the end the render can show.
func TestEmptyProjectPeekScrollUsesRenderedWidth(t *testing.T) {
	const (
		wide   = 120
		narrow = 60
		height = 24
	)

	for _, test := range []struct {
		name    string
		prepare func(*testing.T) *OutlinerModel
	}{
		{
			// The stale width comes from another projection entirely: a populated
			// outline drew and measured at 120, then the scope changed and the
			// terminal shrank, and nothing has written LastViewport since.
			name: "populated_to_empty_resize",
			prepare: func(t *testing.T) *OutlinerModel {
				t.Helper()
				outliner := emptyProjectPeekOutliner(t, wide, height, true)
				outliner.SetSize(narrow, height)
				return outliner
			},
		},
		{
			// No frame with rows was ever drawn, so LastViewport holds nothing and
			// the pre-viewport fallback is what answers — a fallback that subtracts
			// a scrollbar column the empty state never reserves.
			name: "fresh_empty",
			prepare: func(t *testing.T) *OutlinerModel {
				t.Helper()
				return emptyProjectPeekOutliner(t, narrow, height, false)
			},
		},
		{
			// The reader is already part-way down the pane when the terminal
			// changes. The resize invalidates the wide document's offset; the
			// document the keys resume against has to be the narrow one on screen.
			name: "resize_while_empty",
			prepare: func(t *testing.T) *OutlinerModel {
				t.Helper()
				outliner := emptyProjectPeekOutliner(t, wide, height, true)
				assertEmptyProjectPeekIsDrawn(t, outliner, wide, "resize_while_empty before the resize")
				if !outliner.HalfPagePeek(1) {
					t.Fatal("the fixture would not scroll at 120 columns")
				}
				outliner.SetSize(narrow, height)
				return outliner
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			outliner := test.prepare(t)

			frame := emptyProjectPeekFrame(t, outliner, narrow, test.name)
			joined := strings.Join(frame, "\n")
			if want := emptyMessageLead(outliner.OutlinerEmptyMessage()); !strings.Contains(joined, want) {
				t.Fatalf("%s: the frame is not the empty state (%q):\n%s", test.name, want, joined)
			}
			if _, doc, ok := outliner.peekCursorDocument(); !ok || !doc.Paged() {
				t.Fatalf("%s: the fixture fit one window, so nothing pages", test.name)
			}
			if strings.Contains(joined, emptyProjectPeekSentinel) {
				t.Fatalf("%s: the last comment row is already on screen, so the walk proves nothing:\n%s",
					test.name, joined)
			}

			walkEmptyProjectPeek(t, outliner, narrow, 1, test.name+" down")

			// The end of the document, said three ways: the keys refuse, the grammar
			// offers only the way back, and the rendered counter agrees with both.
			if outliner.CanScrollPeek(1) {
				t.Fatalf("%s: the walk stopped with room left below", test.name)
			}
			if !outliner.CanScrollPeek(-1) {
				t.Fatalf("%s: the document never moved at all", test.name)
			}
			if peek := outliner.PeekCommandContext(); !peek.Visible || peek.CanPageDown || !peek.CanPageUp {
				t.Fatalf("%s: end context = %+v, want a visible pane offering the way back alone", test.name, peek)
			}
			above, below := assertEmptyProjectPeekIsDrawn(t, outliner, narrow, test.name+" at the end")
			if below != 0 || above == 0 {
				t.Fatalf("%s: the end counter reads %d above · %d below", test.name, above, below)
			}
			end := strings.Join(emptyProjectPeekFrame(t, outliner, narrow, test.name), "\n")
			if !strings.Contains(end, emptyProjectPeekSentinel) {
				t.Fatalf("%s: the last comment row is unreachable:\n%s", test.name, end)
			}

			// And back, on the same mixed sequence, to the top of the same document.
			walkEmptyProjectPeek(t, outliner, narrow, -1, test.name+" up")
			if outliner.CanScrollPeek(-1) {
				t.Fatalf("%s: the return stopped with room left above", test.name)
			}
			row, doc, ok := outliner.peekCursorDocument()
			if !ok {
				t.Fatalf("%s: the cursor lost its Peek document", test.name)
			}
			if got := outliner.peekOffsetForRow(row, doc); got != 0 {
				t.Fatalf("%s: the return settled at offset %d, want the top", test.name, got)
			}
		})
	}
}
