package tui

import (
	"fmt"
	"strings"
)

// buildSubtitle builds the subtitle showing item counts.
// Recent and Suggested section items (duplicates) are excluded from counts.
func (m *CommandPaletteModel) buildSubtitle() string {
	if m.actionMode {
		return fmt.Sprintf("%d available", len(m.availableActions))
	}

	counts := make(map[ItemCategory]int)
	for _, filtered := range m.filteredItems {
		if !filtered.IsFrequent && !filtered.IsSuggested {
			counts[filtered.Item.Category]++
		}
	}

	var parts []string
	if c := counts[ItemCategoryCommands]; c > 0 {
		parts = append(parts, fmt.Sprintf("%d %s", c, pluralize(c, "command", "commands")))
	}
	if c := counts[ItemCategoryTasks]; c > 0 {
		parts = append(parts, fmt.Sprintf("%d %s", c, pluralize(c, "task", "tasks")))
	}
	if c := counts[ItemCategoryProjects]; c > 0 {
		parts = append(parts, fmt.Sprintf("%d %s", c, pluralize(c, "project", "projects")))
	}

	if len(parts) == 0 {
		total := 0
		for _, fi := range m.filteredItems {
			if !fi.IsFrequent && !fi.IsSuggested {
				total++
			}
		}
		return m.decorateSubtitle(fmt.Sprintf("%d items", total))
	}

	return m.decorateSubtitle(strings.Join(parts, " · "))
}

// decorateSubtitle appends a "showing all" marker while transient show-all is
// active on an empty query, so the count reflects the expanded discovery set.
func (m *CommandPaletteModel) decorateSubtitle(base string) string {
	if m.showAll && strings.TrimSpace(m.searchInput.Value()) == "" {
		return base + " · showing all"
	}
	return base
}

// buildFooter constructs the footer text with navigation hints and category counts.
// Adapts to available width by showing shorter hints when space is limited.
func (m *CommandPaletteModel) buildFooter() string {
	counts := make(map[ItemCategory]int)
	for _, filtered := range m.filteredItems {
		if !filtered.IsFrequent && !filtered.IsSuggested {
			counts[filtered.Item.Category]++
		}
	}

	type catLabel struct {
		cat              ItemCategory
		singular, plural string
	}

	catLabels := []catLabel{
		{ItemCategoryCommands, "command", "commands"},
		{ItemCategoryProjects, "project", "projects"},
		{ItemCategoryTasks, "task", "tasks"},
		{ItemCategoryTags, "tag", "tags"},
		{ItemCategorySections, "section", "sections"},
		{ItemCategoryNotes, "note", "notes"},
		{ItemCategoryViews, "view", "views"},
	}

	var countParts []string
	for _, cl := range catLabels {
		if c := counts[cl.cat]; c > 0 {
			countParts = append(countParts, fmt.Sprintf("%d %s", c, pluralize(c, cl.singular, cl.plural)))
		}
	}

	countSummary := strings.Join(countParts, " · ")
	if countSummary == "" {
		countSummary = "0"
	}

	query := strings.TrimSpace(m.searchInput.Value())
	if query == "" {
		showAllHint := "^A show all"
		if m.showAll {
			showAllHint = "^A show less"
		}
		return fmt.Sprintf("Type to search · ↑↓/^J^K navigate · Enter open · Shift+Enter Peek · %s · Esc close", showAllHint)
	}

	hints := "↑↓/^J^K · Enter · Shift+Enter · Space select · Tab actions · Esc"
	return fmt.Sprintf("%s | %s", hints, countSummary)
}

// buildActionModeFooter builds a compact footer for action mode.
func (m *CommandPaletteModel) buildActionModeFooter() string {
	return "↑↓/^J^K navigate · Enter choose · Tab/Esc back"
}

// getContextualHint returns a context-aware hint based on current palette state.
// Returns empty string if no hint should be shown.
func (m *CommandPaletteModel) getContextualHint() string {
	query := strings.TrimSpace(m.searchInput.Value())

	if !m.showTips {
		return ""
	}

	if query != "" && !strings.HasPrefix(query, ":") {
		return ""
	}

	if len(m.filteredItems) > 0 && m.cursor < len(m.filteredItems) {
		item := m.filteredItems[m.cursor].Item
		switch item.Category { //nolint:exhaustive
		case ItemCategoryTasks:
			return "Tip: Enter opens · Shift+Enter Peeks · Space selects · Tab shows actions"
		case ItemCategoryCommands:
			if item.Command != nil && item.Command.Shortcut != "" {
				return fmt.Sprintf("Tip: Press %s to execute, or Enter to confirm", item.Command.Shortcut)
			}
			return "Tip: Press Enter to execute command"
		case ItemCategoryProjects:
			return "Tip: Enter opens the project · Tab shows actions"
		case ItemCategoryTags:
			return "Tip: Enter=filter by tag  Tab=manage tags"
		case ItemCategorySections:
			return "Tip: Enter=filter by section"
		}
	}

	if query == "" {
		if m.showAll {
			return "Tip: Showing all commands and entities · Ctrl+A collapses back to suggestions"
		}
		return "Tip: Type to search, use a category prefix, or Ctrl+A to show everything"
	}

	categoryFilter, _ := m.parseQueryPrefix(query)
	if categoryFilter != "" {
		switch categoryFilter { //nolint:exhaustive
		case ItemCategoryTasks:
			return "Tip: Use priority:1-2 for high priority, @overdue for overdue tasks"
		case ItemCategoryCommands:
			return "Tip: Commands show their keyboard shortcut on the right"
		}
	}

	return ""
}
