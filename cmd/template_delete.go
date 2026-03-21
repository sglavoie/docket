package cmd

import (
	"fmt"

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
}

func runTemplateDelete(cmd *cobra.Command, args []string) error {
	if len(args) < 1 {
		return tterrors.NewRequiredFieldError("template ID")
	}

	templateID := args[0]

	cfg, configManager, err := LoadConfigStrict()
	if err != nil {
		return err
	}

	t := cfg.GetTaskTemplate(templateID)
	if t == nil {
		return fmt.Errorf("task template not found: %s", templateID)
	}

	templateName := t.Name

	if !ConfirmYN(fmt.Sprintf("Delete template '%s'? [y/N]: ", templateName), templateDeleteForce) {
		fmt.Println("Canceled")
		return nil
	}

	if !cfg.DeleteTaskTemplate(templateID) {
		return fmt.Errorf("failed to delete task template: %s", templateID)
	}

	if err := saveConfig(configManager, cfg); err != nil {
		return err
	}

	fmt.Printf("Deleted task template: %s [%s]\n", templateName, templateID)
	return nil
}
