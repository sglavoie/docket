package workflow

import (
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
	if len(triggers) == 0 {
		return false
	}

	logic := rule.GetTriggerLogic()

	if logic == LogicOr {
		for _, trigger := range triggers {
			if matchesTriggerWithContext(task, prev, trigger, evalCtx) {
				return true
			}
		}
		return false
	}

	for _, trigger := range triggers {
		if !matchesTriggerWithContext(task, prev, trigger, evalCtx) {
			return false
		}
	}
	return true
}

// matchesTriggerGroup recursively evaluates a nested trigger group against a task.
func matchesTriggerGroup(task, prev *models.Task, group *TriggerGroup, evalCtx *EvaluationContext) bool {
	logic := group.getLogic()

	var results []bool
	for _, trigger := range group.Triggers {
		results = append(results, matchesTriggerWithContext(task, prev, trigger, evalCtx))
	}
	for _, sub := range group.Groups {
		results = append(results, matchesTriggerGroup(task, prev, &sub, evalCtx))
	}

	if len(results) == 0 {
		return false
	}

	if logic == LogicOr {
		for _, r := range results {
			if r {
				return true
			}
		}
		return false
	}

	for _, r := range results {
		if !r {
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
func matchesEventTrigger(task, prev *models.Task, trigger RuleTrigger) bool {
	switch trigger.Field {
	case FieldStatusChanged:
		return matchesTransition(
			getPrevValue(prev, func(t *models.Task) string { return string(t.Status) }),
			string(task.Status),
			trigger.Value,
		)

	case FieldSectionChanged:
		return matchesTransition(
			getPrevValue(prev, func(t *models.Task) string { return t.Section }),
			task.Section,
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
		return compareBool(hasChildren, trigger.Value)

	case FieldChildrenProgress:
		progress := int(task.GetProgress())
		return compareValues(strconv.Itoa(progress), trigger.Operation, trigger.Value)

	case FieldAllChildrenDone:
		if len(task.Children) == 0 {
			return false
		}
		allDone := task.GetProgress() == 100
		return compareBool(allDone, trigger.Value)

	case FieldIsBlocked:
		blocked := hasIncompleteBlockers(task, evalCtx.TaskMap)
		return compareBool(blocked, trigger.Value)

	case FieldBlockerCount:
		count := countIncompleteBlockers(task, evalCtx.TaskMap)
		return compareValues(strconv.Itoa(count), trigger.Operation, trigger.Value)

	case FieldDependencyCount:
		count := len(task.DependsOn)
		return compareValues(strconv.Itoa(count), trigger.Operation, trigger.Value)
	}

	return false
}

// compareBool compares a boolean value against a trigger value ("true" or "false").
func compareBool(value bool, triggerValue string) bool {
	expected := strings.EqualFold(triggerValue, "true")
	return value == expected
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

// matchesRule checks if a task matches all of the rule's trigger conditions.
// Handles both single trigger (legacy) and compound triggers.
func matchesRule(task *models.Task, rule *WorkflowRule) bool {
	triggers := rule.GetTriggers()
	if len(triggers) == 0 {
		return false
	}

	logic := rule.GetTriggerLogic()

	if logic == LogicOr {
		for _, trigger := range triggers {
			if matchesTrigger(task, trigger) {
				return true
			}
		}
		return false
	}

	for _, trigger := range triggers {
		if !matchesTrigger(task, trigger) {
			return false
		}
	}
	return true
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
		fieldValue = task.Section
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
		return compareBool(task.DueDate != nil, trigger.Value)
	case FieldHasTags:
		return compareBool(len(task.Tags) > 0, trigger.Value)
	case FieldDescriptionEmpty:
		return compareBool(strings.TrimSpace(task.Description) == "", trigger.Value)
	default:
		return false
	}

	return compareValues(fieldValue, trigger.Operation, trigger.Value)
}

func matchesTagTrigger(tags []string, trigger RuleTrigger) bool {
	switch trigger.Operation {
	case OpEquals, OpContains:
		for _, tag := range tags {
			if strings.EqualFold(tag, trigger.Value) {
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
