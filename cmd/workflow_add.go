package cmd

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tterrors"
	"github.com/sglavoie/tt/internal/workflow"
)

var workflowAddCmd = &cobra.Command{
	Use:   "add",
	Short: "Add a new workflow rule",
	Long: `Add a new workflow automation rule.

Rules are defined with trigger condition(s) and one or more actions.
When a task matches the trigger(s), the actions are applied automatically.

STATE TRIGGER FIELDS (match current task state):
  status, priority, section, tags, type
  due_days, created_days, updated_days

BOOLEAN STATE TRIGGER FIELDS (presence/absence checks):
  has_due_date      - Value: "true"/"false"
  has_tags          - Value: "true"/"false"
  description_empty - Value: "true"/"false"

EVENT TRIGGER FIELDS (detect transitions):
  status_changed        - Value: "old->new" (e.g., "blocked->*", "*->done")
  section_changed       - Value: "old->new" (e.g., "Backlog->Active")
  priority_changed      - Value: "old->new" (e.g., "*->1")
  tag_added             - Value: tag name that was added
  tag_removed           - Value: tag name that was removed
  completed             - Value: "true" (fires on done/canceled)
  reopened              - Value: "true" (fires when reopened)
  due_date_changed      - Value: "added"/"removed"/"changed"/"*"
  type_changed          - Value: "old->new" (e.g., "task->bug")
  recurrence_completed  - Value: "true" (recurring task completes cycle)
  streak_broken         - Value: "true" (recurrence streak resets to zero)

HIERARCHICAL TRIGGER FIELDS:
  parent_status, parent_type, parent_tags, level
  has_children, children_progress, all_children_done
  is_blocked, blocker_count

TRIGGER OPERATIONS:
  equals, not_equals, contains, greater_than, less_than

ACTION TYPES:
  set_status, set_priority, set_section, add_tag, remove_tag
  set_due_relative, clear_due, set_active, set_pinned, add_comment, set_type
  complete_parent, block_parent, cascade_tag, inherit_tag, notify_parent
  set_snooze, create_followup, move_to_project, set_recurrence

RULE SCOPING:
  Restrict rules to specific projects, types, or sections:
    --scope-project "MyProject"    Only apply to tasks in this project
    --scope-type task,bug          Only apply to these task types
    --scope-section Active         Only apply to tasks in these sections

COMPOUND TRIGGERS:
  Use multiple -f/-o/-V flag groups to create compound triggers.
  Use --logic to specify AND (default) or OR logic.

Examples:
  # When unblocked, add ready tag (event trigger)
  tt workflow add -n "Unblock Ready" -f status_changed -o equals -V "blocked->*" -a add_tag --action-value ready

  # When completed, move to Archive
  tt workflow add -n "Archive Done" -f completed -o equals -V true -a set_section --action-value Archive

  # Auto-complete epic when all children done (hierarchical)
  tt workflow add -n "Epic Auto-Complete" -f type -o equals -V epic -f all_children_done -o equals -V true --logic and -a set_status --action-value done

  # Inherit urgent tag from parent
  tt workflow add -n "Inherit Urgent" -f parent_tags -o contains -V urgent -a inherit_tag --action-value urgent

  # Tag overdue tasks
  tt workflow add -n "Overdue Alert" -f due_days -o less_than -V 0 -a add_tag --action-value overdue

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
	scopeProjects []string
	scopeTypes    []string
	scopeSections []string
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
	workflowAddCmd.Flags().StringSliceVar(&scopeProjects, "scope-project", nil, "restrict rule to projects (comma-separated names or IDs)")
	workflowAddCmd.Flags().StringSliceVar(&scopeTypes, "scope-type", nil, "restrict rule to task types (comma-separated)")
	workflowAddCmd.Flags().StringSliceVar(&scopeSections, "scope-section", nil, "restrict rule to sections (comma-separated)")

	_ = workflowAddCmd.MarkFlagRequired("name")
	_ = workflowAddCmd.MarkFlagRequired("field")
	_ = workflowAddCmd.MarkFlagRequired("value")
	_ = workflowAddCmd.MarkFlagRequired("action")
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
		// State triggers
		workflow.FieldStatus:      true,
		workflow.FieldPriority:    true,
		workflow.FieldSection:     true,
		workflow.FieldTags:        true,
		workflow.FieldType:        true,
		workflow.FieldDueDays:     true,
		workflow.FieldCreatedDays: true,
		workflow.FieldUpdatedDays: true,
		// Event triggers
		workflow.FieldStatusChanged:       true,
		workflow.FieldSectionChanged:      true,
		workflow.FieldPriorityChanged:     true,
		workflow.FieldTagAdded:            true,
		workflow.FieldTagRemoved:          true,
		workflow.FieldCompleted:           true,
		workflow.FieldReopened:            true,
		workflow.FieldDueDateChanged:      true,
		workflow.FieldTypeChanged:         true,
		workflow.FieldRecurrenceCompleted: true,
		workflow.FieldStreakBroken:        true,
		// Boolean state triggers
		workflow.FieldHasDueDate:       true,
		workflow.FieldHasTags:          true,
		workflow.FieldDescriptionEmpty: true,
		// Hierarchical triggers
		workflow.FieldParentStatus:     true,
		workflow.FieldParentType:       true,
		workflow.FieldParentTags:       true,
		workflow.FieldLevel:            true,
		workflow.FieldHasChildren:      true,
		workflow.FieldChildrenProgress: true,
		workflow.FieldAllChildrenDone:  true,
		workflow.FieldIsBlocked:        true,
		workflow.FieldBlockerCount:     true,
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
			return fmt.Errorf("invalid trigger field: %s (run 'tt workflow add --help' for valid fields)", triggerFields[i])
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
		// Hierarchical actions
		workflow.ActionCompleteParent: true,
		workflow.ActionBlockParent:    true,
		workflow.ActionCascadeTag:     true,
		workflow.ActionInheritTag:     true,
		workflow.ActionNotifyParent:   true,
		// New actions
		workflow.ActionSetSnooze:      true,
		workflow.ActionCreateFollowup: true,
		workflow.ActionMoveToProject:  true,
		workflow.ActionSetRecurrence:  true,
	}
	if !validActions[action] {
		return fmt.Errorf("invalid action type: %s (run 'tt workflow add --help' for valid actions)", actionType)
	}

	// Actions that don't require a value
	noValueRequired := map[workflow.ActionType]bool{
		workflow.ActionSetSection:     true, // empty = Active
		workflow.ActionClearDue:       true,
		workflow.ActionCompleteParent: true,
		workflow.ActionBlockParent:    true,
	}

	// Validate action value based on action type
	if !noValueRequired[action] && actionValue == "" {
		return fmt.Errorf("action value required for %s", action)
	}
	if action == workflow.ActionSetPriority {
		p, err := strconv.Atoi(actionValue)
		if err != nil || ValidatePriority(p, false) != nil {
			return fmt.Errorf("priority must be %d-%d, got: %s", models.PriorityMin, models.PriorityMax, actionValue)
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
	rule.Scope = buildScopeFromFlags(scopeProjects, scopeTypes, scopeSections)

	if err := rule.Validate(); err != nil {
		return fmt.Errorf("invalid rule: %w", err)
	}

	cfg, configManager, err := LoadConfigStrict()
	if err != nil {
		return err
	}

	cfg.AddWorkflowRule(*rule)

	// Invalidate workflow eval states so all projects re-evaluate with new rule
	cfg.InvalidateWorkflowEvalStates()

	if err := configManager.Save(cfg); err != nil {
		return tterrors.NewSaveConfigError(err)
	}

	fmt.Printf("Created workflow rule: %s [%s]\n", rule.Name, rule.ID)
	fmt.Printf("  %s\n", rule.Summary())
	if !rule.Enabled {
		fmt.Println("  (disabled)")
	}

	return nil
}
