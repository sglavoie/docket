package operationhost

import (
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/docket/internal/tui/operation"
)

// activateMenuBarEntry runs the operation a menu row named, through the same
// executor a key press reaches. The menu raises MenuBarActivateMsg rather than
// executing the operation itself because a view model holds no executor, and it
// is already closed by the time this runs, so the operation resolves its target
// against the view the user is left looking at rather than against one with a
// dropdown over it.
func (c *Controller) ActivateMenuBarEntry(shell Shell, id operation.OperationID) tea.Cmd {
	if !c.Wired() {
		return nil
	}
	ctx := c.CommandContext(shell)
	ctx.View = shell.CurrentView()
	return operation.ResultCmd(c.Executor.Execute(operation.OperationExecutionRequest{
		ID:      id,
		Context: ctx,
	}))
}

// ActivateOperation runs one operation, with an optional preset value, against
// the cursor of the view the user is looking at — the route a triage card takes
// once it has seated that cursor on its task.
func (c *Controller) ActivateOperation(shell Shell, id operation.OperationID, value any, hasValue bool) tea.Cmd {
	if !c.Wired() {
		return nil
	}
	ctx := c.CommandContext(shell)
	ctx.View = shell.CurrentView()
	return operation.ResultCmd(c.Executor.Execute(operation.OperationExecutionRequest{
		ID:       id,
		Context:  ctx,
		Value:    value,
		HasValue: hasValue,
	}))
}
