package cmd

import (
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/template"
	"github.com/sglavoie/tt/internal/tterrors"
)

var (
	taskTypeFlag     string
	taskParentFlag   string
	taskPriority     int
	taskTags         []string
	taskSectionFlag  string
	taskStageFlag    string
	taskRecurFlag    string
	taskTemplateFlag string
	isEpic           bool
	isStory          bool
	isBug            bool
	isIdea           bool

	// Type-specific attribute flags (shared registration)
	addTypeAttrs TypeAttrValues
)

var taskAddCmd = &cobra.Command{
	Use:   "add <title>",
	Short: "Add a new task",
	Long: `Add a new task to the current project.

The default type is "task". Use flags to specify other types:
  --epic, --story, --bug, --idea, or --type <type>

Tasks can be nested under epics or stories using --parent.

Use --template to create a task from a template. The template provides
default values for type, priority, tags, section, and creates child tasks.
Other flags override template values.

Examples:
  tt task add "Fix login bug"
  tt task add --epic "User Authentication"
  tt task add --story "Login Flow" --parent 1
  tt task add "Create API endpoint" --parent 2 --priority 1
  tt t a "Quick task" --tags backend,urgent
  tt task add "Login fails" --template def-bug-report`,
	Args: cobra.MinimumNArgs(1),
	RunE: runTaskAdd,
}

func init() {
	taskCmd.AddCommand(taskAddCmd)

	taskAddCmd.Flags().StringVarP(&taskTypeFlag, "type", "t", "", "task type (epic, story, task, bug, idea)")
	taskAddCmd.Flags().StringVarP(&taskParentFlag, "parent", "p", "", "parent task ID or short ID")
	taskAddCmd.Flags().IntVar(&taskPriority, "priority", 3, "priority (1=highest, 5=lowest)")
	taskAddCmd.Flags().StringSliceVar(&taskTags, "tags", nil, "tags for the task")

	taskAddCmd.Flags().BoolVar(&isEpic, "epic", false, "create an epic")
	taskAddCmd.Flags().BoolVar(&isStory, "story", false, "create a story")
	taskAddCmd.Flags().BoolVar(&isBug, "bug", false, "create a bug")
	taskAddCmd.Flags().BoolVar(&isIdea, "idea", false, "create an idea")
	taskAddCmd.Flags().StringVarP(&taskSectionFlag, "section", "s", "", "section for the task (default: project's first section)")
	taskAddCmd.Flags().StringVar(&taskStageFlag, "stage", "", "workflow stage (e.g., Brainstorm, Design, Implement)")
	taskAddCmd.Flags().StringVar(&taskRecurFlag, "recur", "", "recurrence pattern (daily, weekly, monthly, yearly, Nd, Nw, Nm, Ny)")
	taskAddCmd.Flags().StringVar(&taskTemplateFlag, "template", "", "create from template (ID or name)")

	// Type-specific attribute flags
	registerTypeAttrFlags(taskAddCmd, &addTypeAttrs)
}

