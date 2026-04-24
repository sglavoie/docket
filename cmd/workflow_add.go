package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

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
  is_blocked, blocker_count, dependency_count

TRIGGER OPERATIONS:
  equals, not_equals, contains, greater_than, less_than

ACTION TYPES:
  set_status, set_priority, set_section, add_tag, remove_tag
  set_due_relative, clear_due, set_active, set_pinned, add_comment, set_type
  complete_parent, block_parent, cascade_tag, inherit_tag, notify_parent
  set_snooze, create_followup, move_to_project, set_recurrence
  set_project_status (project-level only)

PROJECT-LEVEL RULES:
  Use --level project to create rules that operate on aggregate task data.
  Project triggers: has_in_progress_tasks, active_task_count, all_tasks_status,
    majority_task_type, majority_task_status, completion_ratio,
    days_since_update, project_status, has_tasks_with_status
  Project action: set_project_status (value = icon key: planning, active, etc.)

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
  tt workflow add -n "Stale WIP" -f updated_days -o greater_than -V 7 -f status -o equals -V in_progress --logic and -a add_tag --action-value stale

CONDITIONAL ACTIONS:
  Add an optional condition to the action. The rule's triggers decide if the
  rule fires; the condition is a secondary gate on the specific action.
    --cond-field priority   Only run action if this field matches
    --cond-op greater_than  Condition operation
    --cond-value 2          Condition value

Examples:
  # Set section based on priority (conditional branching)
  tt workflow add -n "Priority Route" -f status -o equals -V todo \
    -a set_section --action-value Active \
    --cond-field priority --cond-op less_than --cond-value 3`,
	RunE: runWorkflowAdd,
}

var (
	ruleName      string
	ruleLevel     string
	rulePriority  int
	triggerFields []string
	triggerOps    []string
	triggerValues []string
	triggerLogic  string
	actionType    string
	actionValue   string
	condField     string
	condOp        string
	condValue     string
	ruleDisabled  bool
	scopeProjects []string
	scopeTypes    []string
	scopeSections []string
)

func init() {
	workflowCmd.AddCommand(workflowAddCmd)

	workflowAddCmd.Flags().StringVarP(&ruleName, "name", "n", "", "rule name (required)")
	workflowAddCmd.Flags().StringVar(&ruleLevel, "level", "task", "rule level: task (default) or project")
	workflowAddCmd.Flags().IntVar(&rulePriority, "priority", 0, "rule priority (1=highest, runs first; 0=default, runs last)")
	workflowAddCmd.Flags().StringArrayVarP(&triggerFields, "field", "f", nil, "trigger field: status, priority, section, tags, type (can specify multiple)")
	workflowAddCmd.Flags().StringArrayVarP(&triggerOps, "op", "o", nil, "trigger operation: equals, not_equals, contains, greater_than, less_than (default: equals)")
	workflowAddCmd.Flags().StringArrayVarP(&triggerValues, "value", "V", nil, "trigger value (can specify multiple)")
	workflowAddCmd.Flags().StringVar(&triggerLogic, "logic", "and", "trigger logic for compound rules: and, or")
	workflowAddCmd.Flags().StringVarP(&actionType, "action", "a", "", "action type (required)")
	workflowAddCmd.Flags().StringVar(&actionValue, "action-value", "", "action value (empty string for set_section means Active)")
	workflowAddCmd.Flags().StringVar(&condField, "cond-field", "", "condition field for the action (action only runs if condition matches)")
	workflowAddCmd.Flags().StringVar(&condOp, "cond-op", "equals", "condition operation (default: equals)")
	workflowAddCmd.Flags().StringVar(&condValue, "cond-value", "", "condition value")
	workflowAddCmd.Flags().BoolVar(&ruleDisabled, "disabled", false, "create rule in disabled state")
	workflowAddCmd.Flags().StringSliceVar(&scopeProjects, "scope-project", nil, "restrict rule to projects (comma-separated names or IDs)")
	workflowAddCmd.Flags().StringSliceVar(&scopeTypes, "scope-type", nil, "restrict rule to task types (comma-separated)")
	workflowAddCmd.Flags().StringSliceVar(&scopeSections, "scope-section", nil, "restrict rule to sections (comma-separated)")

	_ = workflowAddCmd.MarkFlagRequired("name")
	_ = workflowAddCmd.MarkFlagRequired("field")
	_ = workflowAddCmd.MarkFlagRequired("value")
	_ = workflowAddCmd.MarkFlagRequired("action")
	addOutputFlags(workflowAddCmd)
}

func runWorkflowAdd(cmd *cobra.Command, args []string) error {
	out, err := getOutputOpts(cmd)
	if err != nil {
		return err
	}

	// Build and validate triggers
	triggers, err := buildTriggersFromFlags(triggerFields, triggerOps, triggerValues)
	if err != nil {
		return err
	}

	// Validate trigger logic
	logic := workflow.TriggerLogic(strings.ToLower(triggerLogic))
	if logic != workflow.LogicAnd && logic != workflow.LogicOr {
		return fmt.Errorf("invalid trigger logic: %s (valid: and, or)", triggerLogic)
	}

	// Build and validate action
	ruleAction, err := buildActionFromFlags(actionType, actionValue)
	if err != nil {
		return err
	}

	// Build action condition if specified
	if condField != "" {
		cond, condErr := buildConditionFromFlags(condField, condOp, condValue)
		if condErr != nil {
			return condErr
		}
		ruleAction.Condition = cond
	}

	actions := []workflow.RuleAction{ruleAction}

	// Create rule - use compound constructor if multiple triggers
	var rule *workflow.WorkflowRule
	if len(triggers) == 1 {
		rule = workflow.NewRule(ruleName, triggers[0], actions)
	} else {
		rule = workflow.NewCompoundRule(ruleName, triggers, logic, actions)
	}
	rule.Enabled = !ruleDisabled
	level := workflow.RuleLevel(strings.ToLower(ruleLevel))
	if level != workflow.RuleLevelTask && level != workflow.RuleLevelProject {
		return fmt.Errorf("invalid level: %s (valid: task, project)", ruleLevel)
	}
	if level == workflow.RuleLevelProject {
		rule.Level = workflow.RuleLevelProject
	}
	if rulePriority < 0 || rulePriority > 99 {
		return fmt.Errorf("priority must be 0-99, got: %d", rulePriority)
	}
	rule.Priority = rulePriority
	rule.Scope = buildScopeFromFlags(scopeProjects, scopeTypes, scopeSections)

	err = rule.Validate()
	if err != nil {
		return fmt.Errorf("invalid rule: %w", err)
	}

	cfg, configManager, err := LoadConfigStrict()
	if err != nil {
		return err
	}

	cfg.AddWorkflowRule(*rule)

	// Invalidate workflow eval states so all projects re-evaluate with new rule
	cfg.InvalidateWorkflowEvalStates()

	if err := saveConfig(configManager, cfg); err != nil {
		return err
	}

	if out.Quiet {
		return nil
	}
	if out.JSON {
		return printJSON(struct {
			ID      string `json:"id"`
			Name    string `json:"name"`
			Enabled bool   `json:"enabled"`
			Summary string `json:"summary"`
		}{ID: rule.ID, Name: rule.Name, Enabled: rule.Enabled, Summary: rule.Summary()})
	}
	fmt.Printf("Created workflow rule: %s [%s]\n", rule.Name, rule.ID)
	if rule.IsProjectLevel() {
		fmt.Println("  Level: project")
	}
	fmt.Printf("  %s\n", rule.Summary())
	if !rule.Enabled {
		fmt.Println("  (disabled)")
	}

	return nil
}
