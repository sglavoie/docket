package models

import (
	"testing"
	"time"
)

func TestSetCheckInComparesCalendarDatesAcrossZones(t *testing.T) {
	for _, tc := range []struct {
		name    string
		day     int
		offset  int
		changed bool
	}{
		{"same date, different instants", 20, 14, false},
		{"different dates, same instant", 21, 14, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			task := checkInTask(t, StateBacklog)
			stored := time.Date(2026, time.March, 20, 0, 0, 0, 0, time.FixedZone("west", -10*3600))
			if err := task.SetCheckIn(CheckInSchedule{Next: &stored, RepeatDays: 7}, checkInClock); err != nil {
				t.Fatal(err)
			}
			day := time.Date(2026, time.March, tc.day, 0, 0, 0, 0, time.FixedZone("device", tc.offset*3600))
			later := checkInClock.Add(time.Hour)
			if err := task.SetCheckIn(CheckInSchedule{Next: &day, RepeatDays: 7}, later); err != nil {
				t.Fatal(err)
			}
			if got := task.UpdatedAt.Equal(later); got != tc.changed {
				t.Fatalf("schedule changed = %t, want %t", got, tc.changed)
			}
			if got := task.NextCheckIn.Day(); got != tc.day {
				t.Fatalf("check-in day = %d, want %d", got, tc.day)
			}
		})
	}
}
