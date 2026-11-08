package outliner

import (
	"fmt"
	"slices"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/tui/chrome"
	"github.com/sglavoie/docket/internal/tui/operation"
	"github.com/sglavoie/docket/internal/tui/query"
	"github.com/sglavoie/docket/internal/tui/status"
	"github.com/sglavoie/docket/internal/tui/taskref"
	"github.com/sglavoie/docket/internal/tui/theme"
)

// reviewAdvanceExhaustedStatus is what a review pass says when it has run out of
// visible work. It is the acknowledgement's own answer to the keystroke rather
// than an advisory, so it is a status line and not an AdvisoryLine.
const reviewAdvanceExhaustedStatus = "No further visible review candidates"

// PlanReviewAdvance turns an ordinary relocation capture into a review
// acknowledgement's: the successors the cursor may land on are recorded now,
// from the rows the user is looking at, because the acknowledgement itself is
// what makes the source stop being a candidate and a list rebuilt after the
// commit can no longer be asked "who was next".
//
// The sequence is ordered and deduplicated by identity, with the source seeded
// into the seen set. Every row builder as it stands draws an identity once — a
// promoted lane moves a task rather than copying it — so neither guard fires
// today. Both are here because the sequence is consumed after a rebuild the
// planning code cannot see, and landing on a second copy of a row the user has
// already answered is not advancing.
//
// Candidacy is taskOwedAttention: the automatic question, so a pass
// visits what raised its hand — a flag, or a check-in whose day has come — and
// not everything a person could choose to acknowledge. An early review is still
// allowed by name on the task under the cursor; it is only the advancing that
// skips a schedule whose day is still ahead. Acknowledging always takes the
// source out of that set, so a pass ends rather than circling.
//
// Eligibility is read here and again when the cursor lands. Recording it twice
// is not redundancy: this reading is what keeps an ineligible row out of the
// sequence, and the later one is what drops a candidate the commit or another
// surface changed while the transaction was in flight.
func (m *OutlinerModel) PlanReviewAdvance(capture *OutlinerRelocationCapture) {
	if m == nil || capture == nil {
		return
	}
	capture.ReviewAdvance = true
	now := time.Now()
	seen := map[taskref.TaskIdentity]bool{capture.Task: true}
	for i := m.Cursor + 1; i < len(m.Rows); i++ {
		row := m.Rows[i]
		if !row.IsTaskRow() || row.Task == nil {
			continue
		}
		identity := m.TaskIdentityForRow(row)
		if !identity.Valid() || seen[identity] {
			continue
		}
		seen[identity] = true
		if !taskOwedAttention(row.Task, m.reviewRowProject(row), now) {
			continue
		}
		capture.ReviewSuccessors = append(capture.ReviewSuccessors, identity)
	}
}

// taskOwedAttention is the attention question every review reader asks:
// models.TaskNeedsAttention over work that is not effectively hidden. It reads
// no facet, so the Review lane, its count, and a review pass skip hidden work
// even where fH has put it on screen, since hidden work is never owed attention.
func taskOwedAttention(task *models.Task, project *models.Project, now time.Time) bool {
	return models.TaskOwesReviewAttention(project, task, now)
}

// reviewRowProject resolves the project a row's task is read inside, the way
// TaskIdentityForRow does, so the ancestor walk sees the task's own tree.
func (m *OutlinerModel) reviewRowProject(row OutlinerRow) *models.Project {
	if m.Project != nil {
		return m.Project
	}
	return m.crossResolveProject(row.ProjectID)
}

