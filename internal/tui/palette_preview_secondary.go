package tui

import (
	"fmt"
	"slices"
	"strings"

	"charm.land/lipgloss/v2"
)

// renderTagPreview shows tasks with this tag.
func (m *CommandPaletteModel) renderTagPreview(item PaletteItem) string {
	if item.Tag == "" {
		return MutedStyle.Render("No tag details.")
	}
	var b strings.Builder

	b.WriteString(TaskJumperPreviewHeaderStyle.Render(CategoryTagStyle.Render("#" + item.Tag)))
	b.WriteString("\n\n")

	var tasksWithTag []PaletteItem
	for _, fi := range m.filteredItems {
		if fi.Item.Category == ItemCategoryTasks && fi.Item.Task != nil && slices.Contains(fi.Item.Task.Tags, item.Tag) {
			tasksWithTag = append(tasksWithTag, fi.Item)
		}
	}

	if len(tasksWithTag) == 0 {
		b.WriteString(MutedStyle.Render("No active tasks with this tag."))
		return b.String()
	}

	b.WriteString(SubtitleStyle.Render(fmt.Sprintf("Tasks (%d)", len(tasksWithTag))))
	b.WriteString("\n")
	m.writeTaskListPreview(&b, tasksWithTag)
	return b.String()
}

// renderSectionPreview shows tasks in this section.
func (m *CommandPaletteModel) renderSectionPreview(item PaletteItem) string {
	if item.Section == "" {
		return MutedStyle.Render("No section details.")
	}
	var b strings.Builder

	b.WriteString(TaskJumperPreviewHeaderStyle.Render(CategorySectionStyle.Render("§ " + item.Section)))
	b.WriteString("\n\n")

	var tasksInSection []PaletteItem
	for _, fi := range m.filteredItems {
		if fi.Item.Category == ItemCategoryTasks && fi.Item.Task != nil && fi.Item.Task.Section == item.Section {
			tasksInSection = append(tasksInSection, fi.Item)
		}
	}

	if len(tasksInSection) == 0 {
		b.WriteString(MutedStyle.Render("No active tasks in this section."))
		return b.String()
	}

	b.WriteString(SubtitleStyle.Render(fmt.Sprintf("Tasks (%d)", len(tasksInSection))))
	b.WriteString("\n")
	m.writeTaskListPreview(&b, tasksInSection)
	return b.String()
}

func (m *CommandPaletteModel) writeTaskListPreview(b *strings.Builder, items []PaletteItem) {
	maxShow := min(len(items), 5)
	wrapStyle := lipgloss.NewStyle().Width(max(m.previewWidth-2, 1))
	for i := range maxShow {
		task := items[i].Task
		line := fmt.Sprintf("  %s #%d: %s",
			GetTypeBadge(string(task.Type), task.Type.Icon()),
			task.ShortID,
			task.Title)
		b.WriteString(wrapStyle.Render(line))
		b.WriteString("\n")
	}
	if len(items) > maxShow {
		b.WriteString(MutedStyle.Render(fmt.Sprintf("  ... and %d more", len(items)-maxShow)))
		b.WriteString("\n")
	}
}

// renderSavedSearchPreview shows details for a saved search.
func (m *CommandPaletteModel) renderSavedSearchPreview(item PaletteItem) string {
	if item.SavedSearch == "" {
		return MutedStyle.Render("No saved search details.")
	}
	var b strings.Builder

	b.WriteString(TaskJumperPreviewHeaderStyle.Render(CategorySavedSearchStyle.Render("★ " + item.Name)))
	b.WriteString("\n\n")

	b.WriteString(SubtitleStyle.Render("Query"))
	b.WriteString("\n")
	b.WriteString(BoldStyle.Render(item.SavedSearch))
	b.WriteString("\n\n")

	b.WriteString(SubtitleStyle.Render("Components"))
	b.WriteString("\n")

	query := item.SavedSearch
	categoryFilter, searchQuery := m.parseQueryPrefix(query)
	if categoryFilter != "" {
		fmt.Fprintf(&b, "  Category: %s\n", getCategoryDisplayName(categoryFilter))
	}

	remaining, filters := parseFieldFilters(searchQuery)
	for _, f := range filters {
		switch f.field {
		case "priority":
			fmt.Fprintf(&b, "  Priority: %s\n", f.value)
		case "status":
			m.writeSavedSearchAltOrValue(&b, "Status", f)
		case "tag":
			m.writeSavedSearchAltOrValue(&b, "Tag", f)
		case "section":
			m.writeSavedSearchAltOrValue(&b, "Section", f)
		case "date":
			fmt.Fprintf(&b, "  Due: %s\n", f.value)
		}
	}

	if remaining != "" {
		fmt.Fprintf(&b, "  Search: %s\n", remaining)
	}

	b.WriteString("\n")
	b.WriteString(HelpStyle.Render("Press Enter to run · Tab for actions · d to delete"))

	return b.String()
}

