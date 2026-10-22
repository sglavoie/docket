package tui

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/control"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/coordinator"
	"github.com/sglavoie/tt/internal/tui/pulse"
	"github.com/sglavoie/tt/internal/tui/status"
	"github.com/sglavoie/tt/internal/tui/taskref"
	"github.com/sglavoie/tt/internal/tui/theme"
)

// =============================================================================
// The control socket: a host asking the running session to act
// =============================================================================
//
// `tt --control PATH` serves internal/control's socket for a host such as
// tt.app. Every request arrives here as a controlRequestMsg and is answered
// from inside Update, so it reads the same state a keystroke would and commits
// through the same coordinator, transaction, and undo ledger. A request is
// refused, never queued, while the user is mid-interaction: the guard is the
// external reload's, since a commit reloads the projects a picker, an editor, or
// a selection is acting on.

// controlPublisher is the socket's outbound half, as the shell sees it.
type controlPublisher interface {
	Publish(control.Event)
}

// controlState is the shell's link to the socket, nil-safe when tt runs
// without one.
type controlState struct {
	versionLine string
	publisher   controlPublisher
	// published is the last pulse sent, so an unchanged pulse is never resent.
	published   *pulse.WorkspacePulse
	editing     *control.Editing
	textHistory map[any]*desktopTextHistory
}

// WithControlPublisher connects the session to a control socket's events.
// Arrivals are reported from launch on, as --notify reports them.
func WithControlPublisher(publisher controlPublisher) AppOption {
	return func(m *AppModel) {
		m.control.publisher = publisher
		if m.notifications.since.IsZero() {
			m.notifications.since = time.Now()
		}
	}
}

func (c *controlState) enabled() bool { return c != nil && c.publisher != nil }

// controlRequestMsg carries one request and the channel its reply goes to.
// The channel is buffered, so answering never blocks Update.
type controlRequestMsg struct {
	request control.Request
	reply   chan<- control.Reply
}

// controlDispatch is the socket's Dispatch for a running program: it hands the
// request to Update and returns where the answer will arrive.
func controlDispatch(send func(tea.Msg)) control.Dispatch {
	return func(request control.Request) <-chan control.Reply {
		replies := make(chan control.Reply, 1)
		send(controlRequestMsg{request: request, reply: replies})
		return replies
	}
}

// controlCommittedMsg is a socket request's resolved commit: the coordinator's
// own message, handled as the save pipeline handles it, plus the undo toast a
// capture owes. A capture travels as TasksCapturedMsg rather than
// TaskCreatedMsg, because that one pops the live view and the user was not in
// a composer.
type controlCommittedMsg struct {
	inner    tea.Msg
	feedback string
	txID     string
}

func answer(reply chan<- control.Reply, ok bool, message string) {
	reply <- control.Reply{OK: ok, Message: message}
}

// handleControlMessage answers the socket's requests and its own capture
// results.
func (m *AppModel) handleControlMessage(msg tea.Msg) (bool, tea.Cmd) {
	switch msg := msg.(type) {
	case controlRequestMsg:
		return true, m.answerControlRequest(msg.request, msg.reply)
	case controlCaptureCheckedMsg:
		if msg.err != nil {
			answer(msg.reply, false, msg.err.Error())
			return true, nil
		}
		if msg.receipt != nil {
			msg.reply <- captureReceiptReply(*msg.receipt)
			return true, nil
		}
		return true, m.executeControlRequest(msg.request, msg.reply)
	case controlCommittedMsg:
		return true, m.adoptControlCommit(msg)
	default:
		return false, nil
	}
}

func (m *AppModel) answerControlRequest(request control.Request, reply chan<- control.Reply) tea.Cmd {
	if request.Op == control.OpCaptureContext {
		if m.domain.runtime == nil || m.domain.runtime.Catalog().WorkspaceID == "" {
			answer(reply, false, "workspace identity is unavailable")
		} else {
			reply <- control.Reply{OK: true, Workspace: m.domain.runtime.Catalog().WorkspaceID}
		}
		return nil
	}
	if request.Op == control.OpCaptureOnce {
		return m.lookupControlCapture(request, reply)
	}
	return m.executeControlRequest(request, reply)
}

