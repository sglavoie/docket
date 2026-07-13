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
	addOutputFlags(workflowToggleCmd)
}

func runWorkflowToggle(cmd *cobra.Command, args []string) error {
	if len(args) < 1 {
		return tterrors.NewRequiredFieldError("rule ID")
	}

	out, err := getOutputOpts(cmd)
	if err != nil {
		return err
	}

	ruleID := args[0]

	_, configManager, err := LoadConfigStrict()
	if err != nil {
		return err
	}

	store := automationStore(configManager)
	automation, err := store.LoadAutomation()
	if err != nil {
		return err
	}

	rule := automation.GetWorkflowRule(ruleID)
	if rule == nil {
		return fmt.Errorf("workflow rule not found: %s", ruleID)
	}

	ruleName := rule.Name
	enabled, found := automation.ToggleWorkflowRule(ruleID)
	if !found {
		return fmt.Errorf("workflow rule not found: %s", ruleID)
	}

	// Invalidate workflow eval states so all projects re-evaluate with changed rule
	automation.InvalidateWorkflowEvalStates()

	if err := saveAutomation(store, automation); err != nil {
		return err
	}

	status := "enabled"
	if !enabled {
		status = "disabled"
	}

	if out.Quiet {
		return nil
	}
	if out.JSON {
		return printJSON(struct {
			ID      string `json:"id"`
			Name    string `json:"name"`
			Enabled bool   `json:"enabled"`
		}{ID: ruleID, Name: ruleName, Enabled: enabled})
	}
	fmt.Printf("Workflow rule %q [%s] is now %s\n", ruleName, ruleID, status)
	return nil
}
