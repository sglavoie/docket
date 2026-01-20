package cmd

import (
	"fmt"

	"github.com/sglavoie/tt/internal/config"
	"github.com/spf13/cobra"
)

var workflowListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all workflow rules",
	Long: `List all configured workflow automation rules.

Shows rule ID, name, enabled status, trigger conditions, and actions.`,
	RunE: runWorkflowList,
}

func init() {
	workflowCmd.AddCommand(workflowListCmd)
}

func runWorkflowList(cmd *cobra.Command, args []string) error {
	configManager := config.NewManager(GetConfigPath())
	cfg, err := configManager.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	rules := cfg.GetWorkflowRules()
	if len(rules) == 0 {
		fmt.Println("No workflow rules configured.")
		fmt.Println()
		fmt.Println("Use 'tt workflow init' to add default rules.")
		fmt.Println("Use 'tt workflow add' to create a custom rule.")
		return nil
	}

	fmt.Printf("Workflow Rules (%d):\n\n", len(rules))
	for _, rule := range rules {
		status := "enabled"
		if !rule.Enabled {
			status = "disabled"
		}
		sourceMarker := ""
		if rule.Source == "default" {
			sourceMarker = " [default]"
		}
		triggerInfo := ""
		if rule.TriggerCount > 0 {
			triggerInfo = fmt.Sprintf(" | %d triggers", rule.TriggerCount)
		}
		fmt.Printf("  [%s] %s (%s)%s%s\n", rule.ID, rule.Name, status, sourceMarker, triggerInfo)
		fmt.Printf("      %s\n", rule.Summary())
		fmt.Println()
	}

	return nil
}
