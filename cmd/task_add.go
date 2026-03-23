package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/google/uuid"
	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/template"
	"github.com/sglavoie/tt/internal/tterrors"
)

var (
	taskTypeFlag          string
	taskParentFlag        string
	taskPriority          int
	taskTags              []string
	taskSectionFlag       string
	taskStageFlag         string
	taskRecurFlag         string
	taskTemplateFlag      string
	isEpic                bool
	isStory               bool
	isBug                 bool
	isIdea                bool
	taskCreateSectionFlag bool

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
  tt t a "Quick task" --tag backend,urgent
  tt task add "Login fails" --template def-bug-report`,
	RunE: runTaskAdd,
}

func init() {
	taskCmd.AddCommand(taskAddCmd)

	taskAddCmd.Flags().StringVarP(&taskTypeFlag, "type", "t", "", "task type (epic, story, task, bug, idea)")
	taskAddCmd.Flags().StringVar(&taskParentFlag, "parent", "", "parent task ID or short ID")
	taskAddCmd.Flags().IntVarP(&taskPriority, "priority", "P", 3, "priority (1=highest, 5=lowest)")
	taskAddCmd.Flags().StringSliceVarP(&taskTags, "tag", "T", nil, "tags for the task")

	taskAddCmd.Flags().BoolVar(&isEpic, "epic", false, "create an epic")
	taskAddCmd.Flags().BoolVar(&isStory, "story", false, "create a story")
	taskAddCmd.Flags().BoolVar(&isBug, "bug", false, "create a bug")
	taskAddCmd.Flags().BoolVar(&isIdea, "idea", false, "create an idea")
	taskAddCmd.Flags().StringVar(&taskSectionFlag, "section", "", "section for the task (default: project's first section)")
	taskAddCmd.Flags().StringVar(&taskStageFlag, "stage", "", "workflow stage (e.g., Brainstorm, Design, Implement)")
	taskAddCmd.Flags().StringVar(&taskRecurFlag, "recur", "", "recurrence pattern (daily, weekly, monthly, yearly, Nd, Nw, Nm, Ny)")
	taskAddCmd.Flags().StringVar(&taskTemplateFlag, "template", "", "create from template (ID or name)")

	taskAddCmd.Flags().StringP("project", "p", "", "target project slug (default: current project)")

	// Type-specific attribute flags
	registerTypeAttrFlags(taskAddCmd, &addTypeAttrs)

	taskAddCmd.Flags().BoolVar(&taskCreateSectionFlag, "create-section", false, "create the section if it does not exist")
	addOutputFlags(taskAddCmd)

	_ = taskAddCmd.RegisterFlagCompletionFunc("type", completeEnum(taskTypeCompletions))
	_ = taskAddCmd.RegisterFlagCompletionFunc("section", completeProjectSections())
	_ = taskAddCmd.RegisterFlagCompletionFunc("stage", completeConfigStages())
	_ = taskAddCmd.RegisterFlagCompletionFunc("project", completeProjectSlugs())
	_ = taskAddCmd.RegisterFlagCompletionFunc("template", completeTemplates())
}

func runTaskAdd(cmd *cobra.Command, args []string) error {
	out, err := getOutputOpts(cmd)
	if err != nil {
		return err
	}

	if len(args) < 1 {
		return tterrors.NewRequiredFieldError("title")
	}

	title := strings.TrimSpace(strings.Join(args, " "))
	if title == "" {
		return tterrors.NewRequiredFieldError("title")
	}
	if err = models.ValidateTaskTitle(title); err != nil {
		return err
	}

	var (
		project       *models.Project
		cfg           *config.Config
		configManager *config.Manager
	)

	if slug, _ := cmd.Flags().GetString("project"); slug != "" {
		cfg, configManager, err = LoadConfig()
		if err != nil {
			return err
		}
		project = cfg.GetProject(slug)
		if project == nil {
			return tterrors.NewProjectNotFoundError(slug)
		}
		if err = storage.LoadProject(project); err != nil {
			return fmt.Errorf("failed to load project %s: %w", slug, err)
		}
	} else {
		project, cfg, configManager, err = LoadCurrentProject()
		if err != nil {
			return err
		}
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
	task, err := models.NewTask(taskID, shortID, taskType, title)
	if err != nil {
		return err
	}
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

	// Handle section (explicit flag overrides template, using Changed() for consistency)
	if cmd.Flags().Changed("section") {
		if !project.HasSection(taskSectionFlag) {
			if !taskCreateSectionFlag {
				return fmt.Errorf("section %q not found; available: %v (use --create-section to create)", taskSectionFlag, project.GetSections())
			}
			if err := models.ValidateSectionName(taskSectionFlag); err != nil {
				return err
			}
			project.EnsureSection(taskSectionFlag)
		}
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
			return tterrors.NewInvalidValueError("recurrence", taskRecurFlag, "use daily, weekly, weekly:mon,fri, monthly, yearly, Nd, Nw, Nm, Ny")
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
			fmt.Fprintf(os.Stderr, "warning: template %q uses deprecated 'children' field; use 'checklist' instead\n", tmpl.ID)
			for _, childTmpl := range tmpl.Children {
				task.Checklist = append(task.Checklist, models.ChecklistItem{
					Text: childTmpl.Title,
					Done: false,
				})
				checklistCreated++
			}
		}

		// Increment template use count (template was used regardless of checklist)
		cfg.IncrementTemplateUseCount(tmpl.ID)
		if err := saveConfig(configManager, cfg); err != nil {
			return err
		}
	}

	// Save with workflow rules evaluation (state-based rules will fire for new tasks)
	if err := SaveProject(project, &SaveOptions{
		EvaluateRules: true, Config: cfg, ConfigManager: configManager,
		UndoAction: "Add Task", UndoContext: fmt.Sprintf("#%d %s", shortID, title),
	}); err != nil {
		return tterrors.NewSaveProjectError(project.Name, err)
	}

	if err := out.OutputTask(project.Name, "Created", task); err != nil {
		return err
	}
	if !out.Quiet && !out.JSON && checklistCreated > 0 {
		fmt.Printf("  Added %d checklist item(s) from template\n", checklistCreated)
	}
	return nil
}
