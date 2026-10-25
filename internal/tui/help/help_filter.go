package help

import (
	"slices"
	"strings"

	"github.com/sglavoie/docket/internal/fuzzy"
	"github.com/sglavoie/docket/internal/tui/operation"
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
	m.searchCaseRelaxed = false
	m.otherCategoryMatches = 0
	m.tabMatchCounts = nil
	buckets := make(map[string][]operation.HelpItem)
	for _, item := range m.items {
		category := helpSectionForItem(item)
		buckets[category] = append(buckets[category], item)
	}
	source := m.items
	if m.activeTab > 0 && m.activeTab <= len(m.categories) {
		source = buckets[m.categories[m.activeTab-1]]
	}
	if m.searchQuery == "" {
		m.filteredItems = make([]filteredHelpItem, len(source))
		for i, item := range source {
			m.filteredItems[i] = filteredHelpItem{HelpItem: item}
		}
		return
	}

	all, allRelaxed := matchHelpItems(m.items, m.searchQuery)
	m.filteredItems, m.searchCaseRelaxed = all, allRelaxed
	m.tabMatchCounts = []int{len(all)}
	for i, category := range m.categories {
		// Each count predicts selecting that tab, including its own smart-case
		// retry; All can have a strict match while one category needs a retry.
		matches, relaxed := matchHelpItems(buckets[category], m.searchQuery)
		m.tabMatchCounts = append(m.tabMatchCounts, len(matches))
		if m.activeTab == i+1 {
			m.filteredItems, m.searchCaseRelaxed = matches, relaxed
		}
	}
	if len(m.filteredItems) == 0 && m.activeTab > 0 && m.activeTab <= len(m.categories) {
		m.otherCategoryMatches = len(all)
	}
}

func matchHelpItems(items []operation.HelpItem, query string) ([]filteredHelpItem, bool) {
	// Alias positions index the alias, never the rendered key or description.
	match := func(items []operation.HelpItem, matcher fuzzy.Matcher) []filteredHelpItem {
		normalizedQuery := strings.Join(strings.Fields(query), " ")
		exact := func(value string) bool {
			value = strings.Join(strings.Fields(value), " ")
			if matcher.CaseSensitive() {
				return value == normalizedQuery
			}
			return strings.EqualFold(value, normalizedQuery)
		}
		var filtered []filteredHelpItem
		for _, item := range items {
			entry := filteredHelpItem{HelpItem: item}
			entry.exactMatch = exact(item.Key) || exact(item.Description)
			// Operation help joins alternative key spellings with a slash.
			// The whole-key comparison above also preserves the literal / key.
			for _, key := range strings.Fields(strings.ReplaceAll(item.Key, "/", " ")) {
				entry.exactMatch = entry.exactMatch || exact(key)
			}
			for _, alias := range item.Aliases {
				entry.exactMatch = entry.exactMatch || exact(alias)
			}
			matches := true
			aliases := make(map[string]bool)
			for _, term := range strings.Fields(query) {
				keyScore, keyPos := matcher.ScoreWithPositions(item.Key, term)
				descScore, descPos := matcher.ScoreWithPositions(item.Description, term)
				found := keyScore > 0 || descScore > 0 || matcher.Score(item.Category, term) > 0 || matcher.Score(item.Group, term) > 0
				entry.keyPositions = append(entry.keyPositions, keyPos...)
				entry.descPositions = append(entry.descPositions, descPos...)
				for _, alias := range item.Aliases {
					if matcher.Score(alias, term) > 0 {
						aliases[alias] = true
						found = true
					}
				}
				if !found {
					matches = false
					break
				}
			}
			if matches {
				// Positions belong to their own field; aliases keep registry order.
				slices.Sort(entry.keyPositions)
				entry.keyPositions = slices.Compact(entry.keyPositions)
				slices.Sort(entry.descPositions)
				entry.descPositions = slices.Compact(entry.descPositions)
				for _, alias := range item.Aliases {
					if aliases[alias] {
						entry.matchedAliases = append(entry.matchedAliases, alias)
					}
				}
				filtered = append(filtered, entry)
			}
		}
		return filtered
	}

	matcher := fuzzy.SmartCase(query)
	filtered := match(items, matcher)
	if len(filtered) == 0 && matcher.CaseSensitive() {
		filtered = match(items, fuzzy.NewMatcher(false))
		return filtered, len(filtered) > 0
	}
	return filtered, false
}
