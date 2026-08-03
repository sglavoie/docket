package tui

import (
	"slices"
	"sort"
	"strings"
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
	strategy, ok := registry.InputStrategy(id)
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
	strategy, ok := registry.InputStrategy(id)
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
			Key:         displayKeysForOperation(strategy, binding.OperationID),
			Description: descriptor.Label,
			Category:    operationCategoryLabel(descriptor.Category),
		})
	}
	return items
}

// HandledKeysForInputStrategy returns every physical key the grammar consumes
// exactly once, including the keys that only appear inside a chord.
func (registry *OperationRegistry) HandledKeysForInputStrategy(id InputStrategyID) []string {
	strategy, ok := registry.InputStrategy(id)
	if !ok {
		return nil
	}
	seen := make(map[string]bool)
	keys := make([]string, 0)
	for _, binding := range strategy.Bindings {
		for _, path := range binding.paths() {
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

// FooterHintPairsForInputStrategy returns complete "key action" pairs for
// ranked bindings and chord groups. Entries are ordered by the established
// footer rank and filtered through the same applicability checks as dispatch
// and help. Renderers can therefore fit whole pairs without parsing or clipping
// the operation metadata string.
func (registry *OperationRegistry) FooterHintPairsForInputStrategy(
	id InputStrategyID,
	ctx CommandContext,
) []string {
	strategy, ok := registry.InputStrategy(id)
	if !ok {
		return nil
	}
	type rankedHint struct {
		rank int
		text string
	}
	var hints []rankedHint
	for _, binding := range strategy.Bindings {
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
			rank: binding.FooterRank,
			text: binding.DisplayKey + " " + label,
		})
	}
	for _, group := range strategy.ChordGroups {
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
			rank: group.FooterRank,
			text: strings.Join(group.Sequence, "") + " " + label,
		})
	}
	sort.SliceStable(hints, func(i, j int) bool { return hints[i].rank < hints[j].rank })
	parts := make([]string, len(hints))
	for i := range hints {
		parts[i] = hints[i].text
	}
	return parts
}

// FooterHintsForInputStrategy generates compact "key action" hints for ranked
// bindings and ranked chord groups. A positive limit keeps the first N ranked
// hints; zero includes every ranked hint.
func (registry *OperationRegistry) FooterHintsForInputStrategy(
	id InputStrategyID,
	ctx CommandContext,
	limit int,
) string {
	parts := registry.FooterHintPairsForInputStrategy(id, ctx)
	if limit > 0 && len(parts) > limit {
		parts = parts[:limit]
	}
	return strings.Join(parts, footerHelpSeparator)
}

// InputShortcutForOperation returns a strategy's display key for one operation.
// Preset-valued bindings share an operation, so their distinct display keys are
// composed into one spelling.
func (registry *OperationRegistry) InputShortcutForOperation(
	id InputStrategyID,
	operationID OperationID,
) string {
	strategy, ok := registry.InputStrategy(id)
	if !ok {
		return ""
	}
	return displayKeysForOperation(strategy, operationID)
}

func displayKeysForOperation(strategy InputStrategy, operationID OperationID) string {
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
type ChordHint struct {
	Key   string
	Label string
}

// ChordHintsForPrefix returns the which-key entries for a pending chord prefix
// in declaration order. Terminal children are named by their binding, deeper
// prefixes by their chord group, so hints and dispatch read the same trie.
func (registry *OperationRegistry) ChordHintsForPrefix(
	id InputStrategyID,
	prefix []string,
	ctx CommandContext,
) []ChordHint {
	strategy, ok := registry.InputStrategy(id)
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
		var label string
		if child.terminal() {
			binding := strategy.Bindings[child.bindingIndex]
			label = binding.HintLabel
			if label == "" {
				label = binding.FooterLabel
			}
			if label == "" {
				if descriptor, exists := registry.Descriptor(binding.OperationID); exists {
					label = strings.ToLower(descriptor.Label)
				}
			}
		} else {
			label = chordGroupHintLabel(strategy, append(append([]string(nil), prefix...), key))
		}
		hints = append(hints, ChordHint{Key: key, Label: label})
	}
	return hints
}

// ChordHintRowForPrefix renders the pending chord's which-key footer row.
func (registry *OperationRegistry) ChordHintRowForPrefix(
	id InputStrategyID,
	prefix []string,
	ctx CommandContext,
) string {
	hints := registry.ChordHintsForPrefix(id, prefix, ctx)
	if len(hints) == 0 {
		return ""
	}
	parts := make([]string, 0, len(hints)+1)
	parts = append(parts, strings.Join(prefix, "")+"…")
	for _, hint := range hints {
		parts = append(parts, hint.Key+" "+hint.Label)
	}
	return strings.Join(parts, " · ")
}

func (registry *OperationRegistry) chordNodeForPrefix(id InputStrategyID, prefix []string) *inputChordNode {
	if registry == nil {
		return nil
	}
	node := registry.inputChords[id]
	for _, raw := range prefix {
		node = node.child(normalizeOperationKey(raw))
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

func operationCategoryLabel(category OperationCategory) string {
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
