package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

var boardDeleteForce bool

var boardDeleteCmd = &cobra.Command{
	Use:     "delete <id>",
	Aliases: []string{"rm"},
	Short:   "Delete a board item permanently",
	Long: `Delete a board item permanently from the focus board.

Examples:
  tt board delete 1
  tt board delete 1 --force`,
	Args: cobra.ExactArgs(1),
	RunE: runBoardDelete,
}

func init() {
	boardCmd.AddCommand(boardDeleteCmd)
	boardDeleteCmd.Flags().BoolVarP(&boardDeleteForce, "force", "f", false, "skip confirmation")
}

func runBoardDelete(cmd *cobra.Command, args []string) error {
	cfg, _, err := LoadConfig()
	if err != nil {
		return err
	}

	bd, err := storage.LoadBoard(cfg.DataDir)
	if err != nil {
		return fmt.Errorf("failed to load board: %w", err)
	}

	item := storage.FindBoardItem(bd, args[0])
	if item == nil {
		return fmt.Errorf("board item not found: %s", args[0])
	}

	cascade := false
	if item.LinkedTaskID != "" && !boardDeleteForce {
		fmt.Printf("Board item #%d '%s' is linked to a task.\n", item.ShortID, item.Title)
		fmt.Print("Delete both board item and linked task? [y=both / b=board only / n=cancel] ")
		reader := bufio.NewReader(os.Stdin)
		line, _ := reader.ReadString('\n')
		answer := strings.TrimSpace(strings.ToLower(line))
		switch answer {
		case "y", "Y":
			cascade = true
		case "b", "B":
			cascade = false
		default:
			fmt.Println("Canceled.")
			return nil
		}
	} else if !ConfirmYN(fmt.Sprintf("Delete board item #%d '%s'? [y/N] ", item.ShortID, item.Title), boardDeleteForce) {
		fmt.Println("Canceled.")
		return nil
	}

	// Cascade: delete linked task from its project
	if cascade && item.LinkedTaskID != "" && item.ProjectID != "" {
		deleteLinkedTaskCLI(cfg, item)
	}

	if err := storage.DeleteBoardItem(cfg.DataDir, item); err != nil {
		return fmt.Errorf("failed to delete board item: %w", err)
	}

	fmt.Printf("Deleted board item #%d: %s\n", item.ShortID, item.Title)
	return nil
}

// deleteLinkedTaskCLI removes a task linked to a board item from its project.
func deleteLinkedTaskCLI(cfg *config.Config, item *models.BoardItem) {
	for _, project := range cfg.GetActiveProjects() {
		if project.ID != item.ProjectID {
			continue
		}
		if err := storage.LoadProject(project); err != nil {
			fmt.Printf("Warning: could not load project '%s': %v\n", project.Name, err)
			return
		}
		task := project.GetTaskByID(item.LinkedTaskID)
		if task == nil {
			return
		}
		// Collect task and descendant IDs
		idsToRemove := make(map[string]bool)
		var collect func(t *models.Task)
		collect = func(t *models.Task) {
			idsToRemove[t.ID] = true
			for _, child := range t.Children {
				collect(child)
			}
		}
		collect(task)
		// Filter out deleted tasks and clear dangling parents
		var remaining []*models.Task
		for _, t := range project.Tasks {
			if !idsToRemove[t.ID] {
				if t.ParentID != nil && idsToRemove[*t.ParentID] {
					t.ParentID = nil
				}
				remaining = append(remaining, t)
			}
		}
		project.Tasks = remaining
		if err := storage.SaveProject(project); err != nil {
			fmt.Printf("Warning: could not save project '%s': %v\n", project.Name, err)
		} else {
			fmt.Printf("Deleted linked task #%d from project '%s'\n", task.ShortID, project.Name)
		}
		return
	}
}