// finishReviewAdvance seats the cursor on the first recorded successor that is
// still both visible and still asking for attention, and reports exhaustion when
// none is.
// It writes m.Cursor and nothing else: ComputeOutlinerViewport stays the single
// window authority, folds are left as they were, and no project is loaded to
// find a candidate the projection does not already hold. There is no wrap —
// a pass that reaches the bottom is over, not restarted.
//
// Every caller sits behind FinishOutlinerRelocation's projection guard, so mode,
// scope, search, filters, sort, zoom, folds, and focus are already known to be
// the ones the sequence was recorded against. The mode's own cursor-advance
// catalog flag is deliberately not consulted: that flag governs following a task
// that left its lane, while a review pass is a sweep the user asked for by name
// and works the same in every mode.
//
// Nothing is pushed onto jump history. An acknowledgement moves the cursor, not
// the task, so there is no vacated origin for `Ctrl+O` to return to.
func (m *OutlinerModel) finishReviewAdvance(capture *OutlinerRelocationCapture) tea.Cmd {
	if m == nil || capture == nil {
		return nil
	}
	now := time.Now()
	for _, successor := range capture.ReviewSuccessors {
		index, ok := m.RowIndexForTask(successor)
		if !ok {
			continue
		}
		row := m.Rows[index]
		if row.Task == nil || !taskOwedAttention(row.Task, m.reviewRowProject(row), now) {
			continue
		}
		m.Cursor = index
		return nil
	}
	// The source is preferred over whatever the reload's nearest-row
	// reconciliation chose, because the user is still looking at the task they
	// just acknowledged and it is usually still on screen. When it is not — a
	// flagged row leaving the Review lane, say — that reconciliation already ran
	// and its answer stands.
	if index, ok := m.RowIndexForTask(capture.Task); ok {
		m.Cursor = index
	}
	return func() tea.Msg { return status.InfoStatusMsg(reviewAdvanceExhaustedStatus) }
}

// triageState is the card-at-a-time pass over the work a view is asking the
// reader to sort: Inbox's tasks, or Today's Suggested lane. It is the review
// pass's sibling — one task, one decision, advance — and like the query readout
// it is a view-local gated sub-state with no presentation, strategy, or manifest
// of its own. The queue is frozen when the pass opens, so routing one card
// never reorders the ones still to come.
//
// A card commits nothing itself. Each routing key seats the cursor on the
// card's task and hands the shell the operation the row key runs — `wt`, `wn`,
// `wb`, `wd`, `%`, Delete — so feedback, undo, cursor recovery, and the Inbox
// zero moment are the ones a row already earns.
type triageState struct {
	Active  bool
	source  triageSource
	queue   []taskref.TaskIdentity
	handled map[taskref.TaskIdentity]bool
	// trail is the order cards left the pass in, and whether each was routed
	// rather than skipped, which is what `u` walks back.
	trail []triageStep
	// pending is the card whose routing opened a shell question — the project
	// picker or the delete confirmation — and so may still be declined. It leaves
	// the pass only once the rows stop drawing it, which is what a committed move
	// or delete does; a declined one is simply the card again.
	pending taskref.TaskIdentity
	// covered is whether a shell question stands over the view, set by the shell
	// once per update, so the card steps aside rather than stacking under it.
	covered bool
	// closed is a Close the Day pass standing on its closing card: every card
	// has left the pass, or Esc finished it early, and what is left to read is
	// the day. Only that source ever sets it.
	closed bool
}

type triageStep struct {
	identity taskref.TaskIdentity
	routed   bool
	// key is the routing key a routed card took, which is what the closing card
	// counts its destinations from.
	key string
}

type triageSource int

const (
	triageSourceInbox triageSource = iota
	triageSourceSuggested
	triageSourceCloseDay
)

// TriageRouteMsg asks the shell to run one operation against the cursor the
// pass just seated on its card's task. Running it is the shell's because the
// executor is.
type TriageRouteMsg struct {
	OperationID operation.OperationID
	Value       any
	HasValue    bool
}

func (msg TriageRouteMsg) String() string { return "triage route" }

// triageRoutes is the card's keymap: each key names the row operation it runs.
// Space, s, l, and → skip; u walks back; Esc and q stop.
var triageRoutes = map[string]TriageRouteMsg{
	"t": {OperationID: operation.OperationCoreToday},
	"n": {OperationID: operation.OperationCoreNext},
	"b": {OperationID: operation.OperationCoreBacklog},
	"d": {OperationID: operation.OperationTaskSetState, Value: models.StateDone, HasValue: true},
	"%": {OperationID: operation.OperationTaskMoveToProject},
	"x": {OperationID: operation.OperationTaskDelete},
}

