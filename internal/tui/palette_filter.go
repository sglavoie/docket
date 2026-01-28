package tui

import (
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/sglavoie/tt/internal/fuzzy"
)

// taskIDPattern matches queries like "#123" for direct task ID jump.
var taskIDPattern = regexp.MustCompile(`^#(\d+)$`)

// inlineTaskPattern matches queries starting with "+" for inline task creation.
var inlineTaskPattern = regexp.MustCompile(`^\+(.+)$`)

// quotedPhrasePattern matches quoted phrases: "exact phrase" or 'exact phrase'
var quotedPhrasePattern = regexp.MustCompile(`"([^"]+)"|'([^']+)'`)

// parseQueryPrefix extracts a category filter prefix from the query.
// Returns the category to filter by (or empty string for no filter), the remaining search query,
// and whether the :recent prefix was used.
// Prefix format: ":cmd ", ":task ", ":prj ", ":tag ", ":sec ", ":recent " (note the trailing space).
func (m *CommandPaletteModel) parseQueryPrefix(query string) (ItemCategory, string) {
	cat, searchQuery, _ := m.parseQueryPrefixWithRecent(query)
	return cat, searchQuery
}

// parseQueryPrefixWithRecent is like parseQueryPrefix but also returns whether :recent was used.
func (m *CommandPaletteModel) parseQueryPrefixWithRecent(query string) (ItemCategory, string, bool) {
	if !strings.HasPrefix(query, ":") {
		return "", query, false
	}

	// Find the first space after the prefix
	spaceIdx := strings.Index(query, " ")
	if spaceIdx == -1 {
		// No space yet - check if it's a partial prefix (user still typing)
		prefix := strings.ToLower(query[1:])
		for p := range categoryPrefixes {
			if strings.HasPrefix(p, prefix) {
				// Partial match - filter to this category but no search query yet
				return categoryPrefixes[p], "", p == "recent"
			}
		}
		return "", query, false
	}

	// Extract the prefix (without the colon)
	prefix := strings.ToLower(query[1:spaceIdx])
	if cat, ok := categoryPrefixes[prefix]; ok {
		return cat, strings.TrimSpace(query[spaceIdx+1:]), prefix == "recent"
	}

	// Unknown prefix - treat as regular search
	return "", query, false
}

// parseFieldFilters extracts field-specific filters from the query.
// Returns the remaining query after removing field filters, and the filters found.
// Supported syntax:
//   - priority:N or priority:N-M (e.g., priority:1, priority:1-2)
//   - status:val or status:val|val2 (e.g., status:todo, status:todo|progress)
//   - tag:name or tag:name|name2 (e.g., tag:bug, tag:bug|feature)
//   - section:name or section:name|name2 (e.g., section:backlog, section:sprint-1|sprint-2)
//   - @overdue, @today, @tomorrow, @this-week
func parseFieldFilters(query string) (remaining string, filters []fieldFilter) {
	terms := strings.Fields(query)
	var remainingTerms []string

	for _, term := range terms {
		// Check for date predicates (@overdue, @today, etc.)
		if strings.HasPrefix(term, "@") {
			datePred := strings.ToLower(term[1:])
			switch datePred {
			case "overdue", "today", "tomorrow", "this-week", "thisweek":
				filters = append(filters, fieldFilter{
					field: "date",
					op:    "eq",
					value: datePred,
				})
				continue
			}
		}

		// Check for priority:N or priority:N-M
		if strings.HasPrefix(term, "priority:") {
			val := term[9:]
			if strings.Contains(val, "-") {
				filters = append(filters, fieldFilter{
					field: "priority",
					op:    "range",
					value: val,
				})
			} else {
				filters = append(filters, fieldFilter{
					field: "priority",
					op:    "eq",
					value: val,
				})
			}
			continue
		}

		// Check for status:val or status:val|val2
		if strings.HasPrefix(term, "status:") {
			val := term[7:]
			if strings.Contains(val, "|") {
				alts := strings.Split(val, "|")
				filters = append(filters, fieldFilter{
					field: "status",
					op:    "in",
					alt:   alts,
				})
			} else {
				filters = append(filters, fieldFilter{
					field: "status",
					op:    "eq",
					value: val,
				})
			}
			continue
		}

		// Check for tag:name or tag:name|name2
		if strings.HasPrefix(term, "tag:") {
			val := term[4:]
			if strings.Contains(val, "|") {
				alts := strings.Split(val, "|")
				filters = append(filters, fieldFilter{
					field: "tag",
					op:    "in",
					alt:   alts,
				})
			} else {
				filters = append(filters, fieldFilter{
					field: "tag",
					op:    "eq",
					value: val,
				})
			}
			continue
		}

		// Check for section:name or section:name|name2
		if strings.HasPrefix(term, "section:") {
			val := term[8:]
			if strings.Contains(val, "|") {
				alts := strings.Split(val, "|")
				filters = append(filters, fieldFilter{
					field: "section",
					op:    "in",
					alt:   alts,
				})
			} else {
				filters = append(filters, fieldFilter{
					field: "section",
					op:    "eq",
					value: val,
				})
			}
			continue
		}

		// Not a field filter, keep it
		remainingTerms = append(remainingTerms, term)
	}

	return strings.Join(remainingTerms, " "), filters
}

