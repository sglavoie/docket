package outliner

import (
	"fmt"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/tui/chrome"
	"github.com/sglavoie/docket/internal/tui/operation"
	"github.com/sglavoie/docket/internal/tui/taskref"
	"github.com/sglavoie/docket/internal/tui/theme"
)

// Review the Week is the triage pass's fifth source: the work a week lets go
// quiet, walked one card at a time from Task → Review the Week…, then a
// closing card that reads the week back. It is the same view-local gated
// sub-state as the daily bookends — the queue is frozen at opening, every
// routing is the row operation its key names through the shell's executor,
// and each is its own undo entry — so it adds a source and an ending, never a
// second pass. It reads what All draws, so a folded lane or a narrowed view is
// a choice about what the review covers.

// The thresholds past which work counts as gone quiet.
const (
	weekReviewDoingDays   = 7
	weekReviewWaitingDays = 7
	weekReviewNextDays    = 14
)

// weekReviewRoutes is the weekly card's keymap: plan it, send it to Next or
// Backlog, give it a date, finish it, or cancel it, which is the decision a
// stale card most often deserves and the one no other pass offers.
var weekReviewRoutes = map[string]TriageRouteMsg{
	"t": {OperationID: operation.OperationCoreToday},
	"n": {OperationID: operation.OperationCoreNext},
	"b": {OperationID: operation.OperationCoreBacklog},
	"@": {OperationID: operation.OperationTaskSetDueDate},
	"d": {OperationID: operation.OperationTaskSetState, Value: models.StateDone, HasValue: true},
	"c": {OperationID: operation.OperationTaskSetState, Value: models.StateCanceled, HasValue: true},
}

// OpenWeekReview starts a Review the Week pass. It moves the view to All,
// where every lane the review reads is drawn, then freezes the queue; a week
// with nothing gone quiet goes straight to its closing card. The returned
// command is the mode switch's own.
func (m *OutlinerModel) OpenWeekReview() (tea.Cmd, string) {
	if m.IsMultiSelectActive() || m.IsRangeSelectionActive() {
		return nil, "Review the Week works one task at a time: clear the selection first"
	}
	var cmd tea.Cmd
	if m.ActiveOutlinerMode() != chrome.ModeAll {
		_, cmd = m.SelectOutlinerMode(chrome.ModeAll)
	}
	queue := m.weekReviewQueue(time.Now())
	m.PeekMode = false
	m.Triage = triageState{
		Active: true, source: triageSourceWeekReview, queue: queue,
		handled: make(map[taskref.TaskIdentity]bool, len(queue)),
		closed:  len(queue) == 0,
	}
	return cmd, ""
}

// weekReviewQueue is the drawn work that has gone quiet, most pressing first:
// overdue work, then Doing that has not moved in a week, Waiting with no
// check-in to bring it back, and Next that nobody has touched in two weeks.
// Each task is queued once, under the first reason it meets.
func (m *OutlinerModel) weekReviewQueue(now time.Time) []taskref.TaskIdentity {
	seen := make(map[taskref.TaskIdentity]bool)
	var queue []taskref.TaskIdentity
	for rank := 0; rank < weekReviewReasonCount; rank++ {
		for _, row := range m.Rows {
			if !row.IsTaskRow() || row.Task == nil || row.Task.IsTerminalState() {
				continue
			}
			if reason, at := weekReviewReason(row, now); at != rank || reason == "" {
				continue
			}
			if identity := m.TaskIdentityForRow(row); identity.Valid() && !seen[identity] {
				seen[identity] = true
				queue = append(queue, identity)
			}
		}
	}
	return queue
}

// weekReviewReasonCount is how many reasons weekReviewReason ranks.
const weekReviewReasonCount = 4

