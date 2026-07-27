package cmd

import (
	"fmt"
	"strings"

	"github.com/sglavoie/tt/internal/workflow"
)

// collectValidationErrors validates a rule and returns all errors found.
func collectValidationErrors(rule *workflow.WorkflowRule) []string {
	var errors []string

	if rule.Name == "" {
		errors = append(errors, "rule name cannot be empty")
	}

	if rule.TriggerGroup != nil {
		if err := rule.TriggerGroup.Validate(); err != nil {
			errors = append(errors, fmt.Sprintf("trigger group: %s", err.Error()))
		}
	} else {
		triggers := rule.GetTriggers()
		if len(triggers) == 0 {
			errors = append(errors, "at least one trigger is required")
		} else {
			for i, trigger := range triggers {
				if err := trigger.ValidateForAuthoring(); err != nil {
					errors = append(errors, fmt.Sprintf("trigger %d: %s", i, err.Error()))
				}
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
			if err := action.ValidateForAuthoring(); err != nil {
				errors = append(errors, fmt.Sprintf("action %d: %s", i, err.Error()))
			}
		}
	}

	// Validate scope and level consistency (matching WorkflowRule.Validate).
	if err := rule.ValidateForAuthoring(); err != nil {
		// Only append scope/level errors that aren't already captured above.
		errMsg := err.Error()
		if strings.Contains(errMsg, "invalid scope") ||
			strings.Contains(errMsg, "task-level rule") ||
			strings.Contains(errMsg, "project-level rule") {
			errors = append(errors, errMsg)
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
