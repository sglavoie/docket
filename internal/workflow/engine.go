package workflow

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/util"
)

const maxRuleDepth = 10

// RuleConflict represents a conflict when multiple rules try to set the same field.
type RuleConflict struct {
	TaskID    string
	TaskTitle string
	Field     string
	Rules     []string // Rule names that conflict
}

// DeferredMove represents a task that should be moved to a different project.
// The move is deferred because the engine doesn't have access to project storage.
type DeferredMove struct {
	TaskID      string
	Destination string // project name
}

// EvaluationResult contains the result of rule evaluation.
type EvaluationResult struct {
	ModifiedTasks     []*models.Task
	Conflicts         []RuleConflict
	AppliedRules      map[string][]string // taskID -> list of applied rule names
	RuleTriggerCounts map[string]int      // ruleID -> count of tasks affected
	NewFollowupTasks  []*models.Task      // tasks created by create_followup action
	DeferredMoves     []DeferredMove      // moves requested by move_to_project action
}

// TaskChangeContext provides context for event-based triggers.
// It contains both the current task state and the previous state (if available).
type TaskChangeContext struct {
	Task     *models.Task // Current task state
	Previous *models.Task // Previous state (nil for new tasks)
}

// EvaluationContext provides all context needed for rule evaluation.
type EvaluationContext struct {
	Changes     []TaskChangeContext     // Tasks with change context
	AllTasks    []*models.Task          // All tasks for hierarchical lookups
	TaskMap     map[string]*models.Task // taskID -> task for dependency lookups
	ProjectName string                  // Current project name (for scope matching)
	ProjectID   string                  // Current project ID (for scope matching)
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

// sortRulesByPriority returns a stable-sorted copy of rules: non-zero priorities
// ascending (1 first), then zero-priority rules in original order.
func sortRulesByPriority(rules []WorkflowRule) []WorkflowRule {
	sorted := make([]WorkflowRule, len(rules))
	copy(sorted, rules)
	sort.SliceStable(sorted, func(i, j int) bool {
		pi, pj := sorted[i].Priority, sorted[j].Priority
		// Both zero: preserve original order (stable sort)
		if pi == 0 && pj == 0 {
			return false
		}
		// Zero sorts after non-zero
		if pi == 0 {
			return false
		}
		if pj == 0 {
			return true
		}
		// Both non-zero: lower number = higher priority
		return pi < pj
	})
	return sorted
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

	sorted := sortRulesByPriority(rules)
	ctx := newExecutionContext()
	evaluateRulesWithContextRecursive(evalCtx, sorted, ctx, result)
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

// fieldSetter tracks which rule wants to set a field, with its priority.
type fieldSetter struct {
	ruleName string
	ruleID   string
	priority int
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
		fieldSetters := make(map[string][]fieldSetter) // field -> setters

		// Collect matching rules
		type matchedRule struct {
			rule  WorkflowRule
			index int
		}
		var matched []matchedRule

		for i, rule := range rules {
			if !rule.Enabled {
				continue
			}
			if rule.GetLevel() == RuleLevelProject {
				continue
			}
			if !rule.MatchesScope(task, evalCtx.ProjectName, evalCtx.ProjectID) {
				continue
			}
			if ctx.wasApplied(task.ID, rule.ID) {
				continue
			}
			if !matchesRuleWithContext(task, prev, &rule, evalCtx) {
				continue
			}

			matched = append(matched, matchedRule{rule: rule, index: i})
			for _, action := range rule.Actions {
				// Skip actions whose condition is not met for conflict detection
				if action.Condition != nil && !matchesTriggerWithContext(task, prev, *action.Condition, evalCtx) {
					continue
				}
				field := getFieldFromAction(action.ActionType)
				if field != "" {
					fieldSetters[field] = append(fieldSetters[field], fieldSetter{
						ruleName: rule.Name,
						ruleID:   rule.ID,
						priority: rule.Priority,
					})
				}
			}
		}

		// Resolve conflicts per-field using priority
		// Build a set of rule IDs that should be skipped for specific fields
		skippedFieldRules := make(map[string]map[string]bool) // field -> ruleID -> skip
		for field, setters := range fieldSetters {
			if len(setters) <= 1 {
				continue
			}

			// Find the highest priority (lowest non-zero number) among setters
			bestPriority := 0
			for _, s := range setters {
				if s.priority > 0 && (bestPriority == 0 || s.priority < bestPriority) {
					bestPriority = s.priority
				}
			}

			// Collect winners and losers
			var winners []fieldSetter
			for _, s := range setters {
				effectivePri := s.priority
				if effectivePri == 0 {
					effectivePri = 1<<31 - 1 // treat unset as lowest
				}
				bestEffective := bestPriority
				if bestEffective == 0 {
					bestEffective = 1<<31 - 1
				}
				if effectivePri == bestEffective {
					winners = append(winners, s)
				}
			}

			if len(winners) > 1 {
				// Same-priority conflict: report and skip all conflicting rules for this field
				var names []string
				for _, w := range winners {
					names = append(names, w.ruleName)
				}
				result.Conflicts = append(result.Conflicts, RuleConflict{
					TaskID:    task.ID,
					TaskTitle: task.Title,
					Field:     field,
					Rules:     names,
				})
				if skippedFieldRules[field] == nil {
					skippedFieldRules[field] = make(map[string]bool)
				}
				for _, w := range winners {
					skippedFieldRules[field][w.ruleID] = true
				}
			} else {
				// Clear winner: skip lower-priority rules for this field
				winnerID := winners[0].ruleID
				if skippedFieldRules[field] == nil {
					skippedFieldRules[field] = make(map[string]bool)
				}
				for _, s := range setters {
					if s.ruleID != winnerID {
						skippedFieldRules[field][s.ruleID] = true
					}
				}
			}
		}

		// Apply matched rules, skipping conflicted actions per-field
		modified := false
		for _, mr := range matched {
			rule := mr.rule
			ctx.markApplied(task.ID, rule.ID)

			ruleApplied := false
			for _, action := range rule.Actions {
				// Check action condition before applying
				if action.Condition != nil && !matchesTriggerWithContext(task, prev, *action.Condition, evalCtx) {
					continue // condition not met, skip this action
				}
				field := getFieldFromAction(action.ActionType)
				if field != "" {
					if skipSet, ok := skippedFieldRules[field]; ok && skipSet[rule.ID] {
						continue // skip this action due to conflict/lower priority
					}
				}
				if applyActionWithContext(task, action, evalCtx, result) {
					modified = true
					ruleApplied = true
				}
			}
			if ruleApplied {
				result.AppliedRules[task.ID] = append(result.AppliedRules[task.ID], rule.Name)
				result.RuleTriggerCounts[rule.ID]++
			}
		}

		if modified {
			result.ModifiedTasks = append(result.ModifiedTasks, task)
			// Re-evaluate rules for this task since it changed (without event triggers on re-eval)
			newChange := TaskChangeContext{Task: task, Previous: task} // same state, no events fire
			newEvalCtx := &EvaluationContext{
				Changes:     []TaskChangeContext{newChange},
				AllTasks:    evalCtx.AllTasks,
				TaskMap:     evalCtx.TaskMap,
				ProjectName: evalCtx.ProjectName,
				ProjectID:   evalCtx.ProjectID,
			}
			evaluateRulesWithContextRecursive(newEvalCtx, rules, ctx, result)
		}
	}
}

// matchesRuleWithContext checks if a task matches all trigger conditions with full context.
func matchesRuleWithContext(task, prev *models.Task, rule *WorkflowRule, evalCtx *EvaluationContext) bool {
	// Use TriggerGroup if present (takes precedence over flat triggers)
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

	// AND (default): all triggers must match
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

	// Collect results from leaf triggers
	var results []bool
	for _, trigger := range group.Triggers {
		results = append(results, matchesTriggerWithContext(task, prev, trigger, evalCtx))
	}
	// Collect results from sub-groups
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

	// AND
	for _, r := range results {
		if !r {
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

	case FieldDueDateChanged:
		prevDue := prev != nil && prev.DueDate != nil
		curDue := task.DueDate != nil
		if !prevDue && !curDue {
			return false // no change
		}
		// Determine the kind of change
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
		// Fires when a recurring task is completed (recurrence cleared on completion)
		hadRecurrence := prev != nil && prev.Recurrence != nil
		hasRecurrence := task.Recurrence != nil
		isComplete := task.Status.IsComplete()
		return hadRecurrence && !hasRecurrence && isComplete

	case FieldStreakBroken:
		if trigger.Value != "true" {
			return false
		}
		// Fires when a recurrence streak drops from >0 to 0
		hadStreak := prev != nil && prev.Recurrence != nil && prev.Recurrence.Streak > 0
		hasStreak := task.Recurrence != nil && task.Recurrence.Streak > 0
		return hadStreak && !hasStreak
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

// applyActionWithContext applies an action with full context for hierarchical and deferred actions.
func applyActionWithContext(task *models.Task, action RuleAction, evalCtx *EvaluationContext, result *EvaluationResult) bool {
	// Handle hierarchical actions
	if IsHierarchicalAction(action.ActionType) {
		return applyHierarchicalAction(task, action, evalCtx)
	}

	// Handle deferred actions that produce side effects in the result
	switch action.ActionType {
	case ActionCreateFollowup:
		return applyCreateFollowup(task, action, result)
	case ActionMoveToProject:
		return applyMoveToProject(task, action, result)
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

// applyCreateFollowup creates a new follow-up task linked to the original.
// The task is appended to result.NewFollowupTasks for the caller to persist.
func applyCreateFollowup(task *models.Task, action RuleAction, result *EvaluationResult) bool {
	followup := models.NewTask(uuid.New().String(), 0, models.TaskTypeTask, action.Value)
	followup.Priority = models.PriorityDefault
	followup.AddTag("followup")
	// Bidirectional relationship
	followup.AddRelatedTo(task.ID)
	task.AddRelatedTo(followup.ID)
	result.NewFollowupTasks = append(result.NewFollowupTasks, followup)
	return true
}

// applyMoveToProject records a deferred move for the task.
// The actual move is handled by the caller that has access to project storage.
func applyMoveToProject(task *models.Task, action RuleAction, result *EvaluationResult) bool {
	result.DeferredMoves = append(result.DeferredMoves, DeferredMove{
		TaskID:      task.ID,
		Destination: action.Value,
	})
	return true
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

// daysUntil returns the number of days until the given date.
// Returns negative values for past dates (overdue).
func daysUntil(t time.Time) int {
	now := time.Now()
	// Normalize to start of day for consistent comparison
	nowDate := util.StartOfDay(now)
	targetDate := util.StartOfDay(t)
	diff := targetDate.Sub(nowDate)
	return int(diff.Hours() / 24)
}

// daysSince returns the number of days since the given date.
// Always returns 0 or positive values.
func daysSince(t time.Time) int {
	now := time.Now()
	// Normalize to start of day for consistent comparison
	nowDate := util.StartOfDay(now)
	targetDate := util.StartOfDay(t)
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
		if models.IsGTDSection(action.Value) || action.Value == "" {
			if task.Section != action.Value {
				task.Section = action.Value
				return true
			}
		} else {
			if task.CustomSection != action.Value {
				task.CustomSection = action.Value
				return true
			}
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
		if task.DueDate != nil {
			return false
		}
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
		commentText := "[Workflow] " + action.Value
		if len(task.Comments) > 0 {
			last := task.Comments[len(task.Comments)-1]
			if last.Text == commentText {
				return false
			}
		}
		task.AddComment(commentText)
		return true
	case ActionSetType:
		newType := models.TaskType(action.Value)
		if newType.IsValid() && task.Type != newType {
			task.Type = newType
			task.UpdatedAt = time.Now()
			return true
		}
	case ActionSetSnooze:
		days, err := ParseRelativeDuration(action.Value)
		if err != nil {
			return false
		}
		snoozeUntil := time.Now().AddDate(0, 0, days)
		task.SnoozedUntil = &snoozeUntil
		task.UpdatedAt = time.Now()
		return true
	case ActionSetRecurrence:
		pattern := models.ParseRecurrence(action.Value)
		if pattern == nil {
			return false
		}
		task.Recurrence = pattern
		task.UpdatedAt = time.Now()
		return true
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
