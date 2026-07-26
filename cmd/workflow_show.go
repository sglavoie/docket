package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/tterrors"
	"github.com/sglavoie/tt/internal/workflow"
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
	RunE: runWorkflowShow,
}

func init() {
	workflowCmd.AddCommand(workflowShowCmd)
}

func runWorkflowShow(cmd *cobra.Command, args []string) error {
	if len(args) < 1 {
		return tterrors.NewRequiredFieldError("rule ID")
	}

	ruleID := args[0]

	workspace, err := LoadCLIWorkspaceStrict()
	if err != nil {
		return err
	}
	automation := workspace.Runtime.Automation()

	rule := automation.GetWorkflowRule(ruleID)
	if rule == nil {
		return fmt.Errorf("workflow rule not found: %s", ruleID)
	}

	// Display rule details
	fmt.Printf("Rule: %s [%s]\n", rule.Name, rule.ID)

	// Level
	level := "task"
	if rule.IsProjectLevel() {
		level = "project"
	}
	fmt.Printf("Level: %s\n", level)

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

	// Triggers
	fmt.Println()
	if rule.TriggerGroup != nil {
		fmt.Println("Triggers (nested group):")
		printTriggerGroup(rule.TriggerGroup, "  ")
	} else {
		triggers := rule.GetTriggers()
		if len(triggers) > 1 {
			logic := "AND"
			if rule.GetTriggerLogic() == "or" {
				logic = "OR"
			}
			fmt.Printf("Trigger Logic: %s\n", logic)
		}

		fmt.Println("Triggers:")
		if len(triggers) == 0 {
			fmt.Println("  (none)")
		} else {
			for i, trigger := range triggers {
				fmt.Printf("  %d. %s %s %q\n", i, trigger.Field, trigger.Operation, trigger.Value)
			}
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

	// Scope
	if rule.Scope != nil {
		fmt.Println()
		fmt.Println("Scope:")
		if len(rule.Scope.Projects) > 0 {
			fmt.Printf("  Projects: %s\n", strings.Join(rule.Scope.Projects, ", "))
		}
		if len(rule.Scope.Types) > 0 {
			fmt.Printf("  Types: %s\n", strings.Join(rule.Scope.Types, ", "))
		}
		if len(rule.Scope.States) > 0 {
			states := make([]string, 0, len(rule.Scope.States))
			for _, state := range rule.Scope.States {
				states = append(states, string(state))
			}
			fmt.Printf("  States: %s\n", strings.Join(states, ", "))
		}
		// Legacy section scope remains visible until persisted rules migrate.
		if len(rule.Scope.Sections) > 0 {
			fmt.Printf("  Sections: %s\n", strings.Join(rule.Scope.Sections, ", "))
		}
	}

	// Summary
	fmt.Println()
	fmt.Printf("Summary: %s\n", rule.Summary())

	return nil
}

// printTriggerGroup recursively prints a TriggerGroup with indentation.
func printTriggerGroup(group *workflow.TriggerGroup, indent string) {
	logic := "AND"
	if group.Logic == workflow.LogicOr {
		logic = "OR"
	}
	fmt.Printf("%sLogic: %s\n", indent, logic)
	for i, trigger := range group.Triggers {
		fmt.Printf("%s%d. %s %s %q\n", indent, i, trigger.Field, trigger.Operation, trigger.Value)
	}
	for i, sub := range group.Groups {
		fmt.Printf("%sSub-group %d:\n", indent, i)
		printTriggerGroup(&sub, indent+"  ")
	}
}
