package pulse

import (
	"testing"
	"time"

	"github.com/sglavoie/docket/internal/models"
)

func arrivalTask(title string, state models.WorkflowState) *models.Task {
	return &models.Task{ID: title, Title: title, WorkflowState: state}
}

// TestArrivalsReportOnlyWhatCrossedTheWindow pins the three reasons a task
// arrives and the cases that must stay quiet: a check-in already standing at
// the window's start, a date-only deadline, a deferral still running, and
// terminal work.
func TestArrivalsReportOnlyWhatCrossedTheWindow(t *testing.T) {
	since := time.Date(2040, time.March, 4, 23, 59, 0, 0, time.Local)
	now := since.Add(2 * time.Minute) // past local midnight
	today := time.Date(2040, time.March, 5, 0, 0, 0, 0, time.Local)
	yesterday := today.AddDate(0, 0, -1)
	timedDue := since.Add(90 * time.Second) // 00:00:30, so it carries a time
	wake := since.Add(30 * time.Second)
	stillDeferred := now.Add(time.Hour)

	checkIn := arrivalTask("check-in", models.StateWaiting)
	checkIn.NextCheckIn = &today
	standing := arrivalTask("standing", models.StateWaiting)
	standing.NextCheckIn = &yesterday
	deadline := arrivalTask("deadline", models.StateDoing)
	deadline.DueDate = &timedDue
	dateOnly := arrivalTask("date-only", models.StateDoing)
	dateOnly.DueDate = &yesterday
	woke := arrivalTask("woke", models.StateBacklog)
	woke.DeferredUntil = &wake
	asleep := arrivalTask("asleep", models.StateBacklog)
	asleep.DeferredUntil = &stillDeferred
	asleep.NextCheckIn = &today
	done := arrivalTask("done", models.StateDone)
	done.NextCheckIn = &today

	project := &models.Project{ID: "p", Tasks: []*models.Task{checkIn, standing, deadline, dateOnly, woke, asleep, done}}
	got := Arrivals([]*models.Project{project, nil}, since, now)
	want := []Arrival{
		{Kind: ArrivalCheckIn, ProjectID: "p", TaskID: "check-in", Title: "check-in", Reviewable: true},
		{Kind: ArrivalDeadline, ProjectID: "p", TaskID: "deadline", Title: "deadline"},
		{Kind: ArrivalWake, ProjectID: "p", TaskID: "woke", Title: "woke"},
	}
	if len(got) != len(want) {
		t.Fatalf("arrivals = %+v, want %+v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("arrival %d = %+v, want %+v", i, got[i], want[i])
		}
	}
}

