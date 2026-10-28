package tui

import (
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/docket/internal/tui/operation"
	"github.com/sglavoie/docket/internal/tui/operationhost"
	"github.com/sglavoie/docket/internal/tui/route"
)

// =============================================================================
// Interaction ownership of delayed results
// =============================================================================

// interactionOwnership is the shell's memory of which user interaction owns the
// asynchronous work still in flight. It counts interactions and holds nothing
// else: no queue, no draft, no pending offer. A command carries a value copy of
// that count, so no command goroutine reads shell state, and a result arriving
// after the user has moved on is recognizable as the previous interaction's.
//
// The count is the explicit check-in editor's own safeguard and is not the
// automatic offer's: a refused save may restore its draft only on the
// interaction that submitted it, whether or not anything is queued. The offer
// queue is one consumer of the same ownership, which is why retiring an
// interaction also drops it.
type interactionOwnership struct {
	epoch uint64
}

// commandOriginMsg is a delayed result still carrying the interaction that
// produced it. The durable message inside it always reaches its ordinary
// refresh/feedback handler; ownership decides only what may reopen a draft.
type commandOriginMsg struct {
	message tea.Msg
	epoch   uint64
}

// originTaggedCommandMsg keeps the original command visible to command runners
// (including the TUI harness, which skips animation commands) while it carries
// the interaction that issued it. Executing it tags only the results ownership
// is asked about, leaving framework messages to Bubble Tea.
type originTaggedCommandMsg struct {
	cmd   tea.Cmd
	epoch uint64
}

func withCommandOrigin(cmd tea.Cmd, epoch uint64) tea.Cmd {
	if cmd == nil {
		return nil
	}
	return func() tea.Msg { return originTaggedCommandMsg{cmd: cmd, epoch: epoch} }
}

func runOriginTaggedCommand(command originTaggedCommandMsg) tea.Cmd {
	return func() tea.Msg {
		msg := command.cmd()
		epoch := command.epoch
		if batch, ok := msg.(tea.BatchMsg); ok {
			cmds := make([]tea.Cmd, len(batch))
			for i, child := range batch {
				cmds[i] = withCommandOrigin(child, epoch)
			}
			return tea.BatchMsg(cmds)
		}
		if originTaggedMessage(msg) {
			return commandOriginMsg{message: msg, epoch: epoch}
		}
		return msg
	}
}

// originTaggedMessage is the closed set of delayed results whose owning
// interaction matters. A refused check-in save is the one that matters on its
// own terms, since it carries a draft and frozen targets it may hand back only
// to the interaction that submitted them. The three operation envelopes beside
// it are the hops a submission takes from the keypress to that refusal — request
// to editor, selected value to handler, handler result to commit — so tagging
// them is what carries the epoch across the round trip instead of minting a
// fresh one at each hop. Committed transaction evidence is deliberately absent:
// nothing downstream of a commit claims the keyboard.
func originTaggedMessage(msg tea.Msg) bool {
	switch msg.(type) {
	case operationhost.CheckInRejectedMsg,
		operation.OperationCommandResultMsg,
		operation.OperationValueSelectedMsg,
		operation.OperationValueRequestMsg:
		return true
	default:
		return false
	}
}

// beginCommandOriginUpdate resolves the interaction this update belongs to. An
// unrelated input retires the interaction the work in flight belonged to; a
// tagged result is unwrapped and reports the interaction it came from, so the
// caller can tell an authorized continuation from a late one.
func (m *AppModel) beginCommandOriginUpdate(msg tea.Msg) (tea.Msg, uint64, bool) {
	if m.interactionIsRetiredBy(msg) {
		m.retireInteraction()
	}
	if wrapped, ok := msg.(commandOriginMsg); ok {
		// A refused save may restore its draft only on the interaction that owns
		// it: reopening an editor over whatever the user moved to would take the
		// keyboard back for work already abandoned.
		if _, rejected := wrapped.message.(operationhost.CheckInRejectedMsg); rejected && !m.ownsInteraction(wrapped.epoch) {
			return nil, wrapped.epoch, false
		}
		return wrapped.message, wrapped.epoch, true
	}
	return msg, m.interactionEpoch(), false
}

// interactionIsRetiredBy reports whether this message is the user moving on from
// the interaction that owns the work in flight.
func (m *AppModel) interactionIsRetiredBy(msg tea.Msg) bool {
	switch typed := msg.(type) {
	case tea.KeyPressMsg:
		// Submission guards swallow duplicate Enter/Ctrl+S while a commit runs.
		// Those keys are not unrelated navigation and must not retire the
		// interaction that is still submitting.
		duplicate := (typed.String() == "enter" || typed.String() == "ctrl+s") &&
			((m.taskComposer != nil && m.taskComposer.SubmissionInFlight) ||
				(m.crossOutliner != nil && m.crossOutliner.QuickEdit.Pending))
		return !duplicate && !isModifierOnlyKeyPress(typed)
	case tea.PasteMsg, tea.MouseClickMsg, tea.MouseWheelMsg:
		// A click or a wheel step is the user moving on, the way a key is.
		// Motion and release count for nothing.
		return true
	default:
		return false
	}
}

