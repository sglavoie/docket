package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/tterrors"
)

var workflowDeleteForce bool

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
	workflowDeleteCmd.Flags().BoolVarP(&workflowDeleteForce, "force", "f", false, "Skip confirmation prompt")
}

func runWorkflowDelete(cmd *cobra.Command, args []string) error {
	ruleID := args[0]

	cfg, configManager, err := LoadConfigStrict()
	if err != nil {
		return err
	}

	rule := cfg.GetWorkflowRule(ruleID)
	if rule == nil {
		return fmt.Errorf("workflow rule not found: %s", ruleID)
	}

	ruleName := rule.Name

	if !ConfirmYN(fmt.Sprintf("Delete workflow rule '%s'? [y/N]: ", ruleName), workflowDeleteForce) {
		fmt.Println("Canceled.")
		return nil
	}

	if !cfg.DeleteWorkflowRule(ruleID) {
		return fmt.Errorf("failed to delete workflow rule: %s", ruleID)
	}

	// Invalidate workflow eval states so all projects re-evaluate with removed rule
	cfg.InvalidateWorkflowEvalStates()

	if err := configManager.Save(cfg); err != nil {
		return tterrors.NewSaveConfigError(err)
	}

	fmt.Printf("Deleted workflow rule: %s [%s]\n", ruleName, ruleID)
	return nil
}
