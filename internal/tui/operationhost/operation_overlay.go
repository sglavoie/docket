package operationhost

import (
	"fmt"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/listsurface"
	"github.com/sglavoie/tt/internal/tui/operation"
	"github.com/sglavoie/tt/internal/tui/status"
)

// ExecuteRequest runs one request through the single invocation pipeline and
// opens the value step the executor asked for, if it asked for one.
func (c *Controller) ExecuteRequest(
	shell Shell,
	request operation.OperationExecutionRequest,
) tea.Cmd {
	if c.Executor == nil {
		shell.SetError(fmt.Errorf("operation registry is unavailable"))
		return status.StatusTimeout(5 * time.Second)
	}
	result := c.Executor.Execute(request)
	if result.Err != nil {
		shell.SetError(result.Err)
		return status.StatusTimeout(5 * time.Second)
	}
	if result.ValueRequired != nil {
		return c.OpenValueStep(shell, *result.ValueRequired)
	}
	return operation.ResultCmd(result)
}

// OpenValueStep opens the value overlay one provider needs: a typed prompt for
// text values, the ListSurface picker for everything else.
func (c *Controller) OpenValueStep(shell Shell, request operation.OperationValueRequestMsg) tea.Cmd {
	if request.Provider.TextEntry() {
		prompt, err := shell.NewTextPrompt(request.Request, request.Provider)
		if err != nil {
			shell.SetError(err)
			return status.StatusTimeout(5 * time.Second)
		}
		c.TextPrompt = prompt
		return prompt.InitCmd()
	}
	picker, err := shell.NewValuePicker(request.Request, request.Provider)
	if err != nil {
		shell.SetError(err)
		return status.StatusTimeout(5 * time.Second)
	}
	c.ValuePicker = picker
	return nil
}

// applyContextActionSheetResult applies the nested step or completion produced
// by one centralized sheet execution. A failure is already retained on the
// sheet, a pending value or confirmation opens beneath it, and only a completed
// action reports the host lifecycle.
func (c *Controller) ApplySheetResult(shell Shell, msg listsurface.ContextActionSheetResultMsg) tea.Cmd {
	switch {
	case msg.Result.Err != nil:
		return nil
	case msg.Result.ValueRequired != nil:
		return c.OpenValueStep(shell, *msg.Result.ValueRequired)
	case msg.Result.ConfirmationRequired != nil:
		c.ShowConfirmation(*msg.Result.ConfirmationRequired)
		return nil
	}
	return operation.ResultCmd(msg.Result)
}

// resumeContextActionRequest re-enters the executor after a nested value or
// confirmation step. An open sheet keeps ownership of the outcome so a failure
// stays visible on it and only a completed action dismisses it.
func (c *Controller) ResumeRequest(shell Shell, request operation.OperationExecutionRequest) tea.Cmd {
	sheet := c.ActionSheet
	if sheet == nil {
		// The lens picker is the one surface an operation can be raised from
		// without a sheet between, so it owns the resumed outcome the same way:
		// a refusal stays on the list, and only completion tears it down. Both of
		// its value-taking action rows resume here — the name prompt behind Save
		// and the template gallery behind Add built-in view — so a refused add is
		// retained on the picker exactly as a refused save is.
		return shell.ResumeWithoutSheet(request)
	}
	result, dismissed := sheet.Execute(request)
	if dismissed {
		c.ActionSheet = nil
		// The sheet was opened over the picker, which stays mounted: the row it
		// acted on may have been renamed, rewritten, or removed underneath.
		shell.RefreshNamedLensPicker()
	}
	return c.ApplySheetResult(shell, listsurface.ContextActionSheetResultMsg{
		Result: result,
	})
}

// showOperationConfirmation prompts for a destructive contextual action and
// freezes the executor-supplied request, which already carries Confirmed.
func (c *Controller) ShowConfirmation(request operation.OperationConfirmationRequestMsg) {
	prompt := request.Label + "?"
	if count := len(request.Targets.Entities()); count > 1 {
		prompt = fmt.Sprintf("%s · %d targets?", request.Label, count)
	}
	c.ArmConfirm(request.Request, prompt)
}

// handleOperationConfirmKey gives the contextual destructive confirmation modal
// key priority: it consumes every key while active so nothing leaks into the
// sheet beneath, and cancellation leaves that sheet exactly as it was.
func (c *Controller) HandleConfirmKey(shell Shell, key string) (bool, tea.Cmd) {
	switch c.ResolveConfirmKey(key) {
	case chrome.ConfirmAnswerConfirmed:
		return true, c.ResumeRequest(shell, c.TakeRequest())
	case chrome.ConfirmAnswerSwallowed, chrome.ConfirmAnswerCanceled:
		return true, nil
	case chrome.ConfirmAnswerUnarmed:
	}
	return false, nil
}
