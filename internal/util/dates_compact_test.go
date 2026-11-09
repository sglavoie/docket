package util

import (
	"testing"
	"time"
)

// A capture sigil's value ends at the first space, so every spaced phrase the
// grammar reads needs a one-token spelling that resolves to the same day.
func TestCompactDateSpellingsMatchTheirSpacedPhrase(t *testing.T) {
	now := time.Date(2026, 11, 8, 15, 0, 0, 0, time.Local)
	pairs := map[string]string{
		"nov20":      "nov 20",
		"Nov20":      "nov 20",
		"20nov":      "20 nov",
		"nov-20":     "nov 20",
		"20-nov":     "20 nov",
		"next-week":  "next week",
		"next_week":  "next week",
		"next-fri":   "next fri",
		"in-2-weeks": "in 2 weeks",
		"this-sun":   "this sun",
		"jan3":       "jan 3",
	}
	for compact, spaced := range pairs {
		got, err := ParseCalendarDateAt(compact, now)
		if err != nil {
			t.Fatalf("%q: %v", compact, err)
		}
		want, err := ParseCalendarDateAt(spaced, now)
		if err != nil {
			t.Fatalf("%q: %v", spaced, err)
		}
		if !got.Equal(want) {
			t.Fatalf("%q resolved %v, want %v (as %q)", compact, got, want, spaced)
		}
	}
}

func TestCompactDateSpellingLeavesWrittenDatesAlone(t *testing.T) {
	now := time.Date(2026, 11, 8, 15, 0, 0, 0, time.Local)
	got, err := ParseCalendarDateAt("2026-11-20", now)
	if err != nil {
		t.Fatal(err)
	}
	if got.Year() != 2026 || got.Month() != time.November || got.Day() != 20 {
		t.Fatalf("written date moved: %v", got)
	}
	if _, err := ParseCalendarDateAt("nov99", now); err == nil {
		t.Fatal("nov99 should stay refused")
	}
}
