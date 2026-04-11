package tui

import (
	"fmt"
	"strings"
)

// buildSubtitle builds the subtitle showing item counts.
func (m *CommandPaletteModel) buildSubtitle() string {
	if m.actionMode {
		return fmt.Sprintf("%d available", len(m.availableActions))
	}

	counts := make(map[ItemCategory]int)
	for _, filtered := range m.filteredItems {
		counts[filtered.Item.Category]++
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
		return fmt.Sprintf("%d items", len(m.filteredItems))
	}

	return strings.Join(parts, " · ")
}

// buildFooter constructs the footer text with navigation hints and category counts.
// Adapts to available width by showing shorter hints when space is limited.
func (m *CommandPaletteModel) buildFooter() string {
	counts := make(map[ItemCategory]int)
	for _, filtered := range m.filteredItems {
		counts[filtered.Item.Category]++
	}

	type catLabel struct {
		cat              ItemCategory
		singular, plural string
	}

	catLabels := []catLabel{
		{ItemCategorySavedSearch, "saved search", "saved searches"},
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

	var toggles []string
	if m.includeCompleted {
		toggles = append(toggles, "+H")
	}
	if m.sortField != SortByDefault {
		switch m.sortField { //nolint:exhaustive
		case SortByPriority:
			toggles = append(toggles, "P↓")
		case SortByDueDate:
			toggles = append(toggles, "D↓")
		}
	}
	if m.compactMode {
		toggles = append(toggles, "◫")
	}

	toggleSummary := ""
	if len(toggles) > 0 {
		toggleSummary = " [" + strings.Join(toggles, " ") + "]"
	}

	query := strings.TrimSpace(m.searchInput.Value())
	if query == "" {
		return "+ create | : filter | # jump | \"exact\" | -exclude | @due | priority:N"
	}

	if m.isDirectActionEnabled() && len(m.filteredItems) > 0 && m.cursor < len(m.filteredItems) {
		item := m.filteredItems[m.cursor].Item
		if item.Category == ItemCategoryTasks && item.Task != nil {
			return fmt.Sprintf("e=Edit g=View x=Complete s=Start b=Block r=Reopen | %s%s", countSummary, toggleSummary)
		}
	}

	hints := "↑↓ · Enter · Tab · ^H · ^O · ^P · Esc"
	return fmt.Sprintf("%s | %s%s", hints, countSummary, toggleSummary)
}

// buildActionModeFooter builds a compact footer for action mode.
func (m *CommandPaletteModel) buildActionModeFooter() string {
	return "↑↓ Enter Tab Esc"
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
			if m.isDirectActionEnabled() {
				return "Tip: e=edit  g=view  x=complete  s=start  b=block  r=reopen  Tab=more"
			}
			return "Tip: Press Tab for quick actions, or Enter to go to task"
		case ItemCategoryCommands:
			if item.Command != nil && item.Command.Shortcut != "" {
				return fmt.Sprintf("Tip: Press %s to execute, or Enter to confirm", item.Command.Shortcut)
			}
			return "Tip: Press Enter to execute command"
		case ItemCategoryProjects:
			return "Tip: Enter=open project  Tab=more actions (edit, archive)"
		case ItemCategoryTags:
			return "Tip: Enter=filter by tag  Tab=manage tags"
		case ItemCategorySections:
			return "Tip: Enter=filter by section"
		}
	}

	if query == "" {
		return "Tip: Type to search, or use :cmd :task :prj :tag :sec :note :view to filter"
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
