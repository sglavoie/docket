package cmd

import (
	"fmt"

	"github.com/sglavoie/tt/internal/config"
	"github.com/spf13/cobra"
)

var workflowDeleteCmd = &cobra.Command{
	Use:     "delete <id>",
	Aliases: []string{"rm", "remove"},
	Short:   "Delete a workflow rule",
	Long: `Delete a workflow rule by its ID.

Use 'tt workflow list' to see rule IDs.

Example:
  tt workflow delete abc123`,
	Args: cobra.ExactArgs(1),
	RunE: runWorkflowDelete,
}

func init() {
	workflowCmd.AddCommand(workflowDeleteCmd)
}

func runWorkflowDelete(cmd *cobra.Command, args []string) error {
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
	if !cfg.DeleteWorkflowRule(ruleID) {
		return fmt.Errorf("failed to delete workflow rule: %s", ruleID)
	}

	if err := configManager.Save(cfg); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Printf("Deleted workflow rule: %s [%s]\n", ruleName, ruleID)
	return nil
}
