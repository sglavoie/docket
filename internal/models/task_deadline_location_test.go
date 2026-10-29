package models

import (
	"testing"
	"time"
)

func TestDateOnlyDeadlineUsesCalendarDatesAcrossZones(t *testing.T) {
	west := time.FixedZone("west", -10*3600)
	east := time.FixedZone("east", 14*3600)
	for _, zones := range []struct {
		name     string
		due, now *time.Location
	}{
		{"west to east", west, east},
		{"east to west", east, west},
	} {
		for _, day := range []int{19, 20, 21} {
			due := time.Date(2026, time.March, day, 0, 0, 0, 0, zones.due)
			now := time.Date(2026, time.March, 20, 12, 0, 0, 0, zones.now)
			task := &Task{WorkflowState: StateBacklog, DueDate: &due}
			if got, want := task.IsDueTodayAt(now), day == 20; got != want {
				t.Errorf("%s, due %s: due today = %t, want %t", zones.name, due, got, want)
			}
			if got, want := task.IsOverdueAt(now), day < 20; got != want {
				t.Errorf("%s, due %s: overdue = %t, want %t", zones.name, due, got, want)
			}
		}
	}
}

func TestTimedDeadlineKeepsInstantComparisonAcrossZones(t *testing.T) {
	due := time.Date(2026, time.March, 20, 9, 30, 0, 0, time.FixedZone("east", 14*3600))
	task := &Task{WorkflowState: StateBacklog, DueDate: &due}
	for _, delta := range []time.Duration{-time.Minute, 0, time.Minute} {
		now := due.Add(delta).In(time.FixedZone("west", -10*3600))
		if got, want := task.IsOverdueAt(now), delta > 0; got != want {
			t.Errorf("observation %s: overdue = %t, want %t", now, got, want)
		}
	}
}
