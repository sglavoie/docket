package tui

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/docket/internal/app"
	"github.com/sglavoie/docket/internal/control"
	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/storage"
	"github.com/sglavoie/docket/internal/tui/listsurface"
	"github.com/sglavoie/docket/internal/tui/operation"
	"github.com/sglavoie/docket/internal/tui/outliner"
	"github.com/sglavoie/docket/internal/tui/pulse"
	"github.com/sglavoie/docket/internal/tui/status"
	"github.com/sglavoie/docket/internal/tui/taskref"
	"github.com/sglavoie/docket/internal/util"
)

// controlUndoSuperseded is the refusal for a transaction that is no longer the
// newest undo entry: u would take back the newer change instead.
const controlUndoSuperseded = "Something newer was changed in Docket; undo it there"

// controlUndoAfterPreview runs between controlUndo's unlocked preview and the
// guarded restore. Tests alone set it, to land a competing commit there.
var controlUndoAfterPreview func()

// controlUndo undoes the named transaction through the u key's own ledger path,
// and only while it is still the newest undo entry. The preview answers the
// common refusal without retiring the user's interaction; the restore itself
// rechecks the name under the ledger lock, which is what decides.
func (m *AppModel) controlUndo(transactionID string, reply chan<- control.Reply) tea.Cmd {
	transactionID = strings.TrimSpace(transactionID)
	if transactionID == "" {
		answer(reply, false, "an undo needs a transaction ID")
		return nil
	}
	entry, ok, err := m.workspaceHistoryLedger().SelectNewestUndo(storage.GlobalScope())
	if err != nil {
		answer(reply, false, status.FormatUndoRedoRestoreError("undo", err).Error())
		return nil
	}
	if !ok || entry.TransactionID != transactionID {
		answer(reply, false, controlUndoSuperseded)
		return nil
	}
	if controlUndoAfterPreview != nil {
		controlUndoAfterPreview()
	}
	cmd, outcome, restored := m.restoreLedgerEntry(false, transactionID)
	answer(reply, restored, outcome)
	return cmd
}

// controlReveal seats the cursor on the task: in place when the current view
// can show it, otherwise through the same scope-and-All recipe `gp` uses, so
// Esc returns to the view the user left.
func (m *AppModel) controlReveal(task *models.Task, project *models.Project, reply chan<- control.Reply) tea.Cmd {
	out, ok := m.outlinerPrimary()
	if !ok || out == nil {
		answer(reply, false, "the Outliner is not open")
		return nil
	}
	// The user asked for this task; a launch still settling may not move it.
	m.lifecycle.dropLaunchFocus()
	identity := taskref.For(task, project)
	previous := out.Cursor
	out.NavigateToTaskInProject(task, project)
	answer(reply, true, "Shown: "+task.Title)
	if _, found := out.RowIndexForTask(identity); found {
		return listsurface.FlashIfJumped(previous, out.Cursor, &out.CursorFlash)
	}
	targets := operation.ResolvedOperationTargets{Targets: []operation.OperationTarget{{Task: identity, ProjectID: project.ID}}}
	if result, ok := m.coreGotoTaskProject(targets).Message.(operation.OperationCommandResultMsg); ok {
		return result.Cmd
	}
	return nil
}

// controlAcknowledge reviews one task as R does, without the advance: the
// cursor is not where the request came from.
func (m *AppModel) controlAcknowledge(task *models.Task, project *models.Project, reply chan<- control.Reply) tea.Cmd {
	if models.TaskEffectivelyHidden(project, task) {
		answer(reply, false, "task is hidden")
		return nil
	}
	if !models.TaskAllowsExplicitReview(task, time.Now()) {
		answer(reply, false, "task has no review to acknowledge")
		return nil
	}
	identity := taskref.For(task, project)
	capture := m.captureOutlinerRelocationForLocators([]app.TaskLocator{identity.Locator()})
	cmd := withOutlinerRelocationCapture(m.secondaryFeatures.TaskItem.MarkReviewed(identity), capture)
	return replyWhenCommitted(cmd, reply, "Acknowledged: "+task.Title)
}

