package outliner

import (
	"strings"
	"testing"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/tui/chrome"
	"github.com/sglavoie/docket/internal/tui/query"
	"github.com/sglavoie/docket/internal/util"
)

// TestInlinePeekScheduleLineNamesOnlyWhatIsStillOwed is the forward half of the
// split that replaced the old mixed dates line. Every field here is a date in
// front of the task — the deadline, the day a pending deferral brings it back,
// and the check-in with its cadence — and an instant the task has already spent
// may never appear on it.
func TestInlinePeekScheduleLineNamesOnlyWhatIsStillOwed(t *testing.T) {
	now := time.Date(2026, time.August, 10, 12, 0, 0, 0, time.UTC)
	due := time.Date(2026, time.August, 14, 0, 0, 0, 0, time.UTC)
	checkIn := time.Date(2026, time.August, 12, 0, 0, 0, 0, time.UTC)
	lastReview := now.Add(-3 * time.Hour)
	woke := now.Add(-24 * time.Hour)
	pending := now.Add(24 * time.Hour)

	cases := []struct {
		name string
		task *models.Task
		want string
	}{
		{"nothing owed at all", &models.Task{}, ""},
		{
			"every owed field, in the order they are acted on",
			&models.Task{
				DueDate:           &due,
				DeferredUntil:     &pending,
				NextCheckIn:       &checkIn,
				CheckInRepeatDays: 7,
			},
			"Due: Fri Aug 14 (in 4 days) · Defer: " + chrome.FormatDeferDate(pending, now) + " · Check-in: Wed Aug 12 (in 2 days) · Repeat: every 7d",
		},
		{
			// The row carries no deferral now, so Peek names the pending one — and
			// still names it as the deferral it is rather than as a wake-up.
			"a pending deferral names the day it comes back, not a wake-up",
			&models.Task{CreatedAt: now.Add(-24 * time.Hour), DeferredUntil: &pending},
			"Defer: " + chrome.FormatDeferDate(pending, now),
		},
		{
			"a lapsed deferral is history and owes this line nothing",
			&models.Task{CreatedAt: now.Add(-24 * time.Hour), DeferredUntil: &woke, DeferCount: 3},
			"",
		},
		{
			"one-off check-in names repetition as off",
			&models.Task{NextCheckIn: &checkIn},
			"Check-in: Wed Aug 12 (in 2 days) · Repeat: off",
		},
		{
			"spent instants never reach the schedule",
			&models.Task{
				CreatedAt:      now.Add(-12 * 24 * time.Hour),
				UpdatedAt:      now.Add(-2 * 24 * time.Hour),
				LastReviewedAt: &lastReview,
			},
			"",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ansi.Strip(inlinePeekScheduleLine(tc.task, now)); got != tc.want {
				t.Fatalf("schedule line = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestInlinePeekHistoryLineNamesOnlyWhatIsAlreadySpent is the other half. Age,
// last touch, last review, a recent wake, and the avoidance count are all
// records of something that happened, so they close the document rather than
// standing between the schedule and the task's own text — and the deadline and
// the check-in may never appear here.
func TestInlinePeekHistoryLineNamesOnlyWhatIsAlreadySpent(t *testing.T) {
	now := time.Date(2026, time.August, 10, 12, 0, 0, 0, time.UTC)
	due := time.Date(2026, time.August, 14, 0, 0, 0, 0, time.UTC)
	checkIn := time.Date(2026, time.August, 12, 0, 0, 0, 0, time.UTC)
	lastReview := now.Add(-3 * time.Hour)
	woke := now.Add(-24 * time.Hour)
	pending := now.Add(24 * time.Hour)
	stale := now.Add(-query.DeferWakeWindow - time.Hour)

	cases := []struct {
		name string
		task *models.Task
		want string
	}{
		{"no history at all", &models.Task{}, ""},
		{
			"every spent field",
			&models.Task{
				CreatedAt:      now.Add(-12 * 24 * time.Hour),
				UpdatedAt:      now.Add(-2 * 24 * time.Hour),
				DeferredUntil:  &woke,
				DeferCount:     3,
				LastReviewedAt: &lastReview,
			},
			"Created: 12d ago · Updated: 2d ago · Last review: 3h ago · Woke: 1d ago · Deferred: ×3",
		},
		{
			"created only",
			&models.Task{CreatedAt: now.Add(-3 * time.Hour)},
			"Created: 3h ago",
		},
		{
			"an instant under a minute old is now, which already says when",
			&models.Task{CreatedAt: now.Add(-10 * time.Second)},
			"Created: now",
		},
		{
			"an update in the creation's second repeats the creation, so it drops",
			&models.Task{CreatedAt: now.Add(-3 * time.Hour), UpdatedAt: now.Add(-3 * time.Hour).Add(400 * time.Millisecond)},
			"Created: 3h ago",
		},
		{
			"an update a second or more later is a touch of its own",
			&models.Task{CreatedAt: now.Add(-3 * time.Hour), UpdatedAt: now.Add(-3*time.Hour + time.Second)},
			"Created: 3h ago · Updated: 2h ago",
		},
		{
			"a pending deferral has not woken, so history says nothing about it",
			&models.Task{CreatedAt: now.Add(-24 * time.Hour), DeferredUntil: &pending},
			"Created: 1d ago",
		},
		{
			"a lapsed deferral past the wake window falls silent",
			&models.Task{CreatedAt: now.Add(-24 * time.Hour), DeferredUntil: &stale},
			"Created: 1d ago",
		},
		{
			"the avoidance count stands without a wake marker",
			&models.Task{DeferCount: 4},
			"Deferred: ×4",
		},
		{
			"owed dates never reach history",
			&models.Task{DueDate: &due, NextCheckIn: &checkIn, CheckInRepeatDays: 7},
			"",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ansi.Strip(inlinePeekHistoryLine(tc.task, now)); got != tc.want {
				t.Fatalf("history line = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestInlinePeekCheckInFieldsWrapWithinNarrowWidth keeps the schedule and the
// review it answers reachable at a width that holds neither whole. They are two
// sections now, so the claim is read off the document rather than off one line:
// both wrap rather than eliding, and no row outgrows the pane.
func TestInlinePeekCheckInFieldsWrapWithinNarrowWidth(t *testing.T) {
	now := time.Date(2026, time.August, 10, 12, 0, 0, 0, time.UTC)
	checkIn := util.StartOfDay(now.AddDate(0, 0, 2))
	lastReview := now.Add(-37 * time.Hour)
	task := &models.Task{
		NextCheckIn:       &checkIn,
		CheckInRepeatDays: 30,
		LastReviewedAt:    &lastReview,
	}
	const width = 22
	lines := append(
		inlinePeekScheduleLines(task, now, "  ", width),
		inlinePeekHistoryLines(task, now, "  ", width)...,
	)
	plain := ansi.Strip(strings.Join(lines, "\n"))
	for _, want := range []string{"Check-in:", "Wed Aug 12", "Repeat:", "30d", "Last review:", "1d"} {
		if !strings.Contains(plain, want) {
			t.Fatalf("narrow Peek dates missing %q:\n%s", want, plain)
		}
	}
	if strings.Contains(plain, "…") {
		t.Fatalf("narrow Peek elided a date instead of wrapping it:\n%s", plain)
	}
	for index, line := range lines {
		if got, limit := lipgloss.Width(line), width+2; got > limit {
			t.Fatalf("narrow Peek line %d is %d columns, want <= %d: %q", index, got, limit, ansi.Strip(line))
		}
	}
}

// TestInlinePeekDateKeysReadStoredInstantsAlone is the paging half of the split.
// Each section identifies its own content, so every stored value that reaches
// either line moves that line's key — and the deferral instant moves both, since
// the schedule reads its pending half and history reads its wake.
func TestInlinePeekDateKeysReadStoredInstantsAlone(t *testing.T) {
	assertChanged := func(label string, key func(*models.Task) string, task *models.Task, mutate func()) {
		t.Helper()
		before := key(task)
		mutate()
		if after := key(task); after == before {
			t.Fatalf("%s did not change the Peek section identity it belongs to", label)
		}
	}

	checkIn := time.Date(2026, time.August, 12, 0, 0, 0, 0, time.UTC)
	due := time.Date(2026, time.August, 14, 0, 0, 0, 0, time.UTC)
	deferred := time.Date(2026, time.August, 9, 0, 0, 0, 0, time.UTC)
	lastReview := time.Date(2026, time.August, 10, 9, 0, 0, 0, time.UTC)

	schedule := &models.Task{}
	assertChanged("deadline", inlinePeekScheduleKey, schedule, func() { schedule.DueDate = &due })
	assertChanged("deferral", inlinePeekScheduleKey, schedule, func() { schedule.DeferredUntil = &deferred })
	assertChanged("next check-in", inlinePeekScheduleKey, schedule, func() { schedule.NextCheckIn = &checkIn })
	assertChanged("repeat", inlinePeekScheduleKey, schedule, func() { schedule.CheckInRepeatDays = 7 })

	history := &models.Task{}
	assertChanged("creation", inlinePeekHistoryKey, history, func() { history.CreatedAt = due })
	assertChanged("touch", inlinePeekHistoryKey, history, func() { history.UpdatedAt = due })
	assertChanged("wake", inlinePeekHistoryKey, history, func() { history.DeferredUntil = &deferred })
	assertChanged("avoidance count", inlinePeekHistoryKey, history, func() { history.DeferCount = 3 })
	assertChanged("last review", inlinePeekHistoryKey, history, func() { history.LastReviewedAt = &lastReview })

	// The drifting half, stated against one task that carries every field of both
	// lines: ninety minutes of wall clock rewrite what history says, and neither
	// key moves with it. That is what keeps the minute tick from throwing a
	// reader back to the top of the document.
	early := time.Date(2026, time.August, 10, 12, 0, 0, 0, time.UTC)
	late := early.Add(90 * time.Minute)
	steady := &models.Task{
		CreatedAt: early.Add(-90 * time.Minute), UpdatedAt: early.Add(-90 * time.Minute),
		DueDate: &due, DeferredUntil: &deferred, DeferCount: 4,
		NextCheckIn: &checkIn, CheckInRepeatDays: 7, LastReviewedAt: &lastReview,
	}
	scheduleKey, historyKey := inlinePeekScheduleKey(steady), inlinePeekHistoryKey(steady)
	if inlinePeekHistoryLine(steady, early) == inlinePeekHistoryLine(steady, late) {
		t.Fatal("the fixture's history reads the same at both instants, so it proves no drift")
	}
	for label, drifted := range map[string]string{
		"early history": ansi.Strip(inlinePeekHistoryLine(steady, early)),
		"late history":  ansi.Strip(inlinePeekHistoryLine(steady, late)),
		"schedule":      ansi.Strip(inlinePeekScheduleLine(steady, late)),
	} {
		if strings.Contains(scheduleKey, drifted) || strings.Contains(historyKey, drifted) {
			t.Fatalf("a Peek section key carries the %s rendering %q", label, drifted)
		}
	}
	if got := inlinePeekScheduleKey(steady); got != scheduleKey {
		t.Fatalf("schedule key is not stable across two reads: %q then %q", scheduleKey, got)
	}
	if got := inlinePeekHistoryKey(steady); got != historyKey {
		t.Fatalf("history key is not stable across two reads: %q then %q", historyKey, got)
	}
}