// taskMatchesFieldFilters checks if a task matches all field filters.
// Returns false if the item is not a task or doesn't match any filter.
func (m *CommandPaletteModel) taskMatchesFieldFilters(item PaletteItem, filters []fieldFilter) bool {
	// Non-task items don't match field filters (they get filtered out)
	if item.Category != ItemCategoryTasks || item.Task == nil {
		return len(filters) == 0 // Allow non-tasks only if no filters
	}

	task := item.Task
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	for _, f := range filters {
		switch f.field {
		case "priority":
			switch f.op {
			case "eq":
				p, err := strconv.Atoi(f.value)
				if err != nil || task.Priority != p {
					return false
				}
			case "range":
				parts := strings.Split(f.value, "-")
				if len(parts) != 2 {
					return false
				}
				minP, err1 := strconv.Atoi(parts[0])
				maxP, err2 := strconv.Atoi(parts[1])
				if err1 != nil || err2 != nil {
					return false
				}
				if task.Priority < minP || task.Priority > maxP {
					return false
				}
			}

		case "status":
			taskStatus := strings.ToLower(string(task.Status))
			if f.op == "eq" {
				if !strings.Contains(taskStatus, strings.ToLower(f.value)) {
					return false
				}
			} else if f.op == "in" {
				matched := false
				for _, alt := range f.alt {
					if strings.Contains(taskStatus, strings.ToLower(alt)) {
						matched = true
						break
					}
				}
				if !matched {
					return false
				}
			}

		case "date":
			if task.DueDate == nil {
				return false // No due date means can't match date filters
			}
			dueDay := time.Date(task.DueDate.Year(), task.DueDate.Month(), task.DueDate.Day(), 0, 0, 0, 0, now.Location())

			switch f.value {
			case "overdue":
				if !dueDay.Before(today) {
					return false
				}
			case "today":
				if !dueDay.Equal(today) {
					return false
				}
			case "tomorrow":
				tomorrow := today.AddDate(0, 0, 1)
				if !dueDay.Equal(tomorrow) {
					return false
				}
			case "this-week", "thisweek":
				weekEnd := today.AddDate(0, 0, 7)
				if dueDay.Before(today) || !dueDay.Before(weekEnd) {
					return false
				}
			}

		case "tag":
			if f.op == "eq" {
				// Task must have the specified tag (case-insensitive)
				found := false
				for _, tag := range task.Tags {
					if strings.EqualFold(tag, f.value) {
						found = true
						break
					}
				}
				if !found {
					return false
				}
			} else if f.op == "in" {
				// Task must have at least one of the specified tags
				found := false
				for _, tag := range task.Tags {
					for _, alt := range f.alt {
						if strings.EqualFold(tag, alt) {
							found = true
							break
						}
					}
					if found {
						break
					}
				}
				if !found {
					return false
				}
			}

		case "section":
			if f.op == "eq" {
				// Task must be in the specified section (case-insensitive)
				if !strings.EqualFold(task.Section, f.value) {
					return false
				}
			} else if f.op == "in" {
				// Task must be in one of the specified sections
				found := false
				for _, alt := range f.alt {
					if strings.EqualFold(task.Section, alt) {
						found = true
						break
					}
				}
				if !found {
					return false
				}
			}
		}
	}

	return true
}

