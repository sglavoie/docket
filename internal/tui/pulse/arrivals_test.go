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
