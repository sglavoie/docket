package tui

import (
	"fmt"
	"strings"

	"github.com/sglavoie/tt/internal/workflow"
)

// renderEditorPane renders the editor content based on active tab.
func (m *WorkflowRuleCreatorModel) renderEditorPane() string {
	var b strings.Builder
	section, itemIdx, fieldIdx := m.getFocusSection()

	var nameLabel string
	if section == "name" {
		nameLabel = FieldLabelFocusedStyle.Render("Name: ")
	} else {
		nameLabel = FieldLabelStyle.Render("Name: ")
	}
	b.WriteString(nameLabel)
	b.WriteString(m.nameInput.View())
	b.WriteString("\n")

	var priorityLabel string
	if section == "priority" {
		priorityLabel = FieldLabelFocusedStyle.Render("Priority: ")
	} else {
		priorityLabel = FieldLabelStyle.Render("Priority: ")
	}
	b.WriteString(priorityLabel)
	b.WriteString(m.priorityInput.View())
	b.WriteString("\n\n")

	switch m.activeTab {
	case tabTriggers:
		b.WriteString(m.renderTriggersSection(section, itemIdx, fieldIdx))
	case tabActions:
		b.WriteString(m.renderActionsSection(section, itemIdx, fieldIdx))
	case tabScope:
		b.WriteString(m.renderScopeSection(section, fieldIdx))
	}

	if m.err != nil {
		b.WriteString("\n")
		b.WriteString(ValidationErrorStyle.Render(m.err.Error()))
	}

	return b.String()
}

