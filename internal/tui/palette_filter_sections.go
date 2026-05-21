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
// (not in recentPaletteSelections). Limited to 3 items, preferring action/filter categories.
func (m *CommandPaletteModel) injectSuggestedItems() {
	usedIDs := make(map[string]bool, len(recentPaletteSelections))
	for _, rs := range recentPaletteSelections {
		if rs.Category == ItemCategoryCommands {
			usedIDs[rs.ID] = true
		}
	}

	priorityCategories := []CommandCategory{
		CategoryFilters,
		CategoryActions,
		CategorySort,
		CategoryStatus,
		CategoryViews,
	}

	byCategory := make(map[CommandCategory][]PaletteItem)
	for _, fi := range m.filteredItems {
		if fi.Item.Category != ItemCategoryCommands || fi.Item.Command == nil {
			continue
		}
		id := m.getItemID(fi.Item)
		if usedIDs[id] {
			continue
		}
		cat := fi.Item.Command.Category
		byCategory[cat] = append(byCategory[cat], fi.Item)
	}

	const maxSuggested = 3
	var suggested []filteredPaletteItem
	for _, cat := range priorityCategories {
		for _, item := range byCategory[cat] {
			if len(suggested) >= maxSuggested {
				break
			}
			suggested = append(suggested, filteredPaletteItem{
				Item:        item,
				IsSuggested: true,
			})
		}
		if len(suggested) >= maxSuggested {
			break
		}
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
