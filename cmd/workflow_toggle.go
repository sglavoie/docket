package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/tterrors"
)

var workflowToggleCmd = &cobra.Command{
	Use:   "toggle <id>",
	Short: "Enable or disable a workflow rule",
	Long: `Toggle a workflow rule between enabled and disabled states.

Disabled rules are not evaluated when tasks change.

Example:
  tt workflow toggle abc123`,
	RunE: runWorkflowToggle,
}

func init() {
	workflowCmd.AddCommand(workflowToggleCmd)
}

func runWorkflowToggle(cmd *cobra.Command, args []string) error {
	if len(args) < 1 {
		return tterrors.NewRequiredFieldError("rule ID")
	}

	ruleID := args[0]

	cfg, configManager, err := LoadConfigStrict()
	if err != nil {
		return err
	}

	rule := cfg.GetWorkflowRule(ruleID)
	if rule == nil {
		return fmt.Errorf("workflow rule not found: %s", ruleID)
	}

	ruleName := rule.Name
	enabled, found := cfg.ToggleWorkflowRule(ruleID)
	if !found {
		return fmt.Errorf("workflow rule not found: %s", ruleID)
	}

	// Invalidate workflow eval states so all projects re-evaluate with changed rule
	cfg.InvalidateWorkflowEvalStates()

	if err := configManager.Save(cfg); err != nil {
		return tterrors.NewSaveConfigError(err)
	}

	status := "enabled"
	if !enabled {
		status = "disabled"
	}
	fmt.Printf("Workflow rule %q [%s] is now %s\n", ruleName, ruleID, status)
	return nil
}
