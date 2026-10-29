package engine

import (
	"testing"
	"time"

	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/util"
)

func TestDayDeferralReminderSurvivesReplanningAfterItsWake(t *testing.T) {
	for _, tc := range []struct {
		name, zone string
		month      time.Month
		day        int
	}{
		{"ordinary midnight", "America/Toronto", time.October, 9},
		{"skipped midnight", "America/Havana", time.March, 8},
	} {
		t.Run(tc.name, func(t *testing.T) {
			loc, err := time.LoadLocation(tc.zone)
			if err != nil {
				t.Fatal(err)
			}
			wake, err := util.StartOfCalendarDate(2026, tc.month, tc.day, loc)
			if err != nil {
				t.Fatal(err)
			}
			fireAt := time.Date(2026, tc.month, tc.day, 9, 0, 0, 0, loc)
			task := &models.Task{ID: "task", WorkflowState: models.StateBacklog, DeferredUntil: &wake}
			projects := []*models.Project{{ID: "project", Tasks: []*models.Task{task}}}
			for _, now := range []time.Time{wake.Add(-time.Hour), wake, fireAt.Add(-time.Hour)} {
				plan := PlanReminders(projects, ReminderOptions{Now: now, Location: loc, CheckInHour: 9})
				if len(plan.Reminders) != 1 {
					t.Fatalf("at %s: reminders = %+v, want the morning reminder", now, plan.Reminders)
				}
				got := plan.Reminders[0]
				if got.Kind != ReminderDeferralEnds || !got.FireAt.Equal(fireAt) {
					t.Fatalf("at %s: reminder = %+v, want deferral_ends at %s", now, got, fireAt)
				}
			}
			for _, now := range []time.Time{fireAt, fireAt.Add(time.Hour), fireAt.AddDate(0, 0, 1)} {
				if plan := PlanReminders(projects, ReminderOptions{Now: now, Location: loc, CheckInHour: 9}); len(plan.Reminders) != 0 {
					t.Fatalf("at %s: past reminder was replayed: %+v", now, plan.Reminders)
				}
			}
			// An hour-bearing deferral keeps its instant, including after it lapses.
			timedWake := time.Date(2026, tc.month, tc.day, 6, 0, 0, 0, loc)
			task.DeferredUntil = &timedWake
			if plan := PlanReminders(projects, ReminderOptions{Now: fireAt.Add(-time.Hour), Location: loc, CheckInHour: 9}); len(plan.Reminders) != 0 {
				t.Fatalf("expired timed deferral was replayed: %+v", plan.Reminders)
			}
		})
	}
}
