package operation

import (
	"slices"
	"sort"
	"strings"

	"github.com/sglavoie/tt/internal/tui/chrome"
)

// Discover returns every applicable operation without interpreting legacy
// Command shortcuts. It is the read-only discovery seam used by the harness
// and other non-renderer consumers.
func (registry *OperationRegistry) Discover(ctx CommandContext) []OperationDescriptor {
	if registry == nil {
		return nil
	}
	descriptors := make([]OperationDescriptor, 0)
	for _, descriptor := range registry.All() {
		if descriptor.Applicable(ctx) {
			descriptors = append(descriptors, descriptor)
		}
	}
	return descriptors
}

// InputStrategyOperations returns applicable operations in strategy binding
// order. Direct dispatch, help, footers, handled-key metadata, and simplified
// harness discovery all project from this same list. Preset-valued bindings
// share one operation, so each operation appears once.
func (registry *OperationRegistry) InputStrategyOperations(
	id InputStrategyID,
	ctx CommandContext,
) []OperationDescriptor {
	strategy, ok := registry.storedInputStrategy(id)
	if !ok {
		return nil
	}
	operations := make([]OperationDescriptor, 0, len(strategy.Bindings))
	seen := make(map[OperationID]bool, len(strategy.Bindings))
	for _, binding := range strategy.Bindings {
		if seen[binding.OperationID] {
			continue
		}
		seen[binding.OperationID] = true
		descriptor, exists := registry.Descriptor(binding.OperationID)
		if exists && descriptor.Applicable(ctx) {
			operations = append(operations, descriptor)
		}
	}
	return operations
}

// HelpItemsForInputStrategy generates the complete documented key grammar, one
// row per operation with its composed display keys.
func (registry *OperationRegistry) HelpItemsForInputStrategy(
	id InputStrategyID,
	ctx CommandContext,
) []HelpItem {
	strategy, ok := registry.storedInputStrategy(id)
	if !ok {
		return nil
	}
	items := make([]HelpItem, 0, len(strategy.Bindings))
	seen := make(map[OperationID]bool, len(strategy.Bindings))
	for _, binding := range strategy.Bindings {
		if seen[binding.OperationID] {
			continue
		}
		seen[binding.OperationID] = true
		descriptor, exists := registry.Descriptor(binding.OperationID)
		if !exists || !descriptor.Applicable(ctx) {
			continue
		}
		items = append(items, HelpItem{
			Key:         DisplayKeysForOperation(strategy, binding.OperationID),
			Description: descriptor.Label,
			Category:    OperationCategoryLabel(descriptor.Category),
			Aliases:     descriptor.Aliases,
		})
	}
	return items
}

// HandledKeysForInputStrategy returns every physical key the grammar consumes
// exactly once, including the keys that only appear inside a chord.
func (registry *OperationRegistry) HandledKeysForInputStrategy(id InputStrategyID) []string {
	strategy, ok := registry.storedInputStrategy(id)
	if !ok {
		return nil
	}
	seen := make(map[string]bool)
	keys := make([]string, 0)
	for _, binding := range strategy.Bindings {
		for _, path := range binding.Paths() {
			for _, key := range path {
				if !seen[key] {
					seen[key] = true
					keys = append(keys, key)
				}
			}
		}
	}
	return keys
}

// GuidanceHintsForInputStrategy projects ranked bindings and chord groups into
// structured guidance. Entries use the same descriptor applicability and chord
// subtree checks as dispatch and Help, so unavailable operations cannot leak
// into presentation chrome.
func (registry *OperationRegistry) GuidanceHintsForInputStrategy(
	id InputStrategyID,
	ctx CommandContext,
) []chrome.GuidanceHint {
	strategy, ok := registry.storedInputStrategy(id)
	if !ok {
		return nil
	}
	type rankedHint struct {
		index int
		hint  chrome.GuidanceHint
	}
	var hints []rankedHint
	for index, binding := range strategy.Bindings {
		if binding.FooterRank <= 0 {
			continue
		}
		descriptor, exists := registry.Descriptor(binding.OperationID)
		if !exists || !descriptor.Applicable(ctx) {
			continue
		}
		label := binding.FooterLabel
		if label == "" {
			label = strings.ToLower(descriptor.Label)
		}
		hints = append(hints, rankedHint{
			index: index,
			hint: chrome.GuidanceHint{
				Key:      binding.DisplayKey,
				Label:    label,
				Priority: binding.FooterRank,
			},
		})
	}
	for index, group := range strategy.ChordGroups {
		if group.FooterRank <= 0 {
			continue
		}
		if !registry.chordSubtreeIsApplicable(strategy, registry.chordNodeForPrefix(id, group.Sequence), ctx) {
			continue
		}
		label := group.FooterLabel
		if label == "" {
			label = group.HintLabel
		}
		hints = append(hints, rankedHint{
			index: len(strategy.Bindings) + index,
			hint: chrome.GuidanceHint{
				Key:      strings.Join(group.Sequence, ""),
				Label:    label,
				Priority: group.FooterRank,
			},
		})
	}
	sort.SliceStable(hints, func(i, j int) bool {
		if hints[i].hint.Priority != hints[j].hint.Priority {
			return hints[i].hint.Priority < hints[j].hint.Priority
		}
		return hints[i].index < hints[j].index
	})
	structured := make([]chrome.GuidanceHint, len(hints))
	for i := range hints {
		structured[i] = hints[i].hint
	}
	return structured
}

