package workflow

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/sglavoie/tt/internal/models"
)

const maxRuleDepth = 10

// RuleConflict represents a conflict when multiple rules try to set the same field.
type RuleConflict struct {
	TaskID    string
	TaskTitle string
	Field     string
	Rules     []string // Rule names that conflict
}

// EvaluationResult contains the result of rule evaluation.
type EvaluationResult struct {
	ModifiedTasks []*models.Task
	Conflicts     []RuleConflict
	AppliedRules  map[string][]string // taskID -> list of applied rule names
}

// executionContext tracks rule applications to prevent infinite loops.
type executionContext struct {
	applied map[string]map[string]bool // taskID -> ruleID -> applied
	depth   int
}

func newExecutionContext() *executionContext {
	return &executionContext{
		applied: make(map[string]map[string]bool),
		depth:   0,
	}
}

func (ctx *executionContext) markApplied(taskID, ruleID string) {
	if ctx.applied[taskID] == nil {
		ctx.applied[taskID] = make(map[string]bool)
	}
	ctx.applied[taskID][ruleID] = true
}

func (ctx *executionContext) wasApplied(taskID, ruleID string) bool {
	if ctx.applied[taskID] == nil {
		return false
	}
	return ctx.applied[taskID][ruleID]
}

// EvaluateRules evaluates all enabled rules against the given tasks.
// Returns which tasks were modified and any conflicts detected.
func EvaluateRules(tasks []*models.Task, rules []WorkflowRule) *EvaluationResult {
	result := &EvaluationResult{
		ModifiedTasks: []*models.Task{},
		Conflicts:     []RuleConflict{},
		AppliedRules:  make(map[string][]string),
	}

	if len(rules) == 0 {
		return result
	}

	ctx := newExecutionContext()
	evaluateRulesRecursive(tasks, rules, ctx, result)
	return result
}

func evaluateRulesRecursive(tasks []*models.Task, rules []WorkflowRule, ctx *executionContext, result *EvaluationResult) {
	if ctx.depth >= maxRuleDepth {
		return
	}
	ctx.depth++
	defer func() { ctx.depth-- }()

	for _, task := range tasks {
		// Track which fields are being set by which rules for conflict detection
		fieldSetters := make(map[string][]string) // field -> rule names

		for _, rule := range rules {
			if !rule.Enabled {
				continue
			}
			if ctx.wasApplied(task.ID, rule.ID) {
				continue
			}
			if !matchesTrigger(task, rule.Trigger) {
				continue
			}

			// Check for conflicts before applying
			for _, action := range rule.Actions {
				field := getFieldFromAction(action.ActionType)
				if field != "" {
					fieldSetters[field] = append(fieldSetters[field], rule.Name)
				}
			}
		}

		// Detect conflicts (multiple rules setting same field)
		for field, ruleNames := range fieldSetters {
			if len(ruleNames) > 1 {
				result.Conflicts = append(result.Conflicts, RuleConflict{
					TaskID:    task.ID,
					TaskTitle: task.Title,
					Field:     field,
					Rules:     ruleNames,
				})
			}
		}

		// If there are conflicts, skip this task
		hasConflict := false
		for field := range fieldSetters {
			if len(fieldSetters[field]) > 1 {
				hasConflict = true
				break
			}
		}
		if hasConflict {
			continue
		}

		// Apply rules that matched
		modified := false
		for _, rule := range rules {
			if !rule.Enabled {
				continue
			}
			if ctx.wasApplied(task.ID, rule.ID) {
				continue
			}
			if !matchesTrigger(task, rule.Trigger) {
				continue
			}

			ctx.markApplied(task.ID, rule.ID)
			for _, action := range rule.Actions {
				if applyAction(task, action) {
					modified = true
				}
			}
			result.AppliedRules[task.ID] = append(result.AppliedRules[task.ID], rule.Name)
		}

		if modified {
			result.ModifiedTasks = append(result.ModifiedTasks, task)
			// Re-evaluate rules for this task since it changed
			evaluateRulesRecursive([]*models.Task{task}, rules, ctx, result)
		}
	}
}

func getFieldFromAction(actionType ActionType) string {
	switch actionType {
	case ActionSetStatus:
		return "status"
	case ActionSetPriority:
		return "priority"
	case ActionSetSection:
		return "section"
	default:
		return "" // Tags can have multiple add/remove without conflict
	}
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
		// For tags, we check if any tag matches
		return matchesTagTrigger(task.Tags, trigger)
	case FieldType:
		fieldValue = string(task.Type)
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

// applyAction applies a rule action to a task. Returns true if the task was modified.
func applyAction(task *models.Task, action RuleAction) bool {
	switch action.ActionType {
	case ActionSetStatus:
		newStatus := models.TaskStatus(action.Value)
		if newStatus.IsValid() && task.Status != newStatus {
			task.Status = newStatus
			return true
		}
	case ActionSetPriority:
		newPriority, err := strconv.Atoi(action.Value)
		if err == nil && newPriority >= 1 && newPriority <= 5 && task.Priority != newPriority {
			task.Priority = newPriority
			return true
		}
	case ActionSetSection:
		if task.Section != action.Value {
			task.Section = action.Value
			return true
		}
	case ActionAddTag:
		if !task.HasTag(action.Value) {
			task.AddTag(action.Value)
			return true
		}
	case ActionRemoveTag:
		if task.HasTag(action.Value) {
			task.RemoveTag(action.Value)
			return true
		}
	}
	return false
}

// FormatConflicts returns a human-readable description of conflicts.
func FormatConflicts(conflicts []RuleConflict) string {
	if len(conflicts) == 0 {
		return ""
	}

	var lines []string
	for _, c := range conflicts {
		lines = append(lines, fmt.Sprintf(
			"Conflict on task %q: multiple rules (%s) try to set %s",
			c.TaskTitle,
			strings.Join(c.Rules, ", "),
			c.Field,
		))
	}
	return strings.Join(lines, "\n")
}

// TestRule evaluates a single rule against a task without applying changes.
// Returns whether the rule would match and what actions would be taken.
func TestRule(task *models.Task, rule WorkflowRule) (matches bool, actions []string) {
	if !matchesTrigger(task, rule.Trigger) {
		return false, nil
	}

	for _, action := range rule.Actions {
		actions = append(actions, action.String())
	}
	return true, actions
}
