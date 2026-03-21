package cmd

import (
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/tterrors"
)

var (
	boardAddURL      string
	boardAddSource   string
	boardAddPriority int
	boardAddTags     []string
	boardAddProject  string
)

var boardAddCmd = &cobra.Command{
	Use:     "add <title>",
	Aliases: []string{"a", "new"},
	Short:   "Add a new action to the focus board",
	Long: `Add a new action item to the focus board.

Actions are stored as Action-type tasks. By default they go to the
action inbox project; use --project to target a specific project.

Examples:
  tt board add "Review PR #432" --url https://github.com/org/repo/pull/432 --source github
  tt board add "Reply to Sarah's thread" --source slack --priority 2
  tt board add "Follow up on Q2 roadmap" --project myproject --tag planning`,
	RunE: runBoardAdd,
}

func init() {
	boardCmd.AddCommand(boardAddCmd)
	boardAddCmd.Flags().StringVarP(&boardAddURL, "url", "u", "", "URL for the external resource")
	boardAddCmd.Flags().StringVarP(&boardAddSource, "source", "s", "", "source of the item (e.g., slack, github, email)")
	boardAddCmd.Flags().IntVarP(&boardAddPriority, "priority", "P", 0, "priority 1-5 (default 3)")
	boardAddCmd.Flags().StringSliceVarP(&boardAddTags, "tag", "t", nil, "tags for the item")
	boardAddCmd.Flags().StringVar(&boardAddProject, "project", "", "target project slug (default: action inbox)")
}

func runBoardAdd(cmd *cobra.Command, args []string) error {
	title := strings.TrimSpace(strings.Join(args, " "))
	if title == "" {
		return tterrors.NewRequiredFieldError("action title")
	}
	if err := models.ValidateTaskTitle(title); err != nil {
		return err
	}

	cfg, mgr, err := LoadConfig()
	if err != nil {
		return err
	}

	// Determine target project
	var targetProject *models.Project
	if boardAddProject != "" {
		targetProject = cfg.GetProject(boardAddProject)
		if targetProject == nil {
			return tterrors.NewProjectNotFoundError(boardAddProject)
		}
	} else {
		project, created := cfg.EnsureActionInboxProject()
		targetProject = project
		if created {
			if err = storage.CreateProjectDir(targetProject); err != nil {
				return fmt.Errorf("failed to create action inbox: %w", err)
			}
			if err = storage.SaveProject(targetProject); err != nil {
				return fmt.Errorf("failed to save action inbox: %w", err)
			}
			if err = saveConfig(mgr, cfg); err != nil {
				return err
			}
		}
	}

	if err = storage.LoadProject(targetProject); err != nil {
		return fmt.Errorf("failed to load project: %w", err)
	}

	task, err := models.NewTask(uuid.New().String(), targetProject.GetNextShortID(), models.TaskTypeTask, title)
	if err != nil {
		return err
	}
	if boardAddURL != "" {
		task.URL = boardAddURL
	}
	if boardAddSource != "" {
		task.Source = boardAddSource
	}
	if cmd.Flags().Changed("priority") {
		if boardAddPriority < models.PriorityMin || boardAddPriority > models.PriorityMax {
			return fmt.Errorf("priority must be between %d and %d", models.PriorityMin, models.PriorityMax)
		}
		task.Priority = boardAddPriority
	}
	for _, tag := range boardAddTags {
		task.AddTag(tag)
	}

	targetProject.Tasks = append(targetProject.Tasks, task)
	targetProject.BuildTaskTree()

	if err := SaveProject(targetProject, &SaveOptions{Config: cfg, ConfigManager: mgr}); err != nil {
		return fmt.Errorf("failed to save: %w", err)
	}

	fmt.Println(TaskMsg(targetProject.Name, "Added", task))
	return nil
}
