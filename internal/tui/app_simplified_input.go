package tui

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/docket/internal/app"
	"github.com/sglavoie/docket/internal/logging"
	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/tui/chrome"
	"github.com/sglavoie/docket/internal/tui/operation"
	"github.com/sglavoie/docket/internal/tui/presentation"
	"github.com/sglavoie/docket/internal/tui/status"
	"github.com/sglavoie/docket/internal/tui/taskref"
)

// handleSimplifiedInputKey resolves one root-level key through the selected
// OperationRegistry strategy, extending any pending chord prefix first. It
// deliberately returns handled=false while an inline Outliner control owns input
// so the normal view Update path can process search, quick edit, and confirmation
// keystrokes.
func (m *AppModel) handleSimplifiedInputKey(msg tea.KeyPressMsg) (bool, tea.Cmd) {
	if !m.operations.Wired() {
		return false, nil
	}
	view := m.currentView()
	if view != presentation.ViewCrossProjectOutliner {
		m.operations.Chord = nil
		return false, nil
	}
	out, ok := m.outlinerPrimary()
	if !ok || out == nil || !m.activeOutlinerRoute() {
		m.operations.Chord = nil
		return false, nil
	}
	if out.SimplifiedInlineControlOwnsInput() {
		m.operations.Chord = nil
		return false, nil
	}

	ctx := m.operationCommandContext()
	ctx.View = view
	sequence := append(slices.Clone(m.operations.Chord), msg.String())
	return m.dispatchOperationKey(operation.InputStrategySimplified, sequence, ctx)
}

// operation.KeyDispatch is the seam a mounted surface uses to reach
// dispatchOperationKey below. It lives in internal/tui/operation because the
// surfaces that hold one are packages below this shell.

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
		m.operations.Chord = nil
		return true, nil
	}
	binding, _, resolution := m.operations.Registry.InputBindingForSequence(strategyID, sequence, ctx)
	switch resolution {
	case operation.ChordPending:
		m.operations.Chord = sequence
		return true, nil
	case operation.ChordNoMatch:
		// A complete strategy consumes unknown root keys instead of allowing a
		// retired legacy key or an abandoned chord to run in the view beneath it.
		m.operations.Chord = nil
		if cmd := m.entityScopedKeyRejection(strategyID, sequence, ctx); cmd != nil {
			return true, cmd
		}
		return true, m.chordMissStatus(strategyID, sequence)
	case operation.ChordTerminal:
	}
	m.operations.Chord = nil

	result := m.operations.Executor.Execute(operation.OperationExecutionRequest{
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
		if !m.operations.Registry.InputPrefixRefusedOnChecklistRow(strategyID, sequence) {
			return nil
		}
		rejection := errors.New(operation.ChecklistRowRejectionMessage)
		return func() tea.Msg { return status.ErrorMsg{Err: rejection} }
	}
	if !operation.CommandContextNamesNonTaskEntity(ctx) {
		return nil
	}
	category, ok := m.operations.Registry.InputCategoryForPrefix(strategyID, sequence)
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
	label, ok := m.operations.Registry.ChordGroupLabelForPrefix(strategyID, sequence)
	if !ok {
		return nil
	}
	prefix := strings.Join(sequence[:len(sequence)-1], "")
	m.setWarningStatus(strings.Join(sequence, "") + " is not a " + label + " action. Press " + prefix + " to see choices.")
	return m.statusReadingTimeout()
}