func (m *CommandPaletteModel) writeSavedSearchAltOrValue(b *strings.Builder, label string, f fieldFilter) {
	if f.op == "in" {
		fmt.Fprintf(b, "  %s: %s\n", label, strings.Join(f.alt, " or "))
		return
	}
	fmt.Fprintf(b, "  %s: %s\n", label, f.value)
}

// renderSavedFilterPreview shows details for a saved filter.
func (m *CommandPaletteModel) renderSavedFilterPreview(item PaletteItem) string {
	if item.SavedFilter == nil {
		return MutedStyle.Render("No saved filter details.")
	}
	sf := item.SavedFilter
	var b strings.Builder

	b.WriteString(TaskJumperPreviewHeaderStyle.Render(sf.Name))
	b.WriteString("\n\n")

	if sf.Mode == "criteria" {
		b.WriteString(MutedStyle.Render("Mode: criteria"))
	} else {
		b.WriteString(MutedStyle.Render("Mode: snapshot"))
	}
	b.WriteString("\n\n")

	desc := describeSavedFilterSummary(sf)
	b.WriteString(SubtitleStyle.Render("Filters"))
	b.WriteString("\n")
	b.WriteString(lipgloss.NewStyle().Width(max(m.previewWidth-2, 1)).Render(desc))
	b.WriteString("\n\n")

	b.WriteString(HelpStyle.Render("Press Enter to apply · Tab for actions"))

	return b.String()
}

// renderNotePreview shows details for a note.
func (m *CommandPaletteModel) renderNotePreview(item PaletteItem) string {
	if item.Note == nil {
		return MutedStyle.Render("No note details.")
	}
	note := item.Note
	var b strings.Builder

	b.WriteString(TaskJumperPreviewHeaderStyle.Render(truncateToVisualWidth(note.Title, max(m.previewWidth-2, 1))))
	b.WriteString("\n\n")

	if item.Project != nil {
		b.WriteString(MutedStyle.Render("Project: "))
		b.WriteString(BoldStyle.Render(item.Project.Name))
		b.WriteString("\n")
	}

	if len(note.Tags) > 0 {
		tags := make([]string, len(note.Tags))
		for i, tag := range note.Tags {
			tags[i] = TagInlineStyle.Render("#" + tag)
		}
		fmt.Fprintf(&b, "%s %s\n",
			MutedStyle.Render("Tags:"),
			strings.Join(tags, " "))
	}

	if note.Pinned {
		b.WriteString(MutedStyle.Render("Pinned: "))
		b.WriteString("yes\n")
	}

	if len(note.LinkedTasks) > 0 {
		fmt.Fprintf(&b, "%s %d\n",
			MutedStyle.Render("Linked tasks:"),
			len(note.LinkedTasks))
	}

	if note.Body != "" {
		b.WriteString("\n")
		b.WriteString(SubtitleStyle.Render("Content"))
		b.WriteString("\n")
		b.WriteString(renderMarkdown(strings.TrimSpace(note.Body), m.previewWidth-2))
		b.WriteString("\n")
	}

	return b.String()
}

// renderViewPreview shows details for a navigable view.
func (m *CommandPaletteModel) renderViewPreview(item PaletteItem) string {
	var b strings.Builder

	b.WriteString(TaskJumperPreviewHeaderStyle.Render(item.Name))
	b.WriteString("\n\n")

	if item.Description != "" {
		b.WriteString(SubtitleStyle.Render("Description"))
		b.WriteString("\n")
		b.WriteString(lipgloss.NewStyle().Width(max(m.previewWidth-2, 1)).Render(item.Description))
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(HelpStyle.Render("Press Enter to navigate"))

	return b.String()
}