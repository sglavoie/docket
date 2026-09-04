package tui

import (
	"errors"
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/operation"
	"github.com/sglavoie/tt/internal/tui/presentation"
	"github.com/sglavoie/tt/internal/tui/status"
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
	if view != presentation.ViewCrossProjectOutliner {
		m.pendingChord = nil
		return false, nil
	}
	out, ok := m.outlinerPrimary()
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
	sequence := append(slices.Clone(m.pendingChord), msg.String())
	return m.dispatchOperationKey(operation.InputStrategySimplified, sequence, ctx)
}

// operationKeyDispatch is how a mounted surface reaches dispatchOperationKey.
// A view model holds no shell reference, so the shell hands the seam down when
// it mounts the surface.
type operationKeyDispatch func(operation.InputStrategyID, []string, operation.CommandContext) (bool, tea.Cmd)

// dispatchOperationKey resolves one surface's key path against that surface's
// strategy and runs whatever operation it reaches. Chord bookkeeping, the
// abandoned-path explanation, and executor message unwrapping say nothing about
// which entity is under the cursor, so every surface shares this one path.
func (m *AppModel) dispatchOperationKey(
	strategyID operation.InputStrategyID,
	sequence []string,
	ctx operation.CommandContext,
) (bool, tea.Cmd) {
	if len(sequence) > 1 && operation.NormalizeOperationKey(sequence[len(sequence)-1]) == "esc" {
		m.pendingChord = nil
		return true, nil
	}
	binding, _, resolution := m.operationRegistry.InputBindingForSequence(strategyID, sequence, ctx)
	switch resolution {
	case operation.ChordPending:
		m.pendingChord = sequence
		return true, nil
	case operation.ChordNoMatch:
		// A complete strategy consumes unknown root keys instead of allowing a
		// retired legacy key or an abandoned chord to run in the view beneath it.
		m.pendingChord = nil
		if cmd := m.entityScopedKeyRejection(strategyID, sequence, ctx); cmd != nil {
			return true, cmd
		}
		return true, m.chordMissStatus(strategyID, sequence)
	case operation.ChordTerminal:
	}
	m.pendingChord = nil

	result := m.operationExecutor.Execute(operation.OperationExecutionRequest{
		ID:       binding.OperationID,
		Context:  ctx,
		Value:    binding.PresetValue,
		HasValue: binding.HasPresetValue,
	})
	message := result.AsMessage()
	switch typed := message.(type) {
	case nil:
		return true, nil
	case operation.OperationCommandResultMsg:
		return true, typed.Cmd
	default:
		return true, func() tea.Msg { return typed }
	}
}

// entityScopedKeyRejection explains a bound key that a non-task cursor turned
// off. Naming the project on a header row makes every task operation
// inapplicable there, and an inapplicable terminal resolves to ChordNoMatch, so
// without this the key would silently do nothing instead of saying why.
func (m *AppModel) entityScopedKeyRejection(
	strategyID operation.InputStrategyID,
	sequence []string,
	ctx operation.CommandContext,
) tea.Cmd {
	// A checklist cursor found its owner task, so the shared sentence would tell
	// the user to do what they already did. It names the row kind instead, and it
	// asks whether the path reaches a refused operation rather than what category
	// they share, because a shared key's two meanings need not agree.
	if ctx.Entity.HasChecklistCursor {
		if !m.operationRegistry.InputPrefixRefusedOnChecklistRow(strategyID, sequence) {
			return nil
		}
		rejection := errors.New(operation.ChecklistRowRejectionMessage)
		return func() tea.Msg { return status.ErrorMsg{Err: rejection} }
	}
	if !operation.CommandContextNamesNonTaskEntity(ctx) {
		return nil
	}
	category, ok := m.operationRegistry.InputCategoryForPrefix(strategyID, sequence)
	if !ok {
		return nil
	}
	rejection := errors.New(operation.OperationTargetRejectionMessage(category))
	return func() tea.Msg { return status.ErrorMsg{Err: rejection} }
}

// chordMissStatus names the path a second key abandoned. An unbound root key is
// left silent because it never promised anything, but a chord that showed a
// which-key row and then resolved to nothing has to say so.
func (m *AppModel) chordMissStatus(strategyID operation.InputStrategyID, sequence []string) tea.Cmd {
	label, ok := m.operationRegistry.ChordGroupLabelForPrefix(strategyID, sequence)
	if !ok {
		return nil
	}
	m.setWarningStatus(strings.Join(sequence, "") + " is not a " + label + " action")
	return status.DefaultStatusTimeout()
}

// executeSimplifiedGlobalKey routes a grammar-owned global key through the same
// descriptor and executor even when a subordinate route covers the Outliner.
// Ctrl+C is currently the only such key.
func (m *AppModel) executeSimplifiedGlobalKey(key string) (bool, tea.Cmd) {
	if m.operationRegistry == nil || m.operationExecutor == nil {
		return false, nil
	}
	ctx := m.operationCommandContext()
	ctx.View = presentation.ViewCrossProjectOutliner
	ctx.Capabilities.Add(operation.CapabilitySimplifiedInput)
	binding, _, ok := m.operationRegistry.InputBindingForKey(operation.InputStrategySimplified, key, ctx)
	if !ok {
		return false, nil
	}
	result := m.operationExecutor.Execute(operation.OperationExecutionRequest{
		ID:      binding.OperationID,
		Context: ctx,
	})
	switch message := result.AsMessage().(type) {
	case nil:
		return true, nil
	case operation.OperationCommandResultMsg:
		return true, message.Cmd
	default:
		return true, func() tea.Msg { return message }
	}
}

func (m *AppModel) pendingChordGuidance() (chrome.GuidanceSpec, bool) {
	if m == nil || m.operationRegistry == nil || len(m.pendingChord) == 0 {
		return chrome.GuidanceSpec{}, false
	}
	ctx := m.operationCommandContext()
	ctx.View = m.currentView()
	hints := m.operationRegistry.DecoratedChordHintsForPrefix(
		operation.InputStrategySimplified,
		m.pendingChord,
		ctx,
		m.crossOutliner.decoratePendingChordHint,
	)
	if len(hints) == 0 {
		return chrome.GuidanceSpec{}, false
	}
	return pendingChordGuidanceSpec(strings.Join(m.pendingChord, ""), hints), true
}

func pendingChordGuidanceSpec(prefix string, hints []operation.ChordHint) chrome.GuidanceSpec {
	spec := chrome.GuidanceSpec{Rows: 2}
	if prefix = strings.TrimSpace(prefix); prefix != "" {
		spec.Hints = append(spec.Hints, chrome.GuidanceHint{Key: prefix + "…"})
	}
	for index, hint := range hints {
		spec.Hints = append(spec.Hints, chrome.GuidanceHint{
			Key:      hint.Key,
			Label:    hint.Label,
			Priority: index + 1,
		})
	}
	spec.Hints = append(spec.Hints, chrome.GuidanceHint{
		Key:      "Esc",
		Label:    "cancel",
		Required: true,
	})
	return spec
}
