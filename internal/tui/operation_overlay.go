package tui

import (
	"fmt"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/sglavoie/tt/internal/tui/listsurface"
	"github.com/sglavoie/tt/internal/tui/operation"
	"github.com/sglavoie/tt/internal/tui/status"
)

func (m *AppModel) executeOperationOverlayRequest(
	request operation.OperationExecutionRequest,
) tea.Cmd {
	if m.operationExecutor == nil {
		m.err = fmt.Errorf("operation registry is unavailable")
		return status.StatusTimeout(5 * time.Second)
	}
	result := m.operationExecutor.Execute(request)
	if result.Err != nil {
		m.err = result.Err
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
			m.err = err
			return status.StatusTimeout(5 * time.Second)
		}
		m.operationTextPrompt = prompt
		return prompt.initCmd()
	}
	picker, err := m.newOperationValuePicker(request.Request, request.Provider)
	if err != nil {
		m.err = err
		return status.StatusTimeout(5 * time.Second)
	}
	m.operationValuePicker = picker
	return nil
}

func (m *AppModel) clearOperationValueStep() {
	m.operationValuePicker = nil
	m.operationTextPrompt = nil
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
		m.contextActionSheet = nil
		return true, nil
	case NamedLensPickerClosedMsg:
		m.lensPicker = nil
		return true, nil
	case NamedLensPickerResultMsg:
		return true, m.applyNamedLensPickerResult(typed)
	case NamedLensActionsRequestMsg:
		return true, m.openNamedLensActions(typed)
	case SaveViewAsLensRequestMsg:
		return true, m.saveViewAsLens()
	case MenuBarActivateMsg:
		// The menu named an operation and closed; running it is the shell's because
		// the executor is.
		return true, m.activateMenuBarEntry(typed.OperationID)
	case QueryReadoutReloadRequestMsg:
		// The peel already wrote the population fields; only the reload the facet
		// owes is left, and it is the shell's because the projects are.
		return true, m.projectFilterReloadCmd(typed.Status)
	case ConflictBrowserClosedMsg:
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
	sheet := m.contextActionSheet
	if sheet == nil {
		// The lens picker is the one surface an operation can be raised from
		// without a sheet between, so it owns the resumed outcome the same way:
		// a refusal stays on the list, and only completion tears it down. Both of
		// its value-taking action rows resume here — the name prompt behind Save
		// and the template gallery behind Add built-in view — so a refused add is
		// retained on the picker exactly as a refused save is.
		if picker := m.lensPicker; picker != nil {
			result, dismissed := picker.Execute(request)
			if dismissed {
				m.closeNamedLensOverlays()
			}
			return m.applyNamedLensPickerResult(NamedLensPickerResultMsg{Result: result})
		}
		return m.executeOperationOverlayRequest(request)
	}
	result, dismissed := sheet.Execute(request)
	if dismissed {
		m.contextActionSheet = nil
		// The sheet was opened over the picker, which stays mounted: the row it
		// acted on may have been renamed, rewritten, or removed underneath.
		m.refreshNamedLensPicker()
	}
	return m.applyContextActionSheetResult(listsurface.ContextActionSheetResultMsg{
		Result: result,
	})
}

const operationConfirmAction = "contextual-operation"

// showOperationConfirmation prompts for a destructive contextual action and
// freezes the executor-supplied request, which already carries Confirmed.
func (m *AppModel) showOperationConfirmation(request operation.OperationConfirmationRequestMsg) {
	m.pendingOperationRequest = request.Request
	prompt := request.Label + "?"
	if count := len(request.Targets.Entities()); count > 1 {
		prompt = fmt.Sprintf("%s · %d targets?", request.Label, count)
	}
	m.operationConfirm.Show(operationConfirmAction, prompt)
}

// handleOperationConfirmKey gives the contextual destructive confirmation modal
// key priority: it consumes every key while active so nothing leaks into the
// sheet beneath, and cancellation leaves that sheet exactly as it was.
func (m *AppModel) handleOperationConfirmKey(key string) (bool, tea.Cmd) {
	if !m.operationConfirm.IsActive() {
		return false, nil
	}
	confirmed, handled := m.operationConfirm.HandleKey(key)
	if !handled {
		return false, nil
	}
	if confirmed {
		request := m.pendingOperationRequest
		m.pendingOperationRequest = operation.OperationExecutionRequest{}
		return true, m.resumeContextActionRequest(request)
	}
	switch key {
	case "n", "N", "esc":
		m.pendingOperationRequest = operation.OperationExecutionRequest{}
	}
	return true, nil
}
