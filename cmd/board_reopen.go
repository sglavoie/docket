package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/storage"
)

var boardReopenCmd = &cobra.Command{
	Use:   "reopen <id>",
	Short: "Reopen a done or dismissed board item",
	Long: `Reopen a board item that was previously marked done or dismissed.

Examples:
  tt board reopen 1
  tt board reopen 3`,
	Args: cobra.ExactArgs(1),
	RunE: runBoardReopen,
}

func init() {
	boardCmd.AddCommand(boardReopenCmd)
}

func runBoardReopen(cmd *cobra.Command, args []string) error {
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

	if item.IsOpen() {
		fmt.Printf("Board item #%d is already open.\n", item.ShortID)
		return nil
	}

	item.Reopen()

	if err := storage.SaveBoardItem(cfg.DataDir, item); err != nil {
		return fmt.Errorf("failed to save board item: %w", err)
	}

	fmt.Printf("Reopened board item #%d: %s\n", item.ShortID, item.Title)
	return nil
}
