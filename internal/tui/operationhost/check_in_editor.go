package operationhost

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
func (c *Controller) NewCheckInEditor(
	shell Shell,
	request operation.OperationExecutionRequest,
) (*taskform.CheckInEditorModel, error) {
	refs, err := c.TaskRefs(shell, request.Context)
	if err != nil {
		return nil, err
	}
	if len(refs) == 0 {
		return nil, fmt.Errorf("select at least one task")
	}
	bulk := request.Context.Selection.Visual || len(refs) > 1
	if bulk {
		return taskform.NewCheckInEditorModel(
			request,
			fmt.Sprintf("Replace schedule on %d selected tasks", len(refs)),
			models.CheckInSchedule{},
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

// RetainCheckInDraftOnFailure preserves the submitted draft on commit refusal.
func RetainCheckInDraftOnFailure(
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

// ReopenCheckInEditor restores a refused draft against its frozen targets.
func (c *Controller) ReopenCheckInEditor(shell Shell, msg CheckInRejectedMsg, reason string) tea.Cmd {
	editor, err := c.NewCheckInEditor(shell, msg.Request)
	if err != nil {
		shell.SetError(msg.Err)
		return nil
	}
	editor.Restore(msg.Value, reason)
	c.CheckInEditor = editor
	return editor.InitCmd()
}
