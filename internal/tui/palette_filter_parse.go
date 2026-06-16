package tui

import (
	"strings"
)

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

		// Check command category prefixes first
		for p := range commandCategoryPrefixes {
			if strings.HasPrefix(p, prefix) {
				return ItemCategoryCommands, "", false
			}
		}

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

	// Check command category prefixes first
	if _, ok := commandCategoryPrefixes[prefix]; ok {
		return ItemCategoryCommands, strings.TrimSpace(query[spaceIdx+1:]), false
	}

	if cat, ok := categoryPrefixes[prefix]; ok {
		return cat, strings.TrimSpace(query[spaceIdx+1:]), prefix == "recent"
	}

	// Unknown prefix - treat as regular search
	return "", query, false
}

// parseCommandCategoryPrefix returns the CommandCategory if the query uses a command category prefix.
// Returns empty string if no command category prefix is found.
func parseCommandCategoryPrefix(query string) CommandCategory {
	if !strings.HasPrefix(query, ":") {
		return ""
	}
	spaceIdx := strings.Index(query, " ")
	var prefix string
	if spaceIdx == -1 {
		prefix = strings.ToLower(query[1:])
	} else {
		prefix = strings.ToLower(query[1:spaceIdx])
	}
	if cat, ok := commandCategoryPrefixes[prefix]; ok {
		return cat
	}
	// Check partial match (user still typing)
	if spaceIdx == -1 {
		for p, cat := range commandCategoryPrefixes {
			if strings.HasPrefix(p, prefix) {
				return cat
			}
		}
	}
	return ""
}

// parseFieldFilters extracts field-specific filters from the query.
// Returns the remaining query after removing field filters, and the filters found.
// Supported syntax:
//   - priority:N or priority:N-M (e.g., priority:1, priority:1-2)
//   - status:val or status:val|val2 (e.g., status:todo, status:todo|progress)
//   - tag:name or tag:name|name2 (e.g., tag:bug, tag:bug|feature)
//   - section:name or section:name|name2 (e.g., section:inbox, section:next|waiting)
//   - @overdue, @today, @tomorrow, @this-week
func parseFieldFilters(query string) (remaining string, filters []fieldFilter) {
	terms := strings.Fields(query)
	var remainingTerms []string

	for _, term := range terms {
		// Check for date predicates (@overdue, @today, etc.)
		if strings.HasPrefix(term, "@") {
			datePred := strings.ToLower(term[1:])
			switch datePred {
			case "overdue", "today", "tomorrow", "this-week", "thisweek", "none":
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
	terms := strings.FieldsSeq(remaining)

	for term := range terms {
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