// InputShortcutForOperation returns a strategy's display key for one operation.
// Preset-valued bindings share an operation, so their distinct display keys are
// composed into one spelling.
func (registry *OperationRegistry) InputShortcutForOperation(
	id InputStrategyID,
	operationID OperationID,
) string {
	strategy, ok := registry.storedInputStrategy(id)
	if !ok {
		return ""
	}
	return DisplayKeysForOperation(strategy, operationID)
}

func DisplayKeysForOperation(strategy InputStrategy, operationID OperationID) string {
	spellings := make([]string, 0, 1)
	for _, binding := range strategy.Bindings {
		if binding.OperationID != operationID {
			continue
		}
		if !slices.Contains(spellings, binding.DisplayKey) {
			spellings = append(spellings, binding.DisplayKey)
		}
	}
	return strings.Join(spellings, "/")
}

// ChordHint is one which-key entry: the single next key and what it does.
// Group is the presentation division the binding declared, carried on every
// member rather than on the first of a run, so a projection decides where a
// heading falls and the registry only says what belongs together.
type ChordHint struct {
	Key    string
	Label  string
	Group  string
	Dimmed bool
}

// ChordHintsForPrefix returns the which-key entries for a pending chord prefix
// in declaration order. Terminal children are named by their binding, deeper
// prefixes by their chord group, so hints and dispatch read the same trie.
func (registry *OperationRegistry) ChordHintsForPrefix(
	id InputStrategyID,
	prefix []string,
	ctx CommandContext,
) []ChordHint {
	return registry.DecoratedChordHintsForPrefix(id, prefix, ctx, nil)
}

// ChordHintDecorator enriches a live which-key entry without replacing the
// registry's authoritative key path, applicability, or declaration order.
type ChordHintDecorator func(path []string, hint ChordHint) ChordHint

// DecoratedChordHintsForPrefix is the state-aware form of
// ChordHintsForPrefix. The registry still decides which entries exist; the
// caller may decorate the projected label and emphasis with runtime state.
func (registry *OperationRegistry) DecoratedChordHintsForPrefix(
	id InputStrategyID,
	prefix []string,
	ctx CommandContext,
	decorate ChordHintDecorator,
) []ChordHint {
	strategy, ok := registry.storedInputStrategy(id)
	if !ok {
		return nil
	}
	node := registry.chordNodeForPrefix(id, prefix)
	if node == nil || node.terminal() {
		return nil
	}
	hints := make([]ChordHint, 0, len(node.childOrder))
	for _, key := range node.childOrder {
		child := node.children[key]
		if !registry.chordSubtreeIsApplicable(strategy, child, ctx) {
			continue
		}
		var label, group string
		if child.terminal() {
			label, group = registry.chordTerminalHint(strategy, child, ctx)
		} else {
			label = chordGroupHintLabel(strategy, append(append([]string(nil), prefix...), key))
		}
		hint := ChordHint{Key: key, Label: label, Group: group}
		if decorate != nil {
			path := append(append([]string(nil), prefix...), key)
			hint = decorate(path, hint)
		}
		hints = append(hints, hint)
	}
	return hints
}

// chordTerminalHint names a terminal child by the binding the current cursor
// would actually dispatch, so a shared key hints the operation it means here
// rather than the first one declared. Its division comes off that same binding.
func (registry *OperationRegistry) chordTerminalHint(
	strategy InputStrategy,
	node *inputChordNode,
	ctx CommandContext,
) (string, string) {
	for _, index := range node.bindings {
		binding := strategy.Bindings[index]
		descriptor, exists := registry.Descriptor(binding.OperationID)
		if !exists || !descriptor.Applicable(ctx) {
			continue
		}
		switch {
		case binding.HintLabel != "":
			return binding.HintLabel, binding.HintGroup
		case binding.FooterLabel != "":
			return binding.FooterLabel, binding.HintGroup
		default:
			return strings.ToLower(descriptor.Label), binding.HintGroup
		}
	}
	return "", ""
}

func (registry *OperationRegistry) chordNodeForPrefix(id InputStrategyID, prefix []string) *inputChordNode {
	if registry == nil {
		return nil
	}
	node := registry.inputChords[id]
	for _, raw := range prefix {
		node = node.child(NormalizeOperationKey(raw))
		if node == nil {
			return nil
		}
	}
	return node
}

func chordGroupHintLabel(strategy InputStrategy, prefix []string) string {
	for _, group := range strategy.ChordGroups {
		if slices.Equal(group.Sequence, prefix) {
			return group.HintLabel
		}
	}
	return ""
}

func OperationCategoryLabel(category OperationCategory) string {
	switch category {
	case OperationCategoryTask:
		return "Tasks"
	case OperationCategoryNavigation:
		return "Navigation"
	case OperationCategoryMode:
		return "Modes"
	case OperationCategoryScope:
		return "Scope"
	case OperationCategoryFilter:
		return "Discovery"
	case OperationCategorySort:
		return "Sorting"
	case OperationCategoryProject:
		return "Projects"
	case OperationCategoryNote:
		return "Notes"
	case OperationCategoryTag:
		return "Tags"
	case OperationCategorySystem:
		return "System"
	default:
		return "Other"
	}
}
