package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/template"
)

var templateListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all task templates",
	Long: `List all configured task templates.

Shows template ID, name, enabled status, type, and summary.

FLAGS
  --enabled   Show only enabled templates
  --category  Filter by category (bug, feature, process)`,
	RunE: runTemplateList,
}

var (
	templateListEnabled  bool
	templateListCategory string
)

func init() {
	templateCmd.AddCommand(templateListCmd)

	templateListCmd.Flags().BoolVar(&templateListEnabled, "enabled", false, "show only enabled templates")
	templateListCmd.Flags().StringVar(&templateListCategory, "category", "", "filter by category (bug, feature, process)")
}

func runTemplateList(cmd *cobra.Command, args []string) error {
	cfg, _, err := LoadConfigStrict()
	if err != nil {
		return err
	}

	defaultTemplates, customTemplates := cfg.GetTaskTemplatesSorted()
	total := len(defaultTemplates) + len(customTemplates)
	if total == 0 {
		fmt.Println("No task templates configured.")
		fmt.Println()
		fmt.Println("Use 'tt template init' to add default templates.")
		fmt.Println("Use 'tt template add' to create a custom template.")
		return nil
	}

	// Apply filters
	filterTemplates := func(templates []template.TaskTemplate) []template.TaskTemplate {
		var result []template.TaskTemplate
		for _, t := range templates {
			if templateListEnabled && !t.Enabled {
				continue
			}
			if templateListCategory != "" && t.Category != templateListCategory {
				continue
			}
			result = append(result, t)
		}
		return result
	}

	filteredDefaults := filterTemplates(defaultTemplates)
	filteredCustom := filterTemplates(customTemplates)
	filteredTotal := len(filteredDefaults) + len(filteredCustom)

	if filteredTotal == 0 {
		fmt.Println("No templates match the filter criteria.")
		return nil
	}

	fmt.Printf("Task Templates (%d):\n", filteredTotal)

	if len(filteredDefaults) > 0 {
		fmt.Println()
		fmt.Println("── Default Templates ──")
		fmt.Println()
		for _, t := range filteredDefaults {
			status := "enabled"
			if !t.Enabled {
				status = "disabled"
			}
			useInfo := ""
			if t.UseCount > 0 {
				useInfo = fmt.Sprintf(" | used %d times", t.UseCount)
			}
			fmt.Printf("  [%s] %s (%s)%s\n", t.ID, t.Name, status, useInfo)
			fmt.Printf("      %s\n", t.Summary())
			fmt.Println()
		}
	}

	if len(filteredCustom) > 0 {
		fmt.Println()
		fmt.Println("── Custom Templates ──")
		fmt.Println()
		for _, t := range filteredCustom {
			status := "enabled"
			if !t.Enabled {
				status = "disabled"
			}
			useInfo := ""
			if t.UseCount > 0 {
				useInfo = fmt.Sprintf(" | used %d times", t.UseCount)
			}
			fmt.Printf("  [%s] %s (%s)%s\n", t.ID, t.Name, status, useInfo)
			fmt.Printf("      %s\n", t.Summary())
			fmt.Println()
		}
	}

	return nil
}
