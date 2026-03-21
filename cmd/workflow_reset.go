package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
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
	addOutputFlags(workflowResetCmd)
}

func runWorkflowReset(cmd *cobra.Command, args []string) error {
	if !workflowResetAll && len(args) == 0 {
		return fmt.Errorf("provide a rule ID or use --all")
	}

	out, err := getOutputOpts(cmd)
	if err != nil {
		return err
	}

	cfg, configManager, err := LoadConfigStrict()
	if err != nil {
		return err
	}

	if workflowResetAll {
		total := len(cfg.WorkflowRules)
		cfg.ResetAllRuleTriggerCounts()
		if err := saveConfig(configManager, cfg); err != nil {
			return err
		}
		if out.Quiet {
			return nil
		}
		if out.JSON {
			data, _ := json.MarshalIndent(struct {
				Reset int `json:"reset"`
			}{Reset: total}, "", "  ")
			fmt.Println(string(data))
			return nil
		}
		fmt.Printf("Reset trigger counts for all %d rules\n", total)
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

	if err := saveConfig(configManager, cfg); err != nil {
		return err
	}

	if out.Quiet {
		return nil
	}
	if out.JSON {
		data, _ := json.MarshalIndent(struct {
			ID            string `json:"id"`
			Name          string `json:"name"`
			PreviousCount int64  `json:"previous_count"`
		}{ID: ruleID, Name: ruleName, PreviousCount: oldCount}, "", "  ")
		fmt.Println(string(data))
		return nil
	}
	fmt.Printf("Reset trigger count for %q [%s]: %d -> 0\n", ruleName, ruleID, oldCount)
	return nil
}