// renderTriggersSection renders the triggers form fields.
func (m *WorkflowRuleCreatorModel) renderTriggersSection(section string, itemIdx, fieldIdx int) string {
	var b strings.Builder

	b.WriteString(BoldStyle.Render("Triggers"))
	if len(m.triggers) > 1 {
		b.WriteString(MutedStyle.Render(fmt.Sprintf(" (%d)", len(m.triggers))))
	}
	if len(m.triggerGroups) > 0 {
		b.WriteString(MutedStyle.Render(fmt.Sprintf(" [%d group(s)]", len(m.triggerGroups))))
	}
	b.WriteString("\n")

	for i, t := range m.triggers {
		isFocusedTrigger := section == "trigger" && itemIdx == i

		indent := "  "
		groupMarker := ""
		if t.groupIndex >= 0 {
			indent = "    "
			groupMarker = MutedStyle.Render(fmt.Sprintf("[G%d] ", t.groupIndex+1))
		}

		if len(m.triggers) > 1 {
			b.WriteString(MutedStyle.Render(fmt.Sprintf("%s%d. ", indent, i+1)))
		} else {
			b.WriteString(indent)
		}
		b.WriteString(groupMarker)

		fieldLabel := "Field: "
		if isFocusedTrigger && fieldIdx == 0 {
			fieldLabel = FieldLabelFocusedStyle.Render("Field: ")
		}
		b.WriteString(fieldLabel)
		b.WriteString(m.renderSelector(m.triggerFields, t.fieldCursor, isFocusedTrigger && fieldIdx == 0, 3))
		b.WriteString("\n")

		b.WriteString(indent)
		b.WriteString("   ")
		opLabel := "Op: "
		if isFocusedTrigger && fieldIdx == 1 {
			opLabel = FieldLabelFocusedStyle.Render("Op: ")
		}
		b.WriteString(opLabel)
		b.WriteString(m.renderOpsSelector(t.opCursor, isFocusedTrigger && fieldIdx == 1))
		b.WriteString("\n")

		b.WriteString(indent)
		b.WriteString("   ")
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

	for gi, g := range m.triggerGroups {
		b.WriteString("  ")
		b.WriteString(MutedStyle.Render(fmt.Sprintf("Group %d logic: ", gi+1)))
		if g.logic == workflow.LogicAnd {
			b.WriteString(BoldStyle.Render("[AND]"))
		} else {
			b.WriteString(BoldStyle.Render("[OR]"))
		}
		b.WriteString("\n")
	}

	return b.String()
}

// renderActionsSection renders the actions form fields.
func (m *WorkflowRuleCreatorModel) renderActionsSection(section string, itemIdx, fieldIdx int) string {
	var b strings.Builder

	b.WriteString(BoldStyle.Render("Actions"))
	if len(m.actions) > 1 {
		b.WriteString(MutedStyle.Render(fmt.Sprintf(" (%d)", len(m.actions))))
	}
	b.WriteString("\n")

	for i, a := range m.actions {
		isFocusedAction := section == "action" && itemIdx == i

		if len(m.actions) > 1 {
			b.WriteString(MutedStyle.Render(fmt.Sprintf("  %d. ", i+1)))
		} else {
			b.WriteString("  ")
		}

		typeLabel := "Type: "
		if isFocusedAction && fieldIdx == 0 {
			typeLabel = FieldLabelFocusedStyle.Render("Type: ")
		}
		b.WriteString(typeLabel)
		b.WriteString(m.renderActionSelector(a.typeCursor, isFocusedAction && fieldIdx == 0))
		b.WriteString("\n")

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

		if a.hasCondition {
			b.WriteString("     ")
			condFieldLabel := "IF Field: "
			if isFocusedAction && fieldIdx == 2 {
				condFieldLabel = FieldLabelFocusedStyle.Render("IF Field: ")
			}
			b.WriteString(condFieldLabel)
			b.WriteString(m.renderSelector(m.triggerFields, a.condFieldCursor, isFocusedAction && fieldIdx == 2, 3))
			b.WriteString("\n")

			b.WriteString("     ")
			condOpLabel := "IF Op: "
			if isFocusedAction && fieldIdx == 3 {
				condOpLabel = FieldLabelFocusedStyle.Render("IF Op: ")
			}
			b.WriteString(condOpLabel)
			b.WriteString(m.renderOpsSelector(a.condOpCursor, isFocusedAction && fieldIdx == 3))
			b.WriteString("\n")

			b.WriteString("     ")
			condValueLabel := "IF Value: "
			if isFocusedAction && fieldIdx == 4 {
				condValueLabel = FieldLabelFocusedStyle.Render("IF Value: ")
			}
			b.WriteString(condValueLabel)
			b.WriteString(a.condValueInput.View())
			b.WriteString("\n")
		}

		if i < len(m.actions)-1 {
			b.WriteString("\n")
		}
	}

	return b.String()
}

// renderScopeSection renders the scope form fields.
func (m *WorkflowRuleCreatorModel) renderScopeSection(section string, fieldIdx int) string {
	var b strings.Builder

	b.WriteString(BoldStyle.Render("Rule Scope"))
	b.WriteString("\n")
	b.WriteString(MutedStyle.Render("  Leave empty for no restriction"))
	b.WriteString("\n\n")

	projLabel := "  Projects: "
	if section == "scope" && fieldIdx == 0 {
		projLabel = "  " + FieldLabelFocusedStyle.Render("Projects: ")
	}
	b.WriteString(projLabel)
	b.WriteString(m.scopeProjectsInput.View())
	b.WriteString("\n\n")

	typesLabel := "  Types: "
	if section == "scope" && fieldIdx == 1 {
		typesLabel = "  " + FieldLabelFocusedStyle.Render("Types: ")
	}
	b.WriteString(typesLabel)
	b.WriteString(m.scopeTypesInput.View())
	b.WriteString("\n\n")

	sectLabel := "  States: "
	if section == "scope" && fieldIdx == 2 {
		sectLabel = "  " + FieldLabelFocusedStyle.Render("States: ")
	}
	b.WriteString(sectLabel)
	b.WriteString(m.scopeStatesInput.View())
	b.WriteString("\n")

	return b.String()
}