func triageRouteAsks(route TriageRouteMsg) bool {
	return route.OperationID == operation.OperationTaskMoveToProject || route.OperationID == operation.OperationTaskDelete ||
		route.OperationID == operation.OperationTaskSetResumeNote || route.OperationID == operation.OperationTaskSetDueDate
}

// OpenTriage starts a pass over the drawn rows the current view offers for
// sorting: Today's Suggested lane in Today, and every drawn Inbox task
// elsewhere. It returns a refusal when there is nothing to sort, and opens
// nothing.
func (m *OutlinerModel) OpenTriage() string {
	if m.IsMultiSelectActive() || m.IsRangeSelectionActive() {
		return "Triage works one task at a time: clear the selection first"
	}
	source := triageSourceInbox
	if m.ActiveOutlinerMode() == chrome.ModeToday {
		source = triageSourceSuggested
	}
	seen := make(map[taskref.TaskIdentity]bool)
	var queue []taskref.TaskIdentity
	for _, row := range m.Rows {
		if !row.IsTaskRow() || row.Task == nil {
			continue
		}
		if source == triageSourceSuggested && row.Group.SectionKey != todaySuggestionsCollapseKey ||
			source == triageSourceInbox && !row.Task.IsInState(models.StateInbox) {
			continue
		}
		if identity := m.TaskIdentityForRow(row); identity.Valid() && !seen[identity] {
			seen[identity] = true
			queue = append(queue, identity)
		}
	}
	if len(queue) == 0 {
		if source == triageSourceSuggested {
			return "Nothing suggested to plan: Today's Suggested lane is empty or folded"
		}
		return "Nothing to triage: no Inbox tasks are in view"
	}
	// The card is the detail now; a dock under it would describe whatever row
	// the cursor happened to be on.
	m.PeekMode = false
	m.Triage = triageState{Active: true, source: source, queue: queue, handled: make(map[taskref.TaskIdentity]bool, len(queue))}
	return ""
}

// CloseTriage ends the pass, leaving the cursor where the last card put it.
func (m *OutlinerModel) CloseTriage() { m.Triage = triageState{} }

// SetTriageCovered records whether a shell question stands over the view.
func (m *OutlinerModel) SetTriageCovered(covered bool) {
	if m != nil {
		m.Triage.covered = covered
	}
}

// triageCurrent is the card on screen: the first queued task not yet routed or
// skipped that the projection still draws. A task the rows no longer hold —
// another surface finished it, a sync moved it — is passed over silently.
func (m *OutlinerModel) triageCurrent() (taskref.TaskIdentity, int, bool) {
	for _, identity := range m.Triage.queue {
		if m.Triage.handled[identity] {
			continue
		}
		if index, ok := m.RowIndexForTask(identity); ok {
			return identity, index, true
		}
	}
	return taskref.TaskIdentity{}, 0, false
}

// settleTriage resolves a pending card against the rows the projection now
// draws and closes a pass with nothing left to show. It runs after every row
// build, which is when a committed move or delete lands, and before every key,
// the first moment a question the reader declined is known to be over.
func (m *OutlinerModel) settleTriage(declined bool) {
	if !m.Triage.Active || m.Triage.closed {
		return
	}
	if pending := m.Triage.pending; pending.Valid() {
		if _, drawn := m.RowIndexForTask(pending); !drawn {
			m.Triage.handled[pending] = true
			m.Triage.trail = append(m.Triage.trail, triageStep{identity: pending, routed: true})
			m.Triage.pending = taskref.TaskIdentity{}
		} else if declined {
			m.Triage.pending = taskref.TaskIdentity{}
		}
	}
	if _, _, ok := m.triageCurrent(); !ok {
		m.exhaustTriage()
	}
}

