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

Templates define task type, priority, tags, canonical state, and checklist items.
When using a template, the parent task is created with the specified settings,
and checklist items are added to it.

FLAGS
  -n, --name        Template name (required)
  --type            Task type: epic, story, task, bug, idea (default: task)
  --priority        Priority 1-5 (0 for default)
  --tags            Comma-separated tags
  --state           Canonical workflow state
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
  tt template add -n "Release" --type task --tags release --state next \
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
	templateAddState       string
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
	templateAddCmd.Flags().StringVar(&templateAddState, "state", "", "canonical workflow state")
	templateAddCmd.Flags().StringVar(&templateAddDescription, "description", "", "task description template")
	templateAddCmd.Flags().StringVar(&templateAddCategory, "category", "", "category: bug, feature, process")
	templateAddCmd.Flags().StringArrayVar(&templateAddChildren, "item", nil, "checklist item (can specify multiple)")
	templateAddCmd.Flags().StringArrayVar(&templateAddOptChildren, "optional-item", nil, "optional checklist item (can specify multiple)")
	templateAddCmd.Flags().BoolVar(&templateAddDisabled, "disabled", false, "create template in disabled state")
	addOutputFlags(templateAddCmd)

	_ = templateAddCmd.MarkFlagRequired("name")

	_ = templateAddCmd.RegisterFlagCompletionFunc("type", completeEnum(taskTypeCompletions))
	_ = templateAddCmd.RegisterFlagCompletionFunc("category", completeEnum(categoryCompletions))
	_ = templateAddCmd.RegisterFlagCompletionFunc("state", completeEnum(stateCompletions))
}

func runTemplateAdd(cmd *cobra.Command, args []string) error {
	out, err := getOutputOpts(cmd)
	if err != nil {
		return err
	}

	// Validate name
	name := strings.TrimSpace(templateAddName)
	if name == "" {
		return tterrors.NewRequiredFieldError("template name")
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
		return tterrors.NewInvalidValueError("task type", templateAddType, "valid: epic, story, task, bug, idea")
	}

	// Validate priority; normalize 0 (meaning "use default") to PriorityDefault
	err = ValidatePriority(templateAddPriority, true)
	if err != nil {
		return err
	}
	if templateAddPriority == 0 {
		templateAddPriority = models.PriorityDefault
	}

	// Validate category
	if templateAddCategory != "" {
		validCats := map[string]bool{"bug": true, "feature": true, "process": true}
		if !validCats[templateAddCategory] {
			return tterrors.NewInvalidValueError("category", templateAddCategory, "valid: bug, feature, process")
		}
	}

	state := models.StateInbox
	if stagedLifecycleFlagProvided(cmd, "state", templateAddState) {
		state, err = parseCanonicalWorkflowStateInput("state", templateAddState)
		if err != nil {
			return err
		}
	}

	// Parse tags
	var tags []string
	if templateAddTags != "" {
		for tag := range strings.SplitSeq(templateAddTags, ",") {
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
	t.State = state
	t.Description = templateAddDescription
	t.Category = templateAddCategory
	t.Children = children

	// Validate
	err = t.ValidateForAuthoring()
	if err != nil {
		return fmt.Errorf("invalid template: %w", err)
	}

	workspace, err := LoadCLIWorkspaceStrict()
	if err != nil {
		return err
	}
	domains, err := cloneCLIWorkspaceDomains(workspace)
	if err != nil {
		return err
	}

	domains.Automation.AddTaskTemplate(*t)

	if err := commitCLIAutomation(workspace, domains.Automation); err != nil {
		return err
	}

	if out.Quiet {
		return nil
	}
	if out.JSON {
		entry := struct {
			ID      string `json:"id"`
			Name    string `json:"name"`
			Type    string `json:"type"`
			State   string `json:"state"`
			Enabled bool   `json:"enabled"`
			Items   int    `json:"items"`
			Summary string `json:"summary"`
		}{
			ID:      t.ID,
			Name:    t.Name,
			Type:    string(t.Type),
			State:   t.EffectiveState().String(),
			Enabled: t.Enabled,
			Items:   len(children),
			Summary: t.Summary(),
		}
		return printJSON(entry)
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
