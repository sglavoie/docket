package cmd

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/external"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/tterrors"
)

var boardShowJSON bool

var boardShowCmd = &cobra.Command{
	Use:   "show <id>",
	Short: "Show action details",
	Long: `Show full details for a board action.

Use the short ID from 'tt board list' to identify the action.
If glow is available, the description is rendered as markdown.

Examples:
  tt board show 1
  tt board show 42 --json`,
	RunE: runBoardShow,
}

func init() {
	boardCmd.AddCommand(boardShowCmd)
	boardShowCmd.Flags().BoolVar(&boardShowJSON, "json", false, "output as JSON")
}

func runBoardShow(cmd *cobra.Command, args []string) error {
	if len(args) < 1 {
		return tterrors.NewRequiredFieldError("action ID")
	}

	cfg, _, err := LoadConfig()
	if err != nil {
		return err
	}

	task, project, err := FindActionTaskByID(cfg, args[0])
	if err != nil {
		return err
	}
	if task == nil {
		return fmt.Errorf("action not found: %s", args[0])
	}

	if boardShowJSON {
		type actionJSON struct {
			ID          string     `json:"id"`
			ShortID     int        `json:"short_id"`
			Type        string     `json:"type"`
			Title       string     `json:"title"`
			Description string     `json:"description"`
			Status      string     `json:"status"`
			Priority    int        `json:"priority"`
			Source      string     `json:"source,omitempty"`
			URL         string     `json:"url,omitempty"`
			Tags        []string   `json:"tags"`
			Project     string     `json:"project,omitempty"`
			CreatedAt   time.Time  `json:"created_at"`
			UpdatedAt   time.Time  `json:"updated_at"`
			DueDate     *time.Time `json:"due_date"`
			HasTime     bool       `json:"has_time"`
		}
		tags := task.Tags
		if tags == nil {
			tags = []string{}
		}
		slug := ""
		if !project.Internal {
			slug = project.Slug
		}
		entry := actionJSON{
			ID:          task.ID,
			ShortID:     task.ShortID,
			Type:        task.Type.String(),
			Title:       task.Title,
			Description: task.Description,
			Status:      task.Status.String(),
			Priority:    task.Priority,
			Source:      task.Source,
			URL:         task.URL,
			Tags:        tags,
			Project:     slug,
			CreatedAt:   task.CreatedAt,
			UpdatedAt:   task.UpdatedAt,
			DueDate:     task.DueDate,
			HasTime:     task.HasTime,
		}
		return printJSON(entry)
	}

	return printActionDetails(task, project)
}

func printActionDetails(task *models.Task, project *models.Project) error {
	fmt.Printf("Action:   %s\n", task.Title)
	fmt.Printf("ID:       #%d (%s)\n", task.ShortID, shortDisplayID(task.ID))
	fmt.Printf("Status:   %s %s\n", task.Status.Icon(), task.Status)
	fmt.Printf("Priority: P%d\n", task.Priority)

	if !project.Internal {
		fmt.Printf("Project:  %s\n", project.Name)
	}

	if task.Source != "" {
		fmt.Printf("Source:   %s\n", task.Source)
	}
	if task.URL != "" {
		fmt.Printf("URL:      %s\n", task.URL)
	}
	if len(task.Tags) > 0 {
		fmt.Printf("Tags:     %s\n", strings.Join(task.Tags, ", "))
	}
	if task.DueDate != nil {
		fmt.Printf("Due:      %s\n", formatTaskDueDate(task))
	}

	fmt.Printf("Created:  %s\n", storage.FormatRelativeTime(task.CreatedAt))
	fmt.Printf("Updated:  %s\n", storage.FormatRelativeTime(task.UpdatedAt))

	if task.Description != "" {
		fmt.Println()
		if err := external.RenderMarkdown(task.Description); err != nil {
			fmt.Println(task.Description)
		}
	}

	return nil
}
