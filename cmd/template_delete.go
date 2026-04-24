package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/tterrors"
)

var templateDeleteForce bool

var templateDeleteCmd = &cobra.Command{
	Use:     "delete <id>",
	Aliases: []string{"rm", "remove"},
	Short:   "Delete a task template",
	Long: `Delete a task template by its ID.

Use 'tt template list' to see template IDs.

Example:
  tt template delete abc123`,
	RunE: runTemplateDelete,
}

func init() {
	templateCmd.AddCommand(templateDeleteCmd)
	templateDeleteCmd.Flags().BoolVarP(&templateDeleteForce, "force", "f", false, "Skip confirmation prompt")
	addOutputFlags(templateDeleteCmd)
}

func runTemplateDelete(cmd *cobra.Command, args []string) error {
	if len(args) < 1 {
		return tterrors.NewRequiredFieldError("template ID")
	}

	out, err := getOutputOpts(cmd)
	if err != nil {
		return err
	}

	templateID := args[0]

	cfg, configManager, err := LoadConfigStrict()
	if err != nil {
		return err
	}

	t := cfg.GetTaskTemplate(templateID)
	if t == nil {
		return fmt.Errorf("task template not found: %s. Use 'tt template list' to see templates", templateID)
	}

	templateName := t.Name

	if !ConfirmYN(fmt.Sprintf("Delete template '%s'? [y/N]: ", templateName), templateDeleteForce) {
		if !out.Quiet {
			fmt.Fprintf(os.Stderr, "Canceled\n")
		}
		return nil
	}

	if !cfg.DeleteTaskTemplate(templateID) {
		return fmt.Errorf("failed to delete task template: %s", templateID)
	}

	if err := saveConfig(configManager, cfg); err != nil {
		return err
	}

	if out.Quiet {
		return nil
	}
	if out.JSON {
		return printJSON(struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		}{ID: templateID, Name: templateName})
	}
	fmt.Printf("Deleted task template: %s [%s]\n", templateName, templateID)
	return nil
}