func (m *AppModel) executeControlRequest(request control.Request, reply chan<- control.Reply) tea.Cmd {
	switch request.Op {
	case control.OpVersion:
		reply <- control.Reply{OK: true, Version: m.control.versionLine}
		return nil
	case control.OpEdit:
		return m.controlEdit(request.Text, reply)
	case control.OpComplete:
		// Ahead of the busy guard: a completion reads the workspace and commits
		// nothing, so there is nothing for an open picker or editor to lose.
		m.controlComplete(request.Text, reply)
		return nil
	case control.OpPreview:
		m.controlPreview(request.Text, reply)
		return nil
	case control.OpAppearance:
		// Ahead of the busy guard too: a theme changes what is drawn and
		// nothing an open picker or editor holds.
		controlAppearance(request.Text, reply)
		return nil
	case control.OpThemePalette:
		m.controlThemePalette(request.Text, reply)
		return nil
	case control.OpQuit:
		if reason := m.quitBlockReason(); reason != "" {
			answer(reply, false, "tt is busy: "+reason)
			return nil
		}
		answer(reply, true, "quitting")
		return func() tea.Msg { return status.QuitMsg{} }
	case control.OpReveal, control.OpAcknowledge, control.OpDefer, control.OpCapture, control.OpCaptureOnce, control.OpView, control.OpUndo, control.OpDone:
	default:
		answer(reply, false, fmt.Sprintf("unknown operation %q", request.Op))
		return nil
	}
	if !m.safeToAdoptExternalChange() {
		answer(reply, false, "tt is busy: "+m.controlBusyReason(request.Op))
		return nil
	}
	if request.Op == control.OpView {
		// A mode change reads the workspace it already holds and commits
		// nothing, so it owes no reload of its own.
		m.controlView(request.Text, reply)
		return nil
	}
	if request.Op == control.OpUndo {
		// The restore reloads every project the entry touched, so it owes no
		// blurred-workspace reload of its own.
		return m.controlUndo(request.Text, reply)
	}
	// A blurred session may be holding a workspace another machine changed;
	// acting on it would commit against a stale read and record a conflict.
	reload := m.adoptBlurredWorkspaceChange()

	var cmd tea.Cmd
	switch request.Op {
	case control.OpCapture, control.OpCaptureOnce:
		if request.Op == control.OpCaptureOnce {
			submission := storage.CaptureSubmission{ID: request.CaptureID, Digest: fmt.Sprintf("%x", sha256.Sum256([]byte(request.Text)))}
			cmd = m.controlCapture(request.Text, reply, submission)
		} else {
			cmd = m.controlCapture(request.Text, reply)
		}
	default:
		identity := taskref.TaskIdentity{ProjectID: request.Project, TaskID: request.Task}
		task, project := m.taskAndProjectForIdentity(identity)
		if task == nil || project == nil {
			answer(reply, false, "that task is no longer in the workspace")
			return reload
		}
		switch request.Op {
		case control.OpReveal:
			cmd = m.controlReveal(task, project, reply)
		case control.OpAcknowledge:
			cmd = m.controlAcknowledge(task, project, reply)
		case control.OpDefer:
			cmd = m.controlDefer(task, project, request.Minutes, reply)
		case control.OpDone:
			cmd = m.controlDone(task, project, reply)
		}
	}
	return tea.Batch(reload, cmd)
}

// controlViewModes are the modes a host may name: the three the pulse counts.
var controlViewModes = map[string]chrome.OutlinerModeID{
	"inbox": chrome.ModeInbox,
	"today": chrome.ModeToday,
	"due":   chrome.ModeDue,
}

// controlView switches the Outliner to the named mode through the mode jump's
// own SelectOutlinerMode. The busy guard already refused the ranked list, where
// that key refuses too.
func (m *AppModel) controlView(name string, reply chan<- control.Reply) {
	id, known := controlViewModes[name]
	if !known {
		answer(reply, false, fmt.Sprintf("unknown view %q: want inbox, today, or due", name))
		return
	}
	out, ok := m.outlinerPrimary()
	if !ok || out == nil {
		answer(reply, false, "the Outliner is not open")
		return
	}
	// The user asked for this mode; a launch still settling may not move it.
	m.lifecycle.dropLaunchFocus()
	_, _ = out.SelectOutlinerMode(id)
	mode, _ := chrome.OutlinerModeByID(id)
	answer(reply, true, "Shown: "+mode.Label)
}