// TestUpcomingArrivalsAreWhatATickWillReport: each upcoming arrival is the
// one Arrivals reports over the window its instant opens, so a deferral that
// suppresses a check-in moves it to the wake, a deadline inside a deferral is
// not its own arrival, and work outside the horizon, already standing, done,
// or hidden is never booked.
func TestUpcomingArrivalsAreWhatATickWillReport(t *testing.T) {
	now := time.Date(2040, time.March, 4, 15, 0, 0, 0, time.Local)
	tomorrow := time.Date(2040, time.March, 5, 0, 0, 0, 0, time.Local)
	today := time.Date(2040, time.March, 4, 0, 0, 0, 0, time.Local)
	inAnHour := now.Add(time.Hour)
	inTwoHours := now.Add(2 * time.Hour)
	tomorrowNoon := tomorrow.Add(12 * time.Hour)
	farAway := now.Add(UpcomingHorizon + time.Hour)

	checkIn := arrivalTask("check-in", models.StateWaiting)
	checkIn.NextCheckIn = &tomorrow
	standing := arrivalTask("standing", models.StateWaiting)
	standing.NextCheckIn = &today
	deadline := arrivalTask("deadline", models.StateDoing)
	deadline.DueDate = &inAnHour
	dateOnly := arrivalTask("date-only", models.StateDoing)
	dateOnly.DueDate = &tomorrow
	// Deferred past its own check-in day: it arrives once, at its wake, as a check-in.
	deferredCheckIn := arrivalTask("deferred-check-in", models.StateBacklog)
	deferredCheckIn.NextCheckIn = &tomorrow
	deferredCheckIn.DeferredUntil = &tomorrowNoon
	// A deadline inside its deferral is not a moment of its own.
	deferredDeadline := arrivalTask("deferred-deadline", models.StateBacklog)
	deferredDeadline.DueDate = &inAnHour
	deferredDeadline.DeferredUntil = &inTwoHours
	tooFar := arrivalTask("too-far", models.StateBacklog)
	tooFar.DeferredUntil = &farAway
	done := arrivalTask("done", models.StateDone)
	done.DueDate = &inAnHour
	hidden := arrivalTask("hidden", models.StateDoing)
	hidden.DueDate = &inAnHour
	hidden.Hidden = true

	project := &models.Project{ID: "p", Tasks: []*models.Task{
		checkIn, standing, deadline, dateOnly, deferredCheckIn, deferredDeadline, tooFar, done, hidden,
	}}
	got := UpcomingArrivals([]*models.Project{project, nil}, now, UpcomingHorizon, UpcomingLimit)
	want := []UpcomingArrival{
		{Arrival: Arrival{Kind: ArrivalDeadline, ProjectID: "p", TaskID: "deadline", Title: "deadline"}, At: inAnHour},
		{Arrival: Arrival{Kind: ArrivalWake, ProjectID: "p", TaskID: "deferred-deadline", Title: "deferred-deadline"}, At: inTwoHours},
		{Arrival: Arrival{Kind: ArrivalCheckIn, ProjectID: "p", TaskID: "check-in", Title: "check-in", Reviewable: true}, At: tomorrow},
		{Arrival: Arrival{Kind: ArrivalCheckIn, ProjectID: "p", TaskID: "deferred-check-in", Title: "deferred-check-in", Reviewable: true}, At: tomorrowNoon},
	}
	if !EqualUpcoming(got, want) {
		t.Fatalf("upcoming = %+v\nwant %+v", got, want)
	}

	// Each booking is exactly what the tick crossing its instant reports.
	for _, upcoming := range want {
		arrivals := Arrivals([]*models.Project{project}, upcoming.At.Add(-time.Second), upcoming.At.Add(time.Second))
		found := false
		for _, arrival := range arrivals {
			found = found || arrival == upcoming.Arrival
		}
		if !found {
			t.Fatalf("a tick over %s reported %+v, not the booked %+v", upcoming.At, arrivals, upcoming.Arrival)
		}
	}

	if limited := UpcomingArrivals([]*models.Project{project}, now, UpcomingHorizon, 2); !EqualUpcoming(limited, want[:2]) {
		t.Fatalf("limited = %+v, want the earliest two", limited)
	}
}

func TestArrivalNotificationWording(t *testing.T) {
	title, body := ArrivalNotification([]Arrival{{Kind: ArrivalCheckIn, Title: "Call the bank"}})
	if title != "Docket · check-in" || body != "Call the bank" {
		t.Fatalf("single = %q / %q", title, body)
	}
	title, body = ArrivalNotification([]Arrival{
		{Kind: ArrivalCheckIn}, {Kind: ArrivalCheckIn}, {Kind: ArrivalWake},
	})
	if title != "Docket" || body != "2 check-ins · 1 deferral ended" {
		t.Fatalf("several = %q / %q", title, body)
	}
}

// TestDesktopNotificationEscapesItsDelimiters keeps a task title from ending
// the sequence early or splitting the title field.
func TestDesktopNotificationEscapesItsDelimiters(t *testing.T) {
	got := DesktopNotification("a;b", "line\x07one\x1b]two")
	want := "\x1b]777;notify;a,b;line one ]two\x07"
	if got != want {
		t.Fatalf("escape = %q, want %q", got, want)
	}
}
