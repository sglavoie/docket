package tui

import (
	"sort"
	"strconv"
	"strings"
)

// filterItems filters items based on the current search query using category-weighted fuzzy scoring.
// Supports category prefix filtering (e.g., ":cmd help" filters to commands matching "help").
// Supports negation with "-" prefix (e.g., "task -bug" excludes items matching "bug").
// Supports direct task ID jump with "#123" syntax.
func (m *CommandPaletteModel) filterItems() { //nolint:gocyclo
	query := strings.TrimSpace(m.searchInput.Value())

	// Save current cursor item ID for restoration after filtering
	if m.cursor >= 0 && m.cursor < len(m.filteredItems) {
		m.lastCursorItemID = m.getItemID(m.filteredItems[m.cursor].Item)
	} else {
		m.lastCursorItemID = ""
	}

	// Check for direct task ID jump (#123 syntax)
	if matches := taskIDPattern.FindStringSubmatch(query); matches != nil {
		taskID, _ := strconv.Atoi(matches[1])
		m.filterByTaskID(taskID)
		return
	}

	// Parse category prefix if present.
	categoryFilter, searchQuery := m.parseQueryPrefix(query)

	// Parse field filters (priority:, status:, @overdue, etc.) from search query
	searchQuery, fieldFilters := parseFieldFilters(searchQuery)

	// Determine which items to search
	itemsToSearch := m.allItems
	if categoryFilter != "" {
		// Filter to only the specified category
		var filtered []PaletteItem
		for _, item := range m.allItems {
			if item.Category == categoryFilter {
				filtered = append(filtered, item)
			}
		}
		itemsToSearch = filtered
	}

	// Additionally filter by CommandCategory if a command category prefix was used
	if cmdCat := parseCommandCategoryPrefix(query); cmdCat != "" {
		var filtered []PaletteItem
		for _, item := range itemsToSearch {
			if item.Command != nil && item.Command.Category == cmdCat {
				filtered = append(filtered, item)
			}
		}
		itemsToSearch = filtered
	}

	// Apply field filters first (these are hard filters, not fuzzy)
	if len(fieldFilters) > 0 {
		var filtered []PaletteItem
		for _, item := range itemsToSearch {
			if m.taskMatchesFieldFilters(item, fieldFilters) {
				filtered = append(filtered, item)
			}
		}
		itemsToSearch = filtered
	}

	// An empty palette is intentionally command-only and bounded: up to six
	// recent commands followed by up to six contextual commands. Entities enter
	// the result set only after the user types a query or category prefix — or
	// while transient show-all (Ctrl+A) is active, which lists every currently
	// available command and entity sorted by the category order.
	if searchQuery == "" {
		if categoryFilter == "" && !m.showAll {
			m.filteredItems = m.buildEmptyPaletteItems()
			m.restoreCursorPosition()
			return
		}
		sorted := m.sortItemsByCategory(itemsToSearch)
		m.filteredItems = make([]filteredPaletteItem, len(sorted))
		for i, item := range sorted {
			m.filteredItems[i] = filteredPaletteItem{Item: item}
		}
		m.restoreCursorPosition()
		return
	}

	// Parse positive and negative term groups (supports OR with | delimiter)
	positiveGroups, negativeGroups := parseSearchTerms(searchQuery)

	// Score each item and collect matches with positions and match info
	type scoredItem struct {
		item          PaletteItem
		score         int
		namePositions []int
		matchedField  MatchField
		matchedTerm   string
	}
	var scored []scoredItem

	for _, item := range itemsToSearch {
		// Skip items that match any negative term group
		if len(negativeGroups) > 0 && m.itemMatchesNegativeGroups(item, negativeGroups) {
			continue
		}

		// If only negative terms, include all non-excluded items
		if len(positiveGroups) == 0 {
			scored = append(scored, scoredItem{item: item, score: 1})
			continue
		}

		// Score against positive term groups (AND between groups, OR within groups)
		matches, score, positions, matchField, matchTerm := m.itemMatchesPositiveGroups(item, positiveGroups)
		if matches && score > 0 {
			scored = append(scored, scoredItem{
				item:          item,
				score:         score,
				namePositions: positions,
				matchedField:  matchField,
				matchedTerm:   matchTerm,
			})
		}
	}

	// Sort by score descending (best matches first), then apply secondary sort
	sort.SliceStable(scored, func(i, j int) bool {
		// Primary sort by score
		if scored[i].score != scored[j].score {
			return scored[i].score > scored[j].score
		}

		return false
	})

	// Extract sorted results with positions and match info
	m.filteredItems = make([]filteredPaletteItem, len(scored))
	for i, s := range scored {
		m.filteredItems[i] = filteredPaletteItem{
			Item:          s.item,
			NamePositions: s.namePositions,
			MatchedField:  s.matchedField,
			MatchedTerm:   s.matchedTerm,
		}
	}

	m.restoreCursorPosition()
}

// filterByTaskID filters items to show only tasks matching the given ShortID.
// This enables direct task jump with #123 syntax.
func (m *CommandPaletteModel) filterByTaskID(taskID int) {
	var results []filteredPaletteItem

	for _, item := range m.allItems {
		if item.Category == ItemCategoryTasks && item.Task != nil {
			if item.Task.ShortID == taskID {
				results = append(results, filteredPaletteItem{Item: item})
			}
		}
	}

	m.filteredItems = results
	m.cursor = 0
	m.scrollOffset = 0
}

// restoreCursorPosition tries to keep the previously selected item under the cursor.
// If the item is found in the new filtered results, cursor moves to its new position.
// Otherwise cursor and scroll reset to 0.
func (m *CommandPaletteModel) restoreCursorPosition() {
	if m.lastCursorItemID != "" && len(m.filteredItems) > 0 {
		for i, filtered := range m.filteredItems {
			if m.getItemID(filtered.Item) == m.lastCursorItemID {
				m.cursor = i
				// Let calculateScrollOffset handle repositioning naturally
				return
			}
		}
	}
	// Item not found - reset
	m.cursor = 0
	m.scrollOffset = 0
}

// isDirectActionEnabled returns true if single-key actions (e, g, x, s, r) should work directly.
// This is enabled when the search input is empty or contains only a category prefix.
func (m *CommandPaletteModel) isDirectActionEnabled() bool {
	query := strings.TrimSpace(m.searchInput.Value())
	if query == "" {
		return false
	}
	// Check if query is just a category prefix (e.g., ":task") or task ID jump (#123)
	if taskIDPattern.MatchString(query) {
		return true
	}
	// If query starts with ":" but has no space, user is still typing a prefix
	// Disable direct actions to allow typing characters like 'g', 's', etc.
	if strings.HasPrefix(query, ":") && !strings.Contains(query, " ") {
		return false
	}
	_, searchQuery := m.parseQueryPrefix(query)
	return searchQuery == ""
}
