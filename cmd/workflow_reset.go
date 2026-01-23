package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/config"
)

var workflowResetAll bool

var workflowResetCmd = &cobra.Command{
	Use:   "reset [id]",
	Short: "Reset trigger count for a workflow rule",
	Long: `Reset the trigger count for a workflow rule to zero.

Use --all to reset counts for all rules.

Examples:
  tt workflow reset abc123
  tt workflow reset --all`,
	Args: cobra.MaximumNArgs(1),
	RunE: runWorkflowReset,
}

func init() {
	workflowCmd.AddCommand(workflowResetCmd)
	workflowResetCmd.Flags().BoolVar(&workflowResetAll, "all", false, "Reset trigger counts for all rules")
}

func runWorkflowReset(cmd *cobra.Command, args []string) error {
	if !workflowResetAll && len(args) == 0 {
		return fmt.Errorf("provide a rule ID or use --all")
	}

	configManager := config.NewManager(GetConfigPath())
	cfg, err := configManager.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	if workflowResetAll {
		cfg.ResetAllRuleTriggerCounts()
		if err := configManager.Save(cfg); err != nil {
			return fmt.Errorf("failed to save config: %w", err)
		}
		fmt.Printf("Reset trigger counts for all %d rules\n", len(cfg.WorkflowRules))
		return nil
	}

	ruleID := args[0]
	rule := cfg.GetWorkflowRule(ruleID)
	if rule == nil {
		return fmt.Errorf("workflow rule not found: %s", ruleID)
	}

	ruleName := rule.Name
	oldCount := rule.TriggerCount

	if !cfg.ResetRuleTriggerCount(ruleID) {
		return fmt.Errorf("workflow rule not found: %s", ruleID)
	}

	if err := configManager.Save(cfg); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Printf("Reset trigger count for %q [%s]: %d -> 0\n", ruleName, ruleID, oldCount)
	return nil
}
