package cmd

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/workflow"
)

// buildTriggersFromFlags creates triggers from flag arrays.
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

	var triggers []workflow.RuleTrigger
	for i := range fields {
		trigger := workflow.RuleTrigger{
			Field:     workflow.TriggerField(strings.ToLower(fields[i])),
			Operation: workflow.TriggerOperation(strings.ToLower(ops[i])),
			Value:     values[i],
		}

		if err := trigger.ValidateForAuthoring(); err != nil {
			return nil, fmt.Errorf("invalid trigger %d: %w", i+1, err)
		}

		triggers = append(triggers, trigger)
	}

	return triggers, nil
}

// buildConditionFromFlags creates a RuleTrigger from condition CLI flags.
func buildConditionFromFlags(field, op, value string) (*workflow.RuleTrigger, error) {
	if field == "" {
		return nil, fmt.Errorf("condition field is required")
	}
	if value == "" {
		return nil, fmt.Errorf("condition value is required when condition field is specified")
	}

	cond := &workflow.RuleTrigger{
		Field:     workflow.TriggerField(strings.ToLower(field)),
		Operation: workflow.TriggerOperation(strings.ToLower(op)),
		Value:     value,
	}
	if err := cond.ValidateForAuthoring(); err != nil {
		return nil, fmt.Errorf("invalid action condition: %w", err)
	}
	return cond, nil
}

// buildActionFromFlags creates an action from flags.
func buildActionFromFlags(actionType, actionValue string) (workflow.RuleAction, error) {
	action := workflow.RuleAction{
		ActionType: workflow.ActionType(strings.ToLower(actionType)),
		Value:      actionValue,
	}

	if err := action.ValidateForAuthoring(); err != nil {
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
		idx, err := strconv.Atoi(part)
		if err != nil {
			return nil, fmt.Errorf("invalid index: %s", part)
		}
		if !seen[idx] {
			seen[idx] = true
			indices = append(indices, idx)
		}
	}

	// Sort for consistent behavior
	slices.Sort(indices)

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
	if len(slice) == 0 {
		return nil, fmt.Errorf("cannot remove from empty list")
	}
	for _, idx := range indices {
		if idx < 0 || idx >= len(slice) {
			return nil, fmt.Errorf("invalid index %d (valid range: 0-%d)", idx, len(slice)-1)
		}
	}

	// Sort indices in reverse order for safe removal
	sortedIndices := make([]int, len(indices))
	copy(sortedIndices, indices)
	slices.SortFunc(sortedIndices, func(a, b int) int {
		return b - a
	})

	result := make([]T, len(slice))
	copy(result, slice)

	for _, idx := range sortedIndices {
		result = append(result[:idx], result[idx+1:]...)
	}

	return result, nil
}

// buildScopeFromFlags creates a canonical RuleScope from CLI flag values.
// Returns nil if all slices are empty (no restriction).
func buildScopeFromFlags(projects, types, states []string) (*workflow.RuleScope, error) {
	if len(projects) == 0 && len(types) == 0 && len(states) == 0 {
		return nil, nil
	}
	scope := &workflow.RuleScope{
		Projects: projects,
		Types:    types,
	}
	parsedStates, err := parseWorkflowScopeStates(states)
	if err != nil {
		return nil, err
	}
	scope.States = parsedStates
	return scope, nil
}

func parseWorkflowScopeStates(values []string) ([]models.WorkflowState, error) {
	states := make([]models.WorkflowState, 0, len(values))
	for _, raw := range values {
		state, ok := models.ParseWorkflowState(strings.TrimSpace(raw))
		if !ok {
			return nil, fmt.Errorf("invalid workflow state in scope: %s", raw)
		}
		states = append(states, state)
	}
	return states, nil
}

func workflowTriggerFieldCompletions() []string {
	fields := workflow.ValidTriggerFields()
	values := make([]string, 0, len(fields))
	for _, field := range fields {
		values = append(values, string(field))
	}
	return values
}

func workflowTriggerOperationCompletions() []string {
	operations := workflow.ValidTriggerOperations()
	values := make([]string, 0, len(operations))
	for _, operation := range operations {
		values = append(values, string(operation))
	}
	return values
}

func workflowActionTypeCompletions() []string {
	actions := workflow.ValidActionTypes()
	values := make([]string, 0, len(actions))
	for _, action := range actions {
		values = append(values, string(action))
	}
	return values
}
