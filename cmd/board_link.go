package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/storage"
)

var boardUnlink bool

var boardLinkCmd = &cobra.Command{
	Use:   "link <board-id> [task-id]",
	Short: "Link or unlink a board item to a tt task",
	Long: `Link a board item to a tt task by ID, or unlink with --unlink.

Examples:
  tt board link 1 5
  tt board link 1 abc123
  tt board link --unlink 1`,
	Args: cobra.RangeArgs(1, 2),
	RunE: runBoardLink,
}

func init() {
	boardCmd.AddCommand(boardLinkCmd)
	boardLinkCmd.Flags().BoolVar(&boardUnlink, "unlink", false, "remove the link instead of adding")
}

func runBoardLink(cmd *cobra.Command, args []string) error {
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

	if boardUnlink {
		if item.LinkedTaskID == "" {
			fmt.Printf("Board item #%d has no linked task.\n", item.ShortID)
			return nil
		}
		item.ClearLink()
		fmt.Printf("Unlinked board item #%d\n", item.ShortID)
	} else {
		if len(args) < 2 {
			return fmt.Errorf("task ID is required for linking")
		}

		project, _, _, pErr := LoadCurrentProject()
		if pErr != nil {
			return pErr
		}

		task := FindTaskByIdentifier(project, args[1])
		if task == nil {
			return fmt.Errorf("task not found: %s", args[1])
		}

		item.LinkedTaskID = task.ID
		fmt.Printf("Linked board item #%d to task #%d '%s'\n", item.ShortID, task.ShortID, task.Title)
	}

	if err := storage.SaveBoardItem(cfg.DataDir, item); err != nil {
		return fmt.Errorf("failed to save board item: %w", err)
	}

	return nil
}