// handleTriageKey owns every key while the pass is open. Anything it does not
// name is swallowed, since the rows it would act on are under the card.
func (m *OutlinerModel) handleTriageKey(key string) (tea.Model, tea.Cmd) {
	if m.Triage.closed {
		return m, m.handleDayClosedKey(key)
	}
	if key == "esc" && m.Triage.source == triageSourceCloseDay {
		// Stopping a Close the Day pass early still ends on the day it closed.
		m.settleTriage(true)
		if m.Triage.Active {
			m.Triage.closed = true
		}
		return m, nil
	}
	if key == "esc" || key == "q" {
		m.CloseTriage()
		return m, nil
	}
	if m.settleTriage(true); !m.Triage.Active {
		return m, nil
	}
	if key == "u" {
		return m, m.triageBack()
	}
	identity, index, _ := m.triageCurrent()
	route, routed := m.triageRouteFor(key)
	switch {
	case routed && triageRouteAsks(route):
		// Both ask a question first, so the card stays until the rows say the
		// answer was yes.
		m.Triage.pending = identity
	case routed, key == "space", key == "s", key == "l", key == "right":
		m.Triage.handled[identity] = true
		step := triageStep{identity: identity, routed: routed}
		if routed {
			step.key = key
		}
		m.Triage.trail = append(m.Triage.trail, step)
		if _, _, more := m.triageCurrent(); !more {
			// The last card closes the pass at once, so the routing's own feedback
			// — and an Inbox zero, when this emptied it — is what is left on screen.
			m.exhaustTriage()
		}
	default:
		return m, nil
	}
	if !routed {
		return m, nil
	}
	m.Cursor = index
	return m, func() tea.Msg { return route }
}

// triageBack returns the previous card to the pass. A routed card is undone
// through the ordinary ledger, so `u` here is the same `u` the row answers; a
// skipped one simply comes back.
func (m *OutlinerModel) triageBack() tea.Cmd {
	trail := m.Triage.trail
	if len(trail) == 0 {
		return nil
	}
	last := trail[len(trail)-1]
	m.Triage.trail = trail[:len(trail)-1]
	delete(m.Triage.handled, last.identity)
	if !last.routed {
		return nil
	}
	return func() tea.Msg { return TriageRouteMsg{OperationID: operation.OperationCoreUndo} }
}

// composeTriageCard composites the open pass's card over the host, bottom
// anchored so it stands directly above the footer that names its keys.
func (m *OutlinerModel) composeTriageCard(host string, width, frameHeight int) (string, bool) {
	if !m.Triage.Active || m.Triage.covered {
		return "", false
	}
	if m.Triage.closed {
		card := chrome.FitOverlayPopupHeight(chrome.RenderDayClosedCard(m.dayClosedSpec(time.Now()), width), m.outlinerModalBudget(width))
		composed, _ := chrome.OverlayPopupDimmedPlaced(host, card, width, frameHeight)
		return composed, true
	}
	identity, index, ok := m.triageCurrent()
	if !ok {
		return "", false
	}
	task := m.Rows[index].Task
	spec := chrome.TriageCardSpec{
		Heading: "Inbox triage", Title: task.Title, Total: len(m.Triage.queue),
		Meta:       triageMeta(task, m.crossResolveProject(m.Rows[index].ProjectID), time.Now()),
		ResumeNote: task.ResumeNote, Description: task.Description,
	}
	switch m.Triage.source {
	case triageSourceSuggested:
		spec.Heading = "Plan today"
	case triageSourceCloseDay:
		spec.Heading = "Close the day"
		spec.Lead = m.closeDayLead(time.Now())
	}
	// Every entry ahead of the card has left the pass, so its own place in the
	// frozen queue is the count.
	spec.Position = slices.Index(m.Triage.queue, identity) + 1
	card := chrome.FitOverlayPopupHeight(chrome.RenderTriageCard(spec, width), m.outlinerModalBudget(width))
	composed, _ := chrome.OverlayPopupDimmedPlaced(host, card, width, frameHeight)
	return composed, true
}

// triageMeta is the card's one line of context: where the task lives, how long
// it has waited to be sorted, and what is already attached to it.
func triageMeta(task *models.Task, project *models.Project, now time.Time) string {
	parts := make([]string, 0, 4)
	if project != nil && project.Name != "" {
		parts = append(parts, project.Name)
	}
	switch age := query.FormatRecencyAge(task.CreatedAt, now); age {
	case "":
	case "now":
		parts = append(parts, "captured just now")
	default:
		parts = append(parts, "captured "+age+" ago")
	}
	if task.DueDate != nil {
		parts = append(parts, "due "+task.DueDate.Format("Mon Jan 2"))
	}
	for _, tag := range task.Tags {
		parts = append(parts, "#"+tag)
	}
	return strings.Join(parts, " · ")
}

