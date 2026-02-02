package tui

import (
	"fmt"
	"strings"

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

	// Triggers section
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

	// Actions section
	b.WriteString("\n")
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

	// Error display
	if m.err != nil {
		b.WriteString("\n")
		b.WriteString(ValidationErrorStyle.Render(m.err.Error()))
	}

	title := "Create Workflow Rule"
	if m.editingRule != nil {
		title = "Edit Workflow Rule"
	}

	left := HelpStyle.Render("Ctrl+f: fields | Ctrl+t: types | Tab: nav | +/-: add/remove | Ctrl+S: save | Esc: cancel")

	var out strings.Builder
	out.WriteString(renderFullscreenTitle(title, ""))
	out.WriteString("\n\n")
	out.WriteString(b.String())
	out.WriteString("\n")
	out.WriteString(renderStatusBarFooter(left, "", m.width))

	return out.String()
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
