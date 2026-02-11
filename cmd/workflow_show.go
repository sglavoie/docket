package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var workflowShowCmd = &cobra.Command{
	Use:   "show <id>",
	Short: "Show workflow rule details",
	Long: `Display detailed information about a workflow rule.

Shows full rule configuration including:
- Rule metadata (ID, name, source, enabled state)
- Trigger logic and all triggers with indices
- All actions with indices
- Usage statistics (trigger count)

Examples:
  tt workflow show def-overdue
  tt workflow show abc12345
  tt wf show def-activate-on-active`,
	Args: cobra.ExactArgs(1),
	RunE: runWorkflowShow,
}

func init() {
	workflowCmd.AddCommand(workflowShowCmd)
}

func runWorkflowShow(cmd *cobra.Command, args []string) error {
	ruleID := args[0]

	cfg, _, err := LoadConfigStrict()
	if err != nil {
		return err
	}

	rule := cfg.GetWorkflowRule(ruleID)
	if rule == nil {
		return fmt.Errorf("workflow rule not found: %s", ruleID)
	}

	// Display rule details
	fmt.Printf("Rule: %s [%s]\n", rule.Name, rule.ID)

	// Source
	source := "custom"
	if rule.Source == "default" {
		source = "default"
	}
	fmt.Printf("Source: %s\n", source)

	// Enabled state
	enabledStr := "disabled"
	if rule.Enabled {
		enabledStr = "enabled"
	}
	fmt.Printf("Enabled: %s\n", enabledStr)

	// Trigger count
	if rule.TriggerCount > 0 {
		fmt.Printf("Triggered: %d times\n", rule.TriggerCount)
	} else {
		fmt.Printf("Triggered: never\n")
	}

	// Trigger logic
	fmt.Println()
	triggers := rule.GetTriggers()
	if len(triggers) > 1 {
		logic := "AND"
		if rule.GetTriggerLogic() == "or" {
			logic = "OR"
		}
		fmt.Printf("Trigger Logic: %s\n", logic)
	}

	// Triggers
	fmt.Println()
	fmt.Println("Triggers:")
	if len(triggers) == 0 {
		fmt.Println("  (none)")
	} else {
		for i, trigger := range triggers {
			fmt.Printf("  %d. %s %s %q\n", i, trigger.Field, trigger.Operation, trigger.Value)
		}
	}

	// Actions
	fmt.Println()
	fmt.Println("Actions:")
	if len(rule.Actions) == 0 {
		fmt.Println("  (none)")
	} else {
		for i, action := range rule.Actions {
			if action.Value != "" {
				fmt.Printf("  %d. %s → %q\n", i, action.ActionType, action.Value)
			} else {
				fmt.Printf("  %d. %s\n", i, action.ActionType)
			}
		}
	}

	// Summary
	fmt.Println()
	fmt.Printf("Summary: %s\n", rule.Summary())

	return nil
}