// triageGuidance is the pass's own keymap. It takes no footer rank, as a gated
// keymap does not: these keys exist only while the card is up.
func (m *OutlinerModel) triageGuidance() chrome.GuidanceSpec {
	if m.Triage.source == triageSourceCloseDay {
		return m.closeDayGuidance()
	}
	hints := chrome.RequiredGuidanceHints("t today", "n next", "b backlog", "d done")
	hints = append(hints, chrome.OptionalGuidanceHints(3, "% project", "x delete")...)
	hints = append(hints, chrome.RequiredGuidanceHints("Space skip")...)
	if len(m.Triage.trail) > 0 {
		hints = append(hints, chrome.OptionalGuidanceHints(2, "u back")...)
	}
	hints = append(hints, chrome.RequiredGuidanceHints("Esc stop")...)
	return chrome.GuidanceSpec{Hints: hints, Rows: 2}
}

// Close the Day is the triage pass's third source: Today's own work, walked
// one card at a time at the end of the day, then a closing card that reads the
// day back. It is the same view-local gated sub-state as the other two — the
// queue is frozen at opening, every routing is the row operation the card names
// through the shell's executor, and each is its own undo entry — so it adds a
// source and an ending, never a second pass.

// closeDayRoutes is the card's keymap. Today is where every card already
// stands, so `t` is absent, and so are `%` and `x`: closing a day decides when
// work happens, not what or where it is. `@` reschedules, which is the only
// way an overdue card leaves Today, since Today draws it by its date and not
// by its plan. `r` writes the line tomorrow resumes from and leaves the card
// up, since a note decides nothing about the task.
var closeDayRoutes = map[string]TriageRouteMsg{
	"d": {OperationID: operation.OperationTaskSetState, Value: models.StateDone, HasValue: true},
	"n": {OperationID: operation.OperationCoreNext},
	"b": {OperationID: operation.OperationCoreBacklog},
	"@": {OperationID: operation.OperationTaskSetDueDate},
	"r": {OperationID: operation.OperationTaskSetResumeNote},
}

// triageRouteFor is the routing a key names under the open pass's source.
func (m *OutlinerModel) triageRouteFor(key string) (TriageRouteMsg, bool) {
	if m.Triage.source == triageSourceCloseDay {
		route, ok := closeDayRoutes[key]
		return route, ok
	}
	route, ok := triageRoutes[key]
	return route, ok
}

// OpenCloseDay starts a Close the Day pass. It moves the view to Today first,
// since the day is what Today draws, then freezes the queue from the open work
// that lane holds; a day with nothing left open goes straight to its closing
// card. The returned command is the mode switch's own.
func (m *OutlinerModel) OpenCloseDay() (tea.Cmd, string) {
	if m.IsMultiSelectActive() || m.IsRangeSelectionActive() {
		return nil, "Close the Day works one task at a time: clear the selection first"
	}
	var cmd tea.Cmd
	if m.ActiveOutlinerMode() != chrome.ModeToday {
		_, cmd = m.SelectOutlinerMode(chrome.ModeToday)
	}
	queue := m.closeDayQueue()
	// The card is the detail now, as it is for every triage pass.
	m.PeekMode = false
	m.Triage = triageState{
		Active: true, source: triageSourceCloseDay, queue: queue,
		handled: make(map[taskref.TaskIdentity]bool, len(queue)),
		closed:  len(queue) == 0,
	}
	return cmd, ""
}

// closeDayQueue is the open work the Today projection draws as the day's own,
// in drawn order: never the Suggested lane, which is an offer and not a plan.
func (m *OutlinerModel) closeDayQueue() []taskref.TaskIdentity {
	seen := make(map[taskref.TaskIdentity]bool)
	var queue []taskref.TaskIdentity
	for _, row := range m.Rows {
		if !closeDayRow(row) {
			continue
		}
		if identity := m.TaskIdentityForRow(row); identity.Valid() && !seen[identity] {
			seen[identity] = true
			queue = append(queue, identity)
		}
	}
	return queue
}

