package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/storage"
)

var boardDismissCmd = &cobra.Command{
	Use:   "dismiss <id>",
	Short: "Dismiss a board item as irrelevant",
	Long: `Dismiss a board item, marking it as no longer relevant.

Examples:
  tt board dismiss 1
  tt board dismiss 3`,
	Args: cobra.ExactArgs(1),
	RunE: runBoardDismiss,
}

func init() {
	boardCmd.AddCommand(boardDismissCmd)
}

func runBoardDismiss(cmd *cobra.Command, args []string) error {
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

	item.MarkDismissed()

	if err := storage.SaveBoardItem(cfg.DataDir, item); err != nil {
		return fmt.Errorf("failed to save board item: %w", err)
	}

	fmt.Printf("Dismissed board item #%d: %s\n", item.ShortID, item.Title)
	return nil
}
