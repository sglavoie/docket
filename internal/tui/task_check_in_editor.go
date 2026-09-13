package tui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/coordinator"
	"github.com/sglavoie/tt/internal/tui/operation"
	"github.com/sglavoie/tt/internal/tui/taskform"
)

// CheckInRejectedMsg returns a failed asynchronous commit to the editor with
// the exact draft and frozen request that produced it.
type CheckInRejectedMsg struct {
	Request operation.OperationExecutionRequest
	Value   operation.OperationCheckInValue
	Err     error
}

// NewCheckInEditor builds the operation host's specialized value step. A
// single target is prefilled from its schedule; every bulk target set opens as
// an explicit replacement with no selected task used as a hidden default.
func (m *AppModel) NewCheckInEditor(
	request operation.OperationExecutionRequest,
) (*taskform.CheckInEditorModel, error) {
	refs, err := m.operationTaskRefs(request.Context)
	if err != nil {
		return nil, err
	}
	if len(refs) == 0 {
		return nil, fmt.Errorf("select at least one task")
	}
	bulk := request.Context.Selection.Visual || len(refs) > 1
	if request.Context.CheckInOffer {
		return taskform.NewCheckInEditorModel(request, fmt.Sprintf("Optional check-in for %d task(s) · Esc skips setup", len(refs)), models.CheckInSchedule{}, bulk), nil
	}
	if bulk {
		return taskform.NewCheckInEditorModel(
			request,
			fmt.Sprintf("Replace schedule on %d selected tasks", len(refs)),
			refs[0].Task.CheckIn(),
			true,
		), nil
	}
	return taskform.NewCheckInEditorModel(
		request,
		refs[0].Task.Title,
		refs[0].Task.CheckIn(),
		false,
	), nil
}

func (m *AppModel) retainCheckInDraftOnFailure(
	request operation.OperationExecutionRequest,
	value operation.OperationCheckInValue,
	cmd tea.Cmd,
) tea.Cmd {
	return func() tea.Msg {
		msg := cmd()
		if failure, ok := msg.(coordinator.TransactionErrorMsg); ok {
			return CheckInRejectedMsg{Request: request, Value: value, Err: failure.Err}
		}
		return msg
	}
}

func (m *AppModel) reopenCheckInEditor(msg CheckInRejectedMsg) tea.Cmd {
	editor, err := m.NewCheckInEditor(msg.Request)
	if err != nil {
		m.feedback.err = msg.Err
		return nil
	}
	editor.Restore(msg.Value, operationRefusalReason(msg.Err))
	m.operations.CheckInEditor = editor
	return editor.InitCmd()
}
