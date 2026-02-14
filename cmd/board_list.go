package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

var (
	boardListAll    bool
	boardListSource string
	boardListDone   bool
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
}

func runBoardList(cmd *cobra.Command, args []string) error {
	cfg, _, err := LoadConfig()
	if err != nil {
		return err
	}

	var shown int
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
			if !project.Internal {
				projectLabel = fmt.Sprintf(" (%s)", project.Slug)
			}

			fmt.Printf("%s P%d #%-3d %s%s%s%s%s\n", statusIcon, task.Priority, task.ShortID, source, task.Title, url, tags, projectLabel)
			shown++
		}
	}

	if shown == 0 {
		if boardListDone {
			fmt.Println("No completed actions.")
		} else if boardListSource != "" {
			fmt.Printf("No actions from source '%s'.\n", boardListSource)
		} else {
			fmt.Println("No open actions. Add one with: tt board add \"Title\"")
		}
	}

	return nil
}
