package workflow

import (
	"slices"
	"strconv"
	"strings"

	"github.com/sglavoie/tt/internal/models"
)

// matchesRuleWithContext checks if a task matches all trigger conditions with full context.
func matchesRuleWithContext(task, prev *models.Task, rule *WorkflowRule, evalCtx *EvaluationContext) bool {
	if rule.TriggerGroup != nil {
		return matchesTriggerGroup(task, prev, rule.TriggerGroup, evalCtx)
	}

	triggers := rule.GetTriggers()
	return matchesTriggers(triggers, rule.GetTriggerLogic(), func(trigger RuleTrigger) bool {
		return matchesTriggerWithContext(task, prev, trigger, evalCtx)
	})
}

// matchesTriggerGroup recursively evaluates a nested trigger group against a task.
func matchesTriggerGroup(task, prev *models.Task, group *TriggerGroup, evalCtx *EvaluationContext) bool {
	logic := group.getLogic()
	hasClauses := len(group.Triggers) > 0 || len(group.Groups) > 0
	if !hasClauses {
		return false
	}

	if logic == LogicOr {
		for _, trigger := range group.Triggers {
			if matchesTriggerWithContext(task, prev, trigger, evalCtx) {
				return true
			}
		}
		for _, sub := range group.Groups {
			if matchesTriggerGroup(task, prev, &sub, evalCtx) {
				return true
			}
		}
		return false
	}

	for _, trigger := range group.Triggers {
		if !matchesTriggerWithContext(task, prev, trigger, evalCtx) {
			return false
		}
	}
	for _, sub := range group.Groups {
		if !matchesTriggerGroup(task, prev, &sub, evalCtx) {
			return false
		}
	}

	return true
}

// matchesTriggerWithContext checks a trigger with full context for events and hierarchy.
func matchesTriggerWithContext(task, prev *models.Task, trigger RuleTrigger, evalCtx *EvaluationContext) bool {
	if IsEventTrigger(trigger.Field) {
		return matchesEventTrigger(task, prev, trigger)
	}

	if IsHierarchicalTrigger(trigger.Field) {
		return matchesHierarchicalTrigger(task, trigger, evalCtx)
	}

	return matchesTrigger(task, trigger)
}

// matchesEventTrigger checks event-based triggers that detect transitions.
func matchesEventTrigger(task, prev *models.Task, trigger RuleTrigger) bool { //nolint:gocyclo
	switch trigger.Field {
	case FieldStatusChanged:
		return matchesTransition(
			getPrevValue(prev, func(t *models.Task) string { return string(t.Status) }),
			string(task.Status),
			trigger.Value,
		)

	case FieldSectionChanged:
		// Use the effective section (CustomSection if set, otherwise Section)
		// so that internal field clearing (e.g. Section="" when CustomSection
		// is set) is not mistaken for a transition to the Active view.
		return matchesTransition(
			getPrevValue(prev, effectiveSection),
			effectiveSection(task),
			trigger.Value,
		)

	case FieldPriorityChanged:
		return matchesTransition(
			getPrevValue(prev, func(t *models.Task) string { return strconv.Itoa(t.Priority) }),
			strconv.Itoa(task.Priority),
			trigger.Value,
		)

	case FieldTagAdded:
		hadTag := prev != nil && hasTag(prev.Tags, trigger.Value)
		hasTagNow := hasTag(task.Tags, trigger.Value)
		return !hadTag && hasTagNow

	case FieldTagRemoved:
		hadTag := prev != nil && hasTag(prev.Tags, trigger.Value)
		hasTagNow := hasTag(task.Tags, trigger.Value)
		return hadTag && !hasTagNow

	case FieldCompleted:
		if trigger.Value != "true" {
			return false
		}
		wasComplete := prev != nil && prev.Status.IsComplete()
		isComplete := task.Status.IsComplete()
		return !wasComplete && isComplete

	case FieldReopened:
		if trigger.Value != "true" {
			return false
		}
		wasComplete := prev != nil && prev.Status.IsComplete()
		isComplete := task.Status.IsComplete()
		return wasComplete && !isComplete

	case FieldDueDateChanged:
		prevDue := prev != nil && prev.DueDate != nil
		curDue := task.DueDate != nil
		if !prevDue && !curDue {
			return false
		}
		switch {
		case !prevDue && curDue:
			return trigger.Value == "added" || trigger.Value == "*"
		case prevDue && !curDue:
			return trigger.Value == "removed" || trigger.Value == "*"
		case prevDue && curDue:
			if prev.DueDate.Equal(*task.DueDate) {
				return false
			}
			return trigger.Value == "changed" || trigger.Value == "*"
		}
		return false

	case FieldTypeChanged:
		return matchesTransition(
			getPrevValue(prev, func(t *models.Task) string { return string(t.Type) }),
			string(task.Type),
			trigger.Value,
		)

	case FieldRecurrenceCompleted:
		if trigger.Value != "true" {
			return false
		}
		hadRecurrence := prev != nil && prev.Recurrence != nil
		hasRecurrence := task.Recurrence != nil
		isComplete := task.Status.IsComplete()
		return hadRecurrence && !hasRecurrence && isComplete

	case FieldStreakBroken:
		if trigger.Value != "true" {
			return false
		}
		hadStreak := prev != nil && prev.Recurrence != nil && prev.Recurrence.Streak > 0
		hasStreak := task.Recurrence != nil && task.Recurrence.Streak > 0
		return hadStreak && !hasStreak
	default:
	}

	return false
}

