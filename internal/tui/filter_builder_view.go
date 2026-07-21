package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"

	"charm.land/lipgloss/v2"
)

// View renders the filter builder interface.
func (m *FilterBuilderModel) View() tea.View {
	title := "Edit Criteria Filter"
	if m.editingID == "" {
		title = "Create Criteria Filter"
	}

	// Tab bar
	tabs := []string{"Criteria", "Options"}
	tabBar := RenderTabBar(tabs, m.activeTab, m.width, 1)

	availableHeight := max(m.height-7, 1)

	editorContent := m.renderEditorPane()
	editorContent = m.applyEditorScroll(editorContent, availableHeight)

	helpText := m.renderHelpText()

	var out strings.Builder
	out.WriteString(renderFullscreenTitle(title, ""))
	out.WriteString("\n")
	out.WriteString(tabBar)
	out.WriteString("\n\n")
	out.WriteString(editorContent)
	out.WriteString("\n")
	out.WriteString(renderStatusBarFooter(helpText, "", m.width))

	return tea.NewView(out.String())
}

// renderHelpText renders the help footer.
func (m *FilterBuilderModel) renderHelpText() string {
	parts := []string{
		"[ ]: tabs",
		"Tab: nav",
		"←→: cycle",
		"Space: toggle",
		"+/-: add/rem",
		"Ctrl+S: save",
		"Esc: cancel",
	}

	singleLine := strings.Join(parts, " · ")
	if lipgloss.Width(singleLine) <= m.width-2 {
		return HelpStyle.Render(singleLine)
	}

	mid := len(parts) / 2
	line1 := strings.Join(parts[:mid], " · ")
	line2 := strings.Join(parts[mid:], " · ")
	return HelpStyle.Render(line1) + "\n" + HelpStyle.Render(line2)
}

// applyEditorScroll applies vertical scrolling to the editor content.
func (m *FilterBuilderModel) applyEditorScroll(content string, availableHeight int) string {
	lines := strings.Split(content, "\n")
	totalLines := len(lines)

	if totalLines <= availableHeight {
		return content
	}

	maxOffset := totalLines - availableHeight
	if m.editorScrollOffset > maxOffset {
		m.editorScrollOffset = maxOffset
	}
	if m.editorScrollOffset < 0 {
		m.editorScrollOffset = 0
	}

	endLine := min(m.editorScrollOffset+availableHeight, totalLines)
	visibleLines := lines[m.editorScrollOffset:endLine]

	return strings.Join(visibleLines, "\n")
}

// renderEditorPane renders the editor content based on active tab.
func (m *FilterBuilderModel) renderEditorPane() string {
	var b strings.Builder

	switch m.activeTab {
	case slTabCriteria:
		b.WriteString(m.renderCriteriaSection())
	case slTabOptions:
		b.WriteString(m.renderOptionsSection())
	}

	if m.err != nil {
		b.WriteString("\n")
		b.WriteString(ValidationErrorStyle.Render(m.err.Error()))
	}

	return b.String()
}

// renderCriteriaSection renders the criteria form fields.
func (m *FilterBuilderModel) renderCriteriaSection() string {
	var b strings.Builder
	section, itemIdx, fieldIdx := m.getFocusSection()

	// Name input
	var nameLabel string
	if section == "name" {
		nameLabel = FieldLabelFocusedStyle.Render("Name: ")
	} else {
		nameLabel = FieldLabelStyle.Render("Name: ")
	}
	b.WriteString(nameLabel)
	b.WriteString(m.nameInput.View())
	b.WriteString("\n\n")

	// Criteria header
	b.WriteString(BoldStyle.Render("Criteria"))
	if len(m.criteria) > 1 {
		b.WriteString(MutedStyle.Render(fmt.Sprintf(" (%d)", len(m.criteria))))
	}
	b.WriteString("\n")

	for i, c := range m.criteria {
		isFocused := section == "criterion" && itemIdx == i
		field := smartListFields[c.fieldCursor]

		// Criterion number
		if len(m.criteria) > 1 {
			b.WriteString(MutedStyle.Render(fmt.Sprintf("  %d. ", i+1)))
		} else {
			b.WriteString("  ")
		}

		// Field selector
		fLabel := "Field: "
		if isFocused && fieldIdx == 0 {
			fLabel = FieldLabelFocusedStyle.Render("Field: ")
		}
		b.WriteString(fLabel)
		b.WriteString(m.renderFieldSelector(c.fieldCursor, isFocused && fieldIdx == 0))
		b.WriteString("\n")

		// Operation selector
		b.WriteString("     ")
		opLabel := "Op: "
		if isFocused && fieldIdx == 1 {
			opLabel = FieldLabelFocusedStyle.Render("Op: ")
		}
		b.WriteString(opLabel)
		b.WriteString(m.renderOpSelector(field.Operations, c.opCursor, isFocused && fieldIdx == 1))
		b.WriteString("\n")

		// Value input/selector
		b.WriteString("     ")
		valLabel := "Value: "
		if isFocused && fieldIdx == 2 {
			valLabel = FieldLabelFocusedStyle.Render("Value: ")
		}
		b.WriteString(valLabel)
		if field.ValueType == "enum" {
			b.WriteString(m.renderEnumSelector(field.EnumValues, c.valueCursor, isFocused && fieldIdx == 2))
		} else {
			b.WriteString(c.valueInput.View())
		}
		b.WriteString("\n")

		if i < len(m.criteria)-1 {
			b.WriteString("\n")
		}
	}

	// Logic toggle (only if >1 criterion)
	if len(m.criteria) > 1 {
		b.WriteString("\n")
		logicLabel := "Logic: "
		if section == "logic" {
			logicLabel = FieldLabelFocusedStyle.Render("Logic: ")
		}
		b.WriteString("  ")
		b.WriteString(logicLabel)
		if m.logic == "and" {
			if section == "logic" {
				b.WriteString(SelectorOptionSelectedStyle.Render("[AND]"))
			} else {
				b.WriteString(BoldStyle.Render("[AND]"))
			}
			b.WriteString(SelectorOptionStyle.Render(" OR "))
		} else {
			b.WriteString(SelectorOptionStyle.Render(" AND "))
			if section == "logic" {
				b.WriteString(SelectorOptionSelectedStyle.Render("[OR]"))
			} else {
				b.WriteString(BoldStyle.Render("[OR]"))
			}
		}
		b.WriteString("\n")
	}

	return b.String()
}