// executeSimplifiedGlobalKey routes a grammar-owned global key through the same
// descriptor and executor even when a subordinate route covers the Outliner.
// Ctrl+C is currently the only such key.
func (m *AppModel) executeSimplifiedGlobalKey(key string) (bool, tea.Cmd) {
	if !m.operations.Wired() {
		return false, nil
	}
	ctx := m.operationCommandContext()
	ctx.View = presentation.ViewCrossProjectOutliner
	ctx.Capabilities.Add(operation.CapabilitySimplifiedInput)
	binding, _, ok := m.operations.Registry.InputBindingForKey(operation.InputStrategySimplified, key, ctx)
	if !ok {
		return false, nil
	}
	result := m.operations.Executor.Execute(operation.OperationExecutionRequest{
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
	if m == nil || m.operations.Registry == nil || len(m.operations.Chord) == 0 {
		return chrome.GuidanceSpec{}, false
	}
	ctx := m.operationCommandContext()
	ctx.View = m.currentView()
	hints := m.operations.Registry.DecoratedChordHintsForPrefix(
		operation.InputStrategySimplified,
		m.operations.Chord,
		ctx,
		m.crossOutliner.PendingChordHintDecorator(ctx),
	)
	if len(hints) == 0 {
		return chrome.GuidanceSpec{}, false
	}
	return pendingChordGuidanceSpec(strings.Join(m.operations.Chord, ""), hints), true
}

// pendingChordGuidanceSpec projects the which-key entries the registry named.
// A hint's division is carried by every member of it, so this is where a change
// of division becomes a heading: the first entry of each run opens it and the
// rest of the run stays bare. The heading rides that entry rather than standing
// as a hint of its own, so the packer keeps or drops the two together and a
// division the row cannot show is never announced.
//
// A chord's entries are the whole answer to the prefix just pressed, so it asks
// for the packer's full budget. Selection follows declaration order except for
// an entry its binding asked to keep, which ranks with the head of its own run:
// behind the entry carrying the heading, so it is never drawn under another
// division's name, and ahead of every other member of that run. A kept entry
// standing under no heading has none to be stranded from, so it ranks ahead of
// every division instead and is the last optional entry the row sheds.
func pendingChordGuidanceSpec(prefix string, hints []operation.ChordHint) chrome.GuidanceSpec {
	spec := chrome.GuidanceSpec{Rows: chrome.MaxGuidanceRows}
	if prefix = strings.TrimSpace(prefix); prefix != "" {
		spec.Hints = append(spec.Hints, chrome.GuidanceHint{Key: prefix + "…"})
	}
	group, runPriority := "", 0
	for index, hint := range hints {
		heading, priority := "", index+1
		if hint.Group != group || index == 0 {
			heading, group, runPriority = hint.Group, hint.Group, priority
		}
		if hint.Keep {
			priority = runPriority
			if group == "" {
				priority = 0
			}
		}
		spec.Hints = append(spec.Hints, chrome.GuidanceHint{
			Key:      hint.Key,
			Label:    hint.Label,
			Group:    heading,
			Priority: priority,
			Dimmed:   hint.Dimmed,
		})
	}
	spec.Hints = append(spec.Hints, chrome.GuidanceHint{
		Key:      "Esc",
		Label:    "cancel",
		Required: true,
	})
	return spec
}

// Persist the canonical mode DTO for the active Outliner scope. Filter facets
// are run-local and typed navigation continues to own scope and route hierarchy.
func (m *AppModel) saveOutlinerFilterState() {
	outlinerView := m.crossOutliner
	if outlinerView == nil {
		return
	}
	if err := outlinerView.PersistActiveRouteState(); err != nil {
		logging.Warn("failed to persist Outliner route state: %v", err)
	}
}

// applyTagsToTask applies selected tags to a single task as one asynchronous
// transaction through the task operation path. Setting tags is a value
// operation, so it commits through TaskItem.ExecuteOperation like every other
// `#` edit; the shell owns reload and toast via the batch result, and the
// durable write records through the workspace undo ledger.
func (m *AppModel) applyTagsToTask(identity taskref.TaskIdentity, task *models.Task, selectedTags []string) tea.Cmd {
	if task == nil || !identity.Valid() {
		return nil
	}
	return m.secondaryFeatures.TaskItem.ExecuteOperation(app.TaskOperationRequest{
		Operation: app.TaskOperationSetTags,
		Targets:   []app.TaskLocator{identity.Locator()},
		Tags:      selectedTags,
	}, "Set Tags", fmt.Sprintf("Tagged #%d", task.ShortID))
}
