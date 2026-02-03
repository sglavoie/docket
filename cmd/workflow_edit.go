package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/tterrors"
	"github.com/sglavoie/tt/internal/workflow"
)

var (
	// Simple field updates
	workflowEditName    string
	workflowEditEnabled bool

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
	workflowEditRemoveActions  string // Comma-separated indices
	workflowEditClearActions   bool

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
	workflowEditCmd.Flags().BoolVar(&workflowEditEnabled, "enabled", false, "enable or disable rule (must set explicitly)")

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
	workflowEditCmd.Flags().StringVar(&workflowEditRemoveActions, "remove-action", "", "remove actions by index (comma-separated, e.g., '0,1')")
	workflowEditCmd.Flags().BoolVar(&workflowEditClearActions, "clear-actions", false, "clear all actions (use with add-action to replace)")

	// Confirmation flag
	workflowEditCmd.Flags().BoolVarP(&workflowEditYes, "yes", "y", false, "apply changes without preview confirmation")
}

func runWorkflowEdit(cmd *cobra.Command, args []string) error {
	ruleID := args[0]

	// Load config
	configManager := config.NewManager(GetConfigPath())
	cfg, err := configManager.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
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

	if cmd.Flags().Changed("enabled") {
		modifiedRule.Enabled = workflowEditEnabled
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

	if !hasChanges {
		return fmt.Errorf("no changes specified; use --name, --enabled, --add-trigger-*, --remove-trigger, --add-action, etc.")
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
			fmt.Printf("    %d. %s → %q\n", i, action.ActionType, action.Value)
		} else {
			fmt.Printf("    %d. %s\n", i, action.ActionType)
		}
	}

	fmt.Println()
	fmt.Printf("  Summary: %s\n", modifiedRule.Summary())

	return nil
}

// detectMutuallyExclusiveEditFlags checks for conflicting flag combinations.
func detectMutuallyExclusiveEditFlags(cmd *cobra.Command) error {
	if workflowEditClearTriggers && workflowEditRemoveTriggers != "" {
		return fmt.Errorf("cannot use --clear-triggers with --remove-trigger")
	}
	if workflowEditClearActions && workflowEditRemoveActions != "" {
		return fmt.Errorf("cannot use --clear-actions with --remove-action")
	}
	return nil
}
