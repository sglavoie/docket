package tui

import (
	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/fuzzy"
	"github.com/sglavoie/tt/internal/logging"
)

// scoreItemWithDetails scores an item and returns which field matched.
// Returns (score, namePositions, matchedField).
func (m *CommandPaletteModel) scoreItemWithDetails(item PaletteItem, query string) (int, []int, MatchField) {
	nameScore, namePositions := fuzzy.ScoreTermsWithPositions(item.Name, query)
	descScore := fuzzy.ScoreTerms(item.Description, query)

	aliasScore := 0
	if item.Category == ItemCategoryCommands && item.Command != nil {
		for _, alias := range item.Command.Aliases {
			score := fuzzy.ScoreTerms(alias, query)
			if score > aliasScore {
				aliasScore = score
			}
		}
	}

	var positions []int
	if nameScore > 0 {
		positions = namePositions
	}

	bestScore := nameScore
	matchedField := MatchFieldName
	if descScore > bestScore {
		bestScore = descScore
		matchedField = MatchFieldDescription
	} else if aliasScore > bestScore {
		bestScore = aliasScore
		matchedField = MatchFieldAlias
	} else if nameScore > 0 {
		matchedField = MatchFieldName
	}

	if nameScore > 0 && nameScore >= descScore && nameScore >= aliasScore {
		bestScore = nameScore * 2
	}

	if bestScore == 0 {
		return 0, nil, MatchFieldNone
	}

	score := bestScore * categoryWeight(item.Category)
	if m.isRecentSelection(item) {
		score *= recentBoostMultiplier
	}

	return score, positions, matchedField
}

// isRecentSelection checks if an item appears in the recent selections list.
func (m *CommandPaletteModel) isRecentSelection(item PaletteItem) bool {
	itemID := m.getItemID(item)
	for _, recent := range recentPaletteSelections {
		if recent.Category == item.Category && recent.ID == itemID {
			return true
		}
	}
	return false
}

// getItemID returns a unique identifier for a palette item.
func (m *CommandPaletteModel) getItemID(item PaletteItem) string {
	switch item.Category {
	case ItemCategoryCommands:
		if item.Command != nil {
			return item.Command.Name
		}
	case ItemCategoryTasks:
		if item.Task != nil {
			return item.Task.ID
		}
	case ItemCategoryProjects:
		if item.Project != nil {
			return item.Project.Slug
		}
	case ItemCategoryTags:
		return item.Tag
	case ItemCategorySections:
		return item.Section
	case ItemCategorySavedSearch:
		return item.Name
	case ItemCategorySavedFilter:
		if item.SavedFilter != nil {
			return item.SavedFilter.ID
		}
		return item.Name
	case ItemCategoryNotes:
		if item.Note != nil {
			return item.Note.Filename
		}
	case ItemCategoryViews:
		return item.Name
	}
	return ""
}

// recordRecentSelection adds an item to the recent selections list.
// Most recent items are at the front. Duplicates are moved to the front with incremented count.
func recordRecentSelection(item PaletteItem, itemID string) {
	existingCount := 0
	for i, recent := range recentPaletteSelections {
		if recent.Category == item.Category && recent.ID == itemID {
			existingCount = recent.Count
			recentPaletteSelections = append(recentPaletteSelections[:i], recentPaletteSelections[i+1:]...)
			break
		}
	}

	recentPaletteSelections = append([]recentSelection{{Category: item.Category, ID: itemID, Count: existingCount + 1}}, recentPaletteSelections...)
	if len(recentPaletteSelections) > maxRecentItems {
		recentPaletteSelections = recentPaletteSelections[:maxRecentItems]
	}
}

// getItemFrequencyCount returns how many times an item has been selected.
func (m *CommandPaletteModel) getItemFrequencyCount(item PaletteItem) int {
	itemID := m.getItemID(item)
	for _, recent := range recentPaletteSelections {
		if recent.Category == item.Category && recent.ID == itemID {
			return recent.Count
		}
	}
	return 0
}

// persistRecentSelections saves the current recent selections to config.
func (m *CommandPaletteModel) persistRecentSelections() {
	if m.configManager == nil {
		return
	}

	var configItems []config.RecentPaletteItem
	for _, r := range recentPaletteSelections {
		configItems = append(configItems, config.RecentPaletteItem{
			Category: string(r.Category),
			ID:       r.ID,
			Count:    r.Count,
		})
	}

	m.config.SetRecentPaletteItems(configItems)
	if err := m.configManager.Save(m.config); err != nil {
		logging.Warn("failed to save config: %v", err)
	}
}
