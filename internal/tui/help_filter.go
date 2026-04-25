package tui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/sglavoie/tt/internal/fuzzy"
)

// buildCategories extracts ordered unique categories from all items.
func (m *HelpViewModel) buildCategories() []string {
	seen := make(map[string]bool)
	var cats []string
	for _, item := range m.items {
		if !seen[item.Category] {
			seen[item.Category] = true
			cats = append(cats, item.Category)
		}
	}
	sort.Strings(cats)
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
		activeCategory := m.categories[m.activeTab-1]
		source = nil
		for _, item := range m.items {
			if item.Category == activeCategory {
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

		scopeScore := 0
		if m.isExplorer {
			scopeScore, _ = fuzzy.ScoreTermsWithPositions(scopeText(item.Views), m.searchQuery)
		}

		if keyScore > 0 || descScore > 0 || catScore > 0 || scopeScore > 0 {
			filtered = append(filtered, filteredHelpItem{
				HelpItem:      item,
				keyPositions:  keyPos,
				descPositions: descPos,
			})
		}
	}

	m.filteredItems = filtered
}

// scopeText returns a human-readable scope label for a set of views.
func scopeText(views []ViewType) string {
	if len(views) == 0 {
		return "Global"
	}
	if len(views) <= 3 {
		names := make([]string, len(views))
		for i, v := range views {
			names[i] = v.String()
		}
		return strings.Join(names, ", ")
	}
	return fmt.Sprintf("%d views", len(views))
}
