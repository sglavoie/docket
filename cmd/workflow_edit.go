package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/tterrors"
	"github.com/sglavoie/tt/internal/workflow"
)

var (
	// Simple field updates
	workflowEditName     string
	workflowEditLevel    string
	workflowEditEnabled  bool
	workflowEditPriority int

	// Trigger modifications
	workflowEditAddTriggerFields []string
	workflowEditAddTriggerOps    []string
	workflowEditAddTriggerValues []string
	workflowEditRemoveTriggers   string // Comma-separated indices
	workflowEditSetTriggerLogic  string
	workflowEditClearTriggers    bool

	// Action modifications
	workflowEditAddActionType  string
	workflowEditAddActionValue string
	workflowEditCondField      string
	workflowEditCondOp         string
	workflowEditCondValue      string
	workflowEditRemoveActions  string // Comma-separated indices
	workflowEditClearActions   bool

	// Scope flags
	workflowEditScopeProjects []string
	workflowEditScopeTypes    []string
	workflowEditScopeSections []string
	workflowEditClearScope    bool

	// Confirmation flag
	workflowEditYes bool
)

var workflowEditCmd = &cobra.Command{
	Use:   "edit <id>",
	Short: "Edit a workflow rule",
	Long: `Edit a workflow rule's name, enabled state, triggers, or actions.

Use flags to specify which fields to update. Multiple flags can be combined.
Changes are previewed first; use --yes to apply.

Examples:
  # Rename a rule (shows preview)
  tt workflow edit abc123 --name "New Name"
  
  # Apply the change
  tt workflow edit abc123 --name "New Name" --yes
  
  # Enable/disable a rule
  tt workflow edit abc123 --enabled=false --yes
  
  # Add triggers (multiple in one command)
  tt workflow edit abc123 \
    -f status -o equals -V blocked \
    -f priority -o greater_than -V 3 \
    --yes
  
  # Remove triggers by index (use 'tt workflow show' to see indices)
  tt workflow edit abc123 --remove-trigger 0,2 --yes
  
  # Add an action
  tt workflow edit abc123 --add-action add_tag --add-action-value urgent --yes
  
  # Remove action by index
  tt workflow edit abc123 --remove-action 0 --yes
  
  # Change trigger logic
  tt workflow edit abc123 --set-logic or --yes
  
  # Complex edit combining multiple operations
  tt workflow edit abc123 \
    --name "Enhanced Rule" \
    --set-logic or \
    -f due_days -o less_than -V 0 \
    --yes`,
	Args: cobra.ExactArgs(1),
	RunE: runWorkflowEdit,
}

func init() {
	workflowCmd.AddCommand(workflowEditCmd)

	// Simple field flags
	workflowEditCmd.Flags().StringVarP(&workflowEditName, "name", "n", "", "new rule name")
	workflowEditCmd.Flags().StringVar(&workflowEditLevel, "level", "", "rule level: task or project")
	workflowEditCmd.Flags().BoolVar(&workflowEditEnabled, "enabled", false, "enable or disable rule (must set explicitly)")
	workflowEditCmd.Flags().IntVar(&workflowEditPriority, "priority", 0, "rule priority (1=highest, runs first; 0=default, runs last)")

	// Trigger flags
	workflowEditCmd.Flags().StringSliceVarP(&workflowEditAddTriggerFields, "add-trigger-field", "f", nil, "add trigger field (can specify multiple)")
	workflowEditCmd.Flags().StringSliceVarP(&workflowEditAddTriggerOps, "add-trigger-op", "o", nil, "add trigger operation (can specify multiple, defaults to 'equals')")
	workflowEditCmd.Flags().StringSliceVarP(&workflowEditAddTriggerValues, "add-trigger-value", "V", nil, "add trigger value (can specify multiple)")
	workflowEditCmd.Flags().StringVar(&workflowEditRemoveTriggers, "remove-trigger", "", "remove triggers by index (comma-separated, e.g., '0,2')")
	workflowEditCmd.Flags().StringVar(&workflowEditSetTriggerLogic, "set-logic", "", "set trigger logic (and/or)")
	workflowEditCmd.Flags().BoolVar(&workflowEditClearTriggers, "clear-triggers", false, "clear all triggers (use with add-trigger to replace)")

	// Action flags
	workflowEditCmd.Flags().StringVarP(&workflowEditAddActionType, "add-action", "a", "", "add action type")
	workflowEditCmd.Flags().StringVar(&workflowEditAddActionValue, "add-action-value", "", "add action value")
	workflowEditCmd.Flags().StringVar(&workflowEditCondField, "cond-field", "", "condition field for the added action")
	workflowEditCmd.Flags().StringVar(&workflowEditCondOp, "cond-op", "equals", "condition operation for the added action")
	workflowEditCmd.Flags().StringVar(&workflowEditCondValue, "cond-value", "", "condition value for the added action")
	workflowEditCmd.Flags().StringVar(&workflowEditRemoveActions, "remove-action", "", "remove actions by index (comma-separated, e.g., '0,1')")
	workflowEditCmd.Flags().BoolVar(&workflowEditClearActions, "clear-actions", false, "clear all actions (use with add-action to replace)")

	// Scope flags
	workflowEditCmd.Flags().StringSliceVar(&workflowEditScopeProjects, "scope-project", nil, "set rule scope: project names or IDs (comma-separated)")
	workflowEditCmd.Flags().StringSliceVar(&workflowEditScopeTypes, "scope-type", nil, "set rule scope: task types (comma-separated)")
	workflowEditCmd.Flags().StringSliceVar(&workflowEditScopeSections, "scope-section", nil, "set rule scope: sections (comma-separated)")
	workflowEditCmd.Flags().BoolVar(&workflowEditClearScope, "clear-scope", false, "remove all scope restrictions")

	// Confirmation flag
	workflowEditCmd.Flags().BoolVarP(&workflowEditYes, "yes", "y", false, "apply changes without preview confirmation")
}

