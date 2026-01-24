package workflow

import (
	"fmt"
	"strconv"
	"strings"
	"time"

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
	ModifiedTasks     []*models.Task
	Conflicts         []RuleConflict
	AppliedRules      map[string][]string // taskID -> list of applied rule names
	RuleTriggerCounts map[string]int      // ruleID -> count of tasks affected
}

// TaskChangeContext provides context for event-based triggers.
// It contains both the current task state and the previous state (if available).
type TaskChangeContext struct {
	Task     *models.Task // Current task state
	Previous *models.Task // Previous state (nil for new tasks)
}

// EvaluationContext provides all context needed for rule evaluation.
type EvaluationContext struct {
	Changes  []TaskChangeContext     // Tasks with change context
	AllTasks []*models.Task          // All tasks for hierarchical lookups
	TaskMap  map[string]*models.Task // taskID -> task for dependency lookups
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
// This is the legacy function that doesn't support event-based triggers.
func EvaluateRules(tasks []*models.Task, rules []WorkflowRule) *EvaluationResult {
	// Build change context without previous states (no event triggers)
	changes := make([]TaskChangeContext, len(tasks))
	for i, task := range tasks {
		changes[i] = TaskChangeContext{Task: task, Previous: nil}
	}

	evalCtx := &EvaluationContext{
		Changes:  changes,
		AllTasks: tasks,
		TaskMap:  buildTaskMap(tasks),
	}

	return EvaluateRulesWithContext(evalCtx, rules)
}

// EvaluateRulesWithContext evaluates rules with full context for event and hierarchical triggers.
// This is the preferred function when change detection is needed.
func EvaluateRulesWithContext(evalCtx *EvaluationContext, rules []WorkflowRule) *EvaluationResult {
	result := &EvaluationResult{
		ModifiedTasks:     []*models.Task{},
		Conflicts:         []RuleConflict{},
		AppliedRules:      make(map[string][]string),
		RuleTriggerCounts: make(map[string]int),
	}

	if len(rules) == 0 {
		return result
	}

	ctx := newExecutionContext()
	evaluateRulesWithContextRecursive(evalCtx, rules, ctx, result)
	return result
}

// buildTaskMap creates a map of taskID -> task for fast lookups.
func buildTaskMap(tasks []*models.Task) map[string]*models.Task {
	m := make(map[string]*models.Task, len(tasks))
	for _, task := range tasks {
		m[task.ID] = task
	}
	return m
}

func evaluateRulesWithContextRecursive(evalCtx *EvaluationContext, rules []WorkflowRule, ctx *executionContext, result *EvaluationResult) {
	if ctx.depth >= maxRuleDepth {
		return
	}
	ctx.depth++
	defer func() { ctx.depth-- }()

	for _, change := range evalCtx.Changes {
		task := change.Task
		prev := change.Previous

		// Track which fields are being set by which rules for conflict detection
		fieldSetters := make(map[string][]string) // field -> rule names

		for _, rule := range rules {
			if !rule.Enabled {
				continue
			}
			if ctx.wasApplied(task.ID, rule.ID) {
				continue
			}
			if !matchesRuleWithContext(task, prev, &rule, evalCtx) {
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
			if !matchesRuleWithContext(task, prev, &rule, evalCtx) {
				continue
			}

			ctx.markApplied(task.ID, rule.ID)
			for _, action := range rule.Actions {
				if applyActionWithContext(task, action, evalCtx) {
					modified = true
				}
			}
			result.AppliedRules[task.ID] = append(result.AppliedRules[task.ID], rule.Name)
			result.RuleTriggerCounts[rule.ID]++
		}

		if modified {
			result.ModifiedTasks = append(result.ModifiedTasks, task)
			// Re-evaluate rules for this task since it changed (without event triggers on re-eval)
			newChange := TaskChangeContext{Task: task, Previous: task} // same state, no events fire
			newEvalCtx := &EvaluationContext{
				Changes:  []TaskChangeContext{newChange},
				AllTasks: evalCtx.AllTasks,
				TaskMap:  evalCtx.TaskMap,
			}
			evaluateRulesWithContextRecursive(newEvalCtx, rules, ctx, result)
		}
	}
}

// matchesRuleWithContext checks if a task matches all trigger conditions with full context.
func matchesRuleWithContext(task, prev *models.Task, rule *WorkflowRule, evalCtx *EvaluationContext) bool {
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

	// AND (default): all triggers must match
	for _, trigger := range triggers {
		if !matchesTriggerWithContext(task, prev, trigger, evalCtx) {
			return false
		}
	}
	return true
}

// matchesTriggerWithContext checks a trigger with full context for events and hierarchy.
func matchesTriggerWithContext(task, prev *models.Task, trigger RuleTrigger, evalCtx *EvaluationContext) bool {
	// Handle event-based triggers
	if IsEventTrigger(trigger.Field) {
		return matchesEventTrigger(task, prev, trigger)
	}

	// Handle hierarchical triggers
	if IsHierarchicalTrigger(trigger.Field) {
		return matchesHierarchicalTrigger(task, trigger, evalCtx)
	}

	// Fall back to standard trigger matching
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
		// Trigger fires when the specified tag was added
		hadTag := prev != nil && hasTag(prev.Tags, trigger.Value)
		hasTagNow := hasTag(task.Tags, trigger.Value)
		return !hadTag && hasTagNow

	case FieldTagRemoved:
		// Trigger fires when the specified tag was removed
		hadTag := prev != nil && hasTag(prev.Tags, trigger.Value)
		hasTagNow := hasTag(task.Tags, trigger.Value)
		return hadTag && !hasTagNow

	case FieldCompleted:
		// Trigger fires when task becomes done or canceled
		if trigger.Value != "true" {
			return false
		}
		wasComplete := prev != nil && prev.Status.IsComplete()
		isComplete := task.Status.IsComplete()
		return !wasComplete && isComplete

	case FieldReopened:
		// Trigger fires when task is reopened (was complete, now not)
		if trigger.Value != "true" {
			return false
		}
		wasComplete := prev != nil && prev.Status.IsComplete()
		isComplete := task.Status.IsComplete()
		return wasComplete && !isComplete
	}

	return false
}

// matchesTransition checks if a field transition matches the pattern "old->new".
func matchesTransition(oldVal, newVal, pattern string) bool {
	// No change means no transition
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
		// True only if has children and all are complete
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

// applyActionWithContext applies an action with full context for hierarchical actions.
func applyActionWithContext(task *models.Task, action RuleAction, evalCtx *EvaluationContext) bool {
	// Handle hierarchical actions
	if IsHierarchicalAction(action.ActionType) {
		return applyHierarchicalAction(task, action, evalCtx)
	}

	// Fall back to standard action application
	return applyAction(task, action)
}

// applyHierarchicalAction applies actions that operate on parent/children.
func applyHierarchicalAction(task *models.Task, action RuleAction, evalCtx *EvaluationContext) bool {
	switch action.ActionType {
	case ActionCompleteParent:
		if task.Parent != nil && !task.Parent.IsComplete() {
			task.Parent.Complete()
			return true
		}

	case ActionBlockParent:
		if task.Parent != nil && task.Parent.Status != models.StatusBlocked {
			task.Parent.Block()
			return true
		}

	case ActionCascadeTag:
		modified := false
		for _, child := range task.Children {
			if !child.HasTag(action.Value) {
				child.AddTag(action.Value)
				modified = true
			}
		}
		return modified

	case ActionInheritTag:
		if task.Parent != nil && task.Parent.HasTag(action.Value) && !task.HasTag(action.Value) {
			task.AddTag(action.Value)
			return true
		}

	case ActionNotifyParent:
		if task.Parent != nil {
			task.Parent.AddComment("[Workflow] " + action.Value)
			return true
		}
	}

	return false
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

// matchesRule checks if a task matches all of the rule's trigger conditions.
// Handles both single trigger (legacy) and compound triggers.
func matchesRule(task *models.Task, rule *WorkflowRule) bool {
	triggers := rule.GetTriggers()
	if len(triggers) == 0 {
		return false
	}

	logic := rule.GetTriggerLogic()

	if logic == LogicOr {
		// OR: any trigger must match
		for _, trigger := range triggers {
			if matchesTrigger(task, trigger) {
				return true
			}
		}
		return false
	}

	// AND (default): all triggers must match
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
		// For tags, we check if any tag matches
		return matchesTagTrigger(task.Tags, trigger)
	case FieldType:
		fieldValue = string(task.Type)
	case FieldDueDays:
		// Days until due date (negative = overdue)
		if task.DueDate == nil {
			return false // No due date, skip this trigger
		}
		days := daysUntil(*task.DueDate)
		fieldValue = strconv.Itoa(days)
	case FieldCreatedDays:
		// Days since creation
		days := daysSince(task.CreatedAt)
		fieldValue = strconv.Itoa(days)
	case FieldUpdatedDays:
		// Days since last update
		days := daysSince(task.UpdatedAt)
		fieldValue = strconv.Itoa(days)
	default:
		return false
	}

	return compareValues(fieldValue, trigger.Operation, trigger.Value)
}

// daysUntil returns the number of days until the given date.
// Returns negative values for past dates (overdue).
func daysUntil(t time.Time) int {
	now := time.Now()
	// Normalize to start of day for consistent comparison
	nowDate := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	targetDate := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
	diff := targetDate.Sub(nowDate)
	return int(diff.Hours() / 24)
}

// daysSince returns the number of days since the given date.
// Always returns 0 or positive values.
func daysSince(t time.Time) int {
	now := time.Now()
	// Normalize to start of day for consistent comparison
	nowDate := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	targetDate := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
	diff := nowDate.Sub(targetDate)
	days := int(diff.Hours() / 24)
	if days < 0 {
		return 0
	}
	return days
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
		if err == nil && newPriority >= models.PriorityMin && newPriority <= models.PriorityMax && task.Priority != newPriority {
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
	case ActionSetDueRelative:
		days, err := ParseRelativeDuration(action.Value)
		if err == nil {
			newDue := time.Now().AddDate(0, 0, days)
			task.DueDate = &newDue
			task.UpdatedAt = time.Now()
			return true
		}
	case ActionClearDue:
		if task.DueDate != nil {
			task.DueDate = nil
			task.UpdatedAt = time.Now()
			return true
		}
	case ActionSetActive:
		active := action.Value == "true"
		if task.Active != active {
			task.Active = active
			task.UpdatedAt = time.Now()
			return true
		}
	case ActionSetPinned:
		pinned := action.Value == "true"
		if task.Pinned != pinned {
			if pinned {
				task.Pin()
			} else {
				task.Unpin()
			}
			return true
		}
	case ActionAddComment:
		task.AddComment("[Workflow] " + action.Value)
		return true
	case ActionSetType:
		newType := models.TaskType(action.Value)
		if newType.IsValid() && task.Type != newType {
			task.Type = newType
			task.UpdatedAt = time.Now()
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
	if !matchesRule(task, &rule) {
		return false, nil
	}

	for _, action := range rule.Actions {
		actions = append(actions, action.String())
	}
	return true, actions
}
