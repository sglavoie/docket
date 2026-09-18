package tui

import (
	"fmt"
	"runtime/debug"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/sglavoie/tt/internal/logging"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/tui/operationhost"
	"github.com/sglavoie/tt/internal/tui/projecthub"
	"github.com/sglavoie/tt/internal/tui/projectops"
	"github.com/sglavoie/tt/internal/tui/route"
	"github.com/sglavoie/tt/internal/tui/status"
)

func (m *AppModel) Update(msg tea.Msg) (outModel tea.Model, outCmd tea.Cmd) { //nolint:gocyclo
	// Every branch below returns early, so the one deferred closure is also the
	// single place the footer divider flash can compare status text before and
	// after dispatch. It must stay one defer: a second one would run first
	// under LIFO, before recovery and before outCmd is assigned.
	msg, origin, interactive := m.beginCommandOriginUpdate(msg)
	statusBefore := m.captureFooterFlashProbe()
	defer func() {
		if r := recover(); r != nil {
			m.retireInteraction()
			stack := debug.Stack()
			logging.Warn("TUI panic in Update: %v\n%s", r, stack)
			m.feedback.err = fmt.Errorf("internal error: %v", r)
			if m.crossOutliner != nil {
				m.crossOutliner.CancelCrossProjectLoad()
			}
			m.viewNav.navigation.ResetRoot()
			m.operations.ClearOverlays()
			m.checkInSetup = nil
			m.lenses.Close()
			m.feedback.message = ""
			outModel = m
			outCmd = nil
			// The recovery already rebuilt navigation; probing it again here
			// would risk a second panic with no recover left above it.
			return
		}
		if interactive && m.ownsInteraction(origin) {
			m.observeCheckInOfferMessage(msg)
		}
		outCmd = tea.Batch(outCmd, m.drainCheckInOffer())
		if interactive || isInteractionInput(msg) {
			outCmd = withCommandOrigin(outCmd, origin)
		}
		// Every branch above returns early, so this closure is also the one place
		// a bottom panel's host can be resized exactly when the panel opened,
		// moved, or closed rather than on every frame.
		m.syncHostedPanelHost()
		// And the one place the footer this update produced can be measured
		// before it is drawn, so the surface underneath reserves the rows it
		// actually costs instead of the two it was sized against.
		m.syncShellFrameMetrics()
		if flash := m.armFooterFlashSince(statusBefore); flash != nil {
			outCmd = tea.Batch(outCmd, flash)
		}
		if expiry := m.scheduleViewStatusExpiry(statusBefore); expiry != nil {
			outCmd = tea.Batch(outCmd, expiry)
		}
	}()

	if command, ok := msg.(originTaggedCommandMsg); ok {
		return m, runOriginTaggedCommand(command)
	}

	// Handle all trivial close-view messages via the CloseViewMsg interface.
	// This eliminates individual case arms for ~35 Close*Msg types that only
	// pop the view stack.
	if cv, ok := msg.(route.CloseViewMsg); ok {
		var cmd tea.Cmd
		switch cv.ClosePopStyle() {
		case route.PopToAnchor:
			cmd = m.popToAnchorView()
		default:
			cmd = m.popView()
		}
		return m, cmd
	}

	if saved, ok := msg.(checkInSetupSavedMsg); ok {
		return m, m.handleCheckInSetupSaved(saved)
	}

	if handled, cmd := m.handleGlobalMessage(msg); handled {
		return m, cmd
	}
	if handled, cmd := m.handleManageHubMessages(msg); handled {
		return m, cmd
	}

	if handled, cmd := m.handleProjectMessages(msg); handled {
		return m, cmd
	}
	if handled, cmd := m.handleTaskMessages(msg); handled {
		return m, cmd
	}
	if handled, cmd := m.handleViewMessages(msg); handled {
		return m, cmd
	}
	if handled, cmd := m.handleTagMessages(msg); handled {
		return m, cmd
	}
	if handled, cmd := m.handleClipboardMessages(msg); handled {
		return m, cmd
	}
	return m, tea.Batch(m.delegateToCurrentView(msg))
}

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
// interaction matters. A refused check-in save is one on its own terms, since it
// carries a draft and frozen targets it may hand back only to the interaction
// that submitted them; the committed evidence beside it is tagged for the
// automatic offer, which names its own set.
func originTaggedMessage(msg tea.Msg) bool {
	if _, rejected := msg.(operationhost.CheckInRejectedMsg); rejected {
		return true
	}
	return checkInOfferObservedMessage(msg)
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
	case tea.PasteMsg, tea.MouseClickMsg:
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
// delayed results can no longer claim the keyboard.
func (m *AppModel) retireInteraction() {
	m.interaction.epoch++
	m.forgetInteractionScopedWork()
}

// isInteractionInput reports whether the message is a user input rather than a
// timer, animation or reload continuation, whose ordinary command lifecycle
// carries no origin.
func isInteractionInput(msg tea.Msg) bool {
	switch msg.(type) {
	case tea.KeyPressMsg, tea.PasteMsg, tea.MouseClickMsg:
		return true
	default:
		return false
	}
}

// keyInterceptor is implemented by overlay widgets that intercept key presses
// and return (inputCmd, dismissed, resultMsg). The method is exported because
// an overlay may live in a sub-package: an interface naming an unexported
// method can only ever be satisfied from inside the package that declares it,
// so the shell would silently stop seeing an overlay the moment it moved.
type keyInterceptor interface {
	HandleKey(msg tea.KeyPressMsg) (tea.Cmd, bool, tea.Msg)
}

// dispatchOverlayKey forwards a key to an overlay and assembles the resulting
// tea.Cmd. The caller is responsible for nil-checking the overlay before
// calling and for niling the field when dismiss is true.
func dispatchOverlayKey(overlay keyInterceptor, msg tea.KeyPressMsg) (dismiss bool, cmd tea.Cmd) {
	inputCmd, dismissed, resultMsg := overlay.HandleKey(msg)
	var cmds []tea.Cmd
	if inputCmd != nil {
		cmds = append(cmds, inputCmd)
	}
	if resultMsg != nil {
		captured := resultMsg
		cmds = append(cmds, func() tea.Msg { return captured })
	}
	return dismissed, tea.Batch(cmds...)
}

func (m *AppModel) currentViewCapabilities() route.ViewCapabilities {
	view := m.activeView()
	if view == nil {
		return nil
	}
	if caps, ok := view.(route.ViewCapabilities); ok {
		return caps
	}
	return nil
}

// hasActiveTextInput returns true if the current view has an active text input
// that should receive keyboard input (like search boxes, text fields).
// This is used to determine whether global keyboard shortcuts should be intercepted.
// isCtrlEscKey reports whether the key event is Ctrl+Esc. The string check
// covers Bubble Tea's normalized representation; the code/modifier check keeps
// this stable for tests and terminal protocols that preserve the modifier.
func isCtrlEscKey(msg tea.KeyPressMsg) bool {
	if msg.String() == "ctrl+esc" {
		return true
	}
	return (msg.Code == tea.KeyEsc || msg.Code == tea.KeyEscape) && msg.Mod.Contains(tea.ModCtrl)
}

// isModifierOnlyKeyPress reports whether the event is a modifier or lock key
// pressed by itself. Keyboard protocols that report every key as an escape code
// emit these as their own presses, and no surface binds one. Letting one through
// would append "leftshift" to a pending chord and abandon the path mid-typing,
// so the shell drops them the way it already drops key releases.
func isModifierOnlyKeyPress(msg tea.KeyPressMsg) bool {
	switch msg.Code {
	case tea.KeyLeftShift, tea.KeyRightShift,
		tea.KeyLeftCtrl, tea.KeyRightCtrl,
		tea.KeyLeftAlt, tea.KeyRightAlt,
		tea.KeyLeftSuper, tea.KeyRightSuper,
		tea.KeyLeftHyper, tea.KeyRightHyper,
		tea.KeyLeftMeta, tea.KeyRightMeta,
		tea.KeyIsoLevel3Shift, tea.KeyIsoLevel5Shift,
		tea.KeyCapsLock, tea.KeyNumLock, tea.KeyScrollLock:
		return true
	}
	return false
}

// hasActiveConfirmDialog returns true if the current view has an active confirm dialog.
// Used to prevent the global clipboard handler from intercepting y/Y during delete confirmations.

// currentViewAsModel returns the current view as a tea.Model for interface assertions.
func (m *AppModel) currentViewAsModel() tea.Model {
	return m.activeView()
}

func (m *AppModel) handleGlobalKeyPress(msg tea.KeyPressMsg) (bool, tea.Cmd) { //nolint:gocyclo
	key := msg.String()
	if key == "ctrl+c" {
		return m.executeSimplifiedGlobalKey(key)
	}
	if isCtrlEscKey(msg) {
		// Ctrl+Esc was the legacy hard reset. It is intentionally inert in the
		// final grammar, including while a subordinate route is open.
		return true, nil
	}
	// Esc is the canonical close/back operation. App-owned overlays and
	// confirmations receive it before routed controls; a routed control may
	// still capture Esc for its own inline state when no shell overlay is open.
	if key == "esc" {
		if handled, cmd := m.handleProjectArchiveConfirmKey(key); handled {
			return true, cmd
		}
		if handled, cmd := m.handleProjectDeleteConfirmKey(key); handled {
			return true, cmd
		}
		if handled, cmd := m.handleProjectParkConfirmKey(key); handled {
			return true, cmd
		}
		if handled, cmd := m.handleTaskDeleteConfirmKey(key); handled {
			return true, cmd
		}
		if handled, cmd := m.handleOutlineDeleteConfirmKey(key); handled {
			return true, cmd
		}
		if handled, cmd := m.operations.HandleConfirmKey(m, key); handled {
			return true, cmd
		}
		if handled, cmd := m.handleActiveKeyOverlay(msg); handled {
			return true, cmd
		}
		if handled, cmd := m.handleEscKey(); handled {
			return true, cmd
		}
	}

	if handled, cmd := m.handleProjectArchiveConfirmKey(key); handled {
		return true, cmd
	}

	if handled, cmd := m.handleProjectDeleteConfirmKey(key); handled {
		return true, cmd
	}

	if handled, cmd := m.handleProjectParkConfirmKey(key); handled {
		return true, cmd
	}

	if handled, cmd := m.handleTaskDeleteConfirmKey(key); handled {
		return true, cmd
	}

	if handled, cmd := m.handleOutlineDeleteConfirmKey(key); handled {
		return true, cmd
	}

	if handled, cmd := m.operations.HandleConfirmKey(m, key); handled {
		return true, cmd
	}

	if handled, cmd := m.handleActiveKeyOverlay(msg); handled {
		return true, cmd
	}

	// Guard against stale currentProject (e.g., deleted by external CLI command)
	if handled, cmd := m.handleInvalidManageProjectScope(); handled {
		return true, cmd
	}
	if handled, cmd := m.validateCurrentProject(); handled {
		return true, tea.Batch(cmd, status.DefaultStatusTimeout())
	}
	if handled, cmd := m.handleSimplifiedInputKey(msg); handled {
		return true, cmd
	}
	return false, nil
}

// performGlobalTaskUndoRedoForProject applies the single workspace-wide history.
// The project argument remains part of the operation-handler seam until the
// surrounding task-target adapters are simplified.
func (m *AppModel) performGlobalTaskUndoRedoForProject(_ *models.Project, redo bool) tea.Cmd {
	if m.workspaceLedgerRestoreIsNewest(redo) {
		return m.handleLedgerTaskUndoRedo(redo)
	}
	return nil
}

func (m *AppModel) handleProjectArchiveConfirmKey(key string) (bool, tea.Cmd) {
	return projectops.ConfirmKey(m.project.ResolveArchiveKey(key), projecthub.ProjectArchiveConfirmedMsg{})
}

func (m *AppModel) handleProjectDeleteConfirmKey(key string) (bool, tea.Cmd) {
	return projectops.ConfirmKey(m.project.ResolveDeleteKey(key), projecthub.ProjectDeleteConfirmedMsg{})
}

func (m *AppModel) handleProjectParkConfirmKey(key string) (bool, tea.Cmd) {
	return projectops.ConfirmKey(m.project.ResolveParkKey(key), projecthub.ProjectParkConfirmedMsg{})
}

func (m *AppModel) handleActiveKeyOverlay(msg tea.KeyPressMsg) (bool, tea.Cmd) {
	if m.checkInSetup != nil {
		return true, m.handleCheckInSetupKey(msg)
	}
	if m.operations.CheckInEditor != nil {
		dismiss, cmd := dispatchOverlayKey(m.operations.CheckInEditor, msg)
		if dismiss {
			m.operations.CheckInEditor = nil
		}
		return true, cmd
	}
	if m.operations.TextPrompt != nil {
		dismiss, cmd := dispatchOverlayKey(m.operations.TextPrompt, msg)
		if dismiss {
			m.operations.TextPrompt = nil
		}
		return true, cmd
	}
	if m.operations.ValuePicker != nil {
		dismiss, cmd := dispatchOverlayKey(m.operations.ValuePicker, msg)
		if dismiss {
			// The result message decides whether the sheet beneath should also
			// close; cancellation returns to it.
			m.operations.ValuePicker = nil
		}
		return true, cmd
	}
	if m.operations.ActionSheet != nil {
		dismiss, cmd := dispatchOverlayKey(m.operations.ActionSheet, msg)
		if dismiss {
			m.operations.ActionSheet = nil
		}
		return true, cmd
	}
	if m.lenses.Mounted() {
		// Last, because every overlay above nests over the picker rather than
		// replacing it: the picker only sees a key once nothing is on top.
		dismiss, cmd := dispatchOverlayKey(m.lenses.Picker(), msg)
		if dismiss {
			m.lenses.Close()
		}
		return true, cmd
	}
	if m.conflictBrowser != nil {
		// Nothing ever opens above the browser, so its position among the arms is
		// free; it is last because it is the only overlay that cannot be reached
		// from any of the others.
		dismiss, cmd := dispatchOverlayKey(m.conflictBrowser, msg)
		if dismiss {
			m.conflictBrowser = nil
		}
		return true, cmd
	}
	return false, nil
}

// handleActivePasteOverlay gives a terminal bracketed paste to the mounted
// check-in editor or text prompt. A
// paste is not a run of key presses: it arrives as its own message, so an
// overlay covering a view would otherwise watch the paste land in that view —
// or, since the Outliner drops a paste outside quick edit, vanish. Only the text
// editor and prompt are named because they are the overlays that collect typed
// text; the others answer keys from a closed set no paste can spell, and
// leaving them out keeps every path a paste already took unchanged. Returning
// true while either is mounted is what stops the fall-through.
func (m *AppModel) handleActivePasteOverlay(msg tea.PasteMsg) (bool, tea.Cmd) {
	if m.checkInSetup != nil && m.checkInSetup.busy {
		return true, nil
	}
	if m.operations.CheckInEditor != nil {
		return true, m.operations.CheckInEditor.HandlePaste(msg)
	}
	if m.operations.TextPrompt == nil {
		return false, nil
	}
	return true, m.operations.TextPrompt.HandlePaste(msg)
}

// handleUndoToggle reverses the most recent q/w planning toggle as one logical
// action (workflow state + Next order index / Daily Plan entry). It fires
// wherever a toggle was just recorded — the same task views where q/w work — so
// an immediate u undoes the toggle atomically instead of falling through to
// ordinary project-history undo (which would restore the state but leave the
// planning order/list stale). The localToggleUndoIsNewest guard still yields to
// a newer persistent mutation.
func (m *AppModel) handleUndoToggle() tea.Cmd {
	if m.lastToggleUndoKind == toggleUndoNone {
		return nil
	}
	if !m.currentViewOwnsPlanningToggle() {
		return nil
	}
	if !m.localToggleUndoIsNewest() {
		return nil
	}
	var cmd tea.Cmd
	switch m.lastToggleUndoKind {
	case toggleUndoNone:
	case toggleUndoToday:
		cmd = m.undoTodayToggle()
	case toggleUndoQueue:
		cmd = m.undoQueueToggle()
	}
	if cmd != nil {
		return cmd
	}
	return nil
}

// handleRedoToggle replays the most recently undone q/w planning toggle, mirror
// of handleUndoToggle. Reachable from every view where the toggle is reachable,
// so ctrl+r replays both sides of the logical action without navigating away.
func (m *AppModel) handleRedoToggle() tea.Cmd {
	if m.lastToggleRedoKind == toggleUndoNone {
		return nil
	}
	if !m.currentViewOwnsPlanningToggle() {
		return nil
	}
	if !m.localToggleRedoIsNewest() {
		return nil
	}
	var cmd tea.Cmd
	switch m.lastToggleRedoKind {
	case toggleUndoNone:
	case toggleUndoToday:
		cmd = m.redoTodayToggle()
	case toggleUndoQueue:
		cmd = m.redoQueueToggle()
	}
	if cmd != nil {
		return cmd
	}
	return nil
}

// currentViewOwnsPlanningToggle reports whether the current view is one where a
// q/w planning toggle is reachable (a QueueToggleable task view, including the
// planning surface). The bespoke atomic toggle undo/redo only fires from such a
// view so pressing u/ctrl+r elsewhere does not silently
// reverse a planning action taken in another view.
func (m *AppModel) currentViewOwnsPlanningToggle() bool {
	_, ok := m.currentViewAsModel().(QueueToggleable)
	return ok
}

func (m *AppModel) localToggleUndoIsNewest() bool {
	return m.localToggleIsNewest(m.toggleUndoTimestamp(m.lastToggleUndoKind), false)
}

func (m *AppModel) localToggleRedoIsNewest() bool {
	return m.localToggleIsNewest(m.toggleRedoTimestamp(m.lastToggleRedoKind), true)
}

func (m *AppModel) localToggleIsNewest(localAt time.Time, redo bool) bool {
	if localAt.IsZero() {
		return true
	}
	entry, ok, err := ledgerNewestEntry(workspaceLedgerFromConfig(m.domain.runtime), storage.GlobalScope(), redo)
	if err != nil || !ok {
		return true
	}
	return !entry.Timestamp.After(localAt)
}
