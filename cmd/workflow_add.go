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

Rules are defined with trigger condition(s) and one or more actions.
When a task matches the trigger(s), the actions are applied automatically.

Trigger fields: status, priority, section, tags, type, due_days, created_days, updated_days
Trigger operations: equals, not_equals, contains, greater_than, less_than
Action types:
  set_status, set_priority, set_section, add_tag, remove_tag,
  set_due_relative, clear_due, set_active, set_pinned, add_comment, set_type

Compound Triggers:
  Use multiple -f/-o/-V flag groups to create compound triggers.
  Use --logic to specify AND (default) or OR logic.

Examples:
  # Move high-priority tasks to Active
  tt workflow add -n "High Priority Active" -f priority -o equals -V 1 -a set_section --action-value ""

  # Add 'urgent' tag when task is blocked
  tt workflow add -n "Blocked Tag" -f status -o equals -V blocked -a add_tag --action-value urgent

  # Compound: High-priority AND blocked tasks get urgent tag
  tt workflow add -n "Urgent Blocked" -f status -o equals -V blocked -f priority -o equals -V 1 --logic and -a add_tag --action-value urgent

  # Compound OR: Active section OR urgent tag -> high priority
  tt workflow add -n "Prioritize" -f section -o equals -V "" -f tags -o contains -V urgent --logic or -a set_priority --action-value 1

  # Set due date 3 days out when moved to Active section
  tt workflow add -n "Active Due" -f section -o equals -V "" -a set_due_relative --action-value "+3d"

  # Tag overdue tasks
  tt workflow add -n "Overdue Alert" -f due_days -o less_than -V 0 -a add_tag --action-value overdue

  # Bump priority when due within 3 days
  tt workflow add -n "Due Soon" -f due_days -o less_than -V 3 -a set_priority --action-value 1

  # Tag stale in-progress tasks (not updated in 7 days)
  tt workflow add -n "Stale WIP" -f updated_days -o greater_than -V 7 -f status -o equals -V in_progress --logic and -a add_tag --action-value stale`,
	RunE: runWorkflowAdd,
}

var (
	ruleName      string
	triggerFields []string
	triggerOps    []string
	triggerValues []string
	triggerLogic  string
	actionType    string
	actionValue   string
	ruleDisabled  bool
)

func init() {
	workflowCmd.AddCommand(workflowAddCmd)

	workflowAddCmd.Flags().StringVarP(&ruleName, "name", "n", "", "rule name (required)")
	workflowAddCmd.Flags().StringArrayVarP(&triggerFields, "field", "f", nil, "trigger field: status, priority, section, tags, type (can specify multiple)")
	workflowAddCmd.Flags().StringArrayVarP(&triggerOps, "op", "o", nil, "trigger operation: equals, not_equals, contains, greater_than, less_than (default: equals)")
	workflowAddCmd.Flags().StringArrayVarP(&triggerValues, "value", "V", nil, "trigger value (can specify multiple)")
	workflowAddCmd.Flags().StringVar(&triggerLogic, "logic", "and", "trigger logic for compound rules: and, or")
	workflowAddCmd.Flags().StringVarP(&actionType, "action", "a", "", "action type (required)")
	workflowAddCmd.Flags().StringVar(&actionValue, "action-value", "", "action value (empty string for set_section means Active)")
	workflowAddCmd.Flags().BoolVar(&ruleDisabled, "disabled", false, "create rule in disabled state")

	workflowAddCmd.MarkFlagRequired("name")
	workflowAddCmd.MarkFlagRequired("field")
	workflowAddCmd.MarkFlagRequired("value")
	workflowAddCmd.MarkFlagRequired("action")
}

func runWorkflowAdd(cmd *cobra.Command, args []string) error {
	// Validate trigger counts match
	if len(triggerFields) != len(triggerValues) {
		return fmt.Errorf("number of --field flags (%d) must match --value flags (%d)", len(triggerFields), len(triggerValues))
	}

	// Default ops to "equals" if not specified
	if len(triggerOps) == 0 {
		triggerOps = make([]string, len(triggerFields))
		for i := range triggerOps {
			triggerOps[i] = "equals"
		}
	} else if len(triggerOps) != len(triggerFields) {
		return fmt.Errorf("number of --op flags (%d) must match --field flags (%d), or omit --op to default to equals", len(triggerOps), len(triggerFields))
	}

	validFields := map[workflow.TriggerField]bool{
		workflow.FieldStatus:      true,
		workflow.FieldPriority:    true,
		workflow.FieldSection:     true,
		workflow.FieldTags:        true,
		workflow.FieldType:        true,
		workflow.FieldDueDays:     true,
		workflow.FieldCreatedDays: true,
		workflow.FieldUpdatedDays: true,
	}
	validOps := map[workflow.TriggerOperation]bool{
		workflow.OpEquals:      true,
		workflow.OpNotEquals:   true,
		workflow.OpContains:    true,
		workflow.OpGreaterThan: true,
		workflow.OpLessThan:    true,
	}

	// Build triggers
	var triggers []workflow.RuleTrigger
	for i := range triggerFields {
		field := workflow.TriggerField(strings.ToLower(triggerFields[i]))
		if !validFields[field] {
			return fmt.Errorf("invalid trigger field: %s (valid: status, priority, section, tags, type, due_days, created_days, updated_days)", triggerFields[i])
		}

		op := workflow.TriggerOperation(strings.ToLower(triggerOps[i]))
		if !validOps[op] {
			return fmt.Errorf("invalid trigger operation: %s (valid: equals, not_equals, contains, greater_than, less_than)", triggerOps[i])
		}

		triggers = append(triggers, workflow.RuleTrigger{
			Field:     field,
			Operation: op,
			Value:     triggerValues[i],
		})
	}

	// Validate trigger logic
	logic := workflow.TriggerLogic(strings.ToLower(triggerLogic))
	if logic != workflow.LogicAnd && logic != workflow.LogicOr {
		return fmt.Errorf("invalid trigger logic: %s (valid: and, or)", triggerLogic)
	}

	// Validate action type
	action := workflow.ActionType(strings.ToLower(actionType))
	validActions := map[workflow.ActionType]bool{
		workflow.ActionSetStatus:      true,
		workflow.ActionSetPriority:    true,
		workflow.ActionSetSection:     true,
		workflow.ActionAddTag:         true,
		workflow.ActionRemoveTag:      true,
		workflow.ActionSetDueRelative: true,
		workflow.ActionClearDue:       true,
		workflow.ActionSetActive:      true,
		workflow.ActionSetPinned:      true,
		workflow.ActionAddComment:     true,
		workflow.ActionSetType:        true,
	}
	if !validActions[action] {
		return fmt.Errorf("invalid action type: %s (valid: set_status, set_priority, set_section, add_tag, remove_tag, set_due_relative, clear_due, set_active, set_pinned, add_comment, set_type)", actionType)
	}

	// Actions that don't require a value
	noValueRequired := map[workflow.ActionType]bool{
		workflow.ActionSetSection: true, // empty = Active
		workflow.ActionClearDue:   true,
	}

	// Validate action value based on action type
	if !noValueRequired[action] && actionValue == "" {
		return fmt.Errorf("action value required for %s", action)
	}
	if action == workflow.ActionSetPriority {
		p, err := strconv.Atoi(actionValue)
		if err != nil || p < 1 || p > 5 {
			return fmt.Errorf("priority must be 1-5, got: %s", actionValue)
		}
	}
	if action == workflow.ActionSetActive || action == workflow.ActionSetPinned {
		if actionValue != "true" && actionValue != "false" {
			return fmt.Errorf("%s value must be 'true' or 'false'", action)
		}
	}
	if action == workflow.ActionSetDueRelative {
		if _, err := workflow.ParseRelativeDuration(actionValue); err != nil {
			return fmt.Errorf("invalid relative duration: %s (use format like +3d, +1w, +2m)", actionValue)
		}
	}

	actions := []workflow.RuleAction{
		{ActionType: action, Value: actionValue},
	}

	// Create rule - use compound constructor if multiple triggers
	var rule *workflow.WorkflowRule
	if len(triggers) == 1 {
		rule = workflow.NewRule(ruleName, triggers[0], actions)
	} else {
		rule = workflow.NewCompoundRule(ruleName, triggers, logic, actions)
	}
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
