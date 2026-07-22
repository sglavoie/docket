package tui

import "strings"

// renderPreview renders the preview pane for the selected item.
func (m *CommandPaletteModel) renderPreview() string {
	if m.actionMode {
		return m.renderActionPreview()
	}

	query := strings.TrimSpace(m.searchInput.Value())
	if strings.HasPrefix(query, "?") {
		return m.renderHelpReference()
	}

	if len(m.filteredItems) == 0 || m.cursor >= len(m.filteredItems) {
		return MutedStyle.Render("No item selected.")
	}

	filtered := m.filteredItems[m.cursor]
	item := filtered.Item

	var preview string
	switch item.Category {
	case ItemCategoryCommands:
		preview = m.renderCommandPreview(item)
	case ItemCategoryTasks:
		preview = m.renderTaskPreview(item)
	case ItemCategoryProjects:
		preview = m.renderProjectPreview(item)
	case ItemCategoryTags:
		preview = m.renderTagPreview(item)
	case ItemCategorySections:
		preview = m.renderSectionPreview(item)
	case ItemCategorySavedFilter:
		preview = m.renderSavedFilterPreview(item)
	case ItemCategoryNotes:
		preview = m.renderNotePreview(item)
	case ItemCategoryViews:
		preview = m.renderViewPreview(item)
	default:
		return MutedStyle.Render("Unknown item type.")
	}

	if filtered.MatchedField != MatchFieldNone && filtered.MatchedField != MatchFieldName && filtered.MatchedTerm != "" {
		preview += "\n" + m.renderMatchInfo(filtered)
	}

	return preview
}

// renderMatchInfo shows why an item matched the search query.
func (m *CommandPaletteModel) renderMatchInfo(filtered filteredPaletteItem) string {
	var b strings.Builder

	b.WriteString(MutedStyle.Render("─────────────────────"))
	b.WriteString("\n")

	var fieldName string
	switch filtered.MatchedField {
	case MatchFieldDescription:
		fieldName = "description"
	case MatchFieldAlias:
		fieldName = "alias"
	case MatchFieldTag:
		fieldName = "tag"
	case MatchFieldSection:
		fieldName = "section"
	case MatchFieldProjectSlug:
		fieldName = "slug"
	default:
		return ""
	}

	b.WriteString(MutedStyle.Render("Matched in "))
	b.WriteString(MatchHighlightStyle.Render(fieldName))
	b.WriteString(MutedStyle.Render(": "))
	b.WriteString(BoldStyle.Render("\"" + filtered.MatchedTerm + "\""))

	return b.String()
}
