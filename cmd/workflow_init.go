package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/workflow"
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

  --list              Preview default rules without adding them
  --force             Add defaults even if rules already exist
  --enable            Create default rules in enabled state (default: disabled)
  --changed-defaults  Reset edited/deleted default rules to shipped values (requires --force)

EXAMPLES

  Preview default rules:
    tt workflow init --list

  Add default rules (disabled):
    tt workflow init

  Add and enable default rules:
    tt workflow init --enable

  Re-add deleted defaults:
    tt workflow init --force
  
  Reset broken default rules:
    tt workflow init --force --changed-defaults
  
  Reset and enable defaults:
    tt workflow init --force --changed-defaults --enable`,
	RunE: runWorkflowInit,
}

var (
	initList            bool
	initForce           bool
	initEnable          bool
	initChangedDefaults bool
)

func init() {
	workflowCmd.AddCommand(workflowInitCmd)

	workflowInitCmd.Flags().BoolVar(&initList, "list", false, "preview default rules without adding them")
	workflowInitCmd.Flags().BoolVar(&initForce, "force", false, "add defaults even if rules already exist")
	workflowInitCmd.Flags().BoolVar(&initEnable, "enable", false, "create default rules in enabled state")
	workflowInitCmd.Flags().BoolVar(&initChangedDefaults, "changed-defaults", false, "reset edited/deleted default rules to shipped values (requires --force)")
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

	cfg, configManager, err := LoadConfigStrict()
	if err != nil {
		return err
	}

	// Handle --changed-defaults flag (requires --force for safety)
	if initChangedDefaults {
		if !initForce {
			return fmt.Errorf("--changed-defaults requires --force for safety")
		}

		updated, readded, unchanged, affectedIDs := cfg.ResetChangedDefaultRules(initEnable)

		if err := saveConfig(configManager, cfg); err != nil {
			return err
		}

		fmt.Println("Reset changed default rules:")
		fmt.Printf("  Updated: %d\n", updated)
		fmt.Printf("  Re-added (deleted): %d\n", readded)
		fmt.Printf("  Unchanged: %d\n", unchanged)

		if len(affectedIDs) > 0 {
			fmt.Println()
			fmt.Println("Affected rules:")
			for _, id := range affectedIDs {
				rule := cfg.GetWorkflowRule(id)
				if rule != nil {
					fmt.Printf("  [%s] %s\n", rule.ID, rule.Name)
				}
			}
		}

		fmt.Println()
		fmt.Println("Use 'tt workflow list' to review.")
		return nil
	}

	if len(cfg.WorkflowRules) > 0 && !initForce && !cfg.DefaultRulesInitialized {
		fmt.Printf("You have %d existing workflow rule(s).\n", len(cfg.WorkflowRules))
		fmt.Println()
		fmt.Println("To add defaults alongside existing rules, use: tt workflow init --force")
		return nil
	}

	// When already initialized, sync new defaults with their original enabled state.
	// When not initialized or --force, use initEnable flag.
	useOriginalEnabled := cfg.DefaultRulesInitialized && !initForce
	added := cfg.AddDefaultRules(initEnable, useOriginalEnabled)

	if err := saveConfig(configManager, cfg); err != nil {
		return err
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
