package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/storage"
)

var boardDoneCmd = &cobra.Command{
	Use:   "done <id>",
	Short: "Mark a board item as done",
	Long: `Mark a board item as done.

Examples:
  tt board done 1
  tt board done 3`,
	Args: cobra.ExactArgs(1),
	RunE: runBoardDone,
}

func init() {
	boardCmd.AddCommand(boardDoneCmd)
}

func runBoardDone(cmd *cobra.Command, args []string) error {
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

	item.MarkDone()

	if err := storage.SaveBoardItem(cfg.DataDir, item); err != nil {
		return fmt.Errorf("failed to save board item: %w", err)
	}

	fmt.Printf("Marked board item #%d as done: %s\n", item.ShortID, item.Title)
	return nil
}
