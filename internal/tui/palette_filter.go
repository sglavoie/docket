package tui

import (
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/sglavoie/tt/internal/logging"
)

// taskIDPattern matches queries like "#123" for direct task ID jump.
var taskIDPattern = regexp.MustCompile(`^#(\d+)$`)

// inlineTaskPattern matches queries starting with "+" for inline task creation.
var inlineTaskPattern = regexp.MustCompile(`^\+(.+)$`)

// quotedPhrasePattern matches quoted phrases: "exact phrase" or 'exact phrase'
var quotedPhrasePattern = regexp.MustCompile(`"([^"]+)"|'([^']+)'`)

// filterItems filters items based on the current search query using category-weighted fuzzy scoring.
// Supports category prefix filtering (e.g., ":cmd help" filters to commands matching "help").
// Supports negation with "-" prefix (e.g., "task -bug" excludes items matching "bug").
// Supports direct task ID jump with "#123" syntax.
// Supports inline task creation with "+Task title" syntax.
func (m *CommandPaletteModel) filterItems() { //nolint:gocyclo
	query := strings.TrimSpace(m.searchInput.Value())

	// Save current cursor item ID for restoration after filtering
	if m.cursor >= 0 && m.cursor < len(m.filteredItems) {
		m.lastCursorItemID = m.getItemID(m.filteredItems[m.cursor].Item)
	} else {
		m.lastCursorItemID = ""
	}

	// Reset inline task mode
	m.inlineTaskMode = false
	m.inlineTaskParsed = nil

	// Check for :help prefix - show syntax help as palette entries
	if query == ":help" || strings.HasPrefix(query, ":help ") {
		m.filteredItems = m.buildHelpItems()
		m.cursor = 0
		m.scrollOffset = 0
		return
	}

	// Check for inline task creation (+Title syntax)
	if matches := inlineTaskPattern.FindStringSubmatch(query); matches != nil {
		m.inlineTaskMode = true
		m.inlineTaskTitle = strings.TrimSpace(matches[1])
		m.inlineTaskParsed = m.parseInlineTask(m.inlineTaskTitle)
		m.filteredItems = nil
		m.cursor = 0
		m.scrollOffset = 0
		return
	}

	// Check for direct task ID jump (#123 syntax)
	if matches := taskIDPattern.FindStringSubmatch(query); matches != nil {
		taskID, _ := strconv.Atoi(matches[1])
		m.filterByTaskID(taskID)
		return
	}

	// Parse category prefix if present (with recent flag)
	categoryFilter, searchQuery, isRecentFilter := m.parseQueryPrefixWithRecent(query)

	// Persist category filter when used (e.g., ":task " becomes the default for next open)
	// Don't persist :recent as it's a special filter
	if !isRecentFilter {
		m.persistCategoryFilter(query, categoryFilter)
	}

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

	// If :recent prefix, further filter to only recently executed commands
	if isRecentFilter {
		recentCommands := m.config.GetCommandHistory()
		recentSet := make(map[string]bool)
		for _, cmd := range recentCommands {
			recentSet[cmd] = true
		}

		var filtered []PaletteItem
		for _, item := range itemsToSearch {
			if item.Category == ItemCategoryCommands && item.Command != nil {
				if recentSet[item.Command.Name] {
					filtered = append(filtered, item)
				}
			}
		}

		// Sort by history order (most recent first)
		sort.SliceStable(filtered, func(i, j int) bool {
			for _, cmd := range recentCommands {
				if filtered[i].Command != nil && filtered[i].Command.Name == cmd {
					return true
				}
				if filtered[j].Command != nil && filtered[j].Command.Name == cmd {
					return false
				}
			}
			return false
		})

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

	// Empty search query shows all (filtered) items sorted by category (no highlighting)
	if searchQuery == "" {
		sorted := m.sortItemsByCategory(itemsToSearch)
		m.filteredItems = make([]filteredPaletteItem, len(sorted))
		for i, item := range sorted {
			m.filteredItems[i] = filteredPaletteItem{Item: item}
		}
		// Inject "Frequent" section when browsing without any filter
		if categoryFilter == "" && !isRecentFilter {
			m.injectFrequentItems()
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

		// Secondary sort based on sortField (only for tasks)
		itemI := scored[i].item
		itemJ := scored[j].item

		// Non-tasks: just keep score order
		if itemI.Category != ItemCategoryTasks || itemJ.Category != ItemCategoryTasks {
			return false
		}
		if itemI.Task == nil || itemJ.Task == nil {
			return false
		}

		switch m.sortField {
		case SortByPriority:
			// Lower priority number = higher priority (1 is highest)
			return itemI.Task.Priority < itemJ.Task.Priority
		case SortByDueDate:
			// Earlier due date first; no due date goes last
			if itemI.Task.DueDate == nil && itemJ.Task.DueDate == nil {
				return false
			}
			if itemI.Task.DueDate == nil {
				return false // nil goes last
			}
			if itemJ.Task.DueDate == nil {
				return true // nil goes last
			}
			return itemI.Task.DueDate.Before(*itemJ.Task.DueDate)
		default:
			return false
		}
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

// sortItemsByCategory sorts items by the predefined category order for display without search.
// Pinned commands are placed at the very top before all other categories.
func (m *CommandPaletteModel) sortItemsByCategory(items []PaletteItem) []PaletteItem {
	// Create a map for category order lookup
	orderMap := make(map[ItemCategory]int)
	for i, cat := range categoryOrder {
		orderMap[cat] = i
	}

	sorted := make([]PaletteItem, len(items))
	copy(sorted, items)

	sort.SliceStable(sorted, func(i, j int) bool {
		// Pinned commands come first
		iPinned := m.isItemPinned(sorted[i])
		jPinned := m.isItemPinned(sorted[j])
		if iPinned != jPinned {
			return iPinned // Pinned items come before non-pinned
		}
		// Then sort by category
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

// injectFrequentItems prepends a "Frequent" section to filteredItems using the top items
// from recentPaletteSelections sorted by usage count. Only items used more than once are shown.
// The frequent items are marked with IsFrequent=true; they are duplicates of regular items.
func (m *CommandPaletteModel) injectFrequentItems() {
	if len(recentPaletteSelections) == 0 {
		return
	}

	// Sort by count descending to find most-used items
	ranked := make([]recentSelection, len(recentPaletteSelections))
	copy(ranked, recentPaletteSelections)
	sort.SliceStable(ranked, func(i, j int) bool {
		return ranked[i].Count > ranked[j].Count
	})

	const maxFrequent = 7
	var frequentItems []filteredPaletteItem
	for _, rs := range ranked {
		if rs.Count < 2 || len(frequentItems) >= maxFrequent {
			break
		}
		// Find the matching PaletteItem in the already-built filteredItems
		for _, fi := range m.filteredItems {
			id := m.getItemID(fi.Item)
			if fi.Item.Category == rs.Category && id == rs.ID {
				frequentItems = append(frequentItems, filteredPaletteItem{
					Item:       fi.Item,
					IsFrequent: true,
				})
				break
			}
		}
	}

	if len(frequentItems) == 0 {
		return
	}

	m.filteredItems = append(frequentItems, m.filteredItems...)
}

// persistCategoryFilter saves the category filter to config when a category prefix is used.
// Only persists complete prefixes (e.g., ":task " not ":ta").
func (m *CommandPaletteModel) persistCategoryFilter(query string, categoryFilter ItemCategory) {
	if m.configManager == nil {
		return
	}

	// Only persist if we have a valid category filter with a complete prefix
	if categoryFilter != "" {
		// Find the prefix string that matches this category
		for prefix, cat := range categoryPrefixes {
			if cat == categoryFilter {
				newValue := ":" + prefix + " "
				// Only update if changed
				if m.config.GetLastPaletteCategory() != newValue {
					m.config.SetLastPaletteCategory(newValue)
					if err := m.configManager.Save(m.config); err != nil {
						logging.Warn("failed to save config: %v", err)
					}
				}
				return
			}
		}
	} else if query == "" {
		// Clear the filter when query is empty
		if m.config.GetLastPaletteCategory() != "" {
			m.config.SetLastPaletteCategory("")
			if err := m.configManager.Save(m.config); err != nil {
				logging.Warn("failed to save config: %v", err)
			}
		}
	}
}

// isNumberJumpEnabled returns true if number keys should trigger direct selection.
// This is enabled when the search input is empty or contains only a category prefix.
func (m *CommandPaletteModel) isNumberJumpEnabled() bool {
	query := strings.TrimSpace(m.searchInput.Value())
	if query == "" {
		return true
	}
	// Check if query is just a category prefix (e.g., ":cmd", ":task ")
	_, searchQuery := m.parseQueryPrefix(query)
	return searchQuery == ""
}

// findNextSectionStart returns the index of the first item in the next category.
// Categories cycle in the order: Commands → Projects → Tasks → Tags → Sections → Commands.
// Saves the current cursor position for the current category to restore later.
// Returns -1 if there are no items or only one category exists.
func (m *CommandPaletteModel) findNextSectionStart() int {
	if len(m.filteredItems) == 0 || m.cursor >= len(m.filteredItems) {
		return -1
	}

	currentCategory := m.filteredItems[m.cursor].Item.Category

	// Save current cursor position for this category
	m.categoryCursors[currentCategory] = m.cursor

	// Find current category's position in the order
	currentIdx := -1
	for i, cat := range categoryOrder {
		if cat == currentCategory {
			currentIdx = i
			break
		}
	}
	if currentIdx == -1 {
		return -1
	}

	// Try each subsequent category in order (wrapping around)
	for offset := 1; offset <= len(categoryOrder); offset++ {
		nextCatIdx := (currentIdx + offset) % len(categoryOrder)
		targetCategory := categoryOrder[nextCatIdx]

		// Check if we have a saved cursor position for this category
		if savedPos, ok := m.categoryCursors[targetCategory]; ok {
			// Verify the saved position is still valid and in this category
			if savedPos < len(m.filteredItems) && m.filteredItems[savedPos].Item.Category == targetCategory {
				return savedPos
			}
		}

		// Find first item with this category
		for i, filtered := range m.filteredItems {
			if filtered.Item.Category == targetCategory {
				return i
			}
		}
	}

	return -1
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

// buildHelpItems returns a list of filteredPaletteItems that serve as a syntax reference.
func (m *CommandPaletteModel) buildHelpItems() []filteredPaletteItem {
	helpEntries := []struct {
		name string
		desc string
	}{
		{"+Buy milk !2 #errand @tomorrow", "Inline task creation with priority, tag, due date"},
		{":cmd, :task, :prj, :tag, :sec, :note, :view", "Category prefix filters"},
		{":recent", "Show recently executed commands"},
		{"#123", "Jump directly to task by short ID"},
		{"\"exact phrase\"", "Quoted exact substring match"},
		{"-exclude", "Exclude items matching a term"},
		{"bug|fix", "OR search: match either term"},
		{"priority:1 or priority:1-3", "Filter tasks by priority value or range"},
		{"status:todo or status:todo|progress", "Filter tasks by status"},
		{"tag:name or tag:bug|feature", "Filter tasks by tag"},
		{"section:backlog", "Filter tasks by section"},
		{"@overdue @today @tomorrow @this-week", "Filter tasks by due date"},
		{"Ctrl+H", "Toggle completed/canceled tasks"},
		{"Ctrl+O", "Cycle sort: score → priority → due date"},
		{"Ctrl+P", "Toggle compact mode (hide preview)"},
		{"Ctrl+N", "Jump to next category section"},
		{"Ctrl+B", "Save current search as bookmark"},
		{"Ctrl+F", "Pin/unpin a command"},
		{"Tab", "Show actions for selected item"},
		{"Space", "Multi-select tasks for bulk actions"},
	}

	items := make([]filteredPaletteItem, len(helpEntries))
	for i, entry := range helpEntries {
		items[i] = filteredPaletteItem{
			Item: PaletteItem{
				Name:        entry.name,
				Description: entry.desc,
				Category:    ItemCategoryCommands, // Use commands category for badge styling
			},
		}
	}
	return items
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
