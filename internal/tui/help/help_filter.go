package help

import (
	"github.com/sglavoie/tt/internal/fuzzy"
	"github.com/sglavoie/tt/internal/tui/operation"
)

// buildSectionCategories returns the sections present in m.items, ordered by the
// same rule renderBySectionContent applies, so a tab always names a header that
// exists and sits where the body puts it. It is the tab bar's only label source.
func (m *HelpViewModel) buildSectionCategories() []string {
	categories := make([]string, 0, len(m.items))
	for _, item := range m.items {
		categories = append(categories, helpSectionForItem(item))
	}
	return helpSectionsForCategories(categories)
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
	bestTab := func(matcher fuzzy.Matcher) int {
		bestIdx := 0
		bestScore := 0
		for i, cat := range m.categories {
			score, _ := matcher.ScoreTermsWithPositions(cat, query)
			if score > bestScore {
				bestScore = score
				bestIdx = i + 1 // +1 because tab 0 is "All"
			}
		}
		return bestIdx
	}

	matcher := fuzzy.SmartCase(query)
	tab := bestTab(matcher)
	if tab == 0 && matcher.CaseSensitive() {
		tab = bestTab(fuzzy.NewMatcher(false))
	}
	m.activeTab = tab
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
		matches := func(item operation.HelpItem) bool { return helpSectionForItem(item) == activeLabel }
		for _, item := range m.items {
			if matches(item) {
				source = append(source, item)
			}
		}
	}

	m.searchCaseRelaxed = false
	m.otherCategoryMatches = 0
	if m.searchQuery == "" {
		m.filteredItems = make([]filteredHelpItem, len(source))
		for i, item := range source {
			m.filteredItems[i] = filteredHelpItem{HelpItem: item}
		}
		return
	}

	// Aliases feed matching only. Their positions index into the alias, not into
	// the rendered key or description, so adding them to a highlight array would
	// paint the wrong columns.
	match := func(items []operation.HelpItem, matcher fuzzy.Matcher) []filteredHelpItem {
		var filtered []filteredHelpItem
		for _, item := range items {
			keyScore, keyPos := matcher.ScoreTermsWithPositions(item.Key, m.searchQuery)
			descScore, descPos := matcher.ScoreTermsWithPositions(item.Description, m.searchQuery)
			catScore, _ := matcher.ScoreTermsWithPositions(item.Category, m.searchQuery)

			var matchedAliases []string
			for _, alias := range item.Aliases {
				if score, _ := matcher.ScoreTermsWithPositions(alias, m.searchQuery); score > 0 {
					matchedAliases = append(matchedAliases, alias)
				}
			}

			if keyScore > 0 || descScore > 0 || catScore > 0 || len(matchedAliases) > 0 {
				filtered = append(filtered, filteredHelpItem{
					HelpItem:       item,
					keyPositions:   keyPos,
					descPositions:  descPos,
					matchedAliases: matchedAliases,
				})
			}
		}
		return filtered
	}

	matcher := fuzzy.SmartCase(m.searchQuery)
	filtered := match(source, matcher)
	if len(filtered) == 0 && matcher.CaseSensitive() {
		filtered = match(source, fuzzy.NewMatcher(false))
		m.searchCaseRelaxed = len(filtered) > 0
	}
	if len(filtered) == 0 && m.activeTab > 0 && m.activeTab <= len(m.categories) {
		// Count exactly what switching to All would show, including its own
		// smart-case fallback, without changing the selected tab or query.
		otherMatches := match(m.items, matcher)
		if len(otherMatches) == 0 && matcher.CaseSensitive() {
			otherMatches = match(m.items, fuzzy.NewMatcher(false))
		}
		m.otherCategoryMatches = len(otherMatches)
	}

	m.filteredItems = filtered
}
