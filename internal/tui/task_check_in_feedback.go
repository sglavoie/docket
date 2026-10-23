package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/coordinator"
	"github.com/sglavoie/tt/internal/tui/taskref"
	"github.com/sglavoie/tt/internal/tui/textfit"
)

// checkInFeedbackController turns committed state effects into one sentence in
// the footer. Entering Doing or Waiting without a schedule is worth saying and
// is not worth taking the keyboard for: the transition already committed, the
// user is somewhere, and scheduling is a separate transaction they may or may
// not want. So nothing here opens a surface, consumes a key, or defers itself
// until one closes — it observes evidence and speaks once.
//
// seen is the duplicate-transaction suppression the offer queue also kept: one
// commit may be announced by more than one message (a batch result and the
// per-task result beside it), and the note must not be said twice for it. noted
// is a within-one-Update buffer rather than a queue, because the line it rides
// is the line this Update published; drained every Update, it can never arrive
// beside a status belonging to something else.
//
// spoken makes the note a once-per-session reminder rather than a refrain: the
// first unscheduled Doing or Waiting commit teaches the route, and every later
// one stays quiet, since the keyless `Set up check-ins` entry is still in the
// menu bar for whoever wants it.
type checkInFeedbackController struct {
	seen   map[string]bool
	noted  []app.TaskResult
	spoken bool
}

// checkInFeedbackTitleWidth bounds the one title a single-task note names. The
// footer is one line and the route is the actionable half of the sentence, so a
// long title is shortened here rather than allowed to truncate the sentence's
// tail away; the whole title is on the row and in Peek.
const checkInFeedbackTitleWidth = 24

// observeCheckInFeedbackMessage records the committed evidence a note is
// derived from. These are results, never requests: the note answers what the
// transaction did, so a refused, unchanged, or already-scheduled task produces
// no evidence and therefore no sentence.
//
// Interaction ownership is deliberately not consulted. A footer sentence takes
// no keyboard and displaces nothing, and the commit's own toast is published
// whether or not the user moved on, so the note may ride it on the same terms.
func (m *AppModel) observeCheckInFeedbackMessage(msg tea.Msg) {
	switch msg := msg.(type) {
	case coordinator.TransactionResultMsg:
		m.noteCheckInFeedback([]app.TaskResult{msg.Result})
	case coordinator.BatchTransactionResultMsg:
		m.noteCheckInFeedback(msg.Tasks)
	case coordinator.TaskCreatedMsg:
		results := msg.Results
		if len(results) == 0 && msg.Result != nil {
			results = []app.TaskResult{*msg.Result}
		}
		m.noteCheckInFeedback(results)
	case coordinator.TasksCapturedMsg:
		m.noteCheckInFeedback(msg.Results)
	case coordinator.TaskLineEditedMsg:
		m.noteCheckInFeedback([]app.TaskResult{{ProjectID: msg.Result.Task.ProjectID, TaskID: msg.Result.Task.TaskID,
			TransactionID: msg.Result.TransactionID, StateEffect: msg.Result.StateEffect}})
	}
}

// noteCheckInFeedback is the one entry point for that evidence, so the commit
// paths that publish their own toast before any message travels — the external
// content edit and the outline buffer — record on exactly the terms the message
// observers do.
//
// Every transaction named is marked seen, eligible or not, which is what keeps a
// re-delivery of the same commit silent rather than merely deduplicated by task.
func (m *AppModel) noteCheckInFeedback(results []app.TaskResult) {
	if m.checkInFeedback.seen == nil {
		m.checkInFeedback.seen = make(map[string]bool)
	}
	transactions := make(map[string]bool)
	identities := make(map[taskref.TaskIdentity]bool)
	var eligible []app.TaskResult
	for _, result := range results {
		if result.TransactionID == "" || m.checkInFeedback.seen[result.TransactionID] {
			continue
		}
		transactions[result.TransactionID] = true
		identity := taskref.TaskIdentity{ProjectID: result.ProjectID, TaskID: result.TaskID}
		if result.StateEffect.OffersCheckIn() && identity.Valid() && !identities[identity] {
			identities[identity] = true
			eligible = append(eligible, result)
		}
	}
	for id := range transactions {
		m.checkInFeedback.seen[id] = true
	}
	m.checkInFeedback.noted = append(m.checkInFeedback.noted, eligible...)
}