// parseSearchTerms splits a query into positive and negative term groups.
// Terms prefixed with "-" are negative (exclude items matching them).
// Terms containing "|" are OR groups (any alternative matches).
// Quoted strings ("exact phrase") are exact substring matches.
// Example: 'task "fix bug" -done' returns:
//   - positiveGroups: [["task"], ["fix bug" (exact)]]
//   - negativeGroups: [["done"]]
func parseSearchTerms(query string) (positiveGroups []searchTermGroup, negativeGroups []searchTermGroup) {
	// First extract quoted phrases
	quotedMatches := quotedPhrasePattern.FindAllStringSubmatchIndex(query, -1)
	var quotedPhrases []struct {
		phrase     string
		isNegative bool
	}

	// Process quoted phrases in reverse order to preserve indices when removing
	remaining := query
	for i := len(quotedMatches) - 1; i >= 0; i-- {
		match := quotedMatches[i]
		start, end := match[0], match[1]

		// Check if preceded by "-" for negation
		isNegative := false
		if start > 0 && remaining[start-1] == '-' {
			isNegative = true
			start-- // Include the "-" in removal
		}

		// Extract the phrase (either group 1 or group 2, depending on quote type)
		var phrase string
		if match[2] != -1 { // Double quotes
			phrase = remaining[match[2]:match[3]]
		} else if match[4] != -1 { // Single quotes
			phrase = remaining[match[4]:match[5]]
		}

		if phrase != "" {
			quotedPhrases = append([]struct {
				phrase     string
				isNegative bool
			}{{phrase: phrase, isNegative: isNegative}}, quotedPhrases...)
		}

		// Remove the quoted phrase from remaining query
		remaining = remaining[:start] + remaining[end:]
	}

	// Add quoted phrases as exact match groups
	for _, qp := range quotedPhrases {
		group := searchTermGroup{
			alternatives: []string{qp.phrase},
			isNegative:   qp.isNegative,
			isExact:      true,
		}
		if qp.isNegative {
			negativeGroups = append(negativeGroups, group)
		} else {
			positiveGroups = append(positiveGroups, group)
		}
	}

	// Process remaining non-quoted terms
	terms := strings.Fields(remaining)

	for _, term := range terms {
		if strings.HasPrefix(term, "-") && len(term) > 1 {
			// Negative term - remove the "-" prefix
			termContent := term[1:]
			alternatives := strings.Split(termContent, "|")
			// Filter empty alternatives
			var validAlts []string
			for _, alt := range alternatives {
				if alt != "" {
					validAlts = append(validAlts, alt)
				}
			}
			if len(validAlts) > 0 {
				negativeGroups = append(negativeGroups, searchTermGroup{
					alternatives: validAlts,
					isNegative:   true,
				})
			}
		} else if term != "-" && term != "|" {
			// Positive term - may contain OR alternatives
			alternatives := strings.Split(term, "|")
			var validAlts []string
			for _, alt := range alternatives {
				if alt != "" {
					validAlts = append(validAlts, alt)
				}
			}
			if len(validAlts) > 0 {
				positiveGroups = append(positiveGroups, searchTermGroup{
					alternatives: validAlts,
					isNegative:   false,
				})
			}
		}
	}

	return positiveGroups, negativeGroups
}

// itemMatchesNegativeGroups returns true if the item matches any negative term group.
// A group matches if ANY of its alternatives match (OR logic within group).
func (m *CommandPaletteModel) itemMatchesNegativeGroups(item PaletteItem, negativeGroups []searchTermGroup) bool {
	for _, group := range negativeGroups {
		// Group matches if ANY alternative matches (OR logic)
		for _, alt := range group.alternatives {
			if group.isExact {
				// Exact substring match (case-insensitive)
				nameLower := strings.ToLower(item.Name)
				descLower := strings.ToLower(item.Description)
				altLower := strings.ToLower(alt)
				if strings.Contains(nameLower, altLower) || strings.Contains(descLower, altLower) {
					return true
				}
			} else {
				if fuzzy.Match(item.Name, alt) || fuzzy.Match(item.Description, alt) {
					return true
				}
			}
		}
	}
	return false
}

