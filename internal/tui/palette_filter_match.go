package tui

import (
	"strconv"
	"strings"
	"time"

	"github.com/sglavoie/tt/internal/fuzzy"
	"github.com/sglavoie/tt/internal/util"
)

// taskMatchesFieldFilters checks if a task matches all field filters.
// Returns false if the item is not a task or doesn't match any filter.
func (m *CommandPaletteModel) taskMatchesFieldFilters(item PaletteItem, filters []fieldFilter) bool { //nolint:gocyclo
	// Non-task items don't match field filters (they get filtered out)
	if item.Category != ItemCategoryTasks || item.Task == nil {
		return len(filters) == 0 // Allow non-tasks only if no filters
	}

	task := item.Task
	now := time.Now()
	today := util.StartOfDay(now)

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
			taskStatus := legacyStatusColumnForState(task.State())
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
			// Handle "none" filter before the nil check
			if f.value == "none" {
				if task.DueDate != nil {
					return false
				}
				continue
			}
			if task.DueDate == nil {
				return false // No due date means can't match date filters
			}
			dueDay := util.StartOfDay(*task.DueDate)

			switch f.value {
			case "overdue":
				// Completed/canceled tasks are not overdue.
				// When HasTime is set, compare against current time; else compare date.
				if task.IsComplete() {
					return false
				}
				if task.HasTime {
					if !task.DueDate.Before(now) {
						return false
					}
				} else if !dueDay.Before(today) {
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
				if !strings.EqualFold(taskSectionBucket(task), f.value) {
					return false
				}
			} else if f.op == "in" {
				found := false
				for _, alt := range f.alt {
					if strings.EqualFold(taskSectionBucket(task), alt) {
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

// itemMatchesNegativeGroups returns true if the item matches any negative term group.
// A group matches if ANY of its alternatives match (OR logic within group).
func (m *CommandPaletteModel) itemMatchesNegativeGroups(item PaletteItem, negativeGroups []searchTermGroup) bool {
	// A project's slug is a first-class searchable field, so negation must be
	// able to exclude on slug alone (mirroring the positive scoring path).
	slug := ""
	if item.Category == ItemCategoryProjects && item.Project != nil {
		slug = item.Project.Slug
	}
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
				if slug != "" && strings.Contains(strings.ToLower(slug), altLower) {
					return true
				}
			} else {
				if fuzzy.Match(item.Name, alt) || fuzzy.Match(item.Description, alt) {
					return true
				}
				if slug != "" && fuzzy.Match(slug, alt) {
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

				// Project name/slug matches carry the same deterministic
				// identity-quality bonus as the fuzzy scoring path
				// (scoreItemWithDetails). Without it, a quoted exact term is
				// scored by categoryWeight alone, letting a higher-weighted
				// command category outrank an exactly named or slugged project
				// and breaking the project-identity ordering contract.
				identityBonus := 0
				if item.Category == ItemCategoryProjects && item.Project != nil {
					identityBonus = projectIdentityBonus(projectMatchQuality(item.Project.Name, item.Project.Slug, alt))
				}

				if strings.Contains(nameLower, altLower) {
					// Find position of match in name for highlighting
					idx := strings.Index(nameLower, altLower)
					var positions []int
					for i := idx; i < idx+len(alt); i++ {
						positions = append(positions, i)
					}
					// Score exact matches highly
					score := len(alt)*10*categoryWeight(item.Category) + identityBonus
					if score > bestScore {
						bestScore = score
						bestPositions = positions
						groupMatchField = MatchFieldName
						groupMatchTerm = alt
					}
				} else if strings.Contains(descLower, altLower) {
					// Match in description, no name highlighting
					score := len(alt)*5*categoryWeight(item.Category) + identityBonus
					if score > bestScore {
						bestScore = score
						bestPositions = nil
						groupMatchField = MatchFieldDescription
						groupMatchTerm = alt
					}
				} else if item.Category == ItemCategoryProjects && item.Project != nil &&
					item.Project.Slug != "" && strings.Contains(strings.ToLower(item.Project.Slug), altLower) {
					// Slug-only exact match: the project's slug is a first-class
					// searchable identity field. Positions stay nil because they
					// index Name, which the slug match did not touch; provenance
					// records MatchFieldProjectSlug so the preview can attribute it.
					score := len(alt)*10*categoryWeight(item.Category) + identityBonus
					if score > bestScore {
						bestScore = score
						bestPositions = nil
						groupMatchField = MatchFieldProjectSlug
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
