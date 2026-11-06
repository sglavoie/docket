package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/docket/internal/app"
	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/tui/coordinator"
	"github.com/sglavoie/docket/internal/tui/operation"
	"github.com/sglavoie/docket/internal/tui/taskref"
	"github.com/sglavoie/docket/internal/tui/textfit"
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
// one stays quiet, since the keyless `Set Up Check-Ins` entry is still in the
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
	if results, ok := committedTaskResults(msg); ok {
		m.noteCommittedStateEffects(results)
	}
}

// committedTaskResults is the committed results a transaction message carries,
// read alike by every note that answers what a commit did.
func committedTaskResults(msg tea.Msg) ([]app.TaskResult, bool) {
	switch msg := msg.(type) {
	case coordinator.TransactionResultMsg:
		return []app.TaskResult{msg.Result}, true
	case coordinator.BatchTransactionResultMsg:
		return msg.Tasks, true
	case coordinator.TaskCreatedMsg:
		results := msg.Results
		if len(results) == 0 && msg.Result != nil {
			results = []app.TaskResult{*msg.Result}
		}
		return results, true
	case coordinator.TasksCapturedMsg:
		return msg.Results, true
	case coordinator.TaskLineEditedMsg:
		return []app.TaskResult{{ProjectID: msg.Result.Task.ProjectID, TaskID: msg.Result.Task.TaskID,
			TransactionID: msg.Result.TransactionID, StateEffect: msg.Result.StateEffect}}, true
	}
	return nil, false
}

// noteCommittedStateEffects hands one commit's evidence to both notes that read
// state effects: the check-in reminder and the resume-note one.
func (m *AppModel) noteCommittedStateEffects(results []app.TaskResult) {
	m.noteCheckInFeedback(results)
	m.noteResumeNoteFeedback(results)
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
// have moved out from under it the moment it changed state. Set Up Check-Ins
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
			return "No check-in · " + menuKey + ": Set Up Check-Ins"
		}
		return "No check-in: " +
			textfit.TruncateToVisualWidth(titles[0], checkInFeedbackTitleWidth) +
			" · " + menuKey + ": Set Up Check-Ins"
	default:
		return fmt.Sprintf("No check-in: %d tasks · %s: Set Up Check-Ins", len(titles), menuKey)
	}
}

// lineNamesTitle reports whether spoken already names title, whole or cut to the
// width a status change gives it. The prefix it looks for is the one the note
// itself would draw, cut by visual width and less its ellipsis, so a wide CJK or
// emoji title is recognized by the same columns it would be named in rather than
// by a rune count those columns cannot hold. The status change in internal/app
// cuts its title to the same columns, so the two always agree on the prefix.
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

// resumeNoteFeedbackController is the back-to-work half of the resume note. A
// note is only worth reading if it was written when the user knew where they
// stopped, and the one commit that marks that moment is a task leaving Doing.
// So leaving Doing says whether the task carries a note and names the keyless
// `Note Where I Left Off` entry, and starting a task again reads its note back
// on the line that confirms the start.
//
// It is the check-in note's twin on every term that matters: it observes
// committed evidence, opens nothing, takes no keyboard, rides the line this
// Update already published through appendNote, and suppresses a redelivered
// transaction by ID. It differs in one: it speaks on every qualifying commit
// rather than once per session, because each departure is its own moment and
// the note it asks for is a different task's sentence each time.
//
// leftOff is the session's memory of the one task the most recent departure
// named. It is what the menu entry writes for, since `r` reads the cursor and
// the cursor has usually followed the lane the task just left. It is never
// persisted, and a departure of two or more tasks at once forgets it, since a
// resume note is one task's sentence and a selection has no one place it
// stopped.
type resumeNoteFeedbackController struct {
	seen    map[string]bool
	noted   []app.TaskResult
	leftOff taskref.TaskIdentity
}

// leftOffEntryLabel is the menu entry both notes would name, spelled once.
const leftOffEntryLabel = "Note Where I Left Off"

// noteResumeNoteFeedback records every task a commit named, eligible or not,
// since whether the gesture was about one task is decided over all of them.
// Every transaction is marked seen for the reason noteCheckInFeedback gives.
func (m *AppModel) noteResumeNoteFeedback(results []app.TaskResult) {
	if m.resumeNoteFeedback.seen == nil {
		m.resumeNoteFeedback.seen = make(map[string]bool)
	}
	transactions := make(map[string]bool)
	for _, result := range results {
		if result.TransactionID == "" || m.resumeNoteFeedback.seen[result.TransactionID] {
			continue
		}
		transactions[result.TransactionID] = true
		m.resumeNoteFeedback.noted = append(m.resumeNoteFeedback.noted, result)
	}
	for id := range transactions {
		m.resumeNoteFeedback.seen[id] = true
	}
}

