package tui

import (
	"sort"
	"strings"
)

const (
	maxEmptyPaletteRecentCommands     = 6
	maxEmptyPaletteContextualCommands = 6
)

// buildEmptyPaletteItems returns the deliberately small discovery surface
// shown before the user types. Recent commands must also be available in the
// active capability context. Contextual commands exclude those already shown
// as recent so the palette exposes up to twelve distinct operations.
func (m *CommandPaletteModel) buildEmptyPaletteItems() []filteredPaletteItem {
	available := make(map[string]PaletteItem)
	for _, item := range m.allItems {
		if item.Category == ItemCategoryCommands && item.Command != nil {
			available[item.Command.Name] = item
		}
	}

	seen := make(map[string]bool)
	result := make([]filteredPaletteItem, 0, maxEmptyPaletteRecentCommands+maxEmptyPaletteContextualCommands)
	for _, recent := range recentPaletteSelections {
		if len(result) >= maxEmptyPaletteRecentCommands {
			break
		}
		if recent.Category != ItemCategoryCommands {
			continue
		}
		name := recent.ID
		item, ok := available[name]
		if !ok || seen[name] {
			continue
		}
		seen[name] = true
		result = append(result, filteredPaletteItem{Item: item, IsFrequent: true})
	}

	contextual := make([]PaletteItem, 0, maxEmptyPaletteContextualCommands)
	appendContextual := func(item PaletteItem) {
		if len(contextual) >= maxEmptyPaletteContextualCommands || item.Command == nil || seen[item.Command.Name] {
			return
		}
		seen[item.Command.Name] = true
		contextual = append(contextual, item)
	}

	// Route-owned suggestions lead when supplied, followed by the remaining
	// capability-filtered registry order.
	contextualOrder := m.suggestedCommandOrder
	if len(contextualOrder) == 0 && (m.currentView == ViewInboxProcessor || m.currentView == ViewGlobalInboxProcessor) {
		contextualOrder = inboxSuggestedCommandOrder
	}
	for _, name := range contextualOrder {
		if item, ok := available[name]; ok {
			appendContextual(item)
		}
	}
	for _, item := range m.allItems {
		if item.Category == ItemCategoryCommands {
			appendContextual(item)
		}
	}
	for _, item := range contextual {
		result = append(result, filteredPaletteItem{Item: item, IsSuggested: true})
	}
	return result
}

// sortItemsByCategory sorts items by the predefined category order for display without search.
// Pinned commands are placed at the very top before all other categories.
func (m *CommandPaletteModel) sortItemsByCategory(items []PaletteItem) []PaletteItem {
	orderMap := make(map[ItemCategory]int)
	for i, cat := range categoryOrder {
		orderMap[cat] = i
	}

	sorted := make([]PaletteItem, len(items))
	copy(sorted, items)

	sort.SliceStable(sorted, func(i, j int) bool {
		iOrder, iKnown := orderMap[sorted[i].Category]
		jOrder, jKnown := orderMap[sorted[j].Category]
		if !iKnown {
			iOrder = len(categoryOrder)
		}
		if !jKnown {
			jOrder = len(categoryOrder)
		}
		if iOrder != jOrder {
			return iOrder < jOrder
		}
		iName := strings.ToLower(sorted[i].Name)
		jName := strings.ToLower(sorted[j].Name)
		if iName != jName {
			return iName < jName
		}
		return m.getItemID(sorted[i]) < m.getItemID(sorted[j])
	})

	return sorted
}

// inboxSuggestedCommandOrder lists palette commands in the order they should
// appear in the Suggested section while the user is inside an inbox processor.
// It mirrors the triage-action vocabulary surfaced in the tutor's "Inbox
// Processing" topic and the close-message hints from session 5.
var inboxSuggestedCommandOrder = []string{
	"plan today",
	"send to next",
	"set due date today",
	"snooze inbox task",
	"mark task done",
	"edit inbox task",
	"manage inbox task tags",
}