// closeDayRow is the one predicate for the day's open work: a task row Today
// drew in its own lane, not yet finished, and not merely suggested.
func closeDayRow(row OutlinerRow) bool {
	return row.IsTaskRow() && row.Task != nil && row.IsTodaySection &&
		!row.Task.IsTerminalState() && row.Group.SectionKey != todaySuggestionsCollapseKey
}

// exhaustTriage ends a pass with no card left to show. A Close the Day pass
// stands on its closing card instead; every other source closes outright.
func (m *OutlinerModel) exhaustTriage() {
	if m.Triage.source == triageSourceCloseDay {
		m.Triage.pending = taskref.TaskIdentity{}
		m.Triage.closed = true
		return
	}
	m.CloseTriage()
}

// handleDayClosedKey owns the closing card. `u` takes back the last card, as
// it does on any card; every exit key closes the pass, and anything else is
// swallowed, since the rows it would act on are under the card.
func (m *OutlinerModel) handleDayClosedKey(key string) tea.Cmd {
	switch key {
	case "u":
		if len(m.Triage.trail) == 0 {
			return nil
		}
		m.Triage.closed = false
		return m.triageBack()
	case "esc", "q", "enter", "space":
		m.CloseTriage()
	}
	return nil
}

// closeDayLead is the line over every card: what the day already finished, so
// the pass opens on what went well before it asks about what did not.
func (m *OutlinerModel) closeDayLead(now time.Time) string {
	done := len(m.computeDayCanvas(now).Done)
	check := lipgloss.NewStyle().Foreground(theme.ColorSuccess).Render("✓")
	switch done {
	case 0:
		return theme.MutedStyle.Render("Nothing finished yet today")
	case 1:
		return check + theme.MutedStyle.Render(" 1 done today")
	default:
		return check + theme.MutedStyle.Render(fmt.Sprintf(" %d done today", done))
	}
}

// dayClosedSpec reads the closing card. The day's completions and the week
// come from the canvas, so they agree with the blocks Today draws; where the
// pass sent work comes from its trail; and what Today still holds is read off
// the rows as they now stand, after every routing has landed.
func (m *OutlinerModel) dayClosedSpec(now time.Time) chrome.DayClosedCardSpec {
	canvas := m.computeDayCanvas(now)
	spec := chrome.DayClosedCardSpec{Date: now.Format("Monday, January 2"), Week: canvas.WeekReading()}
	for _, task := range canvas.Done {
		spec.Done = append(spec.Done, chrome.DayClosedDone{Title: task.Title, ProjectSlug: task.ProjectSlug})
	}
	for _, step := range m.Triage.trail {
		switch step.key {
		case "n":
			spec.Next++
		case "b":
			spec.Backlog++
		}
	}
	seen := make(map[taskref.TaskIdentity]bool)
	for _, row := range m.Rows {
		if identity := m.TaskIdentityForRow(row); closeDayRow(row) && identity.Valid() && !seen[identity] {
			seen[identity] = true
			spec.StillToday++
		}
	}
	return spec
}

// closeDayGuidance is the pass's keymap, on a card and on the closing card. It
// takes no footer rank, as no gated keymap does.
func (m *OutlinerModel) closeDayGuidance() chrome.GuidanceSpec {
	if m.Triage.closed {
		hints := chrome.RequiredGuidanceHints("Enter close")
		if len(m.Triage.trail) > 0 {
			hints = append(hints, chrome.OptionalGuidanceHints(2, "u back")...)
		}
		return chrome.GuidanceSpec{Hints: hints, Rows: 1}
	}
	hints := chrome.RequiredGuidanceHints("d done", "n next", "b backlog", "Space keep")
	hints = append(hints, chrome.OptionalGuidanceHints(3, "@ date", "r note")...)
	if len(m.Triage.trail) > 0 {
		hints = append(hints, chrome.OptionalGuidanceHints(2, "u back")...)
	}
	hints = append(hints, chrome.RequiredGuidanceHints("Esc finish")...)
	return chrome.GuidanceSpec{Hints: hints, Rows: 2}
}