// renderOptionsSection renders the options form fields.
func (m *FilterBuilderModel) renderOptionsSection() string {
	var b strings.Builder
	section, _, _ := m.getFocusSection()

	b.WriteString(BoldStyle.Render("Results"))
	b.WriteString("\n\n")

	// Include completed
	compLabel := "  Include completed: "
	if section == "include_completed" {
		compLabel = "  " + FieldLabelFocusedStyle.Render("Include completed: ")
	}
	b.WriteString(compLabel)
	if m.includeCompleted {
		if section == "include_completed" {
			b.WriteString(SelectorOptionSelectedStyle.Render("[Yes]"))
		} else {
			b.WriteString(BoldStyle.Render("[Yes]"))
		}
		b.WriteString(SelectorOptionStyle.Render(" No "))
	} else {
		b.WriteString(SelectorOptionStyle.Render(" Yes "))
		if section == "include_completed" {
			b.WriteString(SelectorOptionSelectedStyle.Render("[No]"))
		} else {
			b.WriteString(BoldStyle.Render("[No]"))
		}
	}
	b.WriteString("\n")

	return b.String()
}

// renderFieldSelector renders a scrollable field name selector.
func (m *FilterBuilderModel) renderFieldSelector(cursor int, focused bool) string {
	var b strings.Builder

	visible := 3
	start := max(cursor-visible/2, 0)
	end := start + visible
	if end > len(smartListFields) {
		end = len(smartListFields)
		start = max(end-visible, 0)
	}

	if start > 0 {
		b.WriteString(MutedStyle.Render("< "))
	} else {
		b.WriteString("  ")
	}

	for i := start; i < end; i++ {
		display := truncateToVisualWidth(smartListFields[i].Name, 16)

		if i == cursor {
			if focused {
				b.WriteString(SelectorOptionSelectedStyle.Render(fmt.Sprintf("[%s]", display)))
			} else {
				b.WriteString(BoldStyle.Render(fmt.Sprintf("[%s]", display)))
			}
		} else {
			b.WriteString(SelectorOptionStyle.Render(fmt.Sprintf(" %s ", display)))
		}
	}

	if end < len(smartListFields) {
		b.WriteString(MutedStyle.Render(" >"))
	}

	return b.String()
}

// renderOpSelector renders the operation selector for a given set of operations.
func (m *FilterBuilderModel) renderOpSelector(ops []string, cursor int, focused bool) string {
	var b strings.Builder

	opSymbols := map[string]string{
		"equals":       "=",
		"not_equals":   "!=",
		"contains":     "has",
		"greater_than": ">",
		"less_than":    "<",
	}

	for i, op := range ops {
		display := opSymbols[op]
		if display == "" {
			display = op
		}

		if i == cursor {
			if focused {
				b.WriteString(SelectorOptionSelectedStyle.Render(fmt.Sprintf("[%s]", display)))
			} else {
				b.WriteString(BoldStyle.Render(fmt.Sprintf("[%s]", display)))
			}
		} else {
			b.WriteString(SelectorOptionStyle.Render(fmt.Sprintf(" %s ", display)))
		}
	}

	return b.String()
}

// renderEnumSelector renders a scrollable enum value selector.
func (m *FilterBuilderModel) renderEnumSelector(values []string, cursor int, focused bool) string {
	var b strings.Builder

	visible := 4
	start := max(cursor-visible/2, 0)
	end := start + visible
	if end > len(values) {
		end = len(values)
		start = max(end-visible, 0)
	}

	if start > 0 {
		b.WriteString(MutedStyle.Render("< "))
	} else {
		b.WriteString("  ")
	}

	for i := start; i < end; i++ {
		display := truncateToVisualWidth(values[i], 14)

		if i == cursor {
			if focused {
				b.WriteString(SelectorOptionSelectedStyle.Render(fmt.Sprintf("[%s]", display)))
			} else {
				b.WriteString(BoldStyle.Render(fmt.Sprintf("[%s]", display)))
			}
		} else {
			b.WriteString(SelectorOptionStyle.Render(fmt.Sprintf(" %s ", display)))
		}
	}

	if end < len(values) {
		b.WriteString(MutedStyle.Render(" >"))
	}

	return b.String()
}
