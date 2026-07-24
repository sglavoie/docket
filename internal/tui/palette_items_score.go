package tui

import (
	"strings"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/fuzzy"
	"github.com/sglavoie/tt/internal/logging"
)

// Project identity match-quality tiers, ranked from strongest to weakest. They
// order project matches by how the query relates to the project name or slug.
const (
	projectQualityNone      = 0
	projectQualityFuzzy     = 1 // scattered subsequence match only
	projectQualitySubstring = 2 // contiguous substring somewhere in name/slug
	projectQualityPrefix    = 3 // name or slug starts with the query
	projectQualityExact     = 4 // query equals the whole name or slug
)

// projectIdentityBonus returns a deterministic score bonus that ranks a project
// identity match strictly by its quality tier, ahead of any fuzzy match from a
// higher-weighted category (e.g. commands). The gaps dwarf the ordinary
// fuzzy/category/recency range (a handful of points for short slugs), so the
// ordering exact > prefix > substring > fuzzy holds regardless of the raw base
// score — the base fuzzy score can otherwise invert substring vs. subsequence
// matches (a start-anchored subsequence can outscore a mid-string substring).
// This guarantees an exactly named or slugged project — such as one called
// "tt" — is never buried in a command-heavy context.
func projectIdentityBonus(quality int) int {
	switch quality {
	case projectQualityExact:
		return 200000
	case projectQualityPrefix:
		return 20000
	case projectQualitySubstring:
		return 2000
	case projectQualityFuzzy:
		return 200
	default:
		return 0
	}
}

// projectMatchQuality classifies how a single query term matches a project's
// name or slug, taking the strongest tier across both fields.
func projectMatchQuality(name, slug, query string) int {
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		return projectQualityNone
	}
	n := strings.ToLower(strings.TrimSpace(name))
	s := strings.ToLower(strings.TrimSpace(slug))
	switch {
	case q == n || q == s:
		return projectQualityExact
	case strings.HasPrefix(n, q) || strings.HasPrefix(s, q):
		return projectQualityPrefix
	case strings.Contains(n, q) || strings.Contains(s, q):
		return projectQualitySubstring
	case fuzzy.Match(n, q) || fuzzy.Match(s, q):
		return projectQualityFuzzy
	}
	return projectQualityNone
}

// commandIdentityBonus lets an exact canonical name beat an exact alias, and an
// exact alias beat a longer command name that merely contains the query. The
// bonus stays below project identity tiers so an exactly named project retains
// the palette's established first-class identity behavior.
func commandIdentityBonus(item PaletteItem, query string) int {
	if item.Category != ItemCategoryCommands || item.Command == nil {
		return 0
	}
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		return 0
	}
	if strings.ToLower(strings.TrimSpace(item.Name)) == query {
		return 10000
	}
	for _, alias := range item.Command.Aliases {
		if strings.ToLower(strings.TrimSpace(alias)) == query {
			return 5000
		}
	}
	return 0
}

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

	// A project's slug is an explicit, first-class searchable field.
	slugScore := 0
	if item.Category == ItemCategoryProjects && item.Project != nil {
		slugScore = fuzzy.ScoreTerms(item.Project.Slug, query)
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

	// A slug match that beats name/description/alias takes over provenance so
	// the preview can explain the result. Positions stay nil because they index
	// Name, which the slug match did not necessarily touch.
	if slugScore > bestScore {
		bestScore = slugScore
		matchedField = MatchFieldProjectSlug
		positions = nil
	}

	if nameScore > 0 && nameScore >= descScore && nameScore >= aliasScore && nameScore >= slugScore {
		bestScore = nameScore * 2
		matchedField = MatchFieldName
		positions = namePositions
	}

	if bestScore == 0 {
		return 0, nil, MatchFieldNone
	}

	score := bestScore * categoryWeight(item.Category)
	if m.isRecentSelection(item) {
		score *= recentBoostMultiplier
	}
	score += commandIdentityBonus(item, query)

	// Add the identity bonus last so the deterministic tier gaps are not
	// amplified by the category/recency multipliers above.
	if item.Category == ItemCategoryProjects && item.Project != nil {
		score += projectIdentityBonus(projectMatchQuality(item.Project.Name, item.Project.Slug, query))
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

// persistRecentSelections saves the current recent selections to config.
func (m *CommandPaletteModel) persistRecentSelections() {
	var configItems []config.RecentPaletteItem
	for _, r := range recentPaletteSelections {
		configItems = append(configItems, config.RecentPaletteItem{
			Category: string(r.Category),
			ID:       r.ID,
			Count:    r.Count,
		})
	}

	if err := commitMachineToRuntime(m.runtime, func(machine *config.MachineState) {
		machine.SetRecentPaletteItems(configItems)
	}); err != nil {
		logging.Warn("failed to save machine state: %v", err)
	}
}
