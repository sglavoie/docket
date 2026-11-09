package chrome

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"
)

// The closing card reads the half year back as an activity grid when there is
// history, and draws none without it.
func TestDayClosedCardDrawsTheActivityGrid(t *testing.T) {
	now := time.Date(2026, 11, 8, 18, 0, 0, 0, time.Local)
	spec := DayClosedCardSpec{
		Date: "Sunday, November 8", Done: []DayClosedDone{{Title: "Ship it", ProjectSlug: "web"}},
		History: map[time.Time]int{now: 1, now.AddDate(0, 0, -1): 3}, Now: now,
	}
	plain := ansi.Strip(RenderDayClosedCard(spec, 100))
	for _, want := range []string{"Mon", "less", "more", "4 done in 25 weeks", "2-day streak"} {
		if !strings.Contains(plain, want) {
			t.Fatalf("card lacks %q:\n%s", want, plain)
		}
	}
	spec.History = nil
	if plain := ansi.Strip(RenderDayClosedCard(spec, 100)); strings.Contains(plain, "less") {
		t.Fatalf("a card without history drew a grid:\n%s", plain)
	}
}
