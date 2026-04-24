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
	RunE: runTemplateToggle,
}

func init() {
	templateCmd.AddCommand(templateToggleCmd)
	addOutputFlags(templateToggleCmd)
}

func runTemplateToggle(cmd *cobra.Command, args []string) error {
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
		return fmt.Errorf("task template not found: %s", templateID)
	}

	templateName := t.Name
	enabled, found := cfg.ToggleTaskTemplate(templateID)
	if !found {
		return fmt.Errorf("task template not found: %s", templateID)
	}

	if err := saveConfig(configManager, cfg); err != nil {
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
		}{ID: templateID, Name: templateName, Enabled: enabled})
	}
	fmt.Printf("Task template %q [%s] is now %s\n", templateName, templateID, status)
	return nil
}
