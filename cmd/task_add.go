package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/template"
	"github.com/sglavoie/tt/internal/tterrors"
)

var (
	taskTypeFlag     string
	taskParentFlag   string
	taskPriority     int
	taskTags         []string
	taskSectionFlag  string
	taskRecurFlag    string
	taskAddFromFlag  string
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
	taskAddCmd.Flags().StringVar(&taskRecurFlag, "recur", "", "recurrence pattern (daily, weekly, monthly, yearly, Nd, Nw, Nm, Ny)")
	taskAddCmd.Flags().StringVar(&taskAddFromFlag, "from", "", "recurrence mode with --recur: 'due' (default) or 'completion'")
	taskAddCmd.Flags().StringVar(&taskTemplateFlag, "template", "", "create from template (ID or name)")

	taskAddCmd.Flags().StringP("project", "p", "", "target project slug (default: current project)")

	// Type-specific attribute flags
	registerTypeAttrFlags(taskAddCmd, &addTypeAttrs)

	addOutputFlags(taskAddCmd)

	_ = taskAddCmd.RegisterFlagCompletionFunc("type", completeEnum(taskTypeCompletions))
	_ = taskAddCmd.RegisterFlagCompletionFunc("section", completeProjectSections())
	_ = taskAddCmd.RegisterFlagCompletionFunc("project", completeProjectSlugs())
	_ = taskAddCmd.RegisterFlagCompletionFunc("template", completeTemplates())
}

func runTaskAdd(cmd *cobra.Command, args []string) error { //nolint:gocyclo
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
		project   *models.Project
		workspace *CLIWorkspace
	)

	if slug, _ := cmd.Flags().GetString("project"); slug != "" {
		workspace, err = LoadCLIWorkspace()
		if err != nil {
			return err
		}
		project = workspace.Runtime.GetProject(slug)
		if project == nil {
			return tterrors.NewProjectNotFoundError(slug)
		}
		if err = storage.LoadProject(project); err != nil {
			return fmt.Errorf("failed to load project %s: %w", slug, err)
		}
	} else {
		project, workspace, err = LoadRuntimeCurrentProject()
		if err != nil {
			return err
		}
	}

	// Load template if specified
	var tmpl *template.TaskTemplate
	templateLookup := strings.TrimSpace(taskTemplateFlag)
	if templateLookup != "" {
		tmpl = workspace.Runtime.Automation().GetTaskTemplate(templateLookup)
		if tmpl == nil {
			// Try by name
			tmpl = workspace.Runtime.Automation().GetTaskTemplateByName(templateLookup)
		}
		if tmpl == nil {
			return fmt.Errorf("template not found: %s", templateLookup)
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

	// Resolve tags (template tags first, then flags), section, description,
	// recurrence, parent, checklist, and type attributes into presentation-neutral
	// primitives. The application-layer AddTask use case owns task construction,
	// validation, ID/short-ID minting, and the single atomic commit.
	tags := make([]string, 0, len(taskTags))
	if tmpl != nil {
		tags = append(tags, tmpl.Tags...)
	}
	tags = append(tags, taskTags...)

	// Handle section (explicit flag overrides template, using Changed() for
	// consistency). Sections are limited to built-in GTD buckets; the app layer
	// re-validates and ensures the section on the freshly snapshotted project.
	section := ""
	if cmd.Flags().Changed("section") {
		section = taskSectionFlag
	} else if tmpl != nil && tmpl.Section != "" {
		section = tmpl.Section
	}

	description := ""
	if tmpl != nil && tmpl.Description != "" {
		description = tmpl.Description
	}

	// Handle recurrence (parsed here so --from precedence and error messages match
	// the legacy surface; SetRecurrence itself runs inside the use case).
	var recurrence *models.RecurrencePattern
	if taskRecurFlag != "" {
		recurrence = models.ParseRecurrence(taskRecurFlag)
		if recurrence == nil {
			return tterrors.NewInvalidValueError("recurrence", taskRecurFlag, "use daily, weekly, weekly:mon,fri, monthly, yearly, Nd, Nw, Nm, Ny")
		}
		// --from overrides any mode embedded in the pattern string (e.g. trailing ~)
		if cmd.Flags().Changed("from") {
			switch taskAddFromFlag {
			case "due":
				recurrence.Mode = models.RecurModeFromDue
			case "completion":
				recurrence.Mode = models.RecurModeFromCompletion
			default:
				return fmt.Errorf("invalid --from value %q: must be 'due' or 'completion'", taskAddFromFlag)
			}
		}
	} else if cmd.Flags().Changed("from") {
		return fmt.Errorf("--from requires --recur to be set")
	}

	// Resolve parent to its stable UUID (with the legacy hierarchy/depth checks and
	// error messages); the use case re-validates against the fresh snapshot.
	var parentID *string
	if taskParentFlag != "" {
		parent, parentErr := ValidateParentTask(project, taskParentFlag, taskType)
		if parentErr != nil {
			return parentErr
		}
		pid := parent.ID
		parentID = &pid
	}

	// Materialize the template checklist (prefer Checklist over legacy Children).
	var checklist []string
	if tmpl != nil {
		if len(tmpl.Checklist) > 0 {
			checklist = append(checklist, tmpl.Checklist...)
		} else if len(tmpl.Children) > 0 {
			fmt.Fprintf(os.Stderr, "warning: template %q uses deprecated 'children' field; use 'checklist' instead\n", tmpl.ID)
			for _, childTmpl := range tmpl.Children {
				checklist = append(checklist, childTmpl.Title)
			}
		}
	}
	checklistCreated := len(checklist)

	addTypeAttrs.IdeaValidatedSet = cmd.Flags().Changed("idea.validated")
	addTypeAttrs.StoryPointsSet = cmd.Flags().Changed("story.points")

	// When a template is used, its use-count bump rides the SAME commit as the new
	// task so the pair is atomic — replacing the legacy save-then-bump sequence that
	// could persist a task without the counter (or vice-versa) on a partial failure.
	var artifacts []storage.ArtifactChange
	if tmpl != nil {
		domains, cloneErr := workspace.Runtime.CloneDomains()
		if cloneErr != nil {
			return cloneErr
		}
		domains.Automation.IncrementTemplateUseCount(tmpl.ID)
		arts, artifactErr := encodeAutomationArtifact(workspace.Runtime, domains.Automation)
		if artifactErr != nil {
			return artifactErr
		}
		artifacts = arts
	}

	workflows := newCLIWorkflowRunner(workspace)
	defer workflows.flush()
	cascade, err := withResultRetry(func() (app.BulkResult, error) {
		return workflows.service.AddTaskWithWorkflow(app.AddTaskRequest{
			ProjectID:   project.ID,
			Title:       title,
			Type:        taskType,
			Priority:    priority,
			Tags:        tags,
			Description: description,
			Section:     section,
			Recurrence:  recurrence,
			ParentID:    parentID,
			Checklist:   checklist,
			Attrs:       addTypeAttrs,
			Artifacts:   artifacts,
		}, workflows.options(false, "Add Task", nil))
	})
	if err != nil {
		return translateAppError(err)
	}
	workflows.observe(cascade)
	res := workflowResultTask(cascade)

	if err := emitMutation(out, "Created", res); err != nil {
		return err
	}
	if !out.Quiet && !out.JSON && checklistCreated > 0 {
		fmt.Printf("  Added %d checklist item(s) from template\n", checklistCreated)
	}
	return nil
}
