package cmd

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tterrors"
)

var templateUseCmd = &cobra.Command{
	Use:   "use <template-id> [title]",
	Short: "Create a task from a template",
	Long: `Create a new task using a template.

The template defines the task type, priority, tags, section, and checklist items.
If a title is provided, it will be used; otherwise, the template name is used.

Checklist items are created automatically based on the template definition.

FLAGS
  --skip-optional  Don't create optional children
  --parent         Parent task ID for the new task

EXAMPLES

  # Create a bug report
  tt template use def-bug-report "Login fails on mobile"

  # Create with default title (template name)
  tt template use def-release-checklist

  # Skip optional children
  tt template use def-bug-report "Crash on startup" --skip-optional

  # Create under a parent task
  tt template use def-api-endpoint "GET /users" --parent 42`,
	Args: cobra.RangeArgs(1, 2),
	RunE: runTemplateUse,
}

var (
	templateUseSkipOptional bool
	templateUseParent       string
)

func init() {
	templateCmd.AddCommand(templateUseCmd)

	templateUseCmd.Flags().BoolVar(&templateUseSkipOptional, "skip-optional", false, "don't create optional children")
	templateUseCmd.Flags().StringVarP(&templateUseParent, "parent", "p", "", "parent task ID or short ID")
}

func runTemplateUse(cmd *cobra.Command, args []string) error {
	templateID := args[0]
	title := ""
	if len(args) > 1 {
		title = strings.Join(args[1:], " ")
	}

	// Load config
	cfg, configManager, err := LoadConfigStrict()
	if err != nil {
		return err
	}

	// Find template by ID or name
	t := cfg.GetTaskTemplate(templateID)
	if t == nil {
		// Try by name
		t = cfg.GetTaskTemplateByName(templateID)
	}
	if t == nil {
		return fmt.Errorf("task template not found: %s", templateID)
	}

	// Use template name as title if not provided
	if title == "" {
		title = t.Name
	}

	// Load current project
	project, _, projectConfigManager, err := LoadCurrentProject()
	if err != nil {
		return err
	}

	// Create main task
	taskID := uuid.New().String()
	shortID := project.GetNextShortID()
	task := models.NewTask(taskID, shortID, t.Type, title)

	// Apply template settings
	task.Priority = t.GetEffectivePriority()
	if t.Section != "" {
		project.EnsureSection(t.Section)
		task.Section = t.Section
	}
	for _, tag := range t.Tags {
		task.AddTag(tag)
	}
	if t.Description != "" {
		task.Description = t.Description
	}

	// Handle parent
	if templateUseParent != "" {
		parent, err := ValidateParentTask(project, templateUseParent, task.Type)
		if err != nil {
			return err
		}
		task.ParentID = &parent.ID
	}

	// Add task to project
	project.Tasks = append(project.Tasks, task)

	// Create checklist from template
	checklistCreated := 0
	for _, item := range t.Checklist {
		task.Checklist = append(task.Checklist, models.ChecklistItem{
			Text: item,
			Done: false,
		})
		checklistCreated++
	}

	// Create subtasks from Children (for Epic/Story templates)
	subtasksCreated := 0
	if t.Type.CanHaveChildren() && len(t.Children) > 0 {
		for _, child := range t.Children {
			if child.Optional && templateUseSkipOptional {
				continue
			}
			childID := uuid.New().String()
			childShortID := project.GetNextShortID()
			childType := child.GetChildType()
			childTask := models.NewTask(childID, childShortID, childType, child.Title)
			childTask.ParentID = &task.ID
			childTask.Priority = task.Priority
			childTask.Section = task.Section
			for _, tag := range task.Tags {
				childTask.AddTag(tag)
			}
			project.Tasks = append(project.Tasks, childTask)
			subtasksCreated++
		}
	}

	// Rebuild tree and save
	project.BuildTaskTree()
	project.ReorderRootTasksByCompletion()

	// Increment use count
	cfg.IncrementTemplateUseCount(t.ID)
	if err := configManager.Save(cfg); err != nil {
		return tterrors.NewSaveConfigError(err)
	}

	// Update project timestamp
	project.UpdatedAt = time.Now()

	// Save project
	if err := SaveProject(project, &SaveOptions{EvaluateRules: true, Config: cfg, ConfigManager: projectConfigManager}); err != nil {
		return tterrors.NewSaveProjectError(project.Name, err)
	}

	fmt.Printf("Created %s #%d: %s (from template %q)\n", task.Type.HeadingPrefix(), shortID, title, t.Name)
	if checklistCreated > 0 {
		fmt.Printf("  Added %d checklist item(s)\n", checklistCreated)
	}
	if subtasksCreated > 0 {
		fmt.Printf("  Created %d subtask(s)\n", subtasksCreated)
	}

	return nil
}
