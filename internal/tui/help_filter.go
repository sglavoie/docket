package tui

import (
	"github.com/sglavoie/tt/internal/fuzzy"
)

// buildCategories extracts ordered unique tab labels for the current help mode.
// Help labels tabs by intent bucket so they match the rendered section headers.
func (m *HelpViewModel) buildCategories() []string {
	return m.buildIntentCategories()
}

// buildIntentCategories returns the intent buckets present in m.items, in the
// canonical helpIntentOrder. Buckets with no items are skipped.
func (m *HelpViewModel) buildIntentCategories() []string {
	present := make(map[string]bool)
	for _, item := range m.items {
		present[intentForItem(item, m.sourceView)] = true
	}
	cats := make([]string, 0, len(helpIntentOrder))
	for _, intent := range helpIntentOrder {
		if present[intent] {
			cats = append(cats, intent)
		}
	}
	return cats
}

// matchTab fuzzy-matches the query against category names and selects the best match.
func (m *HelpViewModel) matchTab(query string) {
	if query == "" {
		m.activeTab = 0 // "All" tab
		m.filterItems()
		m.viewport.SetContent(m.renderContent())
		m.viewport.GotoTop()
		return
	}
	bestIdx := 0
	bestScore := 0
	for i, cat := range m.categories {
		score, _ := fuzzy.ScoreTermsWithPositions(cat, query)
		if score > bestScore {
			bestScore = score
			bestIdx = i + 1 // +1 because tab 0 is "All"
		}
	}
	if bestScore > 0 {
		m.activeTab = bestIdx
	} else {
		m.activeTab = 0
	}
	m.filterItems()
	m.viewport.SetContent(m.renderContent())
	m.viewport.GotoTop()
}

func (m *HelpViewModel) filterItems() {
	// Start with all items or tab-filtered items
	source := m.items
	if m.activeTab > 0 && m.activeTab <= len(m.categories) {
		activeLabel := m.categories[m.activeTab-1]
		source = nil
		matches := func(item HelpItem) bool { return intentForItem(item, m.sourceView) == activeLabel }
		for _, item := range m.items {
			if matches(item) {
				source = append(source, item)
			}
		}
	}

	if m.searchQuery == "" {
		m.filteredItems = make([]filteredHelpItem, len(source))
		for i, item := range source {
			m.filteredItems[i] = filteredHelpItem{HelpItem: item}
		}
		return
	}

	var filtered []filteredHelpItem

	for _, item := range source {
		keyScore, keyPos := fuzzy.ScoreTermsWithPositions(item.Key, m.searchQuery)
		descScore, descPos := fuzzy.ScoreTermsWithPositions(item.Description, m.searchQuery)
		catScore, _ := fuzzy.ScoreTermsWithPositions(item.Category, m.searchQuery)

		if keyScore > 0 || descScore > 0 || catScore > 0 {
			filtered = append(filtered, filteredHelpItem{
				HelpItem:      item,
				keyPositions:  keyPos,
				descPositions: descPos,
			})
		}
	}

	m.filteredItems = filtered
}
