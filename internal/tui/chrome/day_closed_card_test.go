package chrome

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

func TestDayClosedCardNamesOnlyTheDestinationsThatReceivedWork(t *testing.T) {
	cases := []struct {
		spec DayClosedCardSpec
		want string
	}{
		{DayClosedCardSpec{}, ""},
		{DayClosedCardSpec{Next: 2}, "2 → Next"},
		{DayClosedCardSpec{Backlog: 1, StillToday: 3}, "1 → Backlog · 3 still in Today"},
	}
	for _, c := range cases {
		if got := dayClosedSettled(c.spec); got != c.want {
			t.Errorf("dayClosedSettled(%+v) = %q, want %q", c.spec, got, c.want)
		}
	}
}

func TestDayClosedCardSendOffFollowsWhatTheDayHeld(t *testing.T) {
	done := []DayClosedDone{{Title: "Ship it"}}
	if got := dayClosedSendOff(DayClosedCardSpec{Done: done}); !strings.HasPrefix(got, "Today is clear") {
		t.Errorf("a finished day with nothing left open says %q", got)
	}
	if got := dayClosedSendOff(DayClosedCardSpec{Done: done, StillToday: 1}); !strings.HasPrefix(got, "Nice work today") {
		t.Errorf("a finished day with work left open says %q", got)
	}
	if got := dayClosedSendOff(DayClosedCardSpec{StillToday: 2}); strings.Contains(got, "work") {
		t.Errorf("a quiet day is not judged: %q", got)
	}
}

func TestDayClosedCardCountsPastFiveTitlesAndFitsTheFloor(t *testing.T) {
	spec := DayClosedCardSpec{Date: "Sunday, November 8"}
	for range 7 {
		spec.Done = append(spec.Done, DayClosedDone{Title: strings.Repeat("long title ", 8), ProjectSlug: "website"})
	}
	card := RenderDayClosedCard(spec, MinTerminalWidth)
	plain := ansi.Strip(card)
	if !strings.Contains(plain, "7 tasks done today") || !strings.Contains(plain, "+ 2 more") {
		t.Fatalf("the card does not count the titles it leaves out:\n%s", plain)
	}
	for _, line := range strings.Split(plain, "\n") {
		if w := lipgloss.Width(line); w > MinTerminalWidth {
			t.Fatalf("line is %d wide at the %d-column floor: %q", w, MinTerminalWidth, line)
		}
	}
}
