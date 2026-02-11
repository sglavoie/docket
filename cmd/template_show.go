package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var templateShowCmd = &cobra.Command{
	Use:   "show <id>",
	Short: "Show task template details",
	Long: `Display detailed information about a task template.

Shows full template configuration including:
- Template metadata (ID, name, source, enabled state, category)
- Task type, priority, section, tags
- Description (if any)
- Checklist items
- Items with optional markers and types
- Usage statistics (use count)

Examples:
  tt template show def-bug-report
  tt template show abc12345
  tt tpl show def-release-checklist`,
	Args: cobra.ExactArgs(1),
	RunE: runTemplateShow,
}

func init() {
	templateCmd.AddCommand(templateShowCmd)
}

func runTemplateShow(cmd *cobra.Command, args []string) error {
	templateID := args[0]

	cfg, _, err := LoadConfigStrict()
	if err != nil {
		return err
	}

	t := cfg.GetTaskTemplate(templateID)
	if t == nil {
		return fmt.Errorf("task template not found: %s", templateID)
	}

	// Display template details
	fmt.Printf("Template: %s [%s]\n", t.Name, t.ID)

	// Source
	source := "custom"
	if t.Source == "default" {
		source = "default"
	}
	fmt.Printf("Source: %s\n", source)

	// Category
	if t.Category != "" {
		fmt.Printf("Category: %s\n", t.Category)
	}

	// Enabled state
	enabledStr := "disabled"
	if t.Enabled {
		enabledStr = "enabled"
	}
	fmt.Printf("Enabled: %s\n", enabledStr)

	// Use count
	if t.UseCount > 0 {
		fmt.Printf("Used: %d times\n", t.UseCount)
	} else {
		fmt.Printf("Used: never\n")
	}

	// Task configuration
	fmt.Println()
	fmt.Println("Task Configuration:")
	fmt.Printf("  Type: %s\n", t.Type)
	if t.Priority > 0 {
		fmt.Printf("  Priority: %d\n", t.Priority)
	} else {
		fmt.Printf("  Priority: (default)\n")
	}
	if t.Section != "" {
		fmt.Printf("  Section: %s\n", t.Section)
	}
	if len(t.Tags) > 0 {
		fmt.Printf("  Tags: %v\n", t.Tags)
	}

	// Description
	if t.Description != "" {
		fmt.Println()
		fmt.Println("Description:")
		fmt.Printf("  %s\n", t.Description)
	}

	// Checklist
	if len(t.Checklist) > 0 {
		fmt.Println()
		fmt.Println("Checklist:")
		for i, item := range t.Checklist {
			fmt.Printf("  %d. %s\n", i+1, item)
		}
	}

	// Items (Children with types)
	if len(t.Children) > 0 {
		fmt.Println()
		fmt.Println("Items:")
		for i, child := range t.Children {
			optional := ""
			if child.Optional {
				optional = " (optional)"
			}
			childType := child.GetChildType()
			fmt.Printf("  %d. %s [%s]%s\n", i+1, child.Title, childType, optional)
		}
	}

	// Summary
	fmt.Println()
	fmt.Printf("Summary: %s\n", t.Summary())

	return nil
}
