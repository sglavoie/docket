package cmd

import (
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/sglavoie/tt/internal/models"
	"github.com/spf13/cobra"
)

var (
	taskTypeFlag    string
	taskParentFlag  string
	taskPriority    int
	taskTags        []string
	taskSectionFlag string
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
	taskAddCmd.Flags().StringVarP(&taskSectionFlag, "section", "s", "", "section for the task (default: Backlog)")
}

func runTaskAdd(cmd *cobra.Command, args []string) error {
	project, _, _, err := getCurrentProject()
	if err != nil {
		return err
	}

	title := strings.Join(args, " ")
	if title == "" {
		return fmt.Errorf("task title is required")
	}

	// Determine task type
	taskType := models.TaskTypeTask
	if isEpic {
		taskType = models.TaskTypeEpic
	} else if isStory {
		taskType = models.TaskTypeStory
	} else if isBug {
		taskType = models.TaskTypeBug
	} else if isIdea {
		taskType = models.TaskTypeIdea
	} else if taskTypeFlag != "" {
		t, ok := models.ParseTaskType(taskTypeFlag)
		if !ok {
			return fmt.Errorf("invalid task type: %s", taskTypeFlag)
		}
		taskType = t
	}

	// Validate priority
	if taskPriority < 1 || taskPriority > 5 {
		return fmt.Errorf("priority must be between 1 and 5")
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

	// Handle parent
	if taskParentFlag != "" {
		parent := findTask(project, taskParentFlag)
		if parent == nil {
			return fmt.Errorf("parent task '%s' not found", taskParentFlag)
		}

		if !task.Type.CanBeChildOf(parent.Type) {
			return fmt.Errorf("%s cannot be a child of %s", task.Type.HeadingPrefix(), parent.Type.HeadingPrefix())
		}

		task.ParentID = &parent.ID
	}

	// Add to project
	project.Tasks = append(project.Tasks, task)

	// Save
	if err := saveProject(project); err != nil {
		return fmt.Errorf("failed to save task: %w", err)
	}

	fmt.Printf("Created %s #%d: %s\n", taskType.HeadingPrefix(), shortID, title)
	return nil
}

// findTask finds a task by ID, short ID, or title prefix.
func findTask(project *models.Project, identifier string) *models.Task {
	// Try as short ID first
	var shortID int
	if _, err := fmt.Sscanf(identifier, "%d", &shortID); err == nil {
		if task := project.GetTaskByShortID(shortID); task != nil {
			return task
		}
	}

	// Try as UUID
	if task := project.GetTaskByID(identifier); task != nil {
		return task
	}

	// Try as title prefix
	identifier = strings.ToLower(identifier)
	for _, task := range project.Tasks {
		if strings.HasPrefix(strings.ToLower(task.Title), identifier) {
			return task
		}
	}

	return nil
}
