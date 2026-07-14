package cmd

import (
	"fmt"
	"os"

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
	RunE: runWorkflowDelete,
}

func init() {
	workflowCmd.AddCommand(workflowDeleteCmd)
	workflowDeleteCmd.Flags().BoolVarP(&workflowDeleteForce, "force", "f", false, "Skip confirmation prompt")
	addOutputFlags(workflowDeleteCmd)
}

func runWorkflowDelete(cmd *cobra.Command, args []string) error {
	if len(args) < 1 {
		return tterrors.NewRequiredFieldError("rule ID")
	}

	out, err := getOutputOpts(cmd)
	if err != nil {
		return err
	}

	ruleID := args[0]

	workspace, err := LoadCLIWorkspaceStrict()
	if err != nil {
		return err
	}
	domains, err := cloneCLIWorkspaceDomains(workspace)
	if err != nil {
		return err
	}
	automation := domains.Automation

	rule := automation.GetWorkflowRule(ruleID)
	if rule == nil {
		return fmt.Errorf("workflow rule not found: %s", ruleID)
	}

	ruleName := rule.Name

	if !ConfirmYN(fmt.Sprintf("Delete workflow rule '%s'? [y/N]: ", ruleName), workflowDeleteForce) {
		if !out.Quiet {
			fmt.Fprintf(os.Stderr, "Canceled\n")
		}
		return nil
	}

	if !automation.DeleteWorkflowRule(ruleID) {
		return fmt.Errorf("failed to delete workflow rule: %s", ruleID)
	}

	// Invalidate workflow eval states so all projects re-evaluate with removed rule
	automation.InvalidateWorkflowEvalStates()

	if err := commitCLIAutomation(workspace, automation); err != nil {
		return err
	}

	if out.Quiet {
		return nil
	}
	if out.JSON {
		return printJSON(struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		}{ID: ruleID, Name: ruleName})
	}
	fmt.Printf("Deleted workflow rule: %s [%s]\n", ruleName, ruleID)
	return nil
}
