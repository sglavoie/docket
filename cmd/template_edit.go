package cmd

import (
	"fmt"
	"slices"
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

  # Add an item (child subtask)
  tt template edit abc123 --add-item "New Step"

  # Add an optional item
  tt template edit abc123 --add-optional-item "Optional Step"

  # Remove an item by index (use 'tt template show' to see indices)
  tt template edit abc123 --remove-item 2

  # Clear all items
  tt template edit abc123 --clear-items

  # Add a checklist item
  tt template edit abc123 --add-checklist "Verify output"

  # Remove a checklist item by index (1-based, use 'tt template show')
  tt template edit abc123 --remove-checklist 2

  # Clear all checklist items
  tt template edit abc123 --clear-checklist`,
	RunE: runTemplateEdit,
}

var (
	templateEditName            string
	templateEditType            string
	templateEditPriority        int
	templateEditTags            string
	templateEditSection         string
	templateEditDescription     string
	templateEditCategory        string
	templateEditAddChildren     []string
	templateEditAddOptChild     []string
	templateEditRemoveChild     []int
	templateEditClearChild      bool
	templateEditEnabled         bool
	templateEditAddChecklist    []string
	templateEditRemoveChecklist []int
	templateEditClearChecklist  bool
)

func init() {
	templateCmd.AddCommand(templateEditCmd)

	templateEditCmd.Flags().StringVarP(&templateEditName, "name", "n", "", "new template name")
	templateEditCmd.Flags().StringVar(&templateEditType, "type", "", "task type: epic, story, task, bug, idea")
	templateEditCmd.Flags().IntVar(&templateEditPriority, "priority", 0, "priority 1-5 (0 for default)")
	templateEditCmd.Flags().StringVar(&templateEditTags, "tags", "", "comma-separated tags (replaces existing)")
	templateEditCmd.Flags().StringVar(&templateEditSection, "section", "", "built-in GTD section (empty = Active)")
	templateEditCmd.Flags().StringVar(&templateEditDescription, "description", "", "task description template")
	templateEditCmd.Flags().StringVar(&templateEditCategory, "category", "", "category: bug, feature, process")
	templateEditCmd.Flags().StringArrayVar(&templateEditAddChildren, "add-item", nil, "add checklist item (can specify multiple)")
	templateEditCmd.Flags().StringArrayVar(&templateEditAddOptChild, "add-optional-item", nil, "add optional checklist item (can specify multiple)")
	templateEditCmd.Flags().IntSliceVar(&templateEditRemoveChild, "remove-item", nil, "remove item by index (can specify multiple)")
	templateEditCmd.Flags().BoolVar(&templateEditClearChild, "clear-items", false, "remove all items")
	templateEditCmd.Flags().BoolVar(&templateEditEnabled, "enabled", false, "enable or disable template")
	templateEditCmd.Flags().StringArrayVar(&templateEditAddChecklist, "add-checklist", nil, "add checklist item (can specify multiple)")
	templateEditCmd.Flags().IntSliceVar(&templateEditRemoveChecklist, "remove-checklist", nil, "remove checklist item by index (1-based, can specify multiple)")
	templateEditCmd.Flags().BoolVar(&templateEditClearChecklist, "clear-checklist", false, "remove all checklist items")

	_ = templateEditCmd.RegisterFlagCompletionFunc("type", completeEnum(taskTypeCompletions))
	_ = templateEditCmd.RegisterFlagCompletionFunc("category", completeEnum(categoryCompletions))
	_ = templateEditCmd.RegisterFlagCompletionFunc("section", completeProjectSections())
	addOutputFlags(templateEditCmd)
}

func runTemplateEdit(cmd *cobra.Command, args []string) error { //nolint:gocyclo
	if len(args) < 1 {
		return tterrors.NewRequiredFieldError("template ID or name")
	}

	out, err := getOutputOpts(cmd)
	if err != nil {
		return err
	}

	templateID := args[0]

	workspace, err := LoadCLIWorkspaceStrict()
	if err != nil {
		return err
	}
	domains, err := cloneCLIWorkspaceDomains(workspace)
	if err != nil {
		return err
	}
	automation := domains.Automation

	t := automation.GetTaskTemplate(templateID)
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
		section, sectionErr := normalizeTemplateSectionInput(templateEditSection)
		if sectionErr != nil {
			return sectionErr
		}
		t.Section = section
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
		t.Children = []template.TemplateChild{}
		hasChanges = true
	}

	if len(templateEditRemoveChild) > 0 {
		if len(t.Children) == 0 {
			return fmt.Errorf("template has no items to remove")
		}
		// Convert 1-based user indices to 0-based, deduplicate, sort descending
		indices := make([]int, 0, len(templateEditRemoveChild))
		for _, idx := range templateEditRemoveChild {
			indices = append(indices, idx-1) // convert to 0-based
		}
		indices = slices.Compact(slices.Sorted(slices.Values(indices)))
		slices.Reverse(indices)
		for _, idx := range indices {
			if idx < 0 || idx >= len(t.Children) {
				return fmt.Errorf("item index %d out of range (1-%d)", idx+1, len(t.Children))
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

	// Handle checklist modifications
	if templateEditClearChecklist {
		t.Checklist = []string{}
		hasChanges = true
	}

	if len(templateEditRemoveChecklist) > 0 {
		if len(t.Checklist) == 0 {
			return fmt.Errorf("template has no checklist items to remove")
		}
		// Convert 1-based user indices to 0-based, deduplicate, sort descending
		indices := make([]int, 0, len(templateEditRemoveChecklist))
		for _, idx := range templateEditRemoveChecklist {
			indices = append(indices, idx-1) // convert to 0-based
		}
		indices = slices.Compact(slices.Sorted(slices.Values(indices)))
		slices.Reverse(indices)
		for _, idx := range indices {
			if idx < 0 || idx >= len(t.Checklist) {
				return fmt.Errorf("checklist index %d out of range (1-%d)", idx+1, len(t.Checklist))
			}
			t.Checklist = append(t.Checklist[:idx], t.Checklist[idx+1:]...)
		}
		hasChanges = true
	}

	for _, item := range templateEditAddChecklist {
		item = strings.TrimSpace(item)
		if item != "" {
			t.Checklist = append(t.Checklist, item)
			hasChanges = true
		}
	}

	if !hasChanges {
		return tterrors.NewNoChangesError("--name, --type, --priority, --tags, etc")
	}

	// Validate
	if err := t.Validate(); err != nil {
		return fmt.Errorf("invalid template: %w", err)
	}

	// Update
	if !automation.UpdateTaskTemplate(*t) {
		return fmt.Errorf("failed to update template")
	}

	if err := commitCLIAutomation(workspace, automation); err != nil {
		return err
	}

	if out.Quiet {
		return nil
	}
	if out.JSON {
		return printJSON(struct {
			ID      string `json:"id"`
			Name    string `json:"name"`
			Type    string `json:"type"`
			Summary string `json:"summary"`
		}{ID: t.ID, Name: t.Name, Type: string(t.Type), Summary: t.Summary()})
	}
	fmt.Printf("Updated task template: %s [%s]\n", t.Name, t.ID)
	fmt.Printf("  %s\n", t.Summary())
	if len(t.Checklist) > 0 {
		fmt.Printf("  Checklist: %d\n", len(t.Checklist))
	}
	if len(t.Children) > 0 {
		fmt.Printf("  Items: %d\n", len(t.Children))
	}

	return nil
}
