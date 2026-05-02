package tui

import (
	"fmt"
	"strings"

	"github.com/sglavoie/tt/internal/template"
)

// renderListPanel renders the template list for the left pane.
func (m *TemplatePickerModel) renderListPanel(width int) string {
	var b strings.Builder

	// Templates list (grouped by category)
	if len(m.filteredTemplates) == 0 && len(m.templates) == 0 {
		b.WriteString(MutedStyle.Render("No templates available."))
		b.WriteString("\n")
		b.WriteString(MutedStyle.Render("Use 'tt template init'"))
	} else if len(m.filteredTemplates) == 0 {
		b.WriteString(MutedStyle.Render("No templates match"))
		b.WriteString("\n")
		b.WriteString(MutedStyle.Render("your search."))
	} else {
		// Group templates by category
		byCategory := make(map[string][]template.TaskTemplate)
		for _, t := range m.filteredTemplates {
			cat := t.Category
			if cat == "" {
				cat = "other"
			}
			byCategory[cat] = append(byCategory[cat], t)
		}

		// Calculate list index for cursor
		listIdx := 0
		for _, cat := range m.categoryOrder {
			templates, ok := byCategory[cat]
			if !ok || len(templates) == 0 {
				continue
			}

			// Category header
			b.WriteString(MutedStyle.Render(strings.ToUpper(cat)))
			b.WriteString("\n")

			for _, t := range templates {
				// Truncate name if too long for width (account for prefix "▸ " = 2 chars)
				maxNameLen := max(width-3, 1)
				name := truncateToVisualWidth(t.Name, maxNameLen)

				if listIdx == m.cursor {
					b.WriteString(SelectedStyle.Render(fmt.Sprintf("▸ %s", name)))
				} else {
					fmt.Fprintf(&b, "  %s", name)
				}
				b.WriteString("\n")
				listIdx++
			}
			b.WriteString("\n")
		}
	}

	// Skip option (always visible)
	b.WriteString(MutedStyle.Render("───────────"))
	b.WriteString("\n")
	if m.cursor == len(m.filteredTemplates) {
		b.WriteString(SelectedStyle.Render("▸ Skip"))
	} else {
		b.WriteString(MutedStyle.Render("  Skip"))
	}

	return b.String()
}
