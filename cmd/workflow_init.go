package cmd

import (
	"fmt"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/workflow"
	"github.com/spf13/cobra"
)

var workflowInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize default workflow rules",
	Long: `Add a set of sensible default workflow rules.

Default rules are created DISABLED by default. Review them with
'tt workflow list' and enable the ones you want with 'tt workflow toggle'.

DEFAULT RULES

  Tag Overdue Tasks
    When due_days < 0: add tag #overdue

  Mark Stale In-Progress
    When status = in_progress AND updated_days > 14: add tag #stale

  Auto-complete Epic
    When type = epic AND all_children_done = true: set status done

  Archive Completed
    When completed = true: move to Archive section

FLAGS

  --list     Preview default rules without adding them
  --force    Add defaults even if rules already exist
  --enable   Create default rules in enabled state (default: disabled)

EXAMPLES

  Preview default rules:
    tt workflow init --list

  Add default rules (disabled):
    tt workflow init

  Add and enable default rules:
    tt workflow init --enable

  Re-add deleted defaults:
    tt workflow init --force`,
	RunE: runWorkflowInit,
}

var (
	initList   bool
	initForce  bool
	initEnable bool
)

func init() {
	workflowCmd.AddCommand(workflowInitCmd)

	workflowInitCmd.Flags().BoolVar(&initList, "list", false, "preview default rules without adding them")
	workflowInitCmd.Flags().BoolVar(&initForce, "force", false, "add defaults even if rules already exist")
	workflowInitCmd.Flags().BoolVar(&initEnable, "enable", false, "create default rules in enabled state")
}

func runWorkflowInit(cmd *cobra.Command, args []string) error {
	defaults := workflow.GetDefaultRules()

	if initList {
		fmt.Println("Default workflow rules:")
		fmt.Println()
		for _, rule := range defaults {
			fmt.Printf("  [%s] %s\n", rule.ID, rule.Name)
			fmt.Printf("      %s\n", rule.Summary())
		}
		fmt.Println()
		fmt.Println("Run 'tt workflow init' to add these rules (disabled by default).")
		return nil
	}

	configManager := config.NewManager(GetConfigPath())
	cfg, err := configManager.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	if cfg.DefaultRulesInitialized && !initForce {
		fmt.Println("Default rules have already been initialized.")
		fmt.Println()
		fmt.Println("To re-add deleted defaults, use: tt workflow init --force")
		return nil
	}

	if len(cfg.WorkflowRules) > 0 && !initForce && !cfg.DefaultRulesInitialized {
		fmt.Printf("You have %d existing workflow rule(s).\n", len(cfg.WorkflowRules))
		fmt.Println()
		fmt.Println("To add defaults alongside existing rules, use: tt workflow init --force")
		return nil
	}

	added := cfg.AddDefaultRules(initEnable)

	if err := configManager.Save(cfg); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	if added == 0 {
		fmt.Println("All default rules already exist.")
		return nil
	}

	state := "disabled"
	if initEnable {
		state = "enabled"
	}

	fmt.Printf("Added %d default workflow rule(s) (%s):\n", added, state)
	fmt.Println()
	for _, rule := range defaults {
		if cfg.HasRuleWithID(rule.ID) {
			fmt.Printf("  [%s] %s\n", rule.ID, rule.Name)
		}
	}
	fmt.Println()
	fmt.Println("Use 'tt workflow list' to view rules.")
	if !initEnable {
		fmt.Println("Use 'tt workflow toggle <id>' to enable a rule.")
	}

	return nil
}