// controlDefer defers one task by minutes from now, through the request the
// date picker's Defer section builds.
func (m *AppModel) controlDefer(task *models.Task, project *models.Project, minutes int, reply chan<- control.Reply) tea.Cmd {
	if minutes <= 0 {
		answer(reply, false, "a deferral needs a positive number of minutes")
		return nil
	}
	if refusal := app.DeferralRefusesState(task.WorkflowState); refusal != "" {
		answer(reply, false, refusal)
		return nil
	}
	until := time.Now().Add(time.Duration(minutes) * time.Minute)
	locator := taskref.For(task, project).Locator()
	request := app.TaskOperationRequest{
		Operation: app.TaskOperationSetDefer,
		Targets:   []app.TaskLocator{locator},
		Date:      &until,
	}
	capture := m.captureOutlinerRelocationForLocators([]app.TaskLocator{locator})
	cmd := withOutlinerRelocationCapture(m.secondaryFeatures.TaskItem.ExecuteOperation(request, "Set Defer", ""), capture)
	return replyWhenCommitted(cmd, reply, "Deferred until "+until.Format("15:04")+": "+task.Title)
}

// controlDone commits Done through the request wd builds, so the lifecycle
// transition, its planning cleanup, and the child guard are the key's own.
func (m *AppModel) controlDone(task *models.Task, project *models.Project, reply chan<- control.Reply) tea.Cmd {
	if task.WorkflowState.IsTerminal() {
		answer(reply, false, fmt.Sprintf("%s is already %s", task.Title, task.WorkflowState))
		return nil
	}
	locator := taskref.For(task, project).Locator()
	request := app.TaskOperationRequest{
		Operation: app.TaskOperationSetState,
		Targets:   []app.TaskLocator{locator},
		State:     models.StateDone,
	}
	capture := m.captureOutlinerRelocationForLocators([]app.TaskLocator{locator})
	cmd := withOutlinerRelocationCapture(m.secondaryFeatures.TaskItem.ExecuteOperation(request, "Change Status",
		fmt.Sprintf("Change Status · Changed %s to %s", util.Count(1, "task"), models.StateDone)), capture)
	return replyWhenCommitted(cmd, reply, "Done: "+task.Title)
}

// controlSearch answers OpSearch from the workspace the session already
// holds. It reads the Outliner's planning membership and nothing it shows, so
// it never moves the cursor, the scope, or the query the user is reading, and
// an open picker or editor has nothing to lose: it answers ahead of the busy
// guard, as a completion does.
func (m *AppModel) controlSearch(text string, limit int, reply chan<- control.Reply) {
	if m.crossOutliner == nil {
		answer(reply, false, "the Outliner is not open")
		return
	}
	switch {
	case limit <= 0:
		limit = control.SearchDefaultLimit
	case limit > control.SearchMaxLimit:
		limit = control.SearchMaxLimit
	}
	found := m.crossOutliner.SearchWorkspace(text, limit, m.ensurePulseLoader())
	if found.Guidance != "" {
		answer(reply, false, found.Guidance)
		return
	}
	now := time.Now()
	results := &control.SearchResults{Items: make([]control.SearchItem, 0, len(found.Matches)), Total: found.Total, Relaxed: found.Relaxed}
	for _, match := range found.Matches {
		results.Items = append(results.Items, controlSearchItem(match, now))
	}
	reply <- control.Reply{OK: true, Search: results}
}

func controlSearchItem(match outliner.WorkspaceSearchMatch, now time.Time) control.SearchItem {
	task, project := match.Task, match.Project
	item := control.SearchItem{
		Project:     project.ID,
		Task:        task.ID,
		Ref:         project.Slug + "#" + strconv.Itoa(task.ShortID),
		Title:       task.Title,
		ProjectName: project.Name,
		State:       string(task.State()),
		Parents:     match.Breadcrumb,
		Match:       match.Match,
		DueHasTime:  task.DueHasTime(),
		Overdue:     task.IsOverdueAt(now),
		Reviewable:  models.TaskAllowsExplicitReview(task, now),
	}
	if task.Priority >= models.PriorityHigh && task.Priority <= models.PriorityLow {
		item.Priority = task.Priority
	}
	if task.DueDate != nil {
		item.Due = task.DueDate.Format(time.RFC3339)
	}
	return item
}

// controlIndex answers OpIndex from the same population a sigil-free search
// reads. The revision hashes the items themselves, so it moves exactly when
// something a host indexed would change and no fingerprint of the files has
// to agree with it.
func (m *AppModel) controlIndex(revision string, reply chan<- control.Reply) {
	if m.crossOutliner == nil {
		answer(reply, false, "the Outliner is not open")
		return
	}
	found := m.crossOutliner.IndexWorkspace(control.IndexMaxItems, m.ensurePulseLoader())
	now := time.Now()
	items := make([]control.IndexItem, 0, len(found.Matches))
	for _, match := range found.Matches {
		items = append(items, control.IndexItem{
			SearchItem:  controlSearchItem(match, now),
			Description: indexDescription(match.Task.Description),
			Tags:        match.Task.Tags,
			Context:     match.Task.Context,
		})
	}
	encoded, err := json.Marshal(items)
	if err != nil {
		answer(reply, false, "could not encode the index: "+err.Error())
		return
	}
	sum := sha256.Sum256(encoded)
	current := hex.EncodeToString(sum[:16])
	if revision == current {
		reply <- control.Reply{OK: true, Index: &control.IndexResults{Revision: current, Unchanged: true}}
		return
	}
	reply <- control.Reply{OK: true, Index: &control.IndexResults{Revision: current, Items: items, Total: found.Total}}
}

