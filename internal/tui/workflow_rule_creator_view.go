package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/sglavoie/tt/internal/workflow"
)

// View renders the workflow rule creator interface.
func (m *WorkflowRuleCreatorModel) View() string {
	// Render picker overlays if active
	if m.showingFieldPicker && m.fieldPicker != nil {
		return m.fieldPicker.View()
	}
	if m.showingActionPicker && m.actionPicker != nil {
		return m.actionPicker.View()
	}

	// Narrow screen with preview toggle: show fullscreen preview
	if !m.isWideScreen() && m.previewShown {
		return m.renderFullscreenPreview()
	}

	title := "Edit Workflow Rule"
	if m.editingRule == nil {
		title = "Create Workflow Rule"
	}

	// Tab bar
	tabs := []string{"Triggers", "Actions"}
	tabBar := RenderTabBar(tabs, m.activeTab, m.width, 1)

	// Calculate available height for editor content
	// Title(1) + blank(1) + tabBar(1) + blank(2) + footer(2) = 7 lines overhead
	availableHeight := m.height - 7

	// Editor content with scrolling
	editorContent := m.renderEditorPane()
	editorContent = m.applyEditorScroll(editorContent, availableHeight)

	var mainContent string
	if m.isWideScreen() {
		// Split layout: editor left, preview right
		editorWidth := m.width * 55 / 100
		previewWidth := m.width - editorWidth - 3 // 3 for separator

		// Calculate pane height
		paneHeight := availableHeight
		if paneHeight < 5 {
			paneHeight = 5
		}

		editorPane := lipgloss.NewStyle().
			Width(editorWidth).
			Height(paneHeight).
			Render(editorContent)

		previewPane := lipgloss.NewStyle().
			Width(previewWidth).
			Height(paneHeight).
			Render(m.renderPreviewPane(previewWidth))

		// Full-height separator
		separator := SeparatorStyle.Render(strings.Repeat("│\n", paneHeight))

		mainContent = lipgloss.JoinHorizontal(lipgloss.Top, editorPane, " ", separator, " ", previewPane)
	} else {
		// Narrow: fullscreen editor only
		mainContent = editorContent
	}

	// Help text with wrapping support
	helpText := m.renderHelpText()

	var out strings.Builder
	out.WriteString(renderFullscreenTitle(title, ""))
	out.WriteString("\n")
	out.WriteString(tabBar)
	out.WriteString("\n\n")
	out.WriteString(mainContent)
	out.WriteString("\n")
	out.WriteString(renderStatusBarFooter(helpText, "", m.width))

	return out.String()
}

// renderHelpText renders the help text, wrapping if necessary.
func (m *WorkflowRuleCreatorModel) renderHelpText() string {
	var parts []string
	if m.isWideScreen() {
		parts = []string{
			"[ ]: tabs",
			"Ctrl+f/t: pickers",
			"Tab: nav",
			"↑↓: items",
			"+/-: add/rem",
			"Ctrl+S: save",
			"Esc: cancel",
		}
	} else {
		parts = []string{
			"[ ]: tabs",
			"Ctrl+p: preview",
			"↑↓: items",
			"+/-: add/rem",
			"Ctrl+S: save",
			"Esc: cancel",
		}
	}

	// Try single line first
	singleLine := strings.Join(parts, " | ")
	if lipgloss.Width(singleLine) <= m.width-2 {
		return HelpStyle.Render(singleLine)
	}

	// Split into two lines
	mid := len(parts) / 2
	line1 := strings.Join(parts[:mid], " | ")
	line2 := strings.Join(parts[mid:], " | ")
	return HelpStyle.Render(line1) + "\n" + HelpStyle.Render(line2)
}

// applyEditorScroll applies vertical scrolling to the editor content.
func (m *WorkflowRuleCreatorModel) applyEditorScroll(content string, availableHeight int) string {
	lines := strings.Split(content, "\n")
	totalLines := len(lines)

	if totalLines <= availableHeight {
		return content
	}

	// Ensure scroll offset is valid
	maxOffset := totalLines - availableHeight
	if m.editorScrollOffset > maxOffset {
		m.editorScrollOffset = maxOffset
	}
	if m.editorScrollOffset < 0 {
		m.editorScrollOffset = 0
	}

	// Extract visible lines
	endLine := m.editorScrollOffset + availableHeight
	if endLine > totalLines {
		endLine = totalLines
	}
	visibleLines := lines[m.editorScrollOffset:endLine]

	return strings.Join(visibleLines, "\n")
}

