package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/config"
)

var workflowToggleCmd = &cobra.Command{
	Use:   "toggle <id>",
	Short: "Enable or disable a workflow rule",
	Long: `Toggle a workflow rule between enabled and disabled states.

Disabled rules are not evaluated when tasks change.

Example:
  tt workflow toggle abc123`,
	Args: cobra.ExactArgs(1),
	RunE: runWorkflowToggle,
}

func init() {
	workflowCmd.AddCommand(workflowToggleCmd)
}

func runWorkflowToggle(cmd *cobra.Command, args []string) error {
	ruleID := args[0]

	configManager := config.NewManager(GetConfigPath())
	cfg, err := configManager.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
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
		return fmt.Errorf("failed to save config: %w", err)
	}

	status := "enabled"
	if !enabled {
		status = "disabled"
	}
	fmt.Printf("Workflow rule %q [%s] is now %s\n", ruleName, ruleID, status)
	return nil
}
