package chrome

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

// TestOverlayPlacementNamesTheCellThePopupDrew holds the placement a click is
// translated by to the compositing itself: the popup's own first character is
// drawn at exactly the reported cell, whether the margins beside it are wide
// enough to show the host or narrow enough to be flanked.
func TestOverlayPlacementNamesTheCellThePopupDrew(t *testing.T) {
	base := strings.TrimSuffix(strings.Repeat(strings.Repeat(".", 40)+"\n", 12), "\n")
	for _, popupWidth := range []int{10, 34} {
		popup := strings.TrimSuffix(strings.Repeat("X"+strings.Repeat("o", popupWidth-1)+"\n", 3), "\n")
		composed, placement := OverlayPopupDimmedPlaced(base, popup, 40, 12)
		lines := strings.Split(ansi.Strip(composed), "\n")
		if !placement.Whole || placement.Rows != 3 || placement.Cols != popupWidth {
			t.Fatalf("width %d: placement = %+v", popupWidth, placement)
		}
		if got := ansi.Cut(lines[placement.Row], placement.Col, placement.Col+1); got != "X" {
			t.Fatalf("width %d: cell %d,%d holds %q, want the popup's first cell", popupWidth, placement.Col, placement.Row, got)
		}
		if !placement.Contains(placement.Col, placement.Row) || placement.Contains(placement.Col-1, placement.Row) ||
			placement.Contains(placement.Col, placement.Row+3) {
			t.Fatalf("width %d: containment disagrees with %+v", popupWidth, placement)
		}
	}

	tall := strings.TrimSuffix(strings.Repeat("Xooo\n", 20), "\n")
	if _, placement := OverlayPopupDimmedPlaced(base, tall, 40, 12); placement.Whole {
		t.Fatal("a popup that lost interior rows to fitting is reported whole")
	}
}
