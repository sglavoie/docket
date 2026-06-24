package tui

import "sort"

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
		iPinned := m.isItemPinned(sorted[i])
		jPinned := m.isItemPinned(sorted[j])
		if iPinned != jPinned {
			return iPinned
		}
		return orderMap[sorted[i].Category] < orderMap[sorted[j].Category]
	})

	return sorted
}

// isItemPinned returns true if the item is a pinned command.
func (m *CommandPaletteModel) isItemPinned(item PaletteItem) bool {
	if item.Category != ItemCategoryCommands || item.Command == nil {
		return false
	}
	return m.pinnedCommands[item.Command.Name]
}

// injectFrequentItems prepends a "Recent" section to filteredItems using the top items
// from recentPaletteSelections in recency order (most recently used first), max 5.
// The recent items are marked with IsFrequent=true; they are duplicates of regular items.
func (m *CommandPaletteModel) injectFrequentItems() {
	if len(recentPaletteSelections) == 0 {
		return
	}

	const maxRecent = 5
	var recentItems []filteredPaletteItem
	for _, rs := range recentPaletteSelections {
		if len(recentItems) >= maxRecent {
			break
		}
		for _, fi := range m.filteredItems {
			id := m.getItemID(fi.Item)
			if fi.Item.Category == rs.Category && id == rs.ID {
				recentItems = append(recentItems, filteredPaletteItem{
					Item:       fi.Item,
					IsFrequent: true,
				})
				break
			}
		}
	}

	if len(recentItems) == 0 {
		return
	}

	m.filteredItems = append(recentItems, m.filteredItems...)
}

// injectSuggestedItems appends a "Suggested" section after the Recent section.
// Suggested items are commands applicable to the current view that have never been used
// (not in recentPaletteSelections). Limited to 3 items.
//
// For inbox processor views the suggestion list is a curated triage-vocabulary
// ordering so a brand-new user lands on the actions Sessions 2-5 made the
// "what should I do next?" answer (plan today, send to Next, due today, snooze)
// before falling back to general action/filter priority. Other views keep the
// generic category-priority ordering.
func (m *CommandPaletteModel) injectSuggestedItems() {
	usedIDs := make(map[string]bool, len(recentPaletteSelections))
	for _, rs := range recentPaletteSelections {
		if rs.Category == ItemCategoryCommands {
			usedIDs[rs.ID] = true
		}
	}

	const maxSuggested = 3
	var suggested []filteredPaletteItem

	if m.currentView == ViewInboxProcessor || m.currentView == ViewGlobalInboxProcessor {
		suggested = m.collectInboxSuggestedItems(usedIDs, maxSuggested)
	}

	if len(suggested) < maxSuggested {
		suggested = append(suggested, m.collectCategorySuggestedItems(usedIDs, maxSuggested-len(suggested), suggested)...)
	}

	if len(suggested) == 0 {
		return
	}

	splitIdx := 0
	for splitIdx < len(m.filteredItems) && m.filteredItems[splitIdx].IsFrequent {
		splitIdx++
	}
	result := make([]filteredPaletteItem, 0, len(m.filteredItems)+len(suggested))
	result = append(result, m.filteredItems[:splitIdx]...)
	result = append(result, suggested...)
	result = append(result, m.filteredItems[splitIdx:]...)
	m.filteredItems = result
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

func (m *CommandPaletteModel) collectInboxSuggestedItems(usedIDs map[string]bool, maxSuggested int) []filteredPaletteItem {
	byName := make(map[string]PaletteItem)
	for _, fi := range m.filteredItems {
		if fi.Item.Category != ItemCategoryCommands || fi.Item.Command == nil {
			continue
		}
		id := m.getItemID(fi.Item)
		if usedIDs[id] {
			continue
		}
		byName[fi.Item.Command.Name] = fi.Item
	}

	suggested := make([]filteredPaletteItem, 0, maxSuggested)
	for _, name := range inboxSuggestedCommandOrder {
		if len(suggested) >= maxSuggested {
			break
		}
		item, ok := byName[name]
		if !ok {
			continue
		}
		suggested = append(suggested, filteredPaletteItem{
			Item:        item,
			IsSuggested: true,
		})
	}
	return suggested
}

func (m *CommandPaletteModel) collectCategorySuggestedItems(usedIDs map[string]bool, remaining int, alreadySuggested []filteredPaletteItem) []filteredPaletteItem {
	if remaining <= 0 {
		return nil
	}

	priorityCategories := []CommandCategory{
		CategoryFilters,
		CategoryActions,
		CategorySort,
		CategoryStatus,
		CategoryViews,
	}

	alreadyIDs := make(map[string]bool, len(alreadySuggested))
	for _, fi := range alreadySuggested {
		alreadyIDs[m.getItemID(fi.Item)] = true
	}

	byCategory := make(map[CommandCategory][]PaletteItem)
	for _, fi := range m.filteredItems {
		if fi.Item.Category != ItemCategoryCommands || fi.Item.Command == nil {
			continue
		}
		id := m.getItemID(fi.Item)
		if usedIDs[id] || alreadyIDs[id] {
			continue
		}
		cat := fi.Item.Command.Category
		byCategory[cat] = append(byCategory[cat], fi.Item)
	}

	var suggested []filteredPaletteItem
	for _, cat := range priorityCategories {
		for _, item := range byCategory[cat] {
			if len(suggested) >= remaining {
				break
			}
			suggested = append(suggested, filteredPaletteItem{
				Item:        item,
				IsSuggested: true,
			})
		}
		if len(suggested) >= remaining {
			break
		}
	}
	return suggested
}
