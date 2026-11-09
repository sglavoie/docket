package outliner

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/tui/chrome"
	"github.com/sglavoie/docket/internal/tui/operation"
)

// Review the Week queues the work gone quiet, most pressing first, names why
// each card is there, routes through the row's own operations, and ends on a
// card that reads the week back.
func TestWeekReviewQueuesQuietWorkAndEndsOnTheWeek(t *testing.T) {
	now := time.Now()
	stuck := doingTask("stuck", "Stuck migration", now.AddDate(0, 0, -10))
	waiting := primaryRowsTask("waiting", models.StateWaiting)
	waiting.Title = "Vendor reply"
	waiting.StateChangedAt = now.AddDate(0, 0, -9)
	fresh := doingTask("fresh", "Fresh work", now.Add(-time.Hour))
	idle := primaryRowsTask("idle", models.StateInbox)
	m := nowCardOutliner(t, 110, 40, stuck, waiting, fresh, idle)
	m.SelectOutlinerMode(chrome.ModeInbox)

	if _, refusal := m.OpenWeekReview(); refusal != "" {
		t.Fatal(refusal)
	}
	if m.ActiveOutlinerMode() != chrome.ModeAll {
		t.Fatalf("the review reads %s, want All", m.ActiveOutlinerMode())
	}
	if got := len(m.Triage.queue); got != 2 {
		t.Fatalf("queued %d tasks, want the stale Doing and the quiet Waiting", got)
	}
	if m.Triage.queue[0].TaskID != "stuck" || m.Triage.queue[1].TaskID != "waiting" {
		t.Fatalf("queue order = %v, want Doing before Waiting", m.Triage.queue)
	}
	host := strings.Repeat(strings.Repeat(" ", 110)+"\n", 39) + strings.Repeat(" ", 110)
	card, ok := m.composeTriageCard(host, 110, 40)
	plain := ansi.Strip(card)
	if !ok || !strings.Contains(plain, "Review the week") || !strings.Contains(plain, "In Doing for 10 days") {
		t.Fatalf("first card:\n%s", plain)
	}

	_, cmd := m.handleTriageKey("c")
	if cmd == nil {
		t.Fatal("c routed nothing")
	}
	if route, _ := cmd().(TriageRouteMsg); route.OperationID != operation.OperationTaskSetState || route.Value != models.StateCanceled {
		t.Fatalf("c routed %+v, want Set Status to Canceled", route)
	}
	m.handleTriageKey("space")
	if !m.Triage.closed {
		t.Fatal("the pass did not stand on its closing card after the last card")
	}
	card, _ = m.composeTriageCard(host, 110, 40)
	plain = ansi.Strip(card)
	for _, want := range []string{"Week reviewed", "2 reviewed: 1 canceled · 1 kept"} {
		if !strings.Contains(plain, want) {
			t.Fatalf("closing card lacks %q:\n%s", want, plain)
		}
	}
	m.handleTriageKey("enter")
	if m.Triage.Active {
		t.Fatal("Enter left the closing card open")
	}
}

// A week with nothing gone quiet goes straight to its closing card.
func TestWeekReviewWithNothingQuietOpensOnTheClosingCard(t *testing.T) {
	m := nowCardOutliner(t, 100, 30, doingTask("fresh", "Fresh", time.Now()))
	if _, refusal := m.OpenWeekReview(); refusal != "" {
		t.Fatal(refusal)
	}
	if !m.Triage.Active || !m.Triage.closed {
		t.Fatal("an empty review did not open on its closing card")
	}
}

// Today reads the day as one bar: what finished today against that plus the
// open work Today draws as its own, and says clear when nothing is left.
func TestTodayProgressLineReadsTheDay(t *testing.T) {
	today := time.Now()
	due := primaryRowsTask("due", models.StateBacklog)
	due.Title = "Due today"
	due.DueDate = &today
	m := nowCardOutliner(t, 100, 30, due)
	m.SelectOutlinerMode(chrome.ModeToday)
	if got := ansi.Strip(m.todayProgressLine(1, 100)); !strings.Contains(got, "1 of 2 done today") || !strings.Contains(got, "████") {
		t.Fatalf("progress line = %q", got)
	}
	due.WorkflowState = models.StateDone
	m.RebuildRows()
	if got := ansi.Strip(m.todayProgressLine(2, 100)); !strings.Contains(got, "2 of 2 done today · clear") {
		t.Fatalf("cleared day = %q", got)
	}
	if got := m.todayProgressLine(0, 100); got != "" {
		t.Fatalf("an empty day drew %q", got)
	}
}
