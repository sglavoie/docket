package cmd

import (
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
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
	Args: cobra.MinimumNArgs(1),
	RunE: runBoardAdd,
}

func init() {
	boardCmd.AddCommand(boardAddCmd)
	boardAddCmd.Flags().StringVarP(&boardAddURL, "url", "u", "", "URL for the external resource")
	boardAddCmd.Flags().StringVarP(&boardAddSource, "source", "s", "", "source of the item (e.g., slack, github, email)")
	boardAddCmd.Flags().IntVarP(&boardAddPriority, "priority", "p", 0, "priority 1-5 (default 3)")
	boardAddCmd.Flags().StringSliceVarP(&boardAddTags, "tag", "t", nil, "tags for the item")
	boardAddCmd.Flags().StringVar(&boardAddProject, "project", "", "target project slug (default: action inbox)")
}

func runBoardAdd(cmd *cobra.Command, args []string) error {
	title := strings.Join(args, " ")
	if title == "" {
		return fmt.Errorf("action title is required")
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
			return fmt.Errorf("project not found: %s", boardAddProject)
		}
	} else {
		project, created := cfg.EnsureActionInboxProject()
		targetProject = project
		if created {
			if err := storage.CreateProjectDir(targetProject); err != nil {
				return fmt.Errorf("failed to create action inbox: %w", err)
			}
			if err := storage.SaveProject(targetProject); err != nil {
				return fmt.Errorf("failed to save action inbox: %w", err)
			}
			_ = mgr.Save(cfg)
		}
	}

	if err := storage.LoadProject(targetProject); err != nil {
		return fmt.Errorf("failed to load project: %w", err)
	}

	task := models.NewTask(uuid.New().String(), targetProject.GetNextShortID(), models.TaskTypeAction, title)
	if boardAddURL != "" {
		task.URL = boardAddURL
	}
	if boardAddSource != "" {
		task.Source = boardAddSource
	}
	if boardAddPriority >= 1 && boardAddPriority <= 5 {
		task.Priority = boardAddPriority
	}
	if len(boardAddTags) > 0 {
		task.Tags = boardAddTags
	}

	targetProject.Tasks = append(targetProject.Tasks, task)
	targetProject.BuildTaskTree()

	if err := storage.SaveProject(targetProject); err != nil {
		return fmt.Errorf("failed to save: %w", err)
	}

	fmt.Printf("Added action #%d: %s\n", task.ShortID, task.Title)
	return nil
}