func (m *AppModel) interactionEpoch() uint64 { return m.interaction.epoch }

// ownsInteraction reports whether the interaction that issued this work is still
// the one in charge of the keyboard.
func (m *AppModel) ownsInteraction(epoch uint64) bool { return epoch == m.interaction.epoch }

// retireInteraction ends the interaction that owned the work in flight, so its
// delayed results can no longer claim the keyboard. Retiring costs nothing else:
// the keyboard is the whole of what an interaction owns, and committed evidence
// belongs to the workspace rather than to whoever was still holding it.
func (m *AppModel) retireInteraction() {
	m.interaction.epoch++
}

// isInteractionInput reports whether the message is a user input rather than a
// timer, animation or reload continuation, whose ordinary command lifecycle
// carries no origin.
func isInteractionInput(msg tea.Msg) bool {
	switch msg.(type) {
	case tea.KeyPressMsg, tea.PasteMsg, tea.MouseClickMsg, tea.MouseWheelMsg:
		return true
	default:
		return false
	}
}

// The pointer seam. Bubble Tea delivers clicks, releases, wheel steps and
// motion; this file keeps clicks and vertical wheel steps, drops the rest where
// they arrive, folds what it keeps into one route.PointerEvent, and hands that
// event to the layer a key press would reach. The pointer only re-expresses
// keys, so it adds no operation and no vocabulary of its own: whoever answers it
// does so through the operation the equivalent key reaches.
//
// A click or a wheel step is a keystroke for every rule that treats keystrokes
// as the user's input: it retires the interaction epoch (above),
// drops the launch focus intent, and abandons a pending chord. The abandonment is
// silent, because a click names no path a warning could quote.

// pointerDoubleClickInterval is the macOS default: a second press on the same
// cell with the same button no later than this after the first is a double-click.
const pointerDoubleClickInterval = 500 * time.Millisecond

// pointerState is the shell's whole memory of the pointer: the session's on/off
// value and the last click, which is all a double-click test needs. Detection is
// a timestamp comparison on the app's clock, never a timer.
type pointerState struct {
	// disabled turns mouse reporting off for this session. The machine-local
	// setting and the process-level `--no-mouse` flag feed it; see coreToggleMouse in app_advisories.go.
	disabled bool
	// clock is the time double-click detection reads. Nil reads the service's
	// clock; the harness points it at its virtual clock, so a scenario's `wait`
	// decides whether two clicks were one double-click.
	clock func() time.Time

	lastClickAt     time.Time
	lastClickX      int
	lastClickY      int
	lastClickButton route.PointerButton

	// overlays is where the last frame composited each shell popup, which is
	// the translation a click needs before the popup owning keys reads it.
	overlays []placedOverlay
}

// pointerNow is the instant double-click detection compares against.
func (m *AppModel) pointerNow() time.Time {
	if m.pointer.clock != nil {
		return m.pointer.clock()
	}
	return m.serviceNow()
}

// pointerEventFromMsg folds one tea mouse message into the pointer event the
// shell routes, reporting false for every message that counts for nothing:
// motion, release, a horizontal wheel step, and a press of a button the
// grammar never reads.
func (m *AppModel) pointerEventFromMsg(msg tea.Msg) (route.PointerEvent, bool) {
	switch msg := msg.(type) {
	case tea.MouseClickMsg:
		button, ok := pointerButton(msg.Button)
		if !ok {
			return route.PointerEvent{}, false
		}
		event := route.PointerEvent{X: msg.X, Y: msg.Y, Button: button, Clicks: 1}
		event.Clicks = m.countPointerClick(event)
		return event, true
	case tea.MouseWheelMsg:
		var delta int
		switch msg.Button {
		case tea.MouseWheelUp:
			delta = -1
		case tea.MouseWheelDown:
			delta = 1
		default:
			return route.PointerEvent{}, false
		}
		// A wheel step between two clicks separates them: the second is no
		// longer the other half of a double-click on what the first one hit.
		m.pointer.lastClickAt = time.Time{}
		return route.PointerEvent{X: msg.X, Y: msg.Y, WheelDelta: delta}, true
	default:
		return route.PointerEvent{}, false
	}
}

func pointerButton(button tea.MouseButton) (route.PointerButton, bool) {
	switch button {
	case tea.MouseLeft:
		return route.PointerButtonLeft, true
	case tea.MouseRight:
		return route.PointerButtonRight, true
	case tea.MouseMiddle:
		return route.PointerButtonMiddle, true
	default:
		return route.PointerButtonNone, false
	}
}

