package tui

import (
	"fmt"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/sglavoie/tt/internal/tui/conflict"
	"github.com/sglavoie/tt/internal/tui/lens"
	"github.com/sglavoie/tt/internal/tui/listsurface"
	"github.com/sglavoie/tt/internal/tui/operation"
	"github.com/sglavoie/tt/internal/tui/outliner"
	"github.com/sglavoie/tt/internal/tui/status"
)

func (m *AppModel) executeOperationOverlayRequest(
	request operation.OperationExecutionRequest,
) tea.Cmd {
	if m.operations.executor == nil {
		m.feedback.err = fmt.Errorf("operation registry is unavailable")
		return status.StatusTimeout(5 * time.Second)
	}
	result := m.operations.executor.Execute(request)
	if result.Err != nil {
		m.feedback.err = result.Err
		return status.StatusTimeout(5 * time.Second)
	}
	if result.ValueRequired != nil {
		return m.openOperationValueStep(*result.ValueRequired)
	}
	return operationResultCmd(result)
}

// openOperationValueStep opens the value overlay one provider needs: a typed
// prompt for text values, the ListSurface picker for everything else.
func (m *AppModel) openOperationValueStep(request operation.OperationValueRequestMsg) tea.Cmd {
	if request.Provider.TextEntry() {
		prompt, err := m.newOperationTextPrompt(request.Request, request.Provider)
		if err != nil {
			m.feedback.err = err
			return status.StatusTimeout(5 * time.Second)
		}
		m.operations.textPrompt = prompt
		return prompt.initCmd()
	}
	picker, err := m.newOperationValuePicker(request.Request, request.Provider)
	if err != nil {
		m.feedback.err = err
		return status.StatusTimeout(5 * time.Second)
	}
	m.operations.valuePicker = picker
	return nil
}

func (m *AppModel) clearOperationValueStep() {
	m.operations.clearValueStep()
}

// operationResultCmd converts a completed execution outcome into the Bubble Tea
// command seam.
func operationResultCmd(result operation.OperationExecutionResult) tea.Cmd {
	switch typed := result.AsMessage().(type) {
	case nil:
		return nil
	case operation.OperationCommandResultMsg:
		return typed.Cmd
	default:
		return func() tea.Msg { return typed }
	}
}

func (m *AppModel) handleOperationOverlayMessage(msg tea.Msg) (bool, tea.Cmd) {
	switch typed := msg.(type) {
	case listsurface.ContextActionSheetResultMsg:
		return true, m.applyContextActionSheetResult(typed)
	case listsurface.ContextActionSheetClosedMsg:
		m.operations.actionSheet = nil
		return true, nil
	case lens.NamedLensPickerClosedMsg:
		m.lenses.close()
		return true, nil
	case lens.NamedLensPickerResultMsg:
		return true, m.applyNamedLensPickerResult(typed)
	case lens.NamedLensActionsRequestMsg:
		return true, m.openNamedLensActions(typed)
	case outliner.SaveViewAsLensRequestMsg:
		return true, m.saveViewAsLens()
	case outliner.MenuBarActivateMsg:
		// The menu named an operation and closed; running it is the shell's because
		// the executor is.
		return true, m.activateMenuBarEntry(typed.OperationID)
	case outliner.QueryReadoutReloadRequestMsg:
		// The peel already wrote the population fields; only the reload the facet
		// owes is left, and it is the shell's because the projects are.
		return true, m.projectFilterReloadCmd(typed.Status)
	case conflict.ConflictBrowserClosedMsg:
		m.conflictBrowser = nil
		return true, nil
	case operation.OperationConfirmationRequestMsg:
		m.showOperationConfirmation(typed)
		return true, nil
	case operation.OperationValueRequestMsg:
		return true, m.openOperationValueStep(typed)
	case operation.OperationValueSelectedMsg:
		m.clearOperationValueStep()
		request := typed.Request
		request.Value = typed.Value
		request.HasValue = true
		return true, m.resumeContextActionRequest(request)
	case operation.OperationValueCanceledMsg:
		m.clearOperationValueStep()
		return true, nil
	case ProjectTagVocabularyActionsMsg:
		m.clearOperationValueStep()
		return true, m.openProjectTagVocabularyActions(typed.Tag)
	default:
		return false, nil
	}
}

// applyContextActionSheetResult applies the nested step or completion produced
// by one centralized sheet execution. A failure is already retained on the
// sheet, a pending value or confirmation opens beneath it, and only a completed
// action reports the host lifecycle.
func (m *AppModel) applyContextActionSheetResult(msg listsurface.ContextActionSheetResultMsg) tea.Cmd {
	switch {
	case msg.Result.Err != nil:
		return nil
	case msg.Result.ValueRequired != nil:
		return m.openOperationValueStep(*msg.Result.ValueRequired)
	case msg.Result.ConfirmationRequired != nil:
		m.showOperationConfirmation(*msg.Result.ConfirmationRequired)
		return nil
	}
	return operationResultCmd(msg.Result)
}

// resumeContextActionRequest re-enters the executor after a nested value or
// confirmation step. An open sheet keeps ownership of the outcome so a failure
// stays visible on it and only a completed action dismisses it.
func (m *AppModel) resumeContextActionRequest(request operation.OperationExecutionRequest) tea.Cmd {
	sheet := m.operations.actionSheet
	if sheet == nil {
		// The lens picker is the one surface an operation can be raised from
		// without a sheet between, so it owns the resumed outcome the same way:
		// a refusal stays on the list, and only completion tears it down. Both of
		// its value-taking action rows resume here — the name prompt behind Save
		// and the template gallery behind Add built-in view — so a refused add is
		// retained on the picker exactly as a refused save is.
		if picker := m.lenses.picker; picker != nil {
			result, dismissed := picker.Execute(request)
			if dismissed {
				m.closeNamedLensOverlays()
			}
			return m.applyNamedLensPickerResult(lens.NamedLensPickerResultMsg{Result: result})
		}
		return m.executeOperationOverlayRequest(request)
	}
	result, dismissed := sheet.Execute(request)
	if dismissed {
		m.operations.actionSheet = nil
		// The sheet was opened over the picker, which stays mounted: the row it
		// acted on may have been renamed, rewritten, or removed underneath.
		m.refreshNamedLensPicker()
	}
	return m.applyContextActionSheetResult(listsurface.ContextActionSheetResultMsg{
		Result: result,
	})
}

// showOperationConfirmation prompts for a destructive contextual action and
// freezes the executor-supplied request, which already carries Confirmed.
func (m *AppModel) showOperationConfirmation(request operation.OperationConfirmationRequestMsg) {
	prompt := request.Label + "?"
	if count := len(request.Targets.Entities()); count > 1 {
		prompt = fmt.Sprintf("%s · %d targets?", request.Label, count)
	}
	m.operations.armConfirm(request.Request, prompt)
}

// handleOperationConfirmKey gives the contextual destructive confirmation modal
// key priority: it consumes every key while active so nothing leaks into the
// sheet beneath, and cancellation leaves that sheet exactly as it was.
func (m *AppModel) handleOperationConfirmKey(key string) (bool, tea.Cmd) {
	switch m.operations.resolveConfirmKey(key) {
	case confirmAnswerConfirmed:
		return true, m.resumeContextActionRequest(m.operations.takeRequest())
	case confirmAnswerSwallowed, confirmAnswerCanceled:
		return true, nil
	case confirmAnswerUnarmed:
	}
	return false, nil
}
