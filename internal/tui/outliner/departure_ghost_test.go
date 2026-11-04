package outliner

import "testing"

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