// weekReviewReason is why a row belongs in the review, with the rank that
// orders the queue, or an empty reason when it does not.
func weekReviewReason(row OutlinerRow, now time.Time) (string, int) {
	task := row.Task
	if task.IsOverdueAt(now) && task.DueDate != nil {
		return "Overdue since " + task.DueDate.Format("Mon Jan 2"), 0
	}
	days := func(since time.Time) int {
		if since.IsZero() {
			return 0
		}
		return int(now.Sub(since).Hours() / 24)
	}
	if task.IsInState(models.StateDoing) {
		if d := days(task.StateChangedAt); d >= weekReviewDoingDays {
			return fmt.Sprintf("In Doing for %d days · still the work in hand?", d), 1
		}
	}
	if task.IsInState(models.StateWaiting) && task.NextCheckIn == nil {
		if d := days(task.StateChangedAt); d >= weekReviewWaitingDays {
			return fmt.Sprintf("Waiting %d days with no check-in", d), 2
		}
	}
	if row.IsNextSection {
		if d := days(task.UpdatedAt); d >= weekReviewNextDays {
			return fmt.Sprintf("In Next, untouched for %d days", d), 3
		}
	}
	return "", -1
}

// weekReviewLead is the line over every weekly card: why this card is here,
// and how far the review has come.
func (m *OutlinerModel) weekReviewLead(row OutlinerRow, now time.Time) string {
	why, _ := weekReviewReason(row, now)
	if why == "" {
		// The card's own routing may have just answered its reason.
		why = "Reviewed"
	}
	decided := 0
	for _, step := range m.Triage.trail {
		if step.routed {
			decided++
		}
	}
	if decided == 0 {
		return theme.MutedStyle.Render(why)
	}
	return theme.MutedStyle.Render(fmt.Sprintf("%d decided · %s", decided, why))
}

// weekReviewedSpec reads the closing card: what the pass decided, from its
// trail; what the week finished, from the completion history; and what is
// still unreviewed, from the queue entries the pass never reached.
func (m *OutlinerModel) weekReviewedSpec(now time.Time) chrome.WeekReviewedCardSpec {
	start := now.AddDate(0, 0, -6)
	spec := chrome.WeekReviewedCardSpec{
		Range:   start.Format("Jan 2") + " – " + now.Format("Jan 2"),
		History: m.CompletionActivity.History, Now: now,
	}
	for _, step := range m.Triage.trail {
		spec.Reviewed++
		if !step.routed {
			spec.Kept++
			continue
		}
		switch step.key {
		case "t":
			spec.Planned++
		case "n", "b":
			spec.Moved++
		case "@":
			spec.Dated++
		case "d":
			spec.Finished++
		case "c":
			spec.Canceled++
		}
	}
	for _, identity := range m.Triage.queue {
		if m.Triage.handled[identity] {
			continue
		}
		if _, drawn := m.RowIndexForTask(identity); drawn {
			spec.StillOpen++
		}
	}
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	for day, count := range m.CompletionActivity.DailyCompletions {
		local := day.In(now.Location())
		date := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, now.Location())
		if !date.After(today) && date.After(today.AddDate(0, 0, -7)) {
			spec.WeekDone += count
		}
	}
	return spec
}

// weekReviewGuidance is the weekly pass's keymap, on a card and on the
// closing card. It takes no footer rank, as no gated keymap does.
func (m *OutlinerModel) weekReviewGuidance() chrome.GuidanceSpec {
	if m.Triage.closed {
		hints := chrome.RequiredGuidanceHints("Enter close")
		if len(m.Triage.trail) > 0 {
			hints = append(hints, chrome.OptionalGuidanceHints(2, "u back")...)
		}
		return chrome.GuidanceSpec{Hints: hints, Rows: 1}
	}
	hints := chrome.RequiredGuidanceHints("t today", "n next", "b backlog", "c cancel", "Space keep")
	hints = append(hints, chrome.OptionalGuidanceHints(3, "@ date", "d done")...)
	if len(m.Triage.trail) > 0 {
		hints = append(hints, chrome.OptionalGuidanceHints(2, "u back")...)
	}
	hints = append(hints, chrome.RequiredGuidanceHints("Esc finish")...)
	return chrome.GuidanceSpec{Hints: hints, Rows: 2}
}