// itemMatchesPositiveGroups returns true and a score if item matches ALL positive term groups.
// Each group must have at least one matching alternative (AND between groups, OR within groups).
// Returns (matches, totalScore, namePositions, matchedField, matchedTerm).
func (m *CommandPaletteModel) itemMatchesPositiveGroups(item PaletteItem, positiveGroups []searchTermGroup) (bool, int, []int, MatchField, string) {
	if len(positiveGroups) == 0 {
		return true, 1, nil, MatchFieldNone, ""
	}

	totalScore := 0
	var allPositions []int
	positionSet := make(map[int]bool)
	bestMatchField := MatchFieldNone
	bestMatchTerm := ""

	for _, group := range positiveGroups {
		// Find best matching alternative in this group
		bestScore := 0
		var bestPositions []int
		groupMatchField := MatchFieldNone
		groupMatchTerm := ""

		for _, alt := range group.alternatives {
			if group.isExact {
				// Exact substring match (case-insensitive)
				nameLower := strings.ToLower(item.Name)
				descLower := strings.ToLower(item.Description)
				altLower := strings.ToLower(alt)

				if strings.Contains(nameLower, altLower) {
					// Find position of match in name for highlighting
					idx := strings.Index(nameLower, altLower)
					var positions []int
					for i := idx; i < idx+len(alt); i++ {
						positions = append(positions, i)
					}
					// Score exact matches highly
					score := len(alt) * 10 * categoryWeight(item.Category)
					if score > bestScore {
						bestScore = score
						bestPositions = positions
						groupMatchField = MatchFieldName
						groupMatchTerm = alt
					}
				} else if strings.Contains(descLower, altLower) {
					// Match in description, no name highlighting
					score := len(alt) * 5 * categoryWeight(item.Category)
					if score > bestScore {
						bestScore = score
						bestPositions = nil
						groupMatchField = MatchFieldDescription
						groupMatchTerm = alt
					}
				}
			} else {
				score, positions, matchField := m.scoreItemWithDetails(item, alt)
				if score > bestScore {
					bestScore = score
					bestPositions = positions
					groupMatchField = matchField
					groupMatchTerm = alt
				}
			}
		}

		// Group must have at least one match
		if bestScore == 0 {
			return false, 0, nil, MatchFieldNone, ""
		}

		totalScore += bestScore
		// Merge positions (avoiding duplicates)
		for _, pos := range bestPositions {
			if !positionSet[pos] {
				positionSet[pos] = true
				allPositions = append(allPositions, pos)
			}
		}
		// Track the best match field (prefer non-name matches as they're more interesting)
		if bestMatchField == MatchFieldNone || (groupMatchField != MatchFieldName && bestMatchField == MatchFieldName) {
			bestMatchField = groupMatchField
			bestMatchTerm = groupMatchTerm
		}
	}

	return true, totalScore, allPositions, bestMatchField, bestMatchTerm
}

// filterItems filters items based on the current search query using category-weighted fuzzy scoring.
// Supports category prefix filtering (e.g., ":cmd help" filters to commands matching "help").
// Supports negation with "-" prefix (e.g., "task -bug" excludes items matching "bug").
// Supports direct task ID jump with "#123" syntax.
// Supports inline task creation with "+Task title" syntax.
func (m *CommandPaletteModel) filterItems() {
	query := strings.TrimSpace(m.searchInput.Value())

	// Reset inline task mode
	m.inlineTaskMode = false
	m.inlineTaskParsed = nil

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
		m.cursor = 0
		m.scrollOffset = 0
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

	// Reset cursor and scroll if out of bounds
	if m.cursor >= len(m.filteredItems) {
		m.cursor = len(m.filteredItems) - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
	m.scrollOffset = 0
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
					_ = m.configManager.Save(m.config)
				}
				return
			}
		}
	} else if query == "" {
		// Clear the filter when query is empty
		if m.config.GetLastPaletteCategory() != "" {
			m.config.SetLastPaletteCategory("")
			_ = m.configManager.Save(m.config)
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
	if len(m.filteredItems) == 0 {
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
