package tui

import (
	"slices"

	tea "charm.land/bubbletea/v2"
)

// handleSimplifiedInputKey resolves one root-level key through the selected
// OperationRegistry strategy, extending any pending chord prefix first. It
// deliberately returns handled=false while an inline Outliner control owns input
// so the normal view Update path can process search, quick edit, confirmation,
// and mode-picker keystrokes.
func (m *AppModel) handleSimplifiedInputKey(msg tea.KeyPressMsg) (bool, tea.Cmd) {
	if m.operationRegistry == nil || m.operationExecutor == nil {
		return false, nil
	}
	view := m.currentView()
	if view != ViewCrossProjectOutliner {
		m.pendingChord = nil
		return false, nil
	}
	out, _, ok := m.outlinerPrimary()
	if !ok || out == nil || !m.activeOutlinerRoute() {
		m.pendingChord = nil
		return false, nil
	}
	if out.simplifiedInlineControlOwnsInput() {
		m.pendingChord = nil
		return false, nil
	}

	ctx := m.operationCommandContext()
	ctx.View = view
	registry := m.operationRegistry
	key := msg.String()
	if len(m.pendingChord) > 0 && normalizeOperationKey(key) == "esc" {
		m.pendingChord = nil
		return true, nil
	}
	sequence := append(slices.Clone(m.pendingChord), key)
	binding, _, resolution := registry.InputBindingForSequence(InputStrategySimplified, sequence, ctx)
	switch resolution {
	case ChordPending:
		m.pendingChord = sequence
		return true, nil
	case ChordNoMatch:
		// A complete strategy consumes unknown root keys instead of allowing a
		// retired legacy key or an abandoned chord to run in the view beneath it.
		m.pendingChord = nil
		return true, nil
	case ChordTerminal:
	}
	m.pendingChord = nil

	result := m.operationExecutor.Execute(OperationExecutionRequest{
		ID:       binding.OperationID,
		Context:  ctx,
		Source:   OperationSourceDirectKey,
		Value:    binding.PresetValue,
		HasValue: binding.HasPresetValue,
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

// pendingChordHintRow is the which-key footer row for an unfinished chord.
func (m *AppModel) pendingChordHintRow() string {
	if m == nil || m.operationRegistry == nil || len(m.pendingChord) == 0 {
		return ""
	}
	ctx := m.operationCommandContext()
	ctx.View = m.currentView()
	return m.operationRegistry.ChordHintRowForPrefix(InputStrategySimplified, m.pendingChord, ctx)
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
