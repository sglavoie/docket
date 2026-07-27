package tui

import (
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
// harness discovery all project from this same list.
func (registry *OperationRegistry) InputStrategyOperations(
	id InputStrategyID,
	ctx CommandContext,
) []OperationDescriptor {
	strategy, ok := registry.InputStrategy(id)
	if !ok {
		return nil
	}
	operations := make([]OperationDescriptor, 0, len(strategy.Bindings))
	for _, binding := range strategy.Bindings {
		descriptor, exists := registry.Descriptor(binding.OperationID)
		if exists && descriptor.Applicable(ctx) {
			operations = append(operations, descriptor)
		}
	}
	return operations
}

// HelpItemsForInputStrategy generates the complete documented key grammar.
func (registry *OperationRegistry) HelpItemsForInputStrategy(
	id InputStrategyID,
	ctx CommandContext,
) []HelpItem {
	strategy, ok := registry.InputStrategy(id)
	if !ok {
		return nil
	}
	items := make([]HelpItem, 0, len(strategy.Bindings))
	for _, binding := range strategy.Bindings {
		descriptor, exists := registry.Descriptor(binding.OperationID)
		if !exists || !descriptor.Applicable(ctx) {
			continue
		}
		items = append(items, HelpItem{
			Key:         binding.DisplayKey,
			Description: descriptor.Label,
			Category:    operationCategoryLabel(descriptor.Category),
		})
	}
	return items
}

// HandledKeysForInputStrategy returns every physical direct key exactly once.
func (registry *OperationRegistry) HandledKeysForInputStrategy(id InputStrategyID) []string {
	strategy, ok := registry.InputStrategy(id)
	if !ok {
		return nil
	}
	seen := make(map[string]bool)
	keys := make([]string, 0)
	for _, binding := range strategy.Bindings {
		for _, key := range binding.Keys {
			if !seen[key] {
				seen[key] = true
				keys = append(keys, key)
			}
		}
	}
	return keys
}

// FooterHintsForInputStrategy generates compact "key action" hints. A positive
// limit keeps the first N ranked hints; zero includes every ranked hint.
func (registry *OperationRegistry) FooterHintsForInputStrategy(
	id InputStrategyID,
	ctx CommandContext,
	limit int,
) string {
	strategy, ok := registry.InputStrategy(id)
	if !ok {
		return ""
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
	sort.SliceStable(hints, func(i, j int) bool { return hints[i].rank < hints[j].rank })
	if limit > 0 && len(hints) > limit {
		hints = hints[:limit]
	}
	parts := make([]string, len(hints))
	for i := range hints {
		parts[i] = hints[i].text
	}
	return strings.Join(parts, " · ")
}

// InputShortcutForOperation returns a strategy's display key for one operation.
func (registry *OperationRegistry) InputShortcutForOperation(
	id InputStrategyID,
	operationID OperationID,
) string {
	strategy, ok := registry.InputStrategy(id)
	if !ok {
		return ""
	}
	for _, binding := range strategy.Bindings {
		if binding.OperationID == operationID {
			return binding.DisplayKey
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
	case OperationCategoryReview:
		return "Review"
	case OperationCategoryNotification:
		return "Notifications"
	case OperationCategorySystem:
		return "System"
	default:
		return "Other"
	}
}