func runWorkflowEdit(cmd *cobra.Command, args []string) error {
	ruleID := args[0]

	// Load config
	cfg, configManager, err := LoadConfigStrict()
	if err != nil {
		return err
	}

	// Get the rule to edit
	rule := cfg.GetWorkflowRule(ruleID)
	if rule == nil {
		return fmt.Errorf("workflow rule not found: %s", ruleID)
	}

	// Create a copy for validation and preview
	modifiedRule := *rule

	// Check for mutually exclusive flags
	if err := detectMutuallyExclusiveEditFlags(cmd); err != nil {
		return err
	}

	// Track if any changes were requested
	hasChanges := false

	// Apply simple field updates
	if cmd.Flags().Changed("name") {
		if strings.TrimSpace(workflowEditName) == "" {
			return fmt.Errorf("rule name cannot be empty")
		}
		modifiedRule.Name = strings.TrimSpace(workflowEditName)
		hasChanges = true
	}

	if cmd.Flags().Changed("level") {
		level := workflow.RuleLevel(strings.ToLower(workflowEditLevel))
		if level != workflow.RuleLevelTask && level != workflow.RuleLevelProject {
			return fmt.Errorf("invalid level: %s (valid: task, project)", workflowEditLevel)
		}
		if level == workflow.RuleLevelProject {
			modifiedRule.Level = workflow.RuleLevelProject
		} else {
			modifiedRule.Level = ""
		}
		hasChanges = true
	}

	if cmd.Flags().Changed("enabled") {
		modifiedRule.Enabled = workflowEditEnabled
		hasChanges = true
	}

	if cmd.Flags().Changed("priority") {
		if workflowEditPriority < 0 || workflowEditPriority > 99 {
			return fmt.Errorf("priority must be 0-99, got: %d", workflowEditPriority)
		}
		modifiedRule.Priority = workflowEditPriority
		hasChanges = true
	}

	// Handle trigger modifications
	if workflowEditClearTriggers {
		// Check that we're adding new triggers when clearing
		if len(workflowEditAddTriggerFields) == 0 {
			return fmt.Errorf("cannot clear all triggers without adding new ones (use --add-trigger-field)")
		}
		modifiedRule.Triggers = []workflow.RuleTrigger{}
		modifiedRule.Trigger = workflow.RuleTrigger{} // Clear legacy field
		hasChanges = true
	}

	if len(workflowEditAddTriggerFields) > 0 {
		// Handle legacy single trigger field migration
		if len(modifiedRule.Triggers) == 0 && modifiedRule.Trigger.Field != "" {
			modifiedRule.Triggers = []workflow.RuleTrigger{modifiedRule.Trigger}
			modifiedRule.Trigger = workflow.RuleTrigger{}
		}

		newTriggers, err := buildTriggersFromFlags(workflowEditAddTriggerFields, workflowEditAddTriggerOps, workflowEditAddTriggerValues)
		if err != nil {
			return err
		}
		modifiedRule.Triggers = append(modifiedRule.Triggers, newTriggers...)
		hasChanges = true
	}

	if workflowEditRemoveTriggers != "" {
		indices, err := parseCommaSeparatedIndices(workflowEditRemoveTriggers)
		if err != nil {
			return fmt.Errorf("invalid --remove-trigger indices: %w", err)
		}
		modifiedRule.Triggers, err = removeByIndices(modifiedRule.Triggers, indices)
		if err != nil {
			return fmt.Errorf("failed to remove triggers: %w", err)
		}
		hasChanges = true
	}

	if cmd.Flags().Changed("set-logic") {
		logic := workflow.TriggerLogic(strings.ToLower(workflowEditSetTriggerLogic))
		if logic != workflow.LogicAnd && logic != workflow.LogicOr {
			return fmt.Errorf("invalid trigger logic: %s (valid: and, or)", workflowEditSetTriggerLogic)
		}
		modifiedRule.TriggerLogic = logic
		hasChanges = true
	}

	// Handle action modifications
	if workflowEditClearActions {
		if workflowEditAddActionType == "" {
			return fmt.Errorf("cannot clear all actions without adding new ones (use --add-action)")
		}
		modifiedRule.Actions = []workflow.RuleAction{}
		hasChanges = true
	}

	if workflowEditAddActionType != "" {
		action, err := buildActionFromFlags(workflowEditAddActionType, workflowEditAddActionValue)
		if err != nil {
			return err
		}
		// Attach condition if specified
		if cmd.Flags().Changed("cond-field") {
			cond, err := buildConditionFromFlags(workflowEditCondField, workflowEditCondOp, workflowEditCondValue)
			if err != nil {
				return err
			}
			action.Condition = cond
		}
		modifiedRule.Actions = append(modifiedRule.Actions, action)
		hasChanges = true
	}

	if workflowEditRemoveActions != "" {
		indices, err := parseCommaSeparatedIndices(workflowEditRemoveActions)
		if err != nil {
			return fmt.Errorf("invalid --remove-action indices: %w", err)
		}
		modifiedRule.Actions, err = removeByIndices(modifiedRule.Actions, indices)
		if err != nil {
			return fmt.Errorf("failed to remove actions: %w", err)
		}
		hasChanges = true
	}

	// Handle scope modifications
	if workflowEditClearScope {
		modifiedRule.Scope = nil
		hasChanges = true
	}
	if cmd.Flags().Changed("scope-project") || cmd.Flags().Changed("scope-type") || cmd.Flags().Changed("scope-section") {
		if workflowEditClearScope {
			return fmt.Errorf("cannot use --clear-scope with --scope-project/--scope-type/--scope-section")
		}
		// Start from existing scope or create new one
		if modifiedRule.Scope == nil {
			modifiedRule.Scope = &workflow.RuleScope{}
		}
		if cmd.Flags().Changed("scope-project") {
			modifiedRule.Scope.Projects = workflowEditScopeProjects
		}
		if cmd.Flags().Changed("scope-type") {
			modifiedRule.Scope.Types = workflowEditScopeTypes
		}
		if cmd.Flags().Changed("scope-section") {
			modifiedRule.Scope.Sections = workflowEditScopeSections
		}
		hasChanges = true
	}

	if !hasChanges {
		return fmt.Errorf("no changes specified; use --name, --enabled, --add-trigger-*, --remove-trigger, --add-action, --scope-*, etc")
	}

	// Atomic validation: collect all errors before failing
	validationErrors := collectValidationErrors(&modifiedRule)
	if len(validationErrors) > 0 {
		return fmt.Errorf("validation failed:\n  - %s", strings.Join(validationErrors, "\n  - "))
	}

	// Show preview
	fmt.Println("Preview of changes:")
	fmt.Println()
	diff := formatGitStyleDiff(rule, &modifiedRule)
	fmt.Println(diff)
	fmt.Println()

	// Require --yes flag
	if err := requireYesFlag(workflowEditYes); err != nil {
		return err
	}

	// Apply the changes
	if !cfg.UpdateWorkflowRule(modifiedRule) {
		return fmt.Errorf("failed to update workflow rule: %s", ruleID)
	}

	// Invalidate eval states to force re-evaluation
	cfg.InvalidateWorkflowEvalStates()

	// Save config
	if err := configManager.Save(cfg); err != nil {
		return tterrors.NewSaveConfigError(err)
	}

	// Display final result
	fmt.Println("Updated workflow rule:")
	fmt.Printf("  Rule: %s [%s]\n", modifiedRule.Name, modifiedRule.ID)
	if modifiedRule.IsProjectLevel() {
		fmt.Println("  Level: project")
	}
	fmt.Println()

	// Show full details
	triggers := modifiedRule.GetTriggers()
	if len(triggers) > 1 {
		logic := "AND"
		if modifiedRule.GetTriggerLogic() == workflow.LogicOr {
			logic = "OR"
		}
		fmt.Printf("  Trigger Logic: %s\n", logic)
	}

	fmt.Println("  Triggers:")
	for i, trigger := range triggers {
		fmt.Printf("    %d. %s %s %q\n", i, trigger.Field, trigger.Operation, trigger.Value)
	}

	fmt.Println()
	fmt.Println("  Actions:")
	for i, action := range modifiedRule.Actions {
		if action.Value != "" {
			fmt.Printf("    %d. %s → %q", i, action.ActionType, action.Value)
		} else {
			fmt.Printf("    %d. %s", i, action.ActionType)
		}
		if action.Condition != nil {
			fmt.Printf(" (if %s)", action.Condition.String())
		}
		fmt.Println()
	}

	if modifiedRule.Scope != nil {
		fmt.Println()
		fmt.Printf("  Scope: %s\n", formatScope(modifiedRule.Scope))
	}

	fmt.Println()
	fmt.Printf("  Summary: %s\n", modifiedRule.Summary())

	return nil
}

// detectMutuallyExclusiveEditFlags checks for conflicting flag combinations.
func detectMutuallyExclusiveEditFlags(_ *cobra.Command) error {
	if workflowEditClearTriggers && workflowEditRemoveTriggers != "" {
		return fmt.Errorf("cannot use --clear-triggers with --remove-trigger")
	}
	if workflowEditClearActions && workflowEditRemoveActions != "" {
		return fmt.Errorf("cannot use --clear-actions with --remove-action")
	}
	return nil
}
