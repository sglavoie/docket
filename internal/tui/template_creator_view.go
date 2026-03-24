package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
)

func (m *TaskTemplateCreatorModel) View() tea.View {
	var b strings.Builder

	// Title
	title := "Create Task Template"
	if m.editingTemplate != nil {
		title = "Edit Task Template"
	}
	b.WriteString(renderFullscreenTitle(title, ""))
	b.WriteString("\n")

	// Tab bar
	tabNames := templateTabNames
	if !m.currentTypeCanHaveChildren() {
		// Hide subtasks tab for types that can't have children
		tabNames = templateTabNames[:2]
	}
	b.WriteString(RenderTabBar(tabNames, m.activeTab, m.width-CreatorTabBarPadding, 1))
	b.WriteString("\n\n")

	// Render content based on active tab
	switch m.activeTab {
	case templateTabFields:
		b.WriteString(m.renderFieldsTab())
	case templateTabChecklist:
		b.WriteString(m.renderChecklistTab())
	case templateTabSubtasks:
		b.WriteString(m.renderSubtasksTab())
	}

	// Error display
	if m.err != nil {
		b.WriteString("\n")
		b.WriteString(ValidationErrorStyle.Render(m.err.Error()))
	}

	// Footer (use " · " separator for renderWrappingHelpLine)
	var helpText string
	switch m.activeTab {
	case templateTabFields:
		helpText = "[]: tabs · Tab: next · ←/→: select · ↓: show all · Ctrl+S: save · Esc: cancel"
	case templateTabChecklist:
		helpText = "[]: tabs · Tab: next · Ctrl+A: add · Ctrl+K/J: reorder · Ctrl+D: delete · Ctrl+S: save · Esc: cancel"
	case templateTabSubtasks:
		helpText = "[]: tabs · Tab: next · ←/→: type · Ctrl+O: optional · Ctrl+T: add · Ctrl+K/J: reorder · Ctrl+D: delete · Ctrl+S: save · Esc: cancel"
	}

	content := b.String()

	var out strings.Builder
	out.WriteString(content)
	out.WriteString("\n")
	// Use wrapping help line for better display on narrow terminals
	wrappedHelp := renderWrappingHelpLine(helpText, m.width)
	out.WriteString(MutedStyle.Render(strings.Repeat("─", m.width)))
	out.WriteString("\n")
	out.WriteString(HelpStyle.Render(wrappedHelp))

	return tea.NewView(out.String())
}

// renderFieldsTab renders the Fields tab content.
func (m *TaskTemplateCreatorModel) renderFieldsTab() string {
	var b strings.Builder

	// Name field
	b.WriteString(m.renderLabel("Name", 0))
	b.WriteString(m.nameInput.View())
	b.WriteString("\n")

	// Type selector
	b.WriteString(m.renderLabel("Type", 1))
	b.WriteString(m.renderTypeSelector())
	b.WriteString("\n")

	// Category selector
	b.WriteString(m.renderLabel("Category", 2))
	b.WriteString(m.renderCategorySelector())
	b.WriteString("\n")

	// Priority selector
	b.WriteString(m.renderLabel("Priority", 3))
	b.WriteString(m.renderPrioritySelector())
	b.WriteString("\n")

	// Section field
	b.WriteString(m.renderLabel("Section", 4))
	b.WriteString(m.sectionInput.View())
	b.WriteString("\n")
	// Render section suggestions if focused and showing
	if m.focusIndex == 4 && m.showSuggestions && len(m.sectionSuggestions) > 0 {
		b.WriteString(m.renderSuggestionsDropdown(m.sectionSuggestions))
	}

	// Tags field
	b.WriteString(m.renderLabel("Tags", 5))
	b.WriteString(m.tagsInput.View())
	b.WriteString("\n")
	// Render tag suggestions if focused and showing
	if m.focusIndex == 5 && m.showSuggestions && len(m.tagSuggestions) > 0 {
		b.WriteString(m.renderSuggestionsDropdown(m.tagSuggestions))
	}

	// Description field - indent each line to align with other fields
	b.WriteString(m.renderLabel("Desc", 6))
	b.WriteString("\n")
	descLines := strings.Split(m.descInput.View(), "\n")
	indent := "              " // 14 chars to match label width
	for i, line := range descLines {
		b.WriteString(indent)
		b.WriteString(line)
		if i < len(descLines)-1 {
			b.WriteString("\n")
		}
	}
	b.WriteString("\n")

	return b.String()
}

// renderChecklistTab renders the Checklist tab content.
func (m *TaskTemplateCreatorModel) renderChecklistTab() string {
	var b strings.Builder

	b.WriteString(MutedStyle.Render(fmt.Sprintf("Checklist Items (%d)", len(m.checklist))))
	b.WriteString("\n\n")

	if len(m.checklist) == 0 {
		b.WriteString(MutedStyle.Render("No checklist items. Press Ctrl+A to add one."))
		b.WriteString("\n")
	} else {
		for i, item := range m.checklist {
			fieldIndex := m.checklistStartIndex() + i
			isFocused := m.focusIndex == fieldIndex

			if isFocused {
				b.WriteString(SelectedItemTextStyle.Render(fmt.Sprintf("▸ %d. ☐ ", i+1)))
			} else {
				fmt.Fprintf(&b, "  %d. ☐ ", i+1)
			}

			b.WriteString(item.View())
			b.WriteString("\n")
		}
	}

	return b.String()
}

