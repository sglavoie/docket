package tui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/workflow"
)

// renderPreviewPane renders the rule preview with syntax highlighting.
func (m *WorkflowRulesModel) renderPreviewPane(width int) string {
	if m.cursor >= len(m.filteredRules) {
		return ""
	}

	rule := m.filteredRules[m.cursor]
	isDefault := m.isDefaultRule(rule.ID)

	var b strings.Builder
	b.WriteString(BoldStyle.Render("Preview"))
	if rule.IsProjectLevel() {
		b.WriteString(" ")
		b.WriteString(MutedStyle.Render("[Project]"))
	}
	if isDefault {
		b.WriteString(" ")
		b.WriteString(MutedStyle.Render("[Default]"))
	}
	b.WriteString("\n\n")
	b.WriteString(m.renderRuleDetails(rule, false, false))

	return lipgloss.NewStyle().Width(width).Render(b.String())
}

// renderFullscreenPreview renders the preview as a fullscreen view for narrow screens.
func (m *WorkflowRulesModel) renderFullscreenPreview() string {
	rule := m.filteredRules[m.cursor]
	isDefault := m.isDefaultRule(rule.ID)

	var b strings.Builder
	b.WriteString(renderFullscreenTitle("Rule Preview", ""))
	b.WriteString("\n\n")
	b.WriteString(m.renderRuleDetails(rule, isDefault, true))
	b.WriteString("\n")
	b.WriteString(renderStatusBarFooter(HelpStyle.Render("Enter/Esc: close · e: edit · Space: toggle · d: delete"), "", m.Width()))

	return b.String()
}

func (m *WorkflowRulesModel) renderRuleDetails(rule workflow.WorkflowRule, isDefault, includeFooterSpacing bool) string {
	var b strings.Builder
	keywordStyle := RuleKeywordStyle
	operatorStyle := RuleOperatorStyle

	b.WriteString(BoldPrimaryStyle.Render(rule.Name))
	if rule.IsProjectLevel() {
		b.WriteString(" ")
		b.WriteString(MutedStyle.Render("[Project]"))
	}
	if isDefault {
		b.WriteString(" ")
		b.WriteString(MutedStyle.Render("[Default]"))
	}
	b.WriteString("\n\n")

	b.WriteString(m.renderRuleStatus(rule))
	b.WriteString("\n\n")
	m.renderRuleTriggersPreview(&b, rule, keywordStyle, operatorStyle)
	m.renderRuleActionsPreview(&b, rule, keywordStyle, operatorStyle)

	if rule.Scope != nil {
		b.WriteString("\n")
		b.WriteString(MutedStyle.Render("Scope: " + formatRuleScope(rule.Scope)))
	}
	if rule.TriggerCount > 0 {
		b.WriteString("\n")
		b.WriteString(MutedStyle.Render(fmt.Sprintf("Triggered %d times", rule.TriggerCount)))
	}
	if includeFooterSpacing {
		b.WriteString("\n")
	}

	return b.String()
}

func (m *WorkflowRulesModel) renderRuleStatus(rule workflow.WorkflowRule) string {
	var parts []string
	if rule.Enabled {
		parts = append(parts, DoneStyle.Render("● Enabled"))
	} else {
		parts = append(parts, MutedStyle.Render("○ Disabled"))
	}
	if rule.Priority > 0 {
		parts = append(parts, MutedStyle.Render(fmt.Sprintf("Priority: %d", rule.Priority)))
	}
	return strings.Join(parts, "  ")
}

func (m *WorkflowRulesModel) renderRuleActionsPreview(b *strings.Builder, rule workflow.WorkflowRule, keywordStyle, operatorStyle lipgloss.Style) {
	for i, action := range rule.Actions {
		prefix := keywordStyle.Render("AND") + " "
		if i == 0 {
			prefix = keywordStyle.Render("THEN") + " "
		}

		line := fmt.Sprintf("%s%s", prefix, string(action.ActionType))
		if action.Value != "" {
			line = fmt.Sprintf("%s%s%s %s", prefix, string(action.ActionType), operatorStyle.Render(":"), action.Value)
		}
		if action.Condition != nil {
			condOp := m.getOpSymbol(action.Condition.Operation)
			line += fmt.Sprintf(" (%s %s %s %s)", keywordStyle.Render("IF"), string(action.Condition.Field), operatorStyle.Render(condOp), action.Condition.Value)
		}

		b.WriteString(line)
		b.WriteString("\n")
	}
}

// renderRuleTriggersPreview renders triggers for a rule in the preview pane,
// handling both flat triggers and nested TriggerGroup.
func (m *WorkflowRulesModel) renderRuleTriggersPreview(b *strings.Builder, rule workflow.WorkflowRule, keywordStyle, operatorStyle lipgloss.Style) {
	if rule.TriggerGroup != nil {
		m.renderTriggerGroupPreview(b, rule.TriggerGroup, true, keywordStyle, operatorStyle)
		return
	}

	triggers := rule.GetTriggers()
	for i, trigger := range triggers {
		prefix := keywordStyle.Render("OR") + " "
		if i == 0 {
			prefix = keywordStyle.Render("IF") + " "
		} else if rule.GetTriggerLogic() == workflow.LogicAnd {
			prefix = keywordStyle.Render("AND") + " "
		}

		value := trigger.Value
		if value == "" {
			value = "?"
		}

		fmt.Fprintf(b, "%s%s %s %s\n", prefix, string(trigger.Field), operatorStyle.Render(m.getOpSymbol(trigger.Operation)), value)
	}
}

// renderTriggerGroupPreview recursively renders a TriggerGroup for preview.
func (m *WorkflowRulesModel) renderTriggerGroupPreview(b *strings.Builder, group *workflow.TriggerGroup, isRoot bool, keywordStyle, operatorStyle lipgloss.Style) {
	logic := group.Logic
	if logic == "" {
		logic = workflow.LogicAnd
	}

	first := true
	writePrefix := func() {
		if first {
			if isRoot {
				b.WriteString(keywordStyle.Render("IF") + " ")
			}
			first = false
			return
		}
		if logic == workflow.LogicOr {
			b.WriteString(keywordStyle.Render("OR") + " ")
			return
		}
		b.WriteString(keywordStyle.Render("AND") + " ")
	}

	for _, trigger := range group.Triggers {
		writePrefix()
		value := trigger.Value
		if value == "" {
			value = "?"
		}
		fmt.Fprintf(b, "%s %s %s\n", string(trigger.Field), operatorStyle.Render(m.getOpSymbol(trigger.Operation)), value)
	}

	for i := range group.Groups {
		writePrefix()
		b.WriteString("( ")
		sub := &group.Groups[i]
		subLogic := sub.Logic
		if subLogic == "" {
			subLogic = workflow.LogicAnd
		}
		for j, trigger := range sub.Triggers {
			if j > 0 {
				if subLogic == workflow.LogicOr {
					b.WriteString(" " + keywordStyle.Render("OR") + " ")
				} else {
					b.WriteString(" " + keywordStyle.Render("AND") + " ")
				}
			}
			value := trigger.Value
			if value == "" {
				value = "?"
			}
			fmt.Fprintf(b, "%s %s %s", string(trigger.Field), operatorStyle.Render(m.getOpSymbol(trigger.Operation)), value)
		}
		b.WriteString(" )\n")
	}
}

// getOpSymbol returns a symbol for the operation.
func (m *WorkflowRulesModel) getOpSymbol(op workflow.TriggerOperation) string {
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
