package chrome

import (
	"testing"

	"charm.land/lipgloss/v2"
)

func TestModeArrivalBetweenNamesTheTabDirection(t *testing.T) {
	modes := OutlinerTabModes()
	if len(modes) < 3 {
		t.Skip("needs three tab modes")
	}
	first, second, last := modes[0].ID, modes[1].ID, modes[len(modes)-1].ID
	cases := []struct {
		from, to OutlinerModeID
		want     ModeArrival
	}{
		{first, first, ModeArrivalNone},
		{first, second, ModeArrivalForward},
		{second, first, ModeArrivalBackward},
		{last, first, ModeArrivalForward},
		{first, last, ModeArrivalBackward},
	}
	for _, c := range cases {
		if got := ModeArrivalBetween(c.from, c.to); got != c.want {
			t.Errorf("ModeArrivalBetween(%s, %s) = %d, want %d", c.from, c.to, got, c.want)
		}
	}
}

// Arriving changes paint, never width, so the title fits exactly as it did.
func TestModeStripArrivingKeepsItsWidth(t *testing.T) {
	for _, mode := range OutlinerTabModes() {
		still := lipgloss.Width(OperationalTitleModeStripArriving(mode.ID, ModeArrivalNone))
		for _, arrival := range []ModeArrival{ModeArrivalForward, ModeArrivalBackward, ModeArrivalJump} {
			if got := lipgloss.Width(OperationalTitleModeStripArriving(mode.ID, arrival)); got != still {
				t.Errorf("%s arriving %d is %d wide, want %d", mode.ID, arrival, got, still)
			}
		}
	}
}
