package cmd

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/workflow"
	"github.com/spf13/cobra"
)

var workflowAddCmd = &cobra.Command{
	Use:   "add",
	Short: "Add a new workflow rule",
	Long: `Add a new workflow automation rule.

Rules are defined with a trigger condition and one or more actions.
When a task matches the trigger, the actions are applied automatically.

Trigger fields: status, priority, section, tags, type
Trigger operations: equals, not_equals, contains, greater_than, less_than
Action types: set_status, set_priority, set_section, add_tag, remove_tag

Examples:
  # Move high-priority tasks to Active
  tt workflow add --name "High Priority Active" --field priority --op equals --value 1 --action set_section --action-value ""

  # Add 'urgent' tag when task is blocked
  tt workflow add --name "Blocked Tag" --field status --op equals --value blocked --action add_tag --action-value urgent

  # Move in-progress tasks out of Backlog
  tt workflow add --name "Start Moves Active" --field status --op equals --value in_progress --action set_section --action-value ""`,
	RunE: runWorkflowAdd,
}

var (
	ruleName     string
	triggerField string
	triggerOp    string
	triggerValue string
	actionType   string
	actionValue  string
	ruleDisabled bool
)

func init() {
	workflowCmd.AddCommand(workflowAddCmd)

	workflowAddCmd.Flags().StringVarP(&ruleName, "name", "n", "", "rule name (required)")
	workflowAddCmd.Flags().StringVarP(&triggerField, "field", "f", "", "trigger field: status, priority, section, tags, type (required)")
	workflowAddCmd.Flags().StringVarP(&triggerOp, "op", "o", "equals", "trigger operation: equals, not_equals, contains, greater_than, less_than")
	workflowAddCmd.Flags().StringVarP(&triggerValue, "value", "V", "", "trigger value (required)")
	workflowAddCmd.Flags().StringVarP(&actionType, "action", "a", "", "action type: set_status, set_priority, set_section, add_tag, remove_tag (required)")
	workflowAddCmd.Flags().StringVar(&actionValue, "action-value", "", "action value (empty string for set_section means Active)")
	workflowAddCmd.Flags().BoolVar(&ruleDisabled, "disabled", false, "create rule in disabled state")

	workflowAddCmd.MarkFlagRequired("name")
	workflowAddCmd.MarkFlagRequired("field")
	workflowAddCmd.MarkFlagRequired("value")
	workflowAddCmd.MarkFlagRequired("action")
}

func runWorkflowAdd(cmd *cobra.Command, args []string) error {
	// Validate trigger field
	field := workflow.TriggerField(strings.ToLower(triggerField))
	validFields := map[workflow.TriggerField]bool{
		workflow.FieldStatus:   true,
		workflow.FieldPriority: true,
		workflow.FieldSection:  true,
		workflow.FieldTags:     true,
		workflow.FieldType:     true,
	}
	if !validFields[field] {
		return fmt.Errorf("invalid trigger field: %s (valid: status, priority, section, tags, type)", triggerField)
	}

	// Validate trigger operation
	op := workflow.TriggerOperation(strings.ToLower(triggerOp))
	validOps := map[workflow.TriggerOperation]bool{
		workflow.OpEquals:      true,
		workflow.OpNotEquals:   true,
		workflow.OpContains:    true,
		workflow.OpGreaterThan: true,
		workflow.OpLessThan:    true,
	}
	if !validOps[op] {
		return fmt.Errorf("invalid trigger operation: %s (valid: equals, not_equals, contains, greater_than, less_than)", triggerOp)
	}

	// Validate action type
	action := workflow.ActionType(strings.ToLower(actionType))
	validActions := map[workflow.ActionType]bool{
		workflow.ActionSetStatus:   true,
		workflow.ActionSetPriority: true,
		workflow.ActionSetSection:  true,
		workflow.ActionAddTag:      true,
		workflow.ActionRemoveTag:   true,
	}
	if !validActions[action] {
		return fmt.Errorf("invalid action type: %s (valid: set_status, set_priority, set_section, add_tag, remove_tag)", actionType)
	}

	// Validate action value based on action type
	if action != workflow.ActionSetSection && actionValue == "" {
		return fmt.Errorf("action value required for %s", action)
	}
	if action == workflow.ActionSetPriority {
		p, err := strconv.Atoi(actionValue)
		if err != nil || p < 1 || p > 5 {
			return fmt.Errorf("priority must be 1-5, got: %s", actionValue)
		}
	}

	trigger := workflow.RuleTrigger{
		Field:     field,
		Operation: op,
		Value:     triggerValue,
	}

	actions := []workflow.RuleAction{
		{ActionType: action, Value: actionValue},
	}

	rule := workflow.NewRule(ruleName, trigger, actions)
	rule.Enabled = !ruleDisabled

	if err := rule.Validate(); err != nil {
		return fmt.Errorf("invalid rule: %w", err)
	}

	configManager := config.NewManager(GetConfigPath())
	cfg, err := configManager.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	cfg.AddWorkflowRule(*rule)

	if err := configManager.Save(cfg); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Printf("Created workflow rule: %s [%s]\n", rule.Name, rule.ID)
	fmt.Printf("  %s\n", rule.Summary())
	if !rule.Enabled {
		fmt.Println("  (disabled)")
	}

	return nil
}
