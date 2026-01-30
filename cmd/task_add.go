package cmd

import (
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tterrors"
)

var (
	taskTypeFlag    string
	taskParentFlag  string
	taskPriority    int
	taskTags        []string
	taskSectionFlag string
	taskRecurFlag   string
	isEpic          bool
	isStory         bool
	isBug           bool
	isIdea          bool
)

var taskAddCmd = &cobra.Command{
	Use:   "add <title>",
	Short: "Add a new task",
	Long: `Add a new task to the current project.

The default type is "task". Use flags to specify other types:
  --epic, --story, --bug, --idea, or --type <type>

Tasks can be nested under epics or stories using --parent.

Examples:
  tt task add "Fix login bug"
  tt task add --epic "User Authentication"
  tt task add --story "Login Flow" --parent 1
  tt task add "Create API endpoint" --parent 2 --priority 1
  tt t a "Quick task" --tags backend,urgent`,
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
	taskAddCmd.Flags().StringVar(&taskRecurFlag, "recur", "", "recurrence pattern (daily, weekly, monthly, yearly, Nd, Nw, Nm, Ny)")
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

	// Determine task type
	taskType, err := ParseTaskTypeFlags(isEpic, isStory, isBug, isIdea, taskTypeFlag)
	if err != nil {
		return err
	}

	// Validate priority
	if taskPriority < models.PriorityMin || taskPriority > models.PriorityMax {
		return tterrors.NewInvalidValueError("priority", taskPriority,
			fmt.Sprintf("must be %d-%d", models.PriorityMin, models.PriorityMax))
	}

	// Create task
	taskID := uuid.New().String()
	shortID := project.GetNextShortID()
	task := models.NewTask(taskID, shortID, taskType, title)
	task.Priority = taskPriority

	// Add tags
	for _, tag := range taskTags {
		task.AddTag(tag)
	}

	// Handle section (defaults to Backlog from NewTask)
	if taskSectionFlag != "" {
		project.EnsureSection(taskSectionFlag)
		task.Section = taskSectionFlag
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

	// Add to project
	project.Tasks = append(project.Tasks, task)

	// Save with workflow rules evaluation (state-based rules will fire for new tasks)
	if err := SaveProject(project, &SaveOptions{EvaluateRules: true, Config: cfg, ConfigManager: configManager}); err != nil {
		return tterrors.NewSaveProjectError(project.Name, err)
	}

	fmt.Printf("Created %s #%d: %s\n", taskType.HeadingPrefix(), shortID, title)
	return nil
}
