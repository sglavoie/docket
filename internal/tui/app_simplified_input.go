package tui

import (
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"
)

// handleSimplifiedInputKey resolves one root-level key through the selected
// OperationRegistry strategy, extending any pending chord prefix first. It
// deliberately returns handled=false while an inline Outliner control owns input
// so the normal view Update path can process search, quick edit, and confirmation
// keystrokes.
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

// executeSimplifiedGlobalKey routes a grammar-owned global key through the same
// descriptor and executor even when a subordinate route covers the Outliner.
// Ctrl+C is currently the only such key.
func (m *AppModel) executeSimplifiedGlobalKey(key string) (bool, tea.Cmd) {
	if m.operationRegistry == nil || m.operationExecutor == nil {
		return false, nil
	}
	ctx := m.operationCommandContext()
	ctx.View = ViewCrossProjectOutliner
	ctx.Capabilities.Add(CapabilitySimplifiedInput)
	binding, _, ok := m.operationRegistry.InputBindingForKey(InputStrategySimplified, key, ctx)
	if !ok {
		return false, nil
	}
	result := m.operationExecutor.Execute(OperationExecutionRequest{
		ID:      binding.OperationID,
		Context: ctx,
		Source:  OperationSourceDirectKey,
	})
	switch message := result.AsMessage().(type) {
	case nil:
		return true, nil
	case OperationCommandResultMsg:
		return true, message.Cmd
	default:
		return true, func() tea.Msg { return message }
	}
}

func (m *AppModel) pendingChordHintLines(width int) []string {
	if m == nil || m.operationRegistry == nil || len(m.pendingChord) == 0 {
		return nil
	}
	ctx := m.operationCommandContext()
	ctx.View = m.currentView()
	hints := m.operationRegistry.ChordHintsForPrefix(InputStrategySimplified, m.pendingChord, ctx)
	if len(hints) == 0 {
		return nil
	}
	return fitPendingChordLines(strings.Join(m.pendingChord, ""), hints, width)
}

func (m *OutlinerModel) simplifiedInlineControlOwnsInput() bool {
	if m == nil {
		return false
	}
	if m.IsSearchMode() ||
		m.quickEdit.IsActive() ||
		m.confirm.IsActive() {
		return true
	}
	return false
}
