package tui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/workflow"
)

// renderPreviewPane renders the rule preview on the right side.
func (m *WorkflowRuleCreatorModel) renderPreviewPane(width int) string {
	var b strings.Builder
	section, itemIdx, fieldIdx := m.getFocusSection()

	keywordStyle := RuleKeywordStyle
	operatorStyle := RuleOperatorStyle
	highlightStyle := RuleHighlightStyle

	b.WriteString(BoldStyle.Render("Preview"))
	b.WriteString("\n")

	if v := strings.TrimSpace(m.priorityInput.Value()); v != "" && v != "0" {
		if section == "priority" {
			b.WriteString(highlightStyle.Render(fmt.Sprintf("Priority: %s", v)))
		} else {
			b.WriteString(MutedStyle.Render(fmt.Sprintf("Priority: %s", v)))
		}
		b.WriteString("\n")
	}
	b.WriteString("\n")

	if len(m.triggerGroups) > 0 {
		m.renderGroupedTriggersPreview(&b, section, itemIdx, fieldIdx, keywordStyle, operatorStyle, highlightStyle)
	} else {
		for i, t := range m.triggers {
			prefix := keywordStyle.Render("IF") + " "
			if i > 0 {
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

			fieldStr, opStr, valueStr := m.renderFocusedTriggerPreviewParts(section, itemIdx, fieldIdx, i, field, op, value, operatorStyle, highlightStyle)
			fmt.Fprintf(&b, "%s%s %s %s\n", prefix, fieldStr, opStr, valueStr)
		}
	}

	for i, a := range m.actions {
		prefix := keywordStyle.Render("THEN") + " "
		if i > 0 {
			prefix = keywordStyle.Render("AND") + " "
		}

		actionType := string(m.actionTypes[a.typeCursor])
		value := a.valueInput.Value()
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

		if a.hasCondition {
			condField := string(m.triggerFields[a.condFieldCursor])
			condOp := m.getOpSymbol(m.triggerOps[a.condOpCursor])
			condValue := a.condValueInput.Value()
			if condValue == "" {
				condValue = "?"
			}
			line += fmt.Sprintf(" (%s %s %s %s)", keywordStyle.Render("IF"), condField, operatorStyle.Render(condOp), condValue)
		}

		b.WriteString(line)
		b.WriteString("\n")
	}

	scopeStr := m.buildScopePreviewString()
	if scopeStr != "" {
		b.WriteString("\n")
		b.WriteString(MutedStyle.Render("Scope: " + scopeStr))
		b.WriteString("\n")
	}

	return lipgloss.NewStyle().Width(width).Render(b.String())
}

func (m *WorkflowRuleCreatorModel) renderFocusedTriggerPreviewParts(section string, itemIdx, fieldIdx, triggerIdx int, field, op, value string, operatorStyle, highlightStyle lipgloss.Style) (string, string, string) {
	if section != "trigger" || itemIdx != triggerIdx {
		return field, operatorStyle.Render(op), value
	}

	switch fieldIdx {
	case 0:
		return highlightStyle.Render(field), operatorStyle.Render(op), value
	case 1:
		return field, highlightStyle.Render(op), value
	case 2:
		return field, operatorStyle.Render(op), highlightStyle.Render(value)
	default:
		return field, operatorStyle.Render(op), value
	}
}

// renderGroupedTriggersPreview renders the preview for triggers that include sub-groups.
func (m *WorkflowRuleCreatorModel) renderGroupedTriggersPreview(b *strings.Builder, section string, itemIdx, fieldIdx int, keywordStyle, operatorStyle, highlightStyle lipgloss.Style) {
	type indexedTrigger struct {
		index   int
		trigger TriggerFormState
	}

	var ungrouped []indexedTrigger
	grouped := make(map[int][]indexedTrigger)

	for i, t := range m.triggers {
		if t.groupIndex < 0 {
			ungrouped = append(ungrouped, indexedTrigger{index: i, trigger: t})
			continue
		}
		grouped[t.groupIndex] = append(grouped[t.groupIndex], indexedTrigger{index: i, trigger: t})
	}

	first := true
	writePrefix := func() {
		if first {
			b.WriteString(keywordStyle.Render("IF") + " ")
			first = false
			return
		}
		if m.triggerLogic == workflow.LogicOr {
			b.WriteString(keywordStyle.Render("OR") + " ")
			return
		}
		b.WriteString(keywordStyle.Render("AND") + " ")
	}

	renderTrigger := func(it indexedTrigger) string {
		field := string(m.triggerFields[it.trigger.fieldCursor])
		op := m.getOpSymbol(m.triggerOps[it.trigger.opCursor])
		value := it.trigger.valueInput.Value()
		if value == "" {
			value = "?"
		}
		fieldStr, opStr, valueStr := m.renderFocusedTriggerPreviewParts(section, itemIdx, fieldIdx, it.index, field, op, value, operatorStyle, highlightStyle)
		return fmt.Sprintf("%s %s %s", fieldStr, opStr, valueStr)
	}

	for _, it := range ungrouped {
		writePrefix()
		b.WriteString(renderTrigger(it))
		b.WriteString("\n")
	}

	for gi := 0; gi < len(m.triggerGroups); gi++ {
		members := grouped[gi]
		if len(members) == 0 {
			continue
		}
		writePrefix()
		b.WriteString("( ")
		for j, it := range members {
			if j > 0 {
				if m.triggerGroups[gi].logic == workflow.LogicOr {
					b.WriteString(" " + keywordStyle.Render("OR") + " ")
				} else {
					b.WriteString(" " + keywordStyle.Render("AND") + " ")
				}
			}
			b.WriteString(renderTrigger(it))
		}
		b.WriteString(" )\n")
	}
}

// buildScopePreviewString returns a summary of scope settings for preview display.
func (m *WorkflowRuleCreatorModel) buildScopePreviewString() string {
	var parts []string
	if v := strings.TrimSpace(m.scopeProjectsInput.Value()); v != "" {
		parts = append(parts, "projects=["+v+"]")
	}
	if v := strings.TrimSpace(m.scopeTypesInput.Value()); v != "" {
		parts = append(parts, "types=["+v+"]")
	}
	if v := strings.TrimSpace(m.scopeStatesInput.Value()); v != "" {
		parts = append(parts, "states=["+v+"]")
	}
	return strings.Join(parts, ", ")
}

// renderFullscreenPreview renders the preview as a fullscreen view for narrow screens.
func (m *WorkflowRuleCreatorModel) renderFullscreenPreview() string {
	preview := m.renderPreviewPane(m.width - 4)
	left := HelpStyle.Render("Ctrl+P: back to editor · Esc: cancel")

	var out strings.Builder
	out.WriteString(renderFullscreenTitle("Rule Preview", ""))
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

	start := max(cursor-visible/2, 0)
	end := start + visible
	if end > len(fields) {
		end = len(fields)
		start = max(end-visible, 0)
	}

	if start > 0 {
		b.WriteString(MutedStyle.Render("< "))
	} else {
		b.WriteString("  ")
	}

	for i := start; i < end; i++ {
		display := truncateToVisualWidth(string(fields[i]), 18)
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

	visible := 3
	start := max(cursor-visible/2, 0)
	end := start + visible
	if end > len(m.actionTypes) {
		end = len(m.actionTypes)
		start = max(end-visible, 0)
	}

	if start > 0 {
		b.WriteString(MutedStyle.Render("< "))
	} else {
		b.WriteString("  ")
	}

	for i := start; i < end; i++ {
		display := truncateToVisualWidth(string(m.actionTypes[i]), 16)
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
	default:
		return false
	}
}
