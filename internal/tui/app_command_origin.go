package tui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/tui/operation"
	"github.com/sglavoie/tt/internal/tui/operationhost"
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
	case tea.KeyPressMsg, tea.PasteMsg, tea.MouseClickMsg:
		return true
	default:
		return false
	}
}
