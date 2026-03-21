package cmd

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/util"
)

var projectShowCmd = &cobra.Command{
	Use:   "show [name|id]",
	Short: "Show project details",
	Long: `Display detailed information about a project.

If no project is specified, shows the current project.

Examples:
  tt project show
  tt project show "My Project"
  tt p show my-project`,
	Args: cobra.MaximumNArgs(1),
	RunE: runProjectShow,
}

func init() {
	projectCmd.AddCommand(projectShowCmd)
	addOutputFlags(projectShowCmd)
}

func runProjectShow(cmd *cobra.Command, args []string) error {
	out, err := getOutputOpts(cmd)
	if err != nil {
		return err
	}

	project, _, _, err := LoadConfigAndProjectOrCurrent(args)
	if err != nil {
		return err
	}

	// Load tasks
	if err := storage.LoadProject(project); err != nil {
		return fmt.Errorf("failed to load project: %w", err)
	}

	if out.JSON {
		return displayProjectShowJSON(project)
	}

	if out.Quiet {
		return nil
	}

	// Display project info
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("39"))
	labelStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("244"))

	fmt.Println(titleStyle.Render(project.Name))
	if project.Description != "" {
		fmt.Println(project.Description)
	}
	fmt.Println()

	fmt.Printf("%s %s\n", labelStyle.Render("ID:"), project.ID)
	fmt.Printf("%s %s\n", labelStyle.Render("Slug:"), project.Slug)
	fmt.Printf("%s %s\n", labelStyle.Render("Context:"), project.Context)
	fmt.Printf("%s %s\n", labelStyle.Render("Path:"), project.Path)
	fmt.Printf("%s %s\n", labelStyle.Render("Created:"), project.CreatedAt.Format(util.DateFormat))

	if project.Archived {
		fmt.Printf("%s %s\n", labelStyle.Render("Archived:"), project.ArchivedAt.Format(util.DateFormat))
	}

	fmt.Println()

	// Task statistics
	fmt.Println(titleStyle.Render("Tasks"))

	if len(project.Tasks) == 0 {
		fmt.Println(NoItemsMsg("tasks", "tt task add <title>"))
		return nil
	}

	// By type
	t := table.NewWriter()
	t.SetStyle(table.StyleRounded)
	t.AppendHeader(table.Row{"Type", "Total", "Active", "Done"})

	for _, taskType := range models.AllTaskTypes() {
		tasks := project.GetTasksByType(taskType)
		if len(tasks) == 0 {
			continue
		}

		active := 0
		done := 0
		for _, task := range tasks {
			if task.Status.IsActive() {
				active++
			} else if task.IsComplete() {
				done++
			}
		}

		t.AppendRow(table.Row{
			fmt.Sprintf("%s %s", taskType.Icon(), taskType.HeadingPrefix()),
			len(tasks),
			active,
			done,
		})
	}

	fmt.Println(t.Render())

	return nil
}

func displayProjectShowJSON(project *models.Project) error {
	type typeStats struct {
		Type   string `json:"type"`
		Total  int    `json:"total"`
		Active int    `json:"active"`
		Done   int    `json:"done"`
	}

	type projectShowJSONEntry struct {
		ID          string      `json:"id"`
		Slug        string      `json:"slug"`
		Name        string      `json:"name"`
		Description string      `json:"description"`
		Context     string      `json:"context"`
		Path        string      `json:"path"`
		StatusIcon  string      `json:"status_icon,omitempty"`
		CreatedAt   time.Time   `json:"created_at"`
		UpdatedAt   time.Time   `json:"updated_at"`
		Archived    bool        `json:"archived"`
		ArchivedAt  *time.Time  `json:"archived_at"`
		Pinned      bool        `json:"pinned"`
		Sections    []string    `json:"sections"`
		TaskCount   int         `json:"task_count"`
		ActiveCount int         `json:"active_count"`
		DoneCount   int         `json:"done_count"`
		TasksByType []typeStats `json:"tasks_by_type"`
	}

	sections := project.GetSections()
	if sections == nil {
		sections = []string{}
	}

	var tasksByType []typeStats
	for _, taskType := range models.AllTaskTypes() {
		tasks := project.GetTasksByType(taskType)
		if len(tasks) == 0 {
			continue
		}
		active := 0
		done := 0
		for _, task := range tasks {
			if task.Status.IsActive() {
				active++
			} else if task.IsComplete() {
				done++
			}
		}
		tasksByType = append(tasksByType, typeStats{
			Type:   taskType.String(),
			Total:  len(tasks),
			Active: active,
			Done:   done,
		})
	}
	if tasksByType == nil {
		tasksByType = []typeStats{}
	}

	entry := projectShowJSONEntry{
		ID:          project.ID,
		Slug:        project.Slug,
		Name:        project.Name,
		Description: project.Description,
		Context:     project.Context,
		Path:        project.Path,
		StatusIcon:  project.StatusIcon,
		CreatedAt:   project.CreatedAt,
		UpdatedAt:   project.UpdatedAt,
		Archived:    project.Archived,
		ArchivedAt:  project.ArchivedAt,
		Pinned:      project.Pinned,
		Sections:    sections,
		TaskCount:   project.CountTasks(),
		ActiveCount: project.CountActiveTasks(),
		DoneCount:   project.CountCompletedTasks(),
		TasksByType: tasksByType,
	}

	data, err := json.MarshalIndent(entry, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal project to JSON: %w", err)
	}
	fmt.Println(string(data))
	return nil
}