// renderSubtasksTab renders the Subtasks tab content.
func (m *TaskTemplateCreatorModel) renderSubtasksTab() string {
	var b strings.Builder

	if !m.currentTypeCanHaveChildren() {
		b.WriteString(MutedStyle.Render(fmt.Sprintf("%s cannot have subtasks. Switch to Epic or Story to add subtasks.", m.types[m.typeCursor])))
		b.WriteString("\n")
		return b.String()
	}

	b.WriteString(MutedStyle.Render(fmt.Sprintf("Subtasks (%d)", len(m.subtasks))))
	b.WriteString("\n\n")

	if len(m.subtasks) == 0 {
		b.WriteString(MutedStyle.Render("No subtasks. Press Ctrl+T to add one."))
		b.WriteString("\n")
	} else {
		for i, st := range m.subtasks {
			fieldIndex := m.subtaskStartIndex() + i
			isFocused := m.focusIndex == fieldIndex

			// Optional indicator
			optMarker := "○"
			if st.optional {
				optMarker = "◌"
			}

			// Type indicator
			typeLabel := string(subtaskTypes[st.typeCursor])

			if isFocused {
				b.WriteString(SelectedItemTextStyle.Render(fmt.Sprintf("▸ %d. %s [%s] ", i+1, optMarker, typeLabel)))
			} else {
				fmt.Fprintf(&b, "  %d. %s [%s] ", i+1, optMarker, typeLabel)
			}

			b.WriteString(st.titleInput.View())
			if st.optional {
				b.WriteString(MutedStyle.Render(" (optional)"))
			}
			b.WriteString("\n")
		}
	}

	return b.String()
}

func (m *TaskTemplateCreatorModel) renderLabel(label string, fieldIndex int) string {
	isFocused := m.focusIndex == fieldIndex
	if isFocused {
		return SelectedItemTextStyle.Render(fmt.Sprintf("▸ %-10s ", label))
	}
	return fmt.Sprintf("  %-10s ", label)
}

func (m *TaskTemplateCreatorModel) renderTypeSelector() string {
	var parts []string
	for i, t := range m.types {
		label := string(t)
		if i == m.typeCursor {
			if m.focusIndex == 1 {
				parts = append(parts, SelectorOptionSelectedStyle.Render("["+label+"]"))
			} else {
				parts = append(parts, BoldStyle.Render("["+label+"]"))
			}
		} else {
			parts = append(parts, MutedStyle.Render(label))
		}
	}
	return strings.Join(parts, " ")
}

func (m *TaskTemplateCreatorModel) renderCategorySelector() string {
	var parts []string
	labels := []string{"(none)", "bug", "feature", "process"}
	for i := range m.categories {
		label := labels[i]
		if i == m.categoryCursor {
			if m.focusIndex == 2 {
				parts = append(parts, SelectorOptionSelectedStyle.Render("["+label+"]"))
			} else {
				parts = append(parts, BoldStyle.Render("["+label+"]"))
			}
		} else {
			parts = append(parts, MutedStyle.Render(label))
		}
	}
	return strings.Join(parts, " ")
}

func (m *TaskTemplateCreatorModel) renderPrioritySelector() string {
	var parts []string
	labels := []string{"default", "1", "2", "3", "4", "5"}
	for i := range labels {
		label := labels[i]
		if i == m.priorityCursor {
			if m.focusIndex == 3 {
				parts = append(parts, SelectorOptionSelectedStyle.Render("["+label+"]"))
			} else {
				parts = append(parts, BoldStyle.Render("["+label+"]"))
			}
		} else {
			parts = append(parts, MutedStyle.Render(label))
		}
	}
	return strings.Join(parts, " ")
}

// renderSuggestionsDropdown renders an inline dropdown of suggestions.
func (m *TaskTemplateCreatorModel) renderSuggestionsDropdown(suggestions []string) string {
	var b strings.Builder
	// Limit to 5 visible suggestions
	maxVisible := min(len(suggestions), 5)

	// Calculate start index for scrolling
	startIdx := 0
	if m.suggestionCursor >= maxVisible {
		startIdx = m.suggestionCursor - maxVisible + 1
	}
	endIdx := min(startIdx+maxVisible, len(suggestions))

	indent := "              " // Match label width (12 chars + "  ")
	for i := startIdx; i < endIdx; i++ {
		suggestion := suggestions[i]
		if i == m.suggestionCursor {
			b.WriteString(indent)
			b.WriteString(SelectorOptionSelectedStyle.Render("▸ " + suggestion))
		} else {
			b.WriteString(indent)
			b.WriteString(MutedStyle.Render("  " + suggestion))
		}
		b.WriteString("\n")
	}
	if len(suggestions) > maxVisible {
		b.WriteString(indent)
		b.WriteString(MutedStyle.Render(fmt.Sprintf("  (%d more...)", len(suggestions)-maxVisible)))
		b.WriteString("\n")
	}
	return b.String()
}
