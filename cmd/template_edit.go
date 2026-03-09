package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/template"
	"github.com/sglavoie/tt/internal/tterrors"
)

var templateEditCmd = &cobra.Command{
	Use:   "edit <id>",
	Short: "Edit a task template",
	Long: `Edit a task template's properties.

Use flags to specify which fields to update. Multiple flags can be combined.
Changes are applied directly.

EXAMPLES

  # Rename a template
  tt template edit abc123 --name "New Name"

  # Change type and priority
  tt template edit abc123 --type story --priority 2

  # Update tags
  tt template edit abc123 --tags backend,api

  # Add an item
  tt template edit abc123 --add-item "New Step"

  # Add an optional item
  tt template edit abc123 --add-optional-item "Optional Step"

  # Remove an item by index (use 'tt template show' to see indices)
  tt template edit abc123 --remove-item 2

  # Clear all items
  tt template edit abc123 --clear-items`,
	Args: cobra.ExactArgs(1),
	RunE: runTemplateEdit,
}

var (
	templateEditName        string
	templateEditType        string
	templateEditPriority    int
	templateEditTags        string
	templateEditSection     string
	templateEditDescription string
	templateEditCategory    string
	templateEditAddChildren []string
	templateEditAddOptChild []string
	templateEditRemoveChild []int
	templateEditClearChild  bool
	templateEditEnabled     bool
)

func init() {
	templateCmd.AddCommand(templateEditCmd)

	templateEditCmd.Flags().StringVarP(&templateEditName, "name", "n", "", "new template name")
	templateEditCmd.Flags().StringVar(&templateEditType, "type", "", "task type: epic, story, task, bug, idea")
	templateEditCmd.Flags().IntVar(&templateEditPriority, "priority", 0, "priority 1-5 (0 for default)")
	templateEditCmd.Flags().StringVar(&templateEditTags, "tags", "", "comma-separated tags (replaces existing)")
	templateEditCmd.Flags().StringVar(&templateEditSection, "section", "", "section name")
	templateEditCmd.Flags().StringVar(&templateEditDescription, "description", "", "task description template")
	templateEditCmd.Flags().StringVar(&templateEditCategory, "category", "", "category: bug, feature, process")
	templateEditCmd.Flags().StringArrayVar(&templateEditAddChildren, "add-item", nil, "add checklist item (can specify multiple)")
	templateEditCmd.Flags().StringArrayVar(&templateEditAddOptChild, "add-optional-item", nil, "add optional checklist item (can specify multiple)")
	templateEditCmd.Flags().IntSliceVar(&templateEditRemoveChild, "remove-item", nil, "remove item by index (can specify multiple)")
	templateEditCmd.Flags().BoolVar(&templateEditClearChild, "clear-items", false, "remove all items")
	templateEditCmd.Flags().BoolVar(&templateEditEnabled, "enabled", false, "enable or disable template")
}

func runTemplateEdit(cmd *cobra.Command, args []string) error {
	templateID := args[0]

	cfg, configManager, err := LoadConfigStrict()
	if err != nil {
		return err
	}

	t := cfg.GetTaskTemplate(templateID)
	if t == nil {
		return fmt.Errorf("task template not found: %s", templateID)
	}

	// Track if any changes were made
	hasChanges := false

	// Apply changes
	if cmd.Flags().Changed("name") {
		name := strings.TrimSpace(templateEditName)
		if name == "" {
			return fmt.Errorf("template name cannot be empty")
		}
		t.Name = name
		hasChanges = true
	}

	if cmd.Flags().Changed("type") {
		taskType := models.TaskType(strings.ToLower(templateEditType))
		validTypes := map[models.TaskType]bool{
			models.TaskTypeEpic:  true,
			models.TaskTypeStory: true,
			models.TaskTypeTask:  true,
			models.TaskTypeBug:   true,
			models.TaskTypeIdea:  true,
		}
		if !validTypes[taskType] {
			return fmt.Errorf("invalid task type: %s", templateEditType)
		}
		t.Type = taskType
		hasChanges = true
	}

	if cmd.Flags().Changed("priority") {
		if templateEditPriority < 0 || templateEditPriority > models.PriorityMax {
			return fmt.Errorf("priority must be 0-%d", models.PriorityMax)
		}
		t.Priority = templateEditPriority
		hasChanges = true
	}

	if cmd.Flags().Changed("tags") {
		var tags []string
		if templateEditTags != "" {
			for tag := range strings.SplitSeq(templateEditTags, ",") {
				tag = strings.TrimSpace(tag)
				if tag != "" {
					tags = append(tags, tag)
				}
			}
		}
		t.Tags = tags
		hasChanges = true
	}

	if cmd.Flags().Changed("section") {
		t.Section = templateEditSection
		hasChanges = true
	}

	if cmd.Flags().Changed("description") {
		t.Description = templateEditDescription
		hasChanges = true
	}

	if cmd.Flags().Changed("category") {
		if templateEditCategory != "" {
			validCats := map[string]bool{"bug": true, "feature": true, "process": true}
			if !validCats[templateEditCategory] {
				return fmt.Errorf("invalid category: %s", templateEditCategory)
			}
		}
		t.Category = templateEditCategory
		hasChanges = true
	}

	if cmd.Flags().Changed("enabled") {
		t.Enabled = templateEditEnabled
		hasChanges = true
	}

	// Handle children modifications
	if templateEditClearChild {
		// Allow clearing without adding new ones:
		// if len(templateEditAddChildren) == 0 && len(templateEditAddOptChild) == 0 {}
		t.Children = []template.TemplateChild{}
		hasChanges = true
	}

	if len(templateEditRemoveChild) > 0 {
		// Sort indices in reverse order to remove from end first
		indices := make([]int, len(templateEditRemoveChild))
		copy(indices, templateEditRemoveChild)
		for i := range indices {
			for j := i + 1; j < len(indices); j++ {
				if indices[j] > indices[i] {
					indices[i], indices[j] = indices[j], indices[i]
				}
			}
		}
		for _, idx := range indices {
			if idx < 0 || idx >= len(t.Children) {
				return fmt.Errorf("child index %d out of range (0-%d)", idx, len(t.Children)-1)
			}
			t.Children = append(t.Children[:idx], t.Children[idx+1:]...)
		}
		hasChanges = true
	}

	for _, title := range templateEditAddChildren {
		title = strings.TrimSpace(title)
		if title != "" {
			t.Children = append(t.Children, template.TemplateChild{
				Title:    title,
				Optional: false,
			})
			hasChanges = true
		}
	}

	for _, title := range templateEditAddOptChild {
		title = strings.TrimSpace(title)
		if title != "" {
			t.Children = append(t.Children, template.TemplateChild{
				Title:    title,
				Optional: true,
			})
			hasChanges = true
		}
	}

	if !hasChanges {
		return fmt.Errorf("no changes specified; use --name, --type, --priority, --tags, etc")
	}

	// Validate
	if err := t.Validate(); err != nil {
		return fmt.Errorf("invalid template: %w", err)
	}

	// Update
	if !cfg.UpdateTaskTemplate(*t) {
		return fmt.Errorf("failed to update template")
	}

	if err := configManager.Save(cfg); err != nil {
		return tterrors.NewSaveConfigError(err)
	}

	fmt.Printf("Updated task template: %s [%s]\n", t.Name, t.ID)
	fmt.Printf("  %s\n", t.Summary())
	if len(t.Children) > 0 {
		fmt.Printf("  Items: %d\n", len(t.Children))
	}

	return nil
}
