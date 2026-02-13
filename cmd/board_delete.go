package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

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

	if !ConfirmYN(fmt.Sprintf("Delete board item #%d '%s'? [y/N] ", item.ShortID, item.Title), boardDeleteForce) {
		fmt.Println("Canceled.")
		return nil
	}

	if err := storage.DeleteBoardItem(cfg.DataDir, item); err != nil {
		return fmt.Errorf("failed to delete board item: %w", err)
	}

	fmt.Printf("Deleted board item #%d: %s\n", item.ShortID, item.Title)
	return nil
}
