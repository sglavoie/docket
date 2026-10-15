package tui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/control"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/operation"
	"github.com/sglavoie/tt/internal/tui/presentation"
	"github.com/sglavoie/tt/internal/tui/status"
)

// Draft history is separate from the workspace ledger. A native Undo while
// typing must never take back a saved task, including when the draft is empty.
type desktopTextHistory struct {
	text         string
	past, future []chrome.DesktopTextBuffer
}

func (m *AppModel) desktopTextBuffer() *chrome.DesktopTextBuffer {
	if m.operations.ValuePicker != nil {
		return m.operations.ValuePicker.DesktopTextBuffer()
	}
	if m.operations.TextPrompt != nil {
		return m.operations.TextPrompt.DesktopTextBuffer()
	}
	if m.operations.CheckInEditor != nil {
		return m.operations.CheckInEditor.DesktopTextBuffer()
	}
	if m.operations.ActionSheet != nil || m.operations.Confirm.IsActive() || m.lenses.Mounted() || m.conflictBrowser != nil {
		return nil
	}
	if provider, ok := m.activeView().(chrome.DesktopTextProvider); ok {
		return provider.DesktopTextBuffer()
	}
	return nil
}

// Keep each field's history while focus moves through a form or a covering
// overlay. A changed external value starts a new history; closed editors are
// bounded to avoid retaining an unbounded number of input models.
func (m *AppModel) desktopHistory(input *chrome.DesktopTextBuffer) *desktopTextHistory {
	histories := m.control.textHistory
	if histories == nil || len(histories) >= 32 && histories[input.Key] == nil {
		histories = make(map[any]*desktopTextHistory)
		m.control.textHistory = histories
	}
	history := histories[input.Key]
	if history == nil || history.text != input.Text {
		history = &desktopTextHistory{text: input.Text}
		histories[input.Key] = history
	}
	return history
}

func (m *AppModel) observeDesktopText(before *chrome.DesktopTextBuffer, msg tea.Msg) {
	after := m.desktopTextBuffer()
	if after == nil {
		// Closing an input ends its history. Merely covering it or moving to
		// another control in the same editor preserves the draft's history.
		if before != nil && !m.Overlays().Active() {
			caps := m.currentViewCapabilities()
			if caps == nil || !caps.HasTextInput() {
				delete(m.control.textHistory, before.Key)
			}
		}
		return
	}
	if request, ok := msg.(controlRequestMsg); ok && request.request.Op == control.OpEdit {
		return
	}
	if before == nil || before.Key != after.Key {
		m.desktopHistory(after)
		return
	}
	history := m.desktopHistory(before)
	if before.Text == after.Text {
		return
	}
	snapshot := *before
	snapshot.Restore = nil
	history.past = append(history.past, snapshot)
	if len(history.past) > 100 {
		history.past = history.past[1:]
	}
	history.future = nil
	history.text = after.Text
}

func (m *AppModel) desktopEditing() control.Editing {
	state := control.Editing{}
	if input := m.desktopTextBuffer(); input != nil {
		history := m.desktopHistory(input)
		if len(history.past) > 0 {
			state.Undo = "Typing"
		}
		if len(history.future) > 0 {
			state.Redo = "Typing"
		}
		// An existing Outliner search can be focused without abandoning its draft.
		state.Search = m.currentView() == presentation.ViewCrossProjectOutliner && m.crossOutliner.IsSearchMode()
		return state
	}
	if m.Overlays().Active() {
		return state
	}
	if caps := m.currentViewCapabilities(); caps != nil && caps.HasTextInput() {
		return state
	}
	switch m.currentView() {
	case presentation.ViewCrossProjectOutliner:
		if m.crossOutliner == nil || m.crossOutliner.SimplifiedInlineControlOwnsInput() {
			return state
		}
		state.Search = true
	case presentation.ViewNotes, presentation.ViewTaskComments, presentation.ViewTaskAttachments:
	default:
		return state
	}
	state.Undo, state.Redo = m.workspaceHistoryPreview()
	return state
}

func (m *AppModel) controlEdit(action string, reply chan<- control.Reply) tea.Cmd {
	state := m.desktopEditing()
	if action == "search" && state.Search {
		answer(reply, true, "")
		if m.crossOutliner.IsSearchMode() {
			return nil
		}
		result := m.operations.Executor.Execute(operation.OperationExecutionRequest{ID: operation.OperationCoreScopeSearch, Context: m.operationCommandContext()})
		return operation.ResultCmd(result)
	}
	redo := action == "redo"
	if action != "undo" && !redo {
		answer(reply, false, fmt.Sprintf("%s is unavailable here", action))
		return nil
	}
	if (!redo && state.Undo == "") || (redo && state.Redo == "") {
		answer(reply, false, "Nothing to "+action+" here")
		return nil
	}
	if input := m.desktopTextBuffer(); input != nil {
		h := m.desktopHistory(input)
		from, to := &h.past, &h.future
		if redo {
			from, to = &h.future, &h.past
		}
		snapshot := (*from)[len(*from)-1]
		*from = (*from)[:len(*from)-1]
		current := *input
		current.Restore = nil
		*to = append(*to, current)
		input.Restore(snapshot.Text, snapshot.Row, snapshot.Column)
		h.text = snapshot.Text
		answer(reply, true, "")
		return nil
	}
	answer(reply, true, "")
	if m.currentView() != presentation.ViewCrossProjectOutliner {
		return status.WorkspaceUndoCmd(redo)
	}
	id := operation.OperationCoreUndo
	if redo {
		id = operation.OperationCoreRedo
	}
	result := m.operations.Executor.Execute(operation.OperationExecutionRequest{ID: id, Context: m.operationCommandContext()})
	return operation.ResultCmd(result)
}

func (m *AppModel) publishDesktopEditing() {
	if !m.control.enabled() {
		return
	}
	state := m.desktopEditing()
	if m.control.editing != nil && *m.control.editing == state {
		return
	}
	m.control.editing = &state
	m.control.publisher.Publish(control.Event{Event: control.EventEditing, Editing: &state})
}
