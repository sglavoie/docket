package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/tterrors"
)

var templateToggleCmd = &cobra.Command{
	Use:   "toggle <id>",
	Short: "Enable or disable a task template",
	Long: `Toggle a task template between enabled and disabled states.

Disabled templates are not shown in the template picker during task creation.

Example:
  tt template toggle def-bug-report`,
	Args: cobra.ExactArgs(1),
	RunE: runTemplateToggle,
}

func init() {
	templateCmd.AddCommand(templateToggleCmd)
}

func runTemplateToggle(cmd *cobra.Command, args []string) error {
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
	enabled, found := cfg.ToggleTaskTemplate(templateID)
	if !found {
		return fmt.Errorf("task template not found: %s", templateID)
	}

	if err := configManager.Save(cfg); err != nil {
		return tterrors.NewSaveConfigError(err)
	}

	status := "enabled"
	if !enabled {
		status = "disabled"
	}
	fmt.Printf("Task template %q [%s] is now %s\n", templateName, templateID, status)
	return nil
}