// renderEditorPane renders the editor content based on active tab.
func (m *WorkflowRuleCreatorModel) renderEditorPane() string {
	var b strings.Builder
	section, itemIdx, fieldIdx := m.getFocusSection()

	// Name input (always shown)
	var nameLabel string
	if section == "name" {
		nameLabel = FieldLabelFocusedStyle.Render("Name: ")
	} else {
		nameLabel = FieldLabelStyle.Render("Name: ")
	}
	b.WriteString(nameLabel)
	b.WriteString(m.nameInput.View())
	b.WriteString("\n\n")

	if m.activeTab == tabTriggers {
		b.WriteString(m.renderTriggersSection(section, itemIdx, fieldIdx))
	} else {
		b.WriteString(m.renderActionsSection(section, itemIdx, fieldIdx))
	}

	// Error display
	if m.err != nil {
		b.WriteString("\n")
		b.WriteString(ValidationErrorStyle.Render(m.err.Error()))
	}

	return b.String()
}

// renderTriggersSection renders the triggers form fields.
func (m *WorkflowRuleCreatorModel) renderTriggersSection(section string, itemIdx, fieldIdx int) string {
	var b strings.Builder

	// Triggers section header
	b.WriteString(BoldStyle.Render("Triggers"))
	if len(m.triggers) > 1 {
		b.WriteString(MutedStyle.Render(fmt.Sprintf(" (%d)", len(m.triggers))))
	}
	b.WriteString("\n")

	for i, t := range m.triggers {
		isFocusedTrigger := section == "trigger" && itemIdx == i

		// Trigger number
		if len(m.triggers) > 1 {
			b.WriteString(MutedStyle.Render(fmt.Sprintf("  %d. ", i+1)))
		} else {
			b.WriteString("  ")
		}

		// Field selector
		fieldLabel := "Field: "
		if isFocusedTrigger && fieldIdx == 0 {
			fieldLabel = FieldLabelFocusedStyle.Render("Field: ")
		}
		b.WriteString(fieldLabel)
		b.WriteString(m.renderSelector(m.triggerFields, t.fieldCursor, isFocusedTrigger && fieldIdx == 0, 3))
		b.WriteString("\n")

		// Operation selector
		b.WriteString("     ")
		opLabel := "Op: "
		if isFocusedTrigger && fieldIdx == 1 {
			opLabel = FieldLabelFocusedStyle.Render("Op: ")
		}
		b.WriteString(opLabel)
		b.WriteString(m.renderOpsSelector(t.opCursor, isFocusedTrigger && fieldIdx == 1))
		b.WriteString("\n")

		// Value input
		b.WriteString("     ")
		valueLabel := "Value: "
		if isFocusedTrigger && fieldIdx == 2 {
			valueLabel = FieldLabelFocusedStyle.Render("Value: ")
		}
		b.WriteString(valueLabel)
		b.WriteString(t.valueInput.View())
		b.WriteString("\n")

		if i < len(m.triggers)-1 {
			b.WriteString("\n")
		}
	}

	// Logic toggle (only if >1 trigger)
	if len(m.triggers) > 1 {
		b.WriteString("\n")
		logicLabel := "Logic: "
		if section == "logic" {
			logicLabel = FieldLabelFocusedStyle.Render("Logic: ")
		}
		b.WriteString("  ")
		b.WriteString(logicLabel)
		if m.triggerLogic == workflow.LogicAnd {
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

// renderActionsSection renders the actions form fields.
func (m *WorkflowRuleCreatorModel) renderActionsSection(section string, itemIdx, fieldIdx int) string {
	var b strings.Builder

	// Actions section header
	b.WriteString(BoldStyle.Render("Actions"))
	if len(m.actions) > 1 {
		b.WriteString(MutedStyle.Render(fmt.Sprintf(" (%d)", len(m.actions))))
	}
	b.WriteString("\n")

	for i, a := range m.actions {
		isFocusedAction := section == "action" && itemIdx == i

		// Action number
		if len(m.actions) > 1 {
			b.WriteString(MutedStyle.Render(fmt.Sprintf("  %d. ", i+1)))
		} else {
			b.WriteString("  ")
		}

		// Type selector
		typeLabel := "Type: "
		if isFocusedAction && fieldIdx == 0 {
			typeLabel = FieldLabelFocusedStyle.Render("Type: ")
		}
		b.WriteString(typeLabel)
		b.WriteString(m.renderActionSelector(a.typeCursor, isFocusedAction && fieldIdx == 0))
		b.WriteString("\n")

		// Value input (skip for actions that don't need values)
		actionType := m.actionTypes[a.typeCursor]
		if !m.actionNeedsNoValue(actionType) {
			b.WriteString("     ")
			valueLabel := "Value: "
			if isFocusedAction && fieldIdx == 1 {
				valueLabel = FieldLabelFocusedStyle.Render("Value: ")
			}
			b.WriteString(valueLabel)
			b.WriteString(a.valueInput.View())
			b.WriteString("\n")
		}

		if i < len(m.actions)-1 {
			b.WriteString("\n")
		}
	}

	return b.String()
}

// renderPreviewPane renders the rule preview on the right side.
func (m *WorkflowRuleCreatorModel) renderPreviewPane(width int) string {
	var b strings.Builder
	section, itemIdx, fieldIdx := m.getFocusSection()

	// Styles for syntax highlighting
	keywordStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("14"))              // Cyan for IF/AND/OR/THEN
	operatorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("13"))             // Magenta for =, !=, <, >, :
	highlightStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("11")).Bold(true) // Yellow for focused field

	b.WriteString(BoldStyle.Render("Preview"))
	b.WriteString("\n\n")

	// Render IF conditions (triggers)
	for i, t := range m.triggers {
		var prefix string
		if i == 0 {
			prefix = keywordStyle.Render("IF") + " "
		} else {
			if m.triggerLogic == workflow.LogicAnd {
				prefix = keywordStyle.Render("AND") + " "
			} else {
				prefix = keywordStyle.Render("OR") + " "
			}
		}

		field := string(m.triggerFields[t.fieldCursor])
		op := m.getOpSymbol(m.triggerOps[t.opCursor])
		value := t.valueInput.Value()
		if value == "" {
			value = "?"
		}

		// Highlight if this trigger is being edited
		isFocused := section == "trigger" && itemIdx == i
		var fieldStr, opStr, valueStr string
		if isFocused {
			switch fieldIdx {
			case 0:
				fieldStr = highlightStyle.Render(field)
				opStr = operatorStyle.Render(op)
				valueStr = value
			case 1:
				fieldStr = field
				opStr = highlightStyle.Render(op)
				valueStr = value
			case 2:
				fieldStr = field
				opStr = operatorStyle.Render(op)
				valueStr = highlightStyle.Render(value)
			default:
				fieldStr = field
				opStr = operatorStyle.Render(op)
				valueStr = value
			}
		} else {
			fieldStr = field
			opStr = operatorStyle.Render(op)
			valueStr = value
		}

		b.WriteString(fmt.Sprintf("%s%s %s %s\n", prefix, fieldStr, opStr, valueStr))
	}

	// Render THEN actions
	for i, a := range m.actions {
		var prefix string
		if i == 0 {
			prefix = keywordStyle.Render("THEN") + " "
		} else {
			prefix = keywordStyle.Render("AND") + " "
		}

		actionType := string(m.actionTypes[a.typeCursor])
		value := a.valueInput.Value()

		// Highlight if this action is being edited
		isFocused := section == "action" && itemIdx == i
		noValue := m.actionNeedsNoValue(m.actionTypes[a.typeCursor])

		var line string
		if noValue {
			if isFocused && fieldIdx == 0 {
				line = fmt.Sprintf("%s%s", prefix, highlightStyle.Render(actionType))
			} else {
				line = fmt.Sprintf("%s%s", prefix, actionType)
			}
		} else {
			if value == "" {
				value = "?"
			}
			colonStr := operatorStyle.Render(":")
			if isFocused {
				switch fieldIdx {
				case 0:
					line = fmt.Sprintf("%s%s%s %s", prefix, highlightStyle.Render(actionType), colonStr, value)
				case 1:
					line = fmt.Sprintf("%s%s%s %s", prefix, actionType, colonStr, highlightStyle.Render(value))
				default:
					line = fmt.Sprintf("%s%s%s %s", prefix, actionType, colonStr, value)
				}
			} else {
				line = fmt.Sprintf("%s%s%s %s", prefix, actionType, colonStr, value)
			}
		}

		b.WriteString(line)
		b.WriteString("\n")
	}

	return lipgloss.NewStyle().Width(width).Render(b.String())
}

