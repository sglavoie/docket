package cmd

import (
	"fmt"
	"slices"
	"strings"

	"github.com/sglavoie/tt/internal/workflow"
)

// formatGitStyleDiff creates a git-style diff showing rule changes.
// Returns a formatted string with - for removed/old values and + for added/new values.
func formatGitStyleDiff(before, after *workflow.WorkflowRule) string {
	var lines []string

	// Name changes
	if before.Name != after.Name {
		lines = append(lines, fmt.Sprintf("- Name: %s", before.Name))
		lines = append(lines, fmt.Sprintf("+ Name: %s", after.Name))
	}

	// Enabled changes
	if before.Enabled != after.Enabled {
		lines = append(lines, fmt.Sprintf("- Enabled: %t", before.Enabled))
		lines = append(lines, fmt.Sprintf("+ Enabled: %t", after.Enabled))
	}

	// Trigger logic changes
	beforeLogic := before.GetTriggerLogic()
	afterLogic := after.GetTriggerLogic()
	if beforeLogic != afterLogic {
		lines = append(lines, fmt.Sprintf("- Trigger Logic: %s", beforeLogic))
		lines = append(lines, fmt.Sprintf("+ Trigger Logic: %s", afterLogic))
	}

	// Trigger changes — compare TriggerGroup summaries when present,
	// fall back to flat trigger comparison otherwise.
	beforeHasGroup := before.TriggerGroup != nil
	afterHasGroup := after.TriggerGroup != nil
	if beforeHasGroup || afterHasGroup {
		beforeSummary := ""
		afterSummary := ""
		if before.TriggerGroup != nil {
			beforeSummary = before.TriggerGroup.Summary()
		}
		if after.TriggerGroup != nil {
			afterSummary = after.TriggerGroup.Summary()
		}
		if beforeSummary != afterSummary {
			lines = append(lines, "")
			lines = append(lines, fmt.Sprintf("- Triggers: %s", beforeSummary))
			lines = append(lines, fmt.Sprintf("+ Triggers: %s", afterSummary))
		}
	} else {
		beforeTriggers := before.GetTriggers()
		afterTriggers := after.GetTriggers()
		if !triggersEqual(beforeTriggers, afterTriggers) {
			lines = append(lines, "")
			lines = append(lines, "- Triggers:")
			for i, t := range beforeTriggers {
				lines = append(lines, fmt.Sprintf("-   %d. %s", i, t.String()))
			}
			lines = append(lines, "+ Triggers:")
			for i, t := range afterTriggers {
				lines = append(lines, fmt.Sprintf("+   %d. %s", i, t.String()))
			}
		}
	}

	// Action changes
	if !actionsEqual(before.Actions, after.Actions) {
		lines = append(lines, "")
		lines = append(lines, "- Actions:")
		for i, a := range before.Actions {
			lines = append(lines, fmt.Sprintf("-   %d. %s", i, a.String()))
		}
		lines = append(lines, "+ Actions:")
		for i, a := range after.Actions {
			lines = append(lines, fmt.Sprintf("+   %d. %s", i, a.String()))
		}
	}

	// Scope changes
	if !scopesEqual(before.Scope, after.Scope) {
		lines = append(lines, "")
		if before.Scope != nil {
			lines = append(lines, fmt.Sprintf("- Scope: %s", formatScope(before.Scope)))
		} else {
			lines = append(lines, "- Scope: (none)")
		}
		if after.Scope != nil {
			lines = append(lines, fmt.Sprintf("+ Scope: %s", formatScope(after.Scope)))
		} else {
			lines = append(lines, "+ Scope: (none)")
		}
	}

	if len(lines) == 0 {
		return "(no changes)"
	}

	return strings.Join(lines, "\n")
}

// scopesEqual compares two RuleScope pointers for equality.
func scopesEqual(a, b *workflow.RuleScope) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return slices.Equal(a.Projects, b.Projects) &&
		slices.Equal(a.Types, b.Types) &&
		slices.Equal(a.Sections, b.Sections)
}

// formatScope returns a human-readable representation of a RuleScope.
func formatScope(s *workflow.RuleScope) string {
	if s == nil {
		return "all"
	}
	var parts []string
	if len(s.Projects) > 0 {
		parts = append(parts, fmt.Sprintf("projects=[%s]", strings.Join(s.Projects, ", ")))
	}
	if len(s.Types) > 0 {
		parts = append(parts, fmt.Sprintf("types=[%s]", strings.Join(s.Types, ", ")))
	}
	if len(s.Sections) > 0 {
		parts = append(parts, fmt.Sprintf("sections=[%s]", strings.Join(s.Sections, ", ")))
	}
	if len(parts) == 0 {
		return "all"
	}
	return strings.Join(parts, ", ")
}

// triggersEqual compares two trigger slices for equality.
func triggersEqual(a, b []workflow.RuleTrigger) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].Field != b[i].Field || a[i].Operation != b[i].Operation || a[i].Value != b[i].Value {
			return false
		}
	}
	return true
}

// actionsEqual compares two action slices for equality.
func actionsEqual(a, b []workflow.RuleAction) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].ActionType != b[i].ActionType || a[i].Value != b[i].Value {
			return false
		}
		// Compare conditions
		if (a[i].Condition == nil) != (b[i].Condition == nil) {
			return false
		}
		if a[i].Condition != nil && b[i].Condition != nil {
			if a[i].Condition.Field != b[i].Condition.Field ||
				a[i].Condition.Operation != b[i].Condition.Operation ||
				a[i].Condition.Value != b[i].Condition.Value {
				return false
			}
		}
	}
	return true
}
