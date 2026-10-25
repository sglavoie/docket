package operation

import (
	"slices"
	"sort"
	"strings"

	"github.com/sglavoie/docket/internal/tui/chrome"
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
		descriptor, exists := registry.Descriptor(binding.OperationID)
		if !exists || !descriptor.Applicable(ctx) {
			seen[binding.OperationID] = true
			continue
		}
		if presets := presetChordHelpItems(strategy, binding.OperationID, descriptor); len(presets) > 0 {
			seen[binding.OperationID] = true
			items = append(items, presets...)
			continue
		}
		seen[binding.OperationID] = true
		items = append(items, HelpItem{
			Key:         DisplayKeysForOperation(strategy, binding.OperationID),
			Description: descriptor.Label,
			Category:    bindingHelpCategory(binding, descriptor),
			Group:       binding.HelpGroup,
			Order:       binding.HelpOrder,
			Aliases:     descriptor.Aliases,
		})
	}
	return items
}

// presetChordHelpItems spells a chord whose one display key compresses several
// presets — `wi/g/w/p/m/d/c` — as one help row per preset instead, `wi` Move
// to Inbox, so `?` names what each letter does rather than leaving the
// reader to decode a run of initials. Any other operation returns nothing and
// keeps its single row.
func presetChordHelpItems(strategy InputStrategy, operationID OperationID, descriptor OperationDescriptor) []HelpItem {
	var items []HelpItem
	for _, binding := range strategy.Bindings {
		if binding.OperationID != operationID {
			continue
		}
		if !binding.HasPresetValue || binding.HintLabel == "" || len(binding.Sequence) == 0 ||
			!strings.Contains(binding.DisplayKey, "/") {
			return nil
		}
		label := binding.HintLabel
		description := binding.HelpLabel
		if description == "" {
			description = descriptor.Label + ": " + strings.ToUpper(label[:1]) + label[1:]
		}
		aliases := append(slices.Clone(descriptor.Aliases), descriptor.Label, label)
		items = append(items, HelpItem{
			Key:         strings.Join(binding.Sequence, ""),
			Description: description,
			Category:    bindingHelpCategory(binding, descriptor),
			Group:       binding.HelpGroup,
			Order:       binding.HelpOrder,
			Aliases:     aliases,
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
// heading falls and the registry only says what belongs together. Keep is the
// binding's HintKeep, read the same way.
type ChordHint struct {
	Key    string
	Label  string
	Group  string
	Keep   bool
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
		var hint ChordHint
		if child.terminal() {
			hint = registry.chordTerminalHint(strategy, child, ctx)
		} else {
			hint = chordGroupHint(strategy, append(append([]string(nil), prefix...), key))
		}
		hint.Key = key
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
) ChordHint {
	for _, index := range node.bindings {
		binding := strategy.Bindings[index]
		descriptor, exists := registry.Descriptor(binding.OperationID)
		if !exists || !descriptor.Applicable(ctx) {
			continue
		}
		hint := ChordHint{Group: binding.HintGroup, Keep: binding.HintKeep}
		switch {
		case binding.HintLabel != "":
			hint.Label = binding.HintLabel
		case binding.FooterLabel != "":
			hint.Label = binding.FooterLabel
		default:
			hint.Label = strings.ToLower(descriptor.Label)
		}
		return hint
	}
	return ChordHint{}
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

// chordGroupHint names a deeper prefix by its chord group, label and division
// both, so a nested chord stands in its parent's row like any terminal sibling.
func chordGroupHint(strategy InputStrategy, prefix []string) ChordHint {
	for _, group := range strategy.ChordGroups {
		if slices.Equal(group.Sequence, prefix) {
			return ChordHint{Label: group.HintLabel, Group: group.HintGroup}
		}
	}
	return ChordHint{}
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

func bindingHelpCategory(binding OperationKeyBinding, descriptor OperationDescriptor) string {
	if binding.HelpCategory != "" {
		return binding.HelpCategory
	}
	return OperationCategoryLabel(descriptor.Category)
}
