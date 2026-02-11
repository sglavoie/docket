package cmd

import (
	"fmt"

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
	cfg, _, err := LoadConfigStrict()
	if err != nil {
		return err
	}

	defaultRules, customRules := cfg.GetWorkflowRulesSorted()
	total := len(defaultRules) + len(customRules)
	if total == 0 {
		fmt.Println("No workflow rules configured.")
		fmt.Println()
		fmt.Println("Use 'tt workflow init' to add default rules.")
		fmt.Println("Use 'tt workflow add' to create a custom rule.")
		return nil
	}

	fmt.Printf("Workflow Rules (%d):\n", total)

	if len(defaultRules) > 0 {
		fmt.Println()
		fmt.Println("── Default Rules ──")
		fmt.Println()
		for _, rule := range defaultRules {
			status := "enabled"
			if !rule.Enabled {
				status = "disabled"
			}
			triggerInfo := ""
			if rule.TriggerCount > 0 {
				triggerInfo = fmt.Sprintf(" | %d triggers", rule.TriggerCount)
			}
			fmt.Printf("  [%s] %s (%s)%s\n", rule.ID, rule.Name, status, triggerInfo)
			fmt.Printf("      %s\n", rule.Summary())
			fmt.Println()
		}
	}

	if len(customRules) > 0 {
		fmt.Println()
		fmt.Println("── Custom Rules ──")
		fmt.Println()
		for _, rule := range customRules {
			status := "enabled"
			if !rule.Enabled {
				status = "disabled"
			}
			triggerInfo := ""
			if rule.TriggerCount > 0 {
				triggerInfo = fmt.Sprintf(" | %d triggers", rule.TriggerCount)
			}
			fmt.Printf("  [%s] %s (%s)%s\n", rule.ID, rule.Name, status, triggerInfo)
			fmt.Printf("      %s\n", rule.Summary())
			fmt.Println()
		}
	}

	return nil
}