// indexDescription is at most IndexDescriptionRunes of a description, cut at a
// rune so the JSON stays valid text.
func indexDescription(description string) string {
	description = strings.TrimSpace(description)
	if utf8.RuneCountInString(description) <= control.IndexDescriptionRunes {
		return description
	}
	return string([]rune(description)[:control.IndexDescriptionRunes])
}

// controlProjects lists the projects a host may focus on: the base workspace,
// the population every count it hears is read from.
func (m *AppModel) controlProjects(reply chan<- control.Reply) {
	if m.domain.runtime == nil {
		answer(reply, false, "the workspace is unavailable")
		return
	}
	projects := m.domain.runtime.TaskWorkspaceProjects()
	listed := make([]control.Project, 0, len(projects))
	for _, project := range projects {
		if project != nil {
			listed = append(listed, control.Project{ID: project.ID, Name: project.Name, Slug: project.Slug})
		}
	}
	reply <- control.Reply{OK: true, Projects: &control.ProjectList{Items: listed}}
}

// controlFocus narrows what the host is told to the named projects, or clears
// the narrowing. It recounts at once, so the host hears the focused pulse,
// agenda, and schedule from this same Update's deferred publish.
func (m *AppModel) controlFocus(projectIDs []string, reply chan<- control.Reply) {
	if len(projectIDs) == 0 {
		m.control.focus = nil
		m.refreshWorkspacePulseAt(time.Now())
		answer(reply, true, "Focus cleared")
		return
	}
	known := map[string]bool{}
	if m.domain.runtime != nil {
		for _, project := range m.domain.runtime.TaskWorkspaceProjects() {
			if project != nil {
				known[project.ID] = true
			}
		}
	}
	focus := map[string]bool{}
	for _, id := range projectIDs {
		if known[id] {
			focus[id] = true
		}
	}
	if len(focus) == 0 {
		m.control.focus = nil
		m.refreshWorkspacePulseAt(time.Now())
		answer(reply, false, "none of those projects is in the workspace, so nothing is narrowed")
		return
	}
	m.control.focus = focus
	m.refreshWorkspacePulseAt(time.Now())
	answer(reply, true, "Focus: "+util.Count(len(focus), "project"))
}

// refreshControlFocus recounts the readout a focused host hears, beside the
// workspace's own recount.
func (m *AppModel) refreshControlFocus(now time.Time) {
	if m.control.focus == nil || !m.control.enabled() {
		m.control.focused = nil
		return
	}
	readout := hostReadout{}
	readout.pulse, readout.agenda, readout.upcoming = outliner.ComputeFocusedReadout(
		m.domain.runtime, m.control.focus, m.todayIDs, now, m.ensurePulseLoader())
	m.control.focused = &readout
}

// publishControlArrivals sends one tick's arrivals with each task's identity,
// so the host can offer to act on it.
func (m *AppModel) publishControlArrivals(arrivals []pulse.Arrival) {
	if !m.control.enabled() {
		return
	}
	if m.control.focus != nil {
		kept := arrivals[:0:0]
		for _, arrival := range arrivals {
			if m.control.focus[arrival.ProjectID] {
				kept = append(kept, arrival)
			}
		}
		arrivals = kept
	}
	if len(arrivals) == 0 {
		return
	}
	title, body := pulse.ArrivalNotification(arrivals)
	event := control.Event{Event: control.EventArrivals, Title: title, Body: body}
	for _, arrival := range arrivals {
		event.Arrivals = append(event.Arrivals, controlArrival(arrival))
	}
	m.control.publisher.Publish(event)
}

func controlArrival(arrival pulse.Arrival) control.Arrival {
	return control.Arrival{
		Kind:       controlArrivalKind(arrival.Kind),
		Project:    arrival.ProjectID,
		Task:       arrival.TaskID,
		Title:      arrival.Title,
		Reviewable: arrival.Reviewable,
	}
}

func controlArrivalKind(kind pulse.ArrivalKind) string {
	switch kind {
	case pulse.ArrivalCheckIn:
		return control.ArrivalCheckIn
	case pulse.ArrivalDeadline:
		return control.ArrivalDeadline
	default:
		return control.ArrivalWake
	}
}
