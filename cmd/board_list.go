package cmd

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

var (
	boardListAll    bool
	boardListSource string
	boardListDone   bool
	boardListJSON   bool
)

var boardListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls", "l"},
	Short:   "List board actions",
	Long: `List action items across all projects. By default shows only open actions.

Examples:
  tt board list
  tt board list --all
  tt board list --source slack
  tt board list --done`,
	RunE: runBoardList,
}

func init() {
	boardCmd.AddCommand(boardListCmd)
	boardListCmd.Flags().BoolVarP(&boardListAll, "all", "a", false, "show all items including done/canceled")
	boardListCmd.Flags().StringVarP(&boardListSource, "source", "s", "", "filter by source")
	boardListCmd.Flags().BoolVarP(&boardListDone, "done", "d", false, "show only done items")
	boardListCmd.Flags().BoolVar(&boardListJSON, "json", false, "output as JSON")
}

func runBoardList(cmd *cobra.Command, args []string) error {
	cfg, _, err := LoadConfig()
	if err != nil {
		return err
	}

	type actionEntry struct {
		task    *models.Task
		project string
	}

	var actions []actionEntry
	for _, project := range cfg.GetAllLoadableProjects() {
		if err := storage.LoadProject(project); err != nil {
			continue
		}
		for _, task := range project.Tasks {
			if task.Type != models.TaskTypeAction {
				continue
			}

			// Filter by status
			if boardListDone {
				if task.Status != models.StatusDone {
					continue
				}
			} else if !boardListAll && task.Status.IsComplete() {
				continue
			}

			// Filter by source
			if boardListSource != "" && task.Source != boardListSource {
				continue
			}

			slug := ""
			if !project.Internal {
				slug = project.Slug
			}
			actions = append(actions, actionEntry{task: task, project: slug})
		}
	}

	if boardListJSON {
		type actionJSONEntry struct {
			ID        string     `json:"id"`
			ShortID   int        `json:"short_id"`
			Title     string     `json:"title"`
			Status    string     `json:"status"`
			Priority  int        `json:"priority"`
			Source    string     `json:"source,omitempty"`
			URL       string     `json:"url,omitempty"`
			Tags      []string   `json:"tags"`
			Project   string     `json:"project,omitempty"`
			CreatedAt time.Time  `json:"created_at"`
			UpdatedAt time.Time  `json:"updated_at"`
			DueDate   *time.Time `json:"due_date"`
		}
		entries := make([]actionJSONEntry, len(actions))
		for i, a := range actions {
			tags := a.task.Tags
			if tags == nil {
				tags = []string{}
			}
			entries[i] = actionJSONEntry{
				ID:        a.task.ID,
				ShortID:   a.task.ShortID,
				Title:     a.task.Title,
				Status:    a.task.Status.String(),
				Priority:  a.task.Priority,
				Source:    a.task.Source,
				URL:       a.task.URL,
				Tags:      tags,
				Project:   a.project,
				CreatedAt: a.task.CreatedAt,
				UpdatedAt: a.task.UpdatedAt,
				DueDate:   a.task.DueDate,
			}
		}
		data, err := json.MarshalIndent(entries, "", "  ")
		if err != nil {
			return fmt.Errorf("marshal actions to JSON: %w", err)
		}
		fmt.Println(string(data))
		return nil
	}

	if len(actions) == 0 {
		if boardListDone {
			fmt.Println("No completed actions.")
		} else if boardListSource != "" {
			fmt.Printf("No actions from source '%s'.\n", boardListSource)
		} else {
			fmt.Println("No open actions. Add one with: tt board add \"Title\"")
		}
		return nil
	}

	for _, a := range actions {
		task := a.task
		statusIcon := " "
		switch task.Status {
		case models.StatusDone:
			statusIcon = "x"
		case models.StatusCanceled:
			statusIcon = "-"
		}

		source := ""
		if task.Source != "" {
			source = fmt.Sprintf("[%s] ", task.Source)
		}

		url := ""
		if task.URL != "" {
			url = " ↗"
		}

		tags := ""
		if len(task.Tags) > 0 {
			tags = " [" + strings.Join(task.Tags, ", ") + "]"
		}

		projectLabel := ""
		if a.project != "" {
			projectLabel = fmt.Sprintf(" (%s)", a.project)
		}

		fmt.Printf("%s P%d #%-3d %s%s%s%s%s\n", statusIcon, task.Priority, task.ShortID, source, task.Title, url, tags, projectLabel)
	}

	return nil
}