// matchesTransition checks if a field transition matches the pattern "old->new".
func matchesTransition(oldVal, newVal, pattern string) bool {
	if oldVal == newVal {
		return false
	}

	parts := strings.Split(pattern, "->")
	if len(parts) != 2 {
		return false
	}

	oldPattern := strings.TrimSpace(parts[0])
	newPattern := strings.TrimSpace(parts[1])

	return matchesPattern(oldVal, oldPattern) && matchesPattern(newVal, newPattern)
}

// matchesPattern checks if a value matches a pattern (* matches anything).
func matchesPattern(value, pattern string) bool {
	if pattern == "*" {
		return true
	}
	return strings.EqualFold(value, pattern)
}

// effectiveSection returns the logical section for a task: CustomSection if
// set, otherwise Section. This avoids confusing the internal empty-Section
// representation (used when CustomSection is active) with the Active view.
func effectiveSection(t *models.Task) string {
	if t.CustomSection != "" {
		return t.CustomSection
	}
	return t.Section
}

// getPrevValue safely gets a value from the previous task state.
func getPrevValue(prev *models.Task, getter func(*models.Task) string) string {
	if prev == nil {
		return ""
	}
	return getter(prev)
}

// hasTag checks if a tag slice contains a tag (case-insensitive).
func hasTag(tags []string, tag string) bool {
	for _, t := range tags {
		if strings.EqualFold(t, tag) {
			return true
		}
	}
	return false
}

// matchesHierarchicalTrigger checks triggers that depend on parent/child relationships.
func matchesHierarchicalTrigger(task *models.Task, trigger RuleTrigger, evalCtx *EvaluationContext) bool {
	// Build a task map on the fly when evalCtx is nil (e.g., matchesRule /
	// TestRule calls that lack full evaluation context).  Without this,
	// FieldIsBlocked / FieldBlockerCount would nil-deref on evalCtx.TaskMap.
	taskMap := func() map[string]*models.Task {
		if evalCtx != nil {
			return evalCtx.TaskMap
		}
		return nil
	}()

	switch trigger.Field {
	case FieldParentStatus:
		if task.Parent == nil {
			return false
		}
		return compareValues(string(task.Parent.Status), trigger.Operation, trigger.Value)

	case FieldParentType:
		if task.Parent == nil {
			return false
		}
		return compareValues(string(task.Parent.Type), trigger.Operation, trigger.Value)

	case FieldParentTags:
		if task.Parent == nil {
			return false
		}
		return matchesTagTrigger(task.Parent.Tags, trigger)

	case FieldLevel:
		return compareValues(strconv.Itoa(task.Level), trigger.Operation, trigger.Value)

	case FieldHasChildren:
		hasChildren := len(task.Children) > 0
		return compareBoolOp(hasChildren, trigger.Operation, trigger.Value)

	case FieldChildrenProgress:
		progress := int(task.GetProgress())
		return compareValues(strconv.Itoa(progress), trigger.Operation, trigger.Value)

	case FieldAllChildrenDone:
		if len(task.Children) == 0 {
			return false
		}
		allDone := task.GetProgress() == 100
		return compareBoolOp(allDone, trigger.Operation, trigger.Value)

	case FieldIsBlocked:
		blocked := hasIncompleteBlockers(task, taskMap)
		return compareBoolOp(blocked, trigger.Operation, trigger.Value)

	case FieldBlockerCount:
		count := countIncompleteBlockers(task, taskMap)
		return compareValues(strconv.Itoa(count), trigger.Operation, trigger.Value)

	case FieldDependencyCount:
		count := len(task.DependsOn)
		return compareValues(strconv.Itoa(count), trigger.Operation, trigger.Value)
	default:
	}

	return false
}

// compareBool compares a boolean value against a trigger value ("true" or "false").
// Uses OpEquals semantics; callers that need to honor OpNotEquals must use
// compareBoolOp instead.
func compareBool(value bool, triggerValue string) bool {
	expected := strings.EqualFold(triggerValue, "true")
	return value == expected
}

// compareBoolOp compares a boolean value against a trigger value, honoring
// the OpEquals/OpNotEquals operation. Without this, "has_due_date != true"
// was silently treated as "has_due_date == true" because compareBool ignored
// the operation. Any operation other than OpNotEquals falls back to OpEquals
// semantics so misconfigured rules still evaluate.
func compareBoolOp(value bool, op TriggerOperation, triggerValue string) bool {
	matches := compareBool(value, triggerValue)
	if op == OpNotEquals {
		return !matches
	}
	return matches
}

