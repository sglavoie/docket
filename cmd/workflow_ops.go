package cmd

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/workflow"
)

// CaptureTaskStates creates a snapshot of task states for event trigger detection.
// Call this before making changes to tasks to capture their previous state.
func CaptureTaskStates(tasks []*models.Task) map[string]*models.Task {
	states := make(map[string]*models.Task, len(tasks))
	for _, task := range tasks {
		states[task.ID] = copyTaskState(task)
	}
	return states
}

// copyTaskState creates a shallow copy of task fields relevant for event detection.
func copyTaskState(task *models.Task) *models.Task {
	if task == nil {
		return nil
	}
	// Copy tags slice
	tagsCopy := make([]string, len(task.Tags))
	copy(tagsCopy, task.Tags)

	return &models.Task{
		ID:       task.ID,
		Status:   task.Status,
		Priority: task.Priority,
		Section:  task.Section,
		Tags:     tagsCopy,
		Type:     task.Type,
		// Preserve parent/child relationships for hierarchical triggers
		Parent:   task.Parent,
		Children: task.Children,
		Level:    task.Level,
	}
}

// BuildEvaluationContext creates an evaluation context for workflow rules.
func BuildEvaluationContext(tasks []*models.Task, prevStates map[string]*models.Task) *workflow.EvaluationContext {
	changes := make([]workflow.TaskChangeContext, len(tasks))
	taskMap := make(map[string]*models.Task, len(tasks))

	for i, task := range tasks {
		taskMap[task.ID] = task
		changes[i] = workflow.TaskChangeContext{
			Task:     task,
			Previous: prevStates[task.ID], // nil if task is new
		}
	}

	return &workflow.EvaluationContext{
		Changes:  changes,
		AllTasks: tasks,
		TaskMap:  taskMap,
	}
}

// WorkflowEvalResult holds the result of evaluating workflow rules for a project.
type WorkflowEvalResult struct {
	ProjectName   string
	ModifiedTasks int
	RulesApplied  map[string]int // rule ID -> count of tasks affected
	Skipped       bool           // true if evaluation was skipped (recently evaluated)
	Error         error
}

// EvaluateProjectWorkflowRules evaluates workflow rules for a project.
// If force is false and the project was recently evaluated, evaluation is skipped.
// The project must already be loaded with tasks.
// Returns the evaluation result and updates the eval state in config.
func EvaluateProjectWorkflowRules(project *models.Project, cfg *config.Config, configManager *config.Manager, force bool) WorkflowEvalResult {
	result := WorkflowEvalResult{
		ProjectName:  project.Name,
		RulesApplied: make(map[string]int),
	}

	// Check if evaluation is needed
	if !force && !cfg.ShouldEvaluateWorkflow(project.ID) {
		result.Skipped = true
		return result
	}

	rules := cfg.GetEnabledWorkflowRules()
	if len(rules) == 0 {
		result.Skipped = true
		return result
	}

	// Run auto-unblock before rule evaluation
	project.CheckAndAutoUnblock()

	// Evaluate rules
	evalResult := workflow.EvaluateRules(project.Tasks, rules)

	result.ModifiedTasks = len(evalResult.ModifiedTasks)
	result.RulesApplied = evalResult.RuleTriggerCounts

	// Only save if there were actual modifications
	if result.ModifiedTasks > 0 {
		if err := SaveProject(project, nil); err != nil {
			result.Error = err
			return result
		}

		// Update trigger counts
		if len(evalResult.RuleTriggerCounts) > 0 && configManager != nil {
			cfg.IncrementRuleTriggerCounts(evalResult.RuleTriggerCounts)
		}
	}

	// Update eval state
	cfg.SetWorkflowEvalState(project.ID, config.WorkflowEvalState{
		LastEvalTime: time.Now(),
		RulesHash:    cfg.ComputeWorkflowRulesHash(),
	})

	// Persist config changes
	if configManager != nil {
		if err := configManager.Save(cfg); err != nil {
			// Non-fatal, just log
			fmt.Fprintln(os.Stderr, "Warning: failed to save workflow eval state:", err)
		}
	}

	return result
}

// Helper functions for workflow edit command