func runTaskAdd(cmd *cobra.Command, args []string) error {
	project, cfg, configManager, err := LoadCurrentProject()
	if err != nil {
		return err
	}

	title := strings.Join(args, " ")
	if title == "" {
		return tterrors.NewRequiredFieldError("title")
	}
	if err := models.ValidateTaskTitle(title); err != nil {
		return err
	}

	// Load template if specified
	var tmpl *template.TaskTemplate
	if taskTemplateFlag != "" {
		tmpl = cfg.GetTaskTemplate(taskTemplateFlag)
		if tmpl == nil {
			// Try by name
			tmpl = cfg.GetTaskTemplateByName(taskTemplateFlag)
		}
		if tmpl == nil {
			return fmt.Errorf("template not found: %s", taskTemplateFlag)
		}
	}

	// Determine task type (flag overrides template)
	var taskType models.TaskType
	if isEpic || isStory || isBug || isIdea || taskTypeFlag != "" {
		taskType, err = ParseTaskTypeFlags(isEpic, isStory, isBug, isIdea, taskTypeFlag)
		if err != nil {
			return err
		}
	} else if tmpl != nil {
		taskType = tmpl.Type
	} else {
		taskType = models.TaskTypeTask
	}

	// Determine priority (flag overrides template, but only if explicitly set)
	priority := taskPriority
	if !cmd.Flags().Changed("priority") && tmpl != nil && tmpl.Priority > 0 {
		priority = tmpl.Priority
	}

	// Validate priority
	if priority < models.PriorityMin || priority > models.PriorityMax {
		return tterrors.NewInvalidValueError("priority", priority,
			fmt.Sprintf("must be %d-%d", models.PriorityMin, models.PriorityMax))
	}

	// Create task
	taskID := uuid.New().String()
	shortID := project.GetNextShortID()
	task := models.NewTask(taskID, shortID, taskType, title)
	task.Priority = priority

	// Add tags from template first, then flags
	if tmpl != nil {
		for _, tag := range tmpl.Tags {
			task.AddTag(tag)
		}
	}
	for _, tag := range taskTags {
		task.AddTag(tag)
	}

	// Handle section (flag overrides template)
	if taskSectionFlag != "" {
		project.EnsureSection(taskSectionFlag)
		task.Section = taskSectionFlag
	} else if tmpl != nil && tmpl.Section != "" {
		project.EnsureSection(tmpl.Section)
		task.Section = tmpl.Section
	}

	// Handle stage
	if taskStageFlag != "" {
		task.Stage = taskStageFlag
	}

	// Add description from template
	if tmpl != nil && tmpl.Description != "" {
		task.Description = tmpl.Description
	}

	// Handle recurrence
	if taskRecurFlag != "" {
		pattern := models.ParseRecurrence(taskRecurFlag)
		if pattern == nil {
			return tterrors.NewInvalidValueError("recurrence", taskRecurFlag, "use daily, weekly, monthly, yearly, Nd, Nw, Nm, Ny")
		}
		task.Recurrence = pattern
	}

	// Handle parent
	if taskParentFlag != "" {
		parent, err := ValidateParentTask(project, taskParentFlag, task.Type)
		if err != nil {
			return err
		}
		task.ParentID = &parent.ID
	}

	// Apply type-specific attributes
	addTypeAttrs.IdeaValidatedSet = cmd.Flags().Changed("idea.validated")
	addTypeAttrs.StoryPointsSet = cmd.Flags().Changed("story.points")
	if err := applyTypeAttrValues(task, addTypeAttrs); err != nil {
		return err
	}

	// Add to project
	project.Tasks = append(project.Tasks, task)

	// Create checklist from template (prefer Checklist over legacy Children)
	checklistCreated := 0
	if tmpl != nil {
		if len(tmpl.Checklist) > 0 {
			for _, item := range tmpl.Checklist {
				task.Checklist = append(task.Checklist, models.ChecklistItem{
					Text: item,
					Done: false,
				})
				checklistCreated++
			}
		} else if len(tmpl.Children) > 0 {
			// Legacy support: convert Children to checklist items
			for _, childTmpl := range tmpl.Children {
				task.Checklist = append(task.Checklist, models.ChecklistItem{
					Text: childTmpl.Title,
					Done: false,
				})
				checklistCreated++
			}
		}

		if checklistCreated > 0 {
			// Increment template use count
			cfg.IncrementTemplateUseCount(tmpl.ID)
			_ = configManager.Save(cfg)
		}
	}

	// Save with workflow rules evaluation (state-based rules will fire for new tasks)
	if err := SaveProject(project, &SaveOptions{EvaluateRules: true, Config: cfg, ConfigManager: configManager}); err != nil {
		return tterrors.NewSaveProjectError(project.Name, err)
	}

	fmt.Printf("Created %s #%d: %s\n", taskType.HeadingPrefix(), shortID, title)
	if checklistCreated > 0 {
		fmt.Printf("  Added %d checklist item(s) from template\n", checklistCreated)
	}
	return nil
}