// countPointerClick records a click and reports whether it completes a
// double-click. A double-click spends the memory, so a third quick press starts
// over as a single click rather than reading as a second double-click.
func (m *AppModel) countPointerClick(event route.PointerEvent) int {
	now := m.pointerNow()
	last := m.pointer
	double := !last.lastClickAt.IsZero() &&
		last.lastClickX == event.X && last.lastClickY == event.Y &&
		last.lastClickButton == event.Button &&
		!now.Before(last.lastClickAt) && now.Sub(last.lastClickAt) <= pointerDoubleClickInterval
	if double {
		m.pointer.lastClickAt = time.Time{}
		return 2
	}
	m.pointer.lastClickAt = now
	m.pointer.lastClickX, m.pointer.lastClickY = event.X, event.Y
	m.pointer.lastClickButton = event.Button
	return 1
}

// handlePointerMessage is the seam's entry from handleGlobalInputMessage. Every
// mouse message is consumed here, whether it becomes an event or not.
func (m *AppModel) handlePointerMessage(msg tea.Msg) (bool, tea.Cmd) {
	// Reporting off means the terminal sends nothing, so a message that still
	// arrives (a host forwarding regardless) is not the user's input either.
	if m.pointer.disabled {
		return true, nil
	}
	event, ok := m.pointerEventFromMsg(msg)
	if !ok {
		return true, nil
	}
	m.lifecycle.dropLaunchFocus()
	return true, m.routePointer(event)
}

// routePointer hands the event to the layer that owns the next key press, in
// the order activeGuidance resolves it: a standing confirmation, a shell-mounted
// modal, a pending chord, then the presentation. An owner that does not answer
// the pointer swallows the event, so a click never falls through a modal to the
// host drawn beneath it.
func (m *AppModel) routePointer(event route.PointerEvent) tea.Cmd {
	if m.lifecycle.quitting || m.viewNav.tooSmall() {
		return nil
	}
	if m.shellConfirmationArmed() {
		return nil
	}
	if m.shellModalMounted() {
		return m.routeOverlayPointer(event)
	}
	if len(m.operations.Chord) > 0 {
		// The chord owned the next key, and the click was that key: the path is
		// abandoned, silently, and the press reaches nothing else.
		m.operations.Chord = nil
		return nil
	}
	view := m.activeView()
	if view == nil {
		return nil
	}
	answerer, ok := view.(route.PointerView)
	if !ok {
		return nil
	}
	// The active presentation is drawn from the screen's top-left cell, so its
	// own coordinates are the screen's; a surface drawn elsewhere translates
	// through route.PointerEvent.Offset.
	out, _ := m.outlinerPrimary()
	menuOpen := out != nil && out.MenuBar.Open
	cmd := answerer.HandlePointer(event)
	if menuOpen && !out.MenuBar.Open {
		// The click that put a menu away did nothing more, so it is no half of a
		// double-click either: a quick second press is a click of its own.
		m.pointer.lastClickAt = time.Time{}
	}
	return cmd
}

// routeOverlayPointer hands the event to the shell popup that owns keys, in
// handleActiveKeyOverlay's order, translated into that popup's own cells. An
// event outside the popup and a popup that does not answer the pointer swallow
// it: the host beneath never receives a click while a popup stands over it. A
// click also needs the popup drawn whole, since a row fitting removed would
// otherwise name the row below it; a wheel step reads no row.
func (m *AppModel) routeOverlayPointer(event route.PointerEvent) tea.Cmd {
	owner := m.overlayPointerOwner()
	answerer, ok := owner.(route.PointerView)
	if !ok {
		return nil
	}
	placement, drawn := m.overlayPlacement(owner)
	if !drawn || !placement.Contains(event.X, event.Y) || (!placement.Whole && !event.IsWheel()) {
		return nil
	}
	return answerer.HandlePointer(event.Offset(placement.Col, placement.Row))
}

// overlayPointerOwner is the popup the next key press would reach. The text
// editors and the check-in setup are named so they win and swallow, which is
// what a text input does with the pointer.
func (m *AppModel) overlayPointerOwner() widthRenderer {
	switch {
	case m.checkInSetup != nil:
		return nil
	case m.operations.CheckInEditor != nil:
		return m.operations.CheckInEditor
	case m.operations.TextPrompt != nil:
		return m.operations.TextPrompt
	case m.operations.ValuePicker != nil:
		return m.operations.ValuePicker
	case m.operations.ActionSheet != nil:
		return m.operations.ActionSheet
	case m.lenses.Mounted():
		return m.lenses.Picker()
	case m.conflictBrowser != nil:
		return m.conflictBrowser
	}
	return nil
}