// renderFullscreenPreview renders the preview as a fullscreen view for narrow screens.
func (m *WorkflowRuleCreatorModel) renderFullscreenPreview() string {
	title := "Rule Preview"

	preview := m.renderPreviewPane(m.width - 4)

	left := HelpStyle.Render("Ctrl+p: back to editor | Esc: cancel")

	var out strings.Builder
	out.WriteString(renderFullscreenTitle(title, ""))
	out.WriteString("\n\n")
	out.WriteString(preview)
	out.WriteString("\n")
	out.WriteString(renderStatusBarFooter(left, "", m.width))

	return out.String()
}

// getOpSymbol returns a symbol for the operation.
func (m *WorkflowRuleCreatorModel) getOpSymbol(op workflow.TriggerOperation) string {
	switch op {
	case workflow.OpEquals:
		return "="
	case workflow.OpNotEquals:
		return "!="
	case workflow.OpContains:
		return "has"
	case workflow.OpGreaterThan:
		return ">"
	case workflow.OpLessThan:
		return "<"
	default:
		return string(op)
	}
}

// renderSelector renders a scrollable selector with context around the current selection.
func (m *WorkflowRuleCreatorModel) renderSelector(fields []workflow.TriggerField, cursor int, focused bool, visible int) string {
	var b strings.Builder

	// Calculate visible range
	start := cursor - visible/2
	if start < 0 {
		start = 0
	}
	end := start + visible
	if end > len(fields) {
		end = len(fields)
		start = end - visible
		if start < 0 {
			start = 0
		}
	}

	// Show left arrow if there are items before
	if start > 0 {
		b.WriteString(MutedStyle.Render("< "))
	} else {
		b.WriteString("  ")
	}

	for i := start; i < end; i++ {
		display := string(fields[i])
		if len(display) > 18 {
			display = display[:15] + "..."
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

	// Show right arrow if there are items after
	if end < len(fields) {
		b.WriteString(MutedStyle.Render(" >"))
	}

	return b.String()
}

// renderOpsSelector renders the operation selector.
func (m *WorkflowRuleCreatorModel) renderOpsSelector(cursor int, focused bool) string {
	var b strings.Builder

	opSymbols := map[workflow.TriggerOperation]string{
		workflow.OpEquals:      "=",
		workflow.OpNotEquals:   "!=",
		workflow.OpContains:    "has",
		workflow.OpGreaterThan: ">",
		workflow.OpLessThan:    "<",
	}

	for i, op := range m.triggerOps {
		display := opSymbols[op]
		if display == "" {
			display = string(op)
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

// renderActionSelector renders the action type selector.
func (m *WorkflowRuleCreatorModel) renderActionSelector(cursor int, focused bool) string {
	var b strings.Builder

	// Show limited context around cursor
	visible := 3
	start := cursor - visible/2
	if start < 0 {
		start = 0
	}
	end := start + visible
	if end > len(m.actionTypes) {
		end = len(m.actionTypes)
		start = end - visible
		if start < 0 {
			start = 0
		}
	}

	if start > 0 {
		b.WriteString(MutedStyle.Render("< "))
	} else {
		b.WriteString("  ")
	}

	for i := start; i < end; i++ {
		display := string(m.actionTypes[i])
		if len(display) > 16 {
			display = display[:13] + "..."
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

	if end < len(m.actionTypes) {
		b.WriteString(MutedStyle.Render(" >"))
	}

	return b.String()
}

// actionNeedsNoValue returns true if the action type doesn't require a value.
func (m *WorkflowRuleCreatorModel) actionNeedsNoValue(actionType workflow.ActionType) bool {
	switch actionType {
	case workflow.ActionClearDue, workflow.ActionCompleteParent, workflow.ActionBlockParent:
		return true
	}
	return false
}
