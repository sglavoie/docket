package tui

import (
	"strings"
	"testing"
)

// outlineOfFour is one project holding four tasks, the cursor on the third.
func outlineOfFour(t *testing.T) *harness {
	h := start(t)
	h.press("o", `"Project"`, "enter", "tab", `"A"`, "enter", `"B"`, "enter", `"C"`, "enter", `"D"`, "esc", "k")
	h.want(
		"  ▾ Project",
		"    ○ A",
		"    ○ B",
		"▌   ○ C",
		"    ○ D",
	)
	return h
}

// ^ pins a row: it is drawn first among its siblings, the cursor goes with
// it, and unpinning puts it back where it was.
func TestPinFloatsARowToTheTopAndBack(t *testing.T) {
	h := outlineOfFour(t)
	h.press("^")
	h.want(
		"  ▾ Project",
		"▌   ○ C ^",
		"    ○ A",
		"    ○ B",
		"    ○ D",
	)
	// A second pin goes after the first, in the order of the outline.
	h.press("G", "^")
	h.want(
		"  ▾ Project",
		"    ○ C ^",
		"▌   ○ D ^",
		"    ○ A",
		"    ○ B",
	)
	h.press("k", "^")
	h.want(
		"  ▾ Project",
		"    ○ D ^",
		"    ○ A",
		"    ○ B",
		"▌   ○ C",
	)
	h.press("u")
	h.want(
		"  ▾ Project",
		"▌   ○ C ^",
		"    ○ D ^",
		"    ○ A",
		"    ○ B",
	)
	if !strings.Contains(h.m.msg, "pin changed") {
		t.Fatalf("undo says %q", h.m.msg)
	}
}

// J and K reorder inside a group and refuse to cross into the other.
func TestReorderStaysInsideItsGroup(t *testing.T) {
	h := outlineOfFour(t)
	// Pinned rows keep the outline's order among themselves.
	h.press("^", "j", "^") // C and A pinned, cursor on A
	h.want(
		"  ▾ Project",
		"▌   ○ A ^",
		"    ○ C ^",
		"    ○ B",
		"    ○ D",
	)
	h.press("J")
	h.want(
		"  ▾ Project",
		"    ○ C ^",
		"▌   ○ A ^",
		"    ○ B",
		"    ○ D",
	)
	h.press("J")
	if got := h.m.msg; !strings.Contains(got, "pinned rows stay above the rest: ^ unpins it") {
		t.Fatalf("msg = %q", got)
	}
	h.press("K")
	h.want(
		"  ▾ Project",
		"▌   ○ A ^",
		"    ○ C ^",
		"    ○ B",
		"    ○ D",
	)
	h.press("j", "j", "K")
	if got := h.m.msg; !strings.Contains(got, "pinned rows stay above the rest: ^ pins it") {
		t.Fatalf("msg = %q", got)
	}
	h.press("J")
	h.want(
		"  ▾ Project",
		"    ○ A ^",
		"    ○ C ^",
		"    ○ D",
		"▌   ○ B",
	)
}

// A count, a selection and the dot act on rows as for ! and x.
func TestPinTakesACountASelectionAndTheDot(t *testing.T) {
	h := outlineOfFour(t)
	h.press("k", "2", "^") // B and C
	h.want(
		"  ▾ Project",
		"▌   ○ B ^",
		"    ○ C ^",
		"    ○ A",
		"    ○ D",
	)
	h.press("G", ".")
	h.want(
		"  ▾ Project",
		"    ○ B ^",
		"    ○ C ^",
		"▌   ○ D ^",
		"    ○ A",
	)
	// A selection all pinned is unpinned together.
	h.press("g", "g", "j", "v", "v", "v", "^")
	h.want(
		"  ▾ Project",
		"▌   ○ A",
		" ┃  ○ B",
		" ┃  ○ C",
		" ┃  ○ D",
	)
}

// A row opened beside a pinned row is pinned too, so it is drawn where it
// was opened; its line need not say so.
func TestNewRowsTakeThePinOfWhereTheyOpen(t *testing.T) {
	h := outlineOfFour(t)
	h.press("^", "o", `"After C"`, "esc")
	h.want(
		"  ▾ Project",
		"    ○ C ^",
		"▌   ○ After C ^",
		"    ○ A",
		"    ○ B",
		"    ○ D",
	)
	h.press("j", "O", `"Before A"`, "esc")
	h.want(
		"  ▾ Project",
		"    ○ C ^",
		"    ○ After C ^",
		"▌   ○ Before A",
		"    ○ A",
		"    ○ B",
		"    ○ D",
	)
	// Deleting the sigil from the line unpins the row, and it goes back
	// where it was among the rest.
	h.press("k", "k", "A", "backspace", "backspace", "esc")
	h.want(
		"  ▾ Project",
		"    ○ After C ^",
		"    ○ Before A",
		"    ○ A",
		"    ○ B",
		"▌   ○ C",
		"    ○ D",
	)
}

// The stamp line sets and takes off the pin, and the filter finds pinned
// rows with $pinned.
func TestStampAndFilterReadThePin(t *testing.T) {
	h := outlineOfFour(t)
	h.press("#", `"work ^"`, "enter")
	h.want(
		"  ▾ Project",
		"▌   ○ C ^ #work",
		"    ○ A",
		"    ○ B",
		"    ○ D",
	)
	h.press("/", `"$pinned"`, "enter")
	h.want(
		"  ▾ Project",
		"▌   ○ C ^ #work",
	)
	h.press("esc", "-", `"^"`, "enter")
	h.want(
		"  ▾ Project",
		"    ○ A",
		"    ○ B",
		"▌   ○ C #work",
		"    ○ D",
	)
}

// A copy carries the pin, and a put row lands in its own group.
func TestPutKeepsThePin(t *testing.T) {
	h := outlineOfFour(t)
	h.press("^", "y", "y", "G", "p")
	h.want(
		"  ▾ Project",
		"    ○ C ^",
		"▌   ○ C ^",
		"    ○ A",
		"    ○ B",
		"    ○ D",
	)
}

// :sort orders each group on its own; the pinned stay first.
func TestSortKeepsPinnedRowsFirst(t *testing.T) {
	h := outlineOfFour(t)
	h.press("G", "^", ":", `"sort"`, "enter")
	h.want(
		"  ▾ Project",
		"▌   ○ D ^",
		"    ○ A",
		"    ○ B",
		"    ○ C",
	)
}

// Pinned projects lead the sidebar's active projects.
func TestPinnedProjectsLeadTheSidebar(t *testing.T) {
	h := start(t).wide()
	h.press("o", `"Alpha"`, "enter", `"Beta"`, "esc", "^")
	side := h.side()
	if a, b := strings.Index(side, "Alpha"), strings.Index(side, "Beta"); a < 0 || b < 0 || b > a {
		t.Fatalf("sidebar:\n%s", side)
	}
}
