package tui

import (
	tea "charm.land/bubbletea/v2"
)

// handleSimplifiedInputKey resolves one root-level key through the selected
// OperationRegistry strategy. It deliberately returns handled=false while an
// inline Outliner control owns input so the normal view Update path can process
// search, quick edit, confirmation, and mode-picker keystrokes.
func (m *AppModel) handleSimplifiedInputKey(msg tea.KeyPressMsg) (bool, tea.Cmd) {
	if m.operationRegistry == nil || m.operationExecutor == nil {
		return false, nil
	}
	view := m.currentView()
	if view != ViewCrossProjectOutliner {
		return false, nil
	}
	out, _, ok := m.outlinerPrimary()
	if !ok || out == nil || !m.activeOutlinerRoute() {
		return false, nil
	}
	if out.simplifiedInlineControlOwnsInput() {
		return false, nil
	}

	ctx := m.operationCommandContext()
	ctx.View = view
	registry := m.operationRegistry
	binding, _, found := registry.InputBindingForKey(
		InputStrategySimplified,
		msg.String(),
		ctx,
	)
	if !found {
		// A complete strategy consumes unknown root keys instead of allowing a
		// retired legacy key or prefix to run in the view beneath it.
		return true, nil
	}

	result := m.operationExecutor.Execute(OperationExecutionRequest{
		ID:      binding.OperationID,
		Context: ctx,
		Source:  OperationSourceDirectKey,
	})
	message := result.AsMessage()
	switch typed := message.(type) {
	case nil:
		return true, nil
	case OperationCommandResultMsg:
		return true, typed.Cmd
	default:
		return true, func() tea.Msg { return typed }
	}
}

func (m *OutlinerModel) simplifiedInlineControlOwnsInput() bool {
	if m == nil {
		return false
	}
	if m.modePickerActive() ||
		m.scoreFilterInput.IsActive() ||
		m.IsSearchMode() ||
		m.quickEdit.IsActive() ||
		m.confirm.IsActive() {
		return true
	}
	return false
}
