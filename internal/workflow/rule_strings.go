package workflow

import (
	"fmt"
	"strings"

	"github.com/sglavoie/tt/internal/models"
)

// String returns a human-readable description of the trigger.
func (t RuleTrigger) String() string {
	opStr := string(t.Operation)
	switch t.Operation {
	case OpEquals:
		opStr = "="
	case OpNotEquals:
		opStr = "!="
	case OpContains:
		opStr = "contains"
	case OpGreaterThan:
		opStr = ">"
	case OpLessThan:
		opStr = "<"
	case OpIn:
		opStr = "in"
	case OpNotIn:
		opStr = "not in"
	}

	return fmt.Sprintf("%s %s %s", t.Field, opStr, t.Value)
}

// String returns a human-readable description of the action.
func (a RuleAction) String() string {
	switch a.ActionType {
	case ActionSetState:
		return fmt.Sprintf("set state to %s", a.Value)
	case ActionSetStatus:
		return fmt.Sprintf("set status to %s", a.Value)
	case ActionSetPriority:
		return fmt.Sprintf("set priority to %s", a.Value)
	case ActionSetSection:
		if a.Value == "" {
			return "move to Active"
		}
		return fmt.Sprintf("move to %s", a.Value)
	case ActionAddTag:
		return fmt.Sprintf("add tag #%s", a.Value)
	case ActionRemoveTag:
		return fmt.Sprintf("remove tag #%s", a.Value)
	case ActionSetDueRelative:
		return fmt.Sprintf("set due date %s", a.Value)
	case ActionClearDue:
		return "clear due date"
	case ActionSetPinned:
		if a.Value == "true" {
			return "pin task"
		}
		return "unpin task"
	case ActionAddComment:
		return fmt.Sprintf("add comment: %s", a.Value)
	case ActionSetType:
		return fmt.Sprintf("set type to %s", a.Value)
	case ActionCompleteParent:
		return "mark parent as done"
	case ActionBlockParent:
		return "mark parent as blocked"
	case ActionCascadeTag:
		return fmt.Sprintf("add tag #%s to children", a.Value)
	case ActionInheritTag:
		return fmt.Sprintf("inherit tag #%s from parent", a.Value)
	case ActionNotifyParent:
		return fmt.Sprintf("notify parent: %s", a.Value)
	case ActionSetSnooze:
		return fmt.Sprintf("snooze for %s", a.Value)
	case ActionCreateFollowup:
		return fmt.Sprintf("create follow-up: %s", a.Value)
	case ActionMoveToProject:
		return fmt.Sprintf("move to project %s", a.Value)
	case ActionSetRecurrence:
		return fmt.Sprintf("set recurrence to %s", a.Value)
	case ActionSetProjectStatus:
		if icon := models.ProjectStatusIconByKey(a.Value); icon != nil {
			return fmt.Sprintf("set project status to %s %s", icon.DisplayIcon(), icon.Label)
		}
		return fmt.Sprintf("set project status to %s", a.Value)
	default:
		return fmt.Sprintf("%s: %s", a.ActionType, a.Value)
	}
}

// stringWithCondition returns the action string with condition suffix if present.
func (a RuleAction) stringWithCondition() string {
	s := a.String()
	if a.Condition != nil {
		s += fmt.Sprintf(" (if %s)", a.Condition.String())
	}
	return s
}

// Summary returns a human-readable expression for the trigger group.
func (g *TriggerGroup) Summary() string {
	logic := g.getLogic()
	var parts []string

	for _, trigger := range g.Triggers {
		parts = append(parts, trigger.String())
	}
	for _, sub := range g.Groups {
		parts = append(parts, "("+sub.Summary()+")")
	}

	joiner := " AND "
	if logic == LogicOr {
		joiner = " OR "
	}

	return strings.Join(parts, joiner)
}

// getLogic returns the trigger group's logic, defaulting to AND.
func (g *TriggerGroup) getLogic() TriggerLogic {
	if g.Logic == LogicOr {
		return LogicOr
	}
	return LogicAnd
}

// Summary returns a short description of the rule.
func (r *WorkflowRule) Summary() string {
	var actionStrs []string
	for _, action := range r.Actions {
		actionStrs = append(actionStrs, action.stringWithCondition())
	}

	var prefix string
	if r.Priority > 0 {
		prefix = fmt.Sprintf("[P%d] ", r.Priority)
	}

	if r.TriggerGroup != nil {
		return fmt.Sprintf("%sWhen (%s): %s", prefix, r.TriggerGroup.Summary(), strings.Join(actionStrs, ", "))
	}

	triggers := r.GetTriggers()
	if len(triggers) == 0 {
		return fmt.Sprintf("%s(no triggers): %s", prefix, strings.Join(actionStrs, ", "))
	}
	if len(triggers) == 1 {
		return fmt.Sprintf("%sWhen %s: %s", prefix, triggers[0].String(), strings.Join(actionStrs, ", "))
	}

	var triggerStrs []string
	for _, trigger := range triggers {
		triggerStrs = append(triggerStrs, trigger.String())
	}

	logic := " AND "
	if r.GetTriggerLogic() == LogicOr {
		logic = " OR "
	}

	return fmt.Sprintf("%sWhen (%s): %s", prefix, strings.Join(triggerStrs, logic), strings.Join(actionStrs, ", "))
}
