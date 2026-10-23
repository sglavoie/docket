package models

import (
	"testing"
	"time"
)

// TestDueHasTimeReadsTheStartOfADateWithNoMidnight holds the date-only signal
// to the start of the date rather than to a literal 00:00: America/Havana
// starts 2026-03-08 at 01:00, so a deadline there is date-only, while any later
// clock that day is still a time.
func TestDueHasTimeReadsTheStartOfADateWithNoMidnight(t *testing.T) {
	havana, err := time.LoadLocation("America/Havana")
	if err != nil {
		t.Fatalf("LoadLocation: %v", err)
	}
	start := time.Date(2026, 3, 8, 1, 0, 0, 0, havana)
	if (&Task{DueDate: &start}).DueHasTime() {
		t.Fatal("the start of a date with no midnight read as a timed deadline")
	}
	later := time.Date(2026, 3, 8, 2, 0, 0, 0, havana)
	if !(&Task{DueDate: &later}).DueHasTime() {
		t.Fatal("02:00 on that date lost its clock")
	}
}