// publishResumeNoteFeedback speaks for the evidence this Update recorded, from
// the same deferred closure the check-in note publishes from. Only a commit
// naming exactly one task says anything: a bulk gesture is about a selection,
// even when only one of its members left or entered Doing, and one task's note
// would misdescribe the rest.
//
// yield is the check-in note having spoken on this same line. That reminder is
// once per session and was the line's tenant first, and two routes on one
// footer line overflow it at ordinary widths, so this note stays quiet for the
// one commit — while still remembering the departure the entry writes for.
func (m *AppModel) publishResumeNoteFeedback(yield bool) tea.Cmd {
	results := m.resumeNoteFeedback.noted
	m.resumeNoteFeedback.noted = nil
	var named []taskref.TaskIdentity
	var effect *app.TaskStateEffect
	seen := make(map[taskref.TaskIdentity]bool)
	anyLeft := false
	for _, result := range results {
		identity := taskref.TaskIdentity{ProjectID: result.ProjectID, TaskID: result.TaskID}
		if !identity.Valid() {
			continue
		}
		anyLeft = anyLeft || result.StateEffect.LeavesDoing()
		if seen[identity] {
			continue
		}
		seen[identity] = true
		named = append(named, identity)
		effect = result.StateEffect
	}
	if len(named) != 1 {
		if anyLeft {
			m.resumeNoteFeedback.leftOff = taskref.TaskIdentity{}
		}
		return nil
	}
	identity := named[0]
	note := ""
	switch {
	case effect.EntersDoing():
		if task := m.committedResumeTask(identity); task != nil && task.State() == models.StateDoing {
			note = resumeStartNote(task.Title, task.ResumeNote, m.feedback.message)
		}
	case effect.LeavesDoing():
		if task := m.committedResumeTask(identity); task != nil && task.State() != models.StateDoing && !task.IsTerminalState() {
			m.resumeNoteFeedback.leftOff = identity
			note = leftOffNote(task.Title, task.ResumeNote, m.feedback.message, m.menuOpenHint())
		}
	}
	if note == "" || yield || m.feedback.appendNote(note) {
		return nil
	}
	m.setInfoStatus(note)
	return m.statusReadingTimeout()
}

// committedResumeTask re-reads the committed task, so a note about it says what
// the store holds now rather than what the commit's request carried, and a
// project that will not read says nothing.
func (m *AppModel) committedResumeTask(identity taskref.TaskIdentity) *models.Task {
	if !m.reloadProjectByID(identity.ProjectID) {
		return nil
	}
	task, project := m.taskAndProjectForIdentity(identity)
	if project == nil {
		return nil
	}
	return task
}

// resumeStartNote is the sentence a start carries: the note itself, whole, so
// what the user left for themselves is the first thing they read on the way
// back. A task without one adds nothing, since starting asks for no note. The
// fitter elides a long line's head rather than its tail, so the note survives
// a narrow footer at the cost of the status it rides.
func resumeStartNote(title, note, spoken string) string {
	if note == "" {
		return ""
	}
	if lineNamesTitle(spoken, title) {
		return "Left off: " + note
	}
	return textfit.TruncateToVisualWidth(title, checkInFeedbackTitleWidth) + " · Left off: " + note
}

// leftOffNote is the sentence a departure carries. A kept note is named as
// kept rather than quoted, because the moment it matters is when it is stale,
// and the route to replace it is the same entry that writes a first one.
func leftOffNote(title, note, spoken, menuKey string) string {
	head := "No resume note"
	if note != "" {
		head = "Resume note kept"
	}
	if !lineNamesTitle(spoken, title) {
		head += ": " + textfit.TruncateToVisualWidth(title, checkInFeedbackTitleWidth)
	}
	return head + " · " + menuKey + ": " + leftOffEntryLabel
}

// openLeftOffNote opens the ordinary resume-note prompt for the remembered
// task. It borrows the Set Resume Note request with a context naming that task
// alone, so the prompt, the save, the refusal that reopens it, and the undo
// entry are exactly the ones `r` produces.
func (m *AppModel) openLeftOffNote() tea.Cmd {
	target := m.resumeNoteFeedback.leftOff
	if !target.Valid() {
		m.setInfoStatus("No task has left Doing in this session")
		return m.statusReadingTimeout()
	}
	task, project := m.taskAndProjectForIdentity(target)
	if task == nil || project == nil || task.IsTerminalState() {
		m.resumeNoteFeedback.leftOff = taskref.TaskIdentity{}
		m.setInfoStatus("The task that last left Doing is no longer open")
		return m.statusReadingTimeout()
	}
	context := m.operationCommandContext()
	context.Selection = operation.SelectionContext{}
	context.CurrentEntity = operation.EntityIdentity{}
	context.Entity = operation.EntityContext{ProjectID: target.ProjectID, TaskID: target.TaskID, Archived: project.Archived}
	request := operation.OperationExecutionRequest{ID: operation.OperationTaskSetResumeNote, Context: context}
	prompt, err := m.newOperationTextPrompt(request, operation.ValueProviderResumeNote)
	if err != nil {
		m.feedback.err = err
		return m.statusReadingTimeout()
	}
	m.operations.TextPrompt = prompt
	return prompt.InitCmd()
}
