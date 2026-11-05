package outliner

import (
	"regexp"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

// A line the render lent the departure ghost names no row, and every row
// below it is read one line lower, by the cursor's screen row and by a click.
func TestDepartureGhostLineShiftsScreenRowReaders(t *testing.T) {
	m := &OutlinerModel{Rows: make([]OutlinerRow, 5)}
	m.LastViewport = outlinerViewport{VisibleRows: 6, startIdx: 0, endIdx: 5, rowWidth: 40, topRow: 2,
		ghostActive: true, ghostOffset: 2}
	m.pointerFrame = outlinerPointerFrame{rows: true}

	m.Cursor = 1
	if row, ok := m.CursorScreenRow(); !ok || row != 3 {
		t.Fatalf("cursor above the ghost: row %d %v, want 3", row, ok)
	}
	m.Cursor = 2
	if row, ok := m.CursorScreenRow(); !ok || row != 5 {
		t.Fatalf("cursor below the ghost: row %d %v, want 5", row, ok)
	}
	if _, ok := m.pointerHitAt(0, 4); ok {
		t.Fatal("a click on the ghost line hit a row")
	}
	if hit, ok := m.pointerHitAt(0, 5); !ok || hit.row != 2 {
		t.Fatalf("click below the ghost = %+v %v, want row 2", hit, ok)
	}
	if hit, ok := m.pointerHitAt(0, 3); !ok || hit.row != 1 {
		t.Fatalf("click above the ghost = %+v %v, want row 1", hit, ok)
	}
}

// A full window lends the ghost its last row's line, except where that row is
// the cursor's.
func TestDepartureGhostSlotNeverHidesTheCursor(t *testing.T) {
	m := &OutlinerModel{Rows: make([]OutlinerRow, 10), ghost: &departureGhost{screenOffset: 1}}
	full := outlinerViewport{VisibleRows: 4, startIdx: 0, endIdx: 4}
	m.Cursor = 1
	if offset, dropLast, ok := m.ghostSlot(full); !ok || !dropLast || offset != 1 {
		t.Fatalf("full window slot = %d %v %v, want 1 true true", offset, dropLast, ok)
	}
	m.Cursor = 3
	if _, _, ok := m.ghostSlot(full); ok {
		t.Fatal("ghost took the cursor's line")
	}
	short := outlinerViewport{VisibleRows: 6, startIdx: 0, endIdx: 4}
	if offset, dropLast, ok := m.ghostSlot(short); !ok || dropLast || offset != 1 {
		t.Fatalf("short window slot = %d %v %v, want 1 false true", offset, dropLast, ok)
	}
}

// A completion lands its check before the strike: the first beat leaves the
// title unstruck with the check in bold, the second strikes the row, and the
// third fades it. Any other departure starts at the strike.
func TestCompletionGhostLandsTheCheckBeforeTheStrike(t *testing.T) {
	struck := regexp.MustCompile(`\x1b\[(?:[0-9:;]*;)?9(?:;[0-9:;]*)?m`)
	ghost := &departureGhost{line: "  ✓   Plan the weekend walk", done: true, icon: "✓"}
	m := &OutlinerModel{ghost: ghost}

	first := m.renderDepartureGhost(40)
	if struck.MatchString(first) {
		t.Fatalf("the check beat struck the row: %q", first)
	}
	if got := ansi.Strip(first); !strings.HasPrefix(got, "  ✓   Plan the weekend walk") || ansi.StringWidth(got) != 40 {
		t.Fatalf("the check beat drew %q", got)
	}
	if want := len(completionGhostSchedule); want != 3 {
		t.Fatalf("a completion paints %d phases, want 3", want)
	}
	ghost.phase = 1
	if second := m.renderDepartureGhost(40); !struck.MatchString(second) {
		t.Fatalf("the second beat left the row unstruck: %q", second)
	}

	other := &departureGhost{line: "  □   Moved away"}
	m.ghost = other
	if look := other.look(); look != ghostStruck {
		t.Fatalf("a non-completion starts at look %d, want the strike", look)
	}
	if len(other.schedule()) != 2 {
		t.Fatalf("a non-completion paints %d phases, want 2", len(other.schedule()))
	}
}