// replyWhenCommitted answers the socket once a coordinator command's commit
// has resolved, and hands its message back to Update.
func replyWhenCommitted(cmd tea.Cmd, reply chan<- control.Reply, success string) tea.Cmd {
	if cmd == nil {
		answer(reply, false, "nothing to do")
		return nil
	}
	return func() tea.Msg {
		msg := cmd()
		if refusal := commitRefusal(msg); refusal != "" {
			answer(reply, false, refusal)
		} else {
			answer(reply, true, success)
		}
		return controlCommittedMsg{inner: msg}
	}
}

// commitRefusal is the error a coordinator's message carries, or "" when it
// committed.
func commitRefusal(msg tea.Msg) string {
	var err error
	switch msg := msg.(type) {
	case coordinator.TransactionErrorMsg:
		err = msg.Err
	case status.ErrorMsg:
		err = msg.Err
	case coordinator.TransactionResultMsg, coordinator.BatchTransactionResultMsg, coordinator.TasksCapturedMsg:
		return ""
	default:
		err = errors.New("tt did not commit the change")
	}
	if err == nil {
		return "tt did not commit the change"
	}
	return err.Error()
}

// =============================================================================
// Events: the pulse and arrivals
// =============================================================================

// publishControlPulse sends the pulse when it changed since the last one sent.
// It runs in Update's one deferred closure, after every refresh a message did.
func (m *AppModel) publishControlPulse() {
	if !m.control.enabled() {
		return
	}
	current := m.workspacePulse
	if m.control.published != nil && *m.control.published == current {
		return
	}
	m.control.published = &current
	m.control.publisher.Publish(control.Event{
		Event: control.EventPulse,
		Pulse: &control.Pulse{Inbox: current.Inbox, Today: current.Today, Overdue: current.Overdue, Done: current.Done},
	})
}

// publishControlArrivals sends one tick's arrivals with each task's identity,
// so the host can offer to act on it.
func (m *AppModel) publishControlArrivals(arrivals []pulse.Arrival) {
	if !m.control.enabled() || len(arrivals) == 0 {
		return
	}
	title, body := pulse.ArrivalNotification(arrivals)
	event := control.Event{Event: control.EventArrivals, Title: title, Body: body}
	for _, arrival := range arrivals {
		event.Arrivals = append(event.Arrivals, control.Arrival{
			Kind:       controlArrivalKind(arrival.Kind),
			Project:    arrival.ProjectID,
			Task:       arrival.TaskID,
			Title:      arrival.Title,
			Reviewable: arrival.Reviewable,
		})
	}
	m.control.publisher.Publish(event)
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

// controlAppearance switches the session between the dark and light themes.
func controlAppearance(text string, reply chan<- control.Reply) {
	if text != "dark" && text != "light" {
		answer(reply, false, fmt.Sprintf("unknown appearance %q: want dark or light", text))
		return
	}
	applySessionTheme(text)
	answer(reply, true, "Theme: "+text)
}

// controlThemePalette applies a Ghostty palette, or restores the machine's
// saved theme when the request carries none.
func (m *AppModel) controlThemePalette(text string, reply chan<- control.Reply) {
	if text == "" {
		if m.domain.runtime == nil || m.domain.runtime.Machine() == nil {
			answer(reply, false, "machine theme is unavailable")
			return
		}
		applySessionTheme(m.domain.runtime.Machine().Theme)
		answer(reply, true, "Theme: saved")
		return
	}
	var palette theme.GhosttyPalette
	if err := json.Unmarshal([]byte(text), &palette); err != nil {
		answer(reply, false, "invalid theme palette: "+err.Error())
		return
	}
	if err := theme.ApplyGhosttyPalette(palette); err != nil {
		answer(reply, false, "invalid theme palette: "+err.Error())
		return
	}
	answer(reply, true, "Theme: "+palette.Name)
}

// controlBusyReason says what to close before a request that would adopt an
// external change can run; navigation names the specific obstacle.
func (m *AppModel) controlBusyReason(op string) string {
	if op != control.OpReveal && op != control.OpView {
		return "finish what is open in tt first"
	}
	switch {
	case m.quitBlockReason() != "":
		return m.quitBlockReason() + " Save or cancel it before navigating."
	case m.crossOutliner != nil && (m.crossOutliner.IsSearchMode() || m.crossOutliner.HasSearchProjection()):
		return "Close search in tt, then try again."
	case m.crossOutliner != nil && m.crossOutliner.MultiSelectMode:
		return "Clear the selection in tt, then try again."
	default:
		return "Close the current panel in tt, then try again."
	}
}