// buildTriggersFromFlags creates triggers from flag arrays.
// Mirrors the logic from workflow_add.go for consistency.
func buildTriggersFromFlags(fields, ops, values []string) ([]workflow.RuleTrigger, error) {
	if len(fields) != len(values) {
		return nil, fmt.Errorf("number of trigger fields (%d) must match values (%d)", len(fields), len(values))
	}

	// Default ops to "equals" if not specified
	if len(ops) == 0 {
		ops = make([]string, len(fields))
		for i := range ops {
			ops[i] = "equals"
		}
	} else if len(ops) != len(fields) {
		return nil, fmt.Errorf("number of trigger ops (%d) must match fields (%d), or omit ops to default to equals", len(ops), len(fields))
	}

	validFields := map[workflow.TriggerField]bool{
		workflow.FieldStatus: true, workflow.FieldPriority: true, workflow.FieldSection: true,
		workflow.FieldTags: true, workflow.FieldType: true, workflow.FieldDueDays: true,
		workflow.FieldCreatedDays: true, workflow.FieldUpdatedDays: true,
		workflow.FieldStatusChanged: true, workflow.FieldSectionChanged: true,
		workflow.FieldPriorityChanged: true, workflow.FieldTagAdded: true,
		workflow.FieldTagRemoved: true, workflow.FieldCompleted: true, workflow.FieldReopened: true,
		workflow.FieldParentStatus: true, workflow.FieldParentType: true, workflow.FieldParentTags: true,
		workflow.FieldLevel: true, workflow.FieldHasChildren: true, workflow.FieldChildrenProgress: true,
		workflow.FieldAllChildrenDone: true, workflow.FieldIsBlocked: true, workflow.FieldBlockerCount: true,
	}
	validOps := map[workflow.TriggerOperation]bool{
		workflow.OpEquals: true, workflow.OpNotEquals: true, workflow.OpContains: true,
		workflow.OpGreaterThan: true, workflow.OpLessThan: true,
	}

	var triggers []workflow.RuleTrigger
	for i := range fields {
		field := workflow.TriggerField(strings.ToLower(fields[i]))
		if !validFields[field] {
			return nil, fmt.Errorf("invalid trigger field: %s", fields[i])
		}

		op := workflow.TriggerOperation(strings.ToLower(ops[i]))
		if !validOps[op] {
			return nil, fmt.Errorf("invalid trigger operation: %s (valid: equals, not_equals, contains, greater_than, less_than)", ops[i])
		}

		trigger := workflow.RuleTrigger{
			Field:     field,
			Operation: op,
			Value:     values[i],
		}

		if err := trigger.Validate(); err != nil {
			return nil, fmt.Errorf("invalid trigger %d: %w", i+1, err)
		}

		triggers = append(triggers, trigger)
	}

	return triggers, nil
}

// buildActionFromFlags creates an action from flags.
func buildActionFromFlags(actionType, actionValue string) (workflow.RuleAction, error) {
	action := workflow.RuleAction{
		ActionType: workflow.ActionType(strings.ToLower(actionType)),
		Value:      actionValue,
	}

	if err := action.Validate(); err != nil {
		return workflow.RuleAction{}, fmt.Errorf("invalid action: %w", err)
	}

	return action, nil
}

// parseCommaSeparatedIndices parses a comma-separated string of indices (e.g., "0,2,4").
// Returns deduplicated and sorted indices.
func parseCommaSeparatedIndices(s string) ([]int, error) {
	if s == "" {
		return nil, nil
	}

	parts := strings.Split(s, ",")
	seen := make(map[int]bool)
	var indices []int

	for _, part := range parts {
		part = strings.TrimSpace(part)
		var idx int
		if _, err := fmt.Sscanf(part, "%d", &idx); err != nil {
			return nil, fmt.Errorf("invalid index: %s", part)
		}
		if !seen[idx] {
			seen[idx] = true
			indices = append(indices, idx)
		}
	}

	// Sort for consistent behavior
	for i := 0; i < len(indices); i++ {
		for j := i + 1; j < len(indices); j++ {
			if indices[i] > indices[j] {
				indices[i], indices[j] = indices[j], indices[i]
			}
		}
	}

	return indices, nil
}

// removeByIndices removes items from a slice by index.
// Generic function that works with any slice type.
// Validates bounds and removes in reverse order to avoid index shifting issues.
func removeByIndices[T any](slice []T, indices []int) ([]T, error) {
	if len(indices) == 0 {
		return slice, nil
	}

	// Validate all indices first
	for _, idx := range indices {
		if idx < 0 || idx >= len(slice) {
			return nil, fmt.Errorf("invalid index %d (valid range: 0-%d)", idx, len(slice)-1)
		}
	}

	// Sort indices in reverse order for safe removal
	sortedIndices := make([]int, len(indices))
	copy(sortedIndices, indices)
	for i := 0; i < len(sortedIndices); i++ {
		for j := i + 1; j < len(sortedIndices); j++ {
			if sortedIndices[i] < sortedIndices[j] {
				sortedIndices[i], sortedIndices[j] = sortedIndices[j], sortedIndices[i]
			}
		}
	}

	result := make([]T, len(slice))
	copy(result, slice)

	for _, idx := range sortedIndices {
		result = append(result[:idx], result[idx+1:]...)
	}

	return result, nil
}

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

	// Trigger changes
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

	if len(lines) == 0 {
		return "(no changes)"
	}

	return strings.Join(lines, "\n")
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
	}
	return true
}

// collectValidationErrors validates a rule and returns all errors found.
func collectValidationErrors(rule *workflow.WorkflowRule) []string {
	var errors []string

	if rule.Name == "" {
		errors = append(errors, "rule name cannot be empty")
	}

	triggers := rule.GetTriggers()
	if len(triggers) == 0 {
		errors = append(errors, "at least one trigger is required")
	} else {
		for i, trigger := range triggers {
			if err := trigger.Validate(); err != nil {
				errors = append(errors, fmt.Sprintf("trigger %d: %s", i, err.Error()))
			}
		}
	}

	if rule.TriggerLogic != "" && rule.TriggerLogic != workflow.LogicAnd && rule.TriggerLogic != workflow.LogicOr {
		errors = append(errors, fmt.Sprintf("invalid trigger logic: %s (valid: and, or)", rule.TriggerLogic))
	}

	if len(rule.Actions) == 0 {
		errors = append(errors, "at least one action is required")
	} else {
		for i, action := range rule.Actions {
			if err := action.Validate(); err != nil {
				errors = append(errors, fmt.Sprintf("action %d: %s", i, err.Error()))
			}
		}
	}

	return errors
}

// requireYesFlag checks if --yes flag is provided and returns an error if not.
func requireYesFlag(yesProvided bool) error {
	if !yesProvided {
		return fmt.Errorf("use --yes to apply changes")
	}
	return nil
}