// publishCheckInFeedback speaks once for the evidence this Update recorded. It
// runs in the one deferred closure in AppModel.Update rather than at a commit
// site, because a handler publishes its own toast after it records — riding that
// line is what keeps the note one sentence in one footer instead of a second
// status that replaces the answer to the key just pressed.
func (m *AppModel) publishCheckInFeedback() tea.Cmd {
	results := m.checkInFeedback.noted
	m.checkInFeedback.noted = nil
	if len(results) == 0 {
		return nil
	}
	if m.checkInFeedback.spoken {
		return nil
	}
	note := checkInFeedbackNote(m.committedCheckInTitles(results), m.feedback.message, m.menuOpenHint())
	if note == "" {
		return nil
	}
	m.checkInFeedback.spoken = true
	if m.feedback.appendNote(note) {
		// The line it rode already carries this Update's severity and its own
		// clock, so advancing either here would reset both.
		return nil
	}
	m.setInfoStatus(note)
	return m.statusReadingTimeout()
}

// committedCheckInTitles re-resolves this commit's own subset and nothing else.
// The refusals are the whole of what keeps the sentence true: a project that
// will not read reports nothing rather than reading a stale cache, and a task
// that has since been scheduled or left active work is dropped, so concurrent
// scheduling is never contradicted by a note about it.
func (m *AppModel) committedCheckInTitles(results []app.TaskResult) []string {
	reloaded := make(map[string]bool)
	var titles []string
	for _, result := range results {
		identity := taskref.TaskIdentity{ProjectID: result.ProjectID, TaskID: result.TaskID}
		loaded, checked := reloaded[identity.ProjectID]
		if !checked {
			loaded = m.reloadProjectByID(identity.ProjectID)
			reloaded[identity.ProjectID] = loaded
		}
		if !loaded {
			continue
		}
		task, project := m.taskAndProjectForIdentity(identity)
		if task == nil || project == nil || !task.CheckIn().IsEmpty() ||
			(task.State() != models.StateDoing && task.State() != models.StateWaiting) {
			continue
		}
		titles = append(titles, task.Title)
	}
	return titles
}

// checkInFeedbackNote is the pure sentence. One task is named, since a note the
// user cannot attach to a task is worth nothing; a committed subset is counted,
// because the footer holds one line and a list of titles is the first thing it
// would truncate away.
//
// The route is the keyless, target-free menu entry, deliberately not a key: `@`
// and the row menu operate on the cursor, and the task this note is about may
// have moved out from under it the moment it changed state. Set up check-ins
// names its own population — unscheduled Doing and Waiting work — which always
// contains whatever the sentence just counted. The note names the negotiated menu shortcut, the
// key that opens the Task menu holding that entry, since "menu" alone does not
// say which one or how to reach it, and opening a menu reads no cursor.
//
// spoken is the line the note will ride. When it already names the one task —
// a status change says which task moved — the note does not repeat the title,
// since a second copy is what would push the route off a narrow footer.
func (m *AppModel) menuOpenHint() string {
	if m.lifecycle.keyDisambiguation {
		return "Ctrl+M"
	}
	return "F10"
}

func checkInFeedbackNote(titles []string, spoken, menuKey string) string {
	switch len(titles) {
	case 0:
		return ""
	case 1:
		if lineNamesTitle(spoken, titles[0]) {
			return "No check-in · " + menuKey + ": Set up check-ins"
		}
		return "No check-in: " +
			textfit.TruncateToVisualWidth(titles[0], checkInFeedbackTitleWidth) +
			" · " + menuKey + ": Set up check-ins"
	default:
		return fmt.Sprintf("No check-in: %d tasks · %s: Set up check-ins", len(titles), menuKey)
	}
}

// lineNamesTitle reports whether spoken already names title, whole or cut to the
// width a status change gives it. The prefix it looks for is the one the note
// itself would draw, cut by visual width and less its ellipsis, so a wide CJK or
// emoji title is recognized by the same columns it would be named in rather than
// by a rune count those columns cannot hold. That prefix is never longer than
// the checkInFeedbackTitleWidth-1 runes a rune-capped line keeps either.
func lineNamesTitle(spoken, title string) bool {
	if title == "" {
		return false
	}
	prefix := textfit.TruncateToVisualWidth(title, checkInFeedbackTitleWidth)
	if prefix != title {
		prefix = strings.TrimSuffix(prefix, "…")
	}
	return prefix != "" && strings.Contains(spoken, prefix)
}
