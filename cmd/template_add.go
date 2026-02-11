package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/template"
	"github.com/sglavoie/tt/internal/tterrors"
)

var templateAddCmd = &cobra.Command{
	Use:   "add",
	Short: "Add a new task template",
	Long: `Add a new task template for creating tasks with pre-configured settings.

Templates define task type, priority, tags, section, and checklist items.
When using a template, the parent task is created with the specified settings,
and checklist items are added to it.

FLAGS
  -n, --name        Template name (required)
  --type            Task type: epic, story, task, bug, idea (default: task)
  --priority        Priority 1-5 (0 for default)
  --tags            Comma-separated tags
  --section         Section name (e.g., Next, Backlog)
  --description     Task description template
  --category        Category: bug, feature, process
  --item              Checklist item (can specify multiple)
  --optional-item     Optional checklist item (can specify multiple)
  --disabled        Create template in disabled state

EXAMPLES

  # Simple template with no children
  tt template add -n "Quick Task" --type task

  # Bug report template
  tt template add -n "Bug Report" --type bug --tags bug \
    --item "Steps to Reproduce" \
    --item "Expected Behavior" \
    --item "Actual Behavior"

  # Feature template with optional items
  tt template add -n "Feature" --type story --category feature \
    --item "User Story" \
    --item "Acceptance Criteria" \
    --optional-item "Design Notes"

  # Release checklist
  tt template add -n "Release" --type task --tags release --section Next \
    --item "Run Tests" \
    --item "Update Changelog" \
    --item "Deploy"`,
	RunE: runTemplateAdd,
}

var (
	templateAddName        string
	templateAddType        string
	templateAddPriority    int
	templateAddTags        string
	templateAddSection     string
	templateAddDescription string
	templateAddCategory    string
	templateAddChildren    []string
	templateAddOptChildren []string
	templateAddDisabled    bool
)

func init() {
	templateCmd.AddCommand(templateAddCmd)

	templateAddCmd.Flags().StringVarP(&templateAddName, "name", "n", "", "template name (required)")
	templateAddCmd.Flags().StringVar(&templateAddType, "type", "task", "task type: epic, story, task, bug, idea")
	templateAddCmd.Flags().IntVar(&templateAddPriority, "priority", 0, "priority 1-5 (0 for default)")
	templateAddCmd.Flags().StringVar(&templateAddTags, "tags", "", "comma-separated tags")
	templateAddCmd.Flags().StringVar(&templateAddSection, "section", "", "section name")
	templateAddCmd.Flags().StringVar(&templateAddDescription, "description", "", "task description template")
	templateAddCmd.Flags().StringVar(&templateAddCategory, "category", "", "category: bug, feature, process")
	templateAddCmd.Flags().StringArrayVar(&templateAddChildren, "item", nil, "checklist item (can specify multiple)")
	templateAddCmd.Flags().StringArrayVar(&templateAddOptChildren, "optional-item", nil, "optional checklist item (can specify multiple)")
	templateAddCmd.Flags().BoolVar(&templateAddDisabled, "disabled", false, "create template in disabled state")

	_ = templateAddCmd.MarkFlagRequired("name")
}

func runTemplateAdd(cmd *cobra.Command, args []string) error {
	// Validate name
	name := strings.TrimSpace(templateAddName)
	if name == "" {
		return fmt.Errorf("template name is required")
	}

	// Parse task type
	taskType := models.TaskType(strings.ToLower(templateAddType))
	validTypes := map[models.TaskType]bool{
		models.TaskTypeEpic:  true,
		models.TaskTypeStory: true,
		models.TaskTypeTask:  true,
		models.TaskTypeBug:   true,
		models.TaskTypeIdea:  true,
	}
	if !validTypes[taskType] {
		return fmt.Errorf("invalid task type: %s (valid: epic, story, task, bug, idea)", templateAddType)
	}

	// Validate priority
	if err := ValidatePriority(templateAddPriority, true); err != nil {
		return err
	}

	// Validate category
	if templateAddCategory != "" {
		validCats := map[string]bool{"bug": true, "feature": true, "process": true}
		if !validCats[templateAddCategory] {
			return fmt.Errorf("invalid category: %s (valid: bug, feature, process)", templateAddCategory)
		}
	}

	// Parse tags
	var tags []string
	if templateAddTags != "" {
		for _, tag := range strings.Split(templateAddTags, ",") {
			tag = strings.TrimSpace(tag)
			if tag != "" {
				tags = append(tags, tag)
			}
		}
	}

	// Build children list
	var children []template.TemplateChild
	for _, title := range templateAddChildren {
		title = strings.TrimSpace(title)
		if title != "" {
			children = append(children, template.TemplateChild{
				Title:    title,
				Optional: false,
			})
		}
	}
	for _, title := range templateAddOptChildren {
		title = strings.TrimSpace(title)
		if title != "" {
			children = append(children, template.TemplateChild{
				Title:    title,
				Optional: true,
			})
		}
	}

	// Create template
	t := template.NewTemplate(name, taskType)
	t.Enabled = !templateAddDisabled
	t.Priority = templateAddPriority
	t.Tags = tags
	t.Section = templateAddSection
	t.Description = templateAddDescription
	t.Category = templateAddCategory
	t.Children = children

	// Validate
	if err := t.Validate(); err != nil {
		return fmt.Errorf("invalid template: %w", err)
	}

	// Load config and save
	cfg, configManager, err := LoadConfigStrict()
	if err != nil {
		return err
	}

	cfg.AddTaskTemplate(*t)

	if err := configManager.Save(cfg); err != nil {
		return tterrors.NewSaveConfigError(err)
	}

	fmt.Printf("Created task template: %s [%s]\n", t.Name, t.ID)
	fmt.Printf("  %s\n", t.Summary())
	if len(children) > 0 {
		fmt.Printf("  Items: %d\n", len(children))
	}
	if !t.Enabled {
		fmt.Println("  (disabled)")
	}

	return nil
}