// hasIncompleteBlockers checks if the task has any incomplete dependencies.
func hasIncompleteBlockers(task *models.Task, taskMap map[string]*models.Task) bool {
	for _, blockerID := range task.DependsOn {
		if blocker, ok := taskMap[blockerID]; ok {
			if !blocker.IsComplete() {
				return true
			}
		}
	}
	return false
}

// countIncompleteBlockers counts incomplete dependencies for a task.
func countIncompleteBlockers(task *models.Task, taskMap map[string]*models.Task) int {
	count := 0
	for _, blockerID := range task.DependsOn {
		if blocker, ok := taskMap[blockerID]; ok {
			if !blocker.IsComplete() {
				count++
			}
		}
	}
	return count
}

func matchesTriggers(triggers []RuleTrigger, logic TriggerLogic, matcher func(RuleTrigger) bool) bool {
	if len(triggers) == 0 {
		return false
	}

	if logic == LogicOr {
		return slices.ContainsFunc(triggers, matcher)
	}

	for _, trigger := range triggers {
		if !matcher(trigger) {
			return false
		}
	}

	return true
}

// matchesRule checks if a task matches all of the rule's trigger conditions.
// Handles single trigger (legacy), compound triggers, and nested TriggerGroup.
func matchesRule(task *models.Task, rule *WorkflowRule) bool {
	if rule.TriggerGroup != nil {
		return matchesTriggerGroup(task, nil, rule.TriggerGroup, nil)
	}

	triggers := rule.GetTriggers()
	return matchesTriggers(triggers, rule.GetTriggerLogic(), func(trigger RuleTrigger) bool {
		return matchesTrigger(task, trigger)
	})
}

// matchesTrigger checks if a task matches the rule's trigger condition.
func matchesTrigger(task *models.Task, trigger RuleTrigger) bool {
	var fieldValue string

	switch trigger.Field {
	case FieldStatus:
		fieldValue = string(task.Status)
	case FieldPriority:
		fieldValue = strconv.Itoa(task.Priority)
	case FieldSection:
		// Use the effective section (CustomSection if set, otherwise Section)
		// so that all operations (equals, not_equals, contains, etc.) compare
		// against the logical section the task belongs to.
		fieldValue = effectiveSection(task)
	case FieldTags:
		return matchesTagTrigger(task.Tags, trigger)
	case FieldType:
		fieldValue = string(task.Type)
	case FieldDueDays:
		if task.DueDate == nil {
			return false
		}
		days := daysUntil(*task.DueDate)
		fieldValue = strconv.Itoa(days)
	case FieldCreatedDays:
		if task.CreatedAt.IsZero() {
			return false
		}
		days := daysSince(task.CreatedAt)
		fieldValue = strconv.Itoa(days)
	case FieldUpdatedDays:
		if task.UpdatedAt.IsZero() {
			return false
		}
		days := daysSince(task.UpdatedAt)
		fieldValue = strconv.Itoa(days)
	case FieldHasDueDate:
		return compareBoolOp(task.DueDate != nil, trigger.Operation, trigger.Value)
	case FieldHasTags:
		return compareBoolOp(len(task.Tags) > 0, trigger.Operation, trigger.Value)
	case FieldDescriptionEmpty:
		return compareBoolOp(strings.TrimSpace(task.Description) == "", trigger.Operation, trigger.Value)
	default:
		return false
	}

	return compareValues(fieldValue, trigger.Operation, trigger.Value)
}

func matchesTagTrigger(tags []string, trigger RuleTrigger) bool {
	switch trigger.Operation {
	case OpEquals:
		// Exact membership: tag set contains a tag equal to the value.
		for _, tag := range tags {
			if strings.EqualFold(tag, trigger.Value) {
				return true
			}
		}
		return false
	case OpContains:
		// Substring membership: any tag contains the value as a substring.
		// This differs from OpEquals (exact match) and allows partial matches
		// such as "back" matching the tag "backend".
		needle := strings.ToLower(trigger.Value)
		if needle == "" {
			return false
		}
		for _, tag := range tags {
			if strings.Contains(strings.ToLower(tag), needle) {
				return true
			}
		}
		return false
	case OpNotEquals:
		for _, tag := range tags {
			if strings.EqualFold(tag, trigger.Value) {
				return false
			}
		}
		return true
	default:
		return false
	}
}

func compareValues(fieldValue string, op TriggerOperation, triggerValue string) bool {
	switch op {
	case OpEquals:
		return strings.EqualFold(fieldValue, triggerValue)
	case OpNotEquals:
		return !strings.EqualFold(fieldValue, triggerValue)
	case OpContains:
		return strings.Contains(strings.ToLower(fieldValue), strings.ToLower(triggerValue))
	case OpGreaterThan:
		fv, err1 := strconv.Atoi(fieldValue)
		tv, err2 := strconv.Atoi(triggerValue)
		if err1 != nil || err2 != nil {
			return fieldValue > triggerValue
		}
		return fv > tv
	case OpLessThan:
		fv, err1 := strconv.Atoi(fieldValue)
		tv, err2 := strconv.Atoi(triggerValue)
		if err1 != nil || err2 != nil {
			return fieldValue < triggerValue
		}
		return fv < tv
	default:
		return false
	}
}
