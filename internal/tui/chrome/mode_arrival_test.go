package chrome

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
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
		still := lipgloss.Width(OperationalTitleModeStripArriving(mode.ID, ModeArrivalNone, nil))
		for _, arrival := range []ModeArrival{ModeArrivalForward, ModeArrivalBackward, ModeArrivalJump} {
			if got := lipgloss.Width(OperationalTitleModeStripArriving(mode.ID, arrival, nil)); got != still {
				t.Errorf("%s arriving %d is %d wide, want %d", mode.ID, arrival, got, still)
			}
		}
	}
}

// A Tab neighbor carries its count only when it has one: a counted mode with
// nothing waiting reads as bare as a mode that is never counted.
func TestModeStripNamesNeighborCounts(t *testing.T) {
	modes := OutlinerTabModes()
	inboxIndex := -1
	for i, mode := range modes {
		if mode.ID == ModeInbox {
			inboxIndex = i
		}
	}
	if inboxIndex < 0 {
		t.Fatal("no Inbox mode in the Tab cycle")
	}
	previous := modes[(inboxIndex-1+len(modes))%len(modes)]
	strip := ansi.Strip(OperationalTitleModeStripArriving(previous.ID, ModeArrivalNone,
		map[OutlinerModeID]int{ModeInbox: 4}))
	if !strings.Contains(strip, "Inbox 4 ›") {
		t.Fatalf("strip = %q, want the Inbox neighbor counted", strip)
	}
	strip = ansi.Strip(OperationalTitleModeStripArriving(previous.ID, ModeArrivalNone,
		map[OutlinerModeID]int{ModeInbox: 0}))
	if !strings.Contains(strip, "Inbox ›") {
		t.Fatalf("strip = %q, want a zero count left bare", strip)
	}
}
