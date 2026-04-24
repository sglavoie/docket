package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/workflow"
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
	addOutputFlags(workflowListCmd)
}

func runWorkflowList(cmd *cobra.Command, args []string) error {
	out, err := getOutputOpts(cmd)
	if err != nil {
		return err
	}

	cfg, _, err := LoadConfigStrict()
	if err != nil {
		return err
	}

	defaultRules, customRules := cfg.GetWorkflowRulesSorted()
	total := len(defaultRules) + len(customRules)

	if out.JSON {
		return displayWorkflowListJSON(defaultRules, customRules)
	}

	if out.Quiet {
		return nil
	}

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
			levelInfo := ""
			if rule.IsProjectLevel() {
				levelInfo = " | project"
			}
			priorityInfo := ""
			if rule.Priority > 0 {
				priorityInfo = fmt.Sprintf(" | P%d", rule.Priority)
			}
			triggerInfo := ""
			if rule.TriggerCount > 0 {
				triggerInfo = fmt.Sprintf(" | %d triggers", rule.TriggerCount)
			}
			fmt.Printf("  [%s] %s (%s%s%s)%s\n", rule.ID, rule.Name, status, levelInfo, priorityInfo, triggerInfo)
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
			levelInfo := ""
			if rule.IsProjectLevel() {
				levelInfo = " | project"
			}
			priorityInfo := ""
			if rule.Priority > 0 {
				priorityInfo = fmt.Sprintf(" | P%d", rule.Priority)
			}
			triggerInfo := ""
			if rule.TriggerCount > 0 {
				triggerInfo = fmt.Sprintf(" | %d triggers", rule.TriggerCount)
			}
			fmt.Printf("  [%s] %s (%s%s%s)%s\n", rule.ID, rule.Name, status, levelInfo, priorityInfo, triggerInfo)
			fmt.Printf("      %s\n", rule.Summary())
			fmt.Println()
		}
	}

	return nil
}

func displayWorkflowListJSON(defaultRules, customRules []workflow.WorkflowRule) error {
	type ruleJSONEntry struct {
		ID           string `json:"id"`
		Name         string `json:"name"`
		Source       string `json:"source"`
		Enabled      bool   `json:"enabled"`
		Level        string `json:"level"`
		Priority     int    `json:"priority"`
		TriggerCount int64  `json:"trigger_count"`
		Summary      string `json:"summary"`
	}

	var entries []ruleJSONEntry
	for _, rule := range defaultRules {
		level := "task"
		if rule.IsProjectLevel() {
			level = "project"
		}
		source := "custom"
		if rule.Source == "default" {
			source = "default"
		}
		entries = append(entries, ruleJSONEntry{
			ID:           rule.ID,
			Name:         rule.Name,
			Source:       source,
			Enabled:      rule.Enabled,
			Level:        level,
			Priority:     rule.Priority,
			TriggerCount: rule.TriggerCount,
			Summary:      rule.Summary(),
		})
	}
	for _, rule := range customRules {
		level := "task"
		if rule.IsProjectLevel() {
			level = "project"
		}
		source := "custom"
		if rule.Source == "default" {
			source = "default"
		}
		entries = append(entries, ruleJSONEntry{
			ID:           rule.ID,
			Name:         rule.Name,
			Source:       source,
			Enabled:      rule.Enabled,
			Level:        level,
			Priority:     rule.Priority,
			TriggerCount: rule.TriggerCount,
			Summary:      rule.Summary(),
		})
	}
	if entries == nil {
		entries = []ruleJSONEntry{}
	}

	return printJSON(entries)
}
