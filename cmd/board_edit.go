package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/external"
	"github.com/sglavoie/tt/internal/storage"
)

var boardEditEditor string

var boardEditCmd = &cobra.Command{
	Use:   "edit <id>",
	Short: "Edit a board item in an external editor",
	Long: `Open a board item in an external editor for editing.

Examples:
  tt board edit 1
  tt board edit 1 --editor vim`,
	Args: cobra.ExactArgs(1),
	RunE: runBoardEdit,
}

func init() {
	boardCmd.AddCommand(boardEditCmd)
	boardEditCmd.Flags().StringVar(&boardEditEditor, "editor", "", "editor to use")
}

func runBoardEdit(cmd *cobra.Command, args []string) error {
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

	editor := resolveEditor(boardEditEditor, cfg.Editor)

	content, err := storage.RenderBoardItemContent(item)
	if err != nil {
		return fmt.Errorf("failed to render board item: %w", err)
	}

	modified, err := external.EditInExternalEditor(editor, content, ".md")
	if err != nil {
		return fmt.Errorf("failed to edit: %w", err)
	}

	if modified == content {
		fmt.Println("No changes made.")
		return nil
	}

	updatedItem, err := storage.ParseBoardItemContent(item.Filename, modified)
	if err != nil {
		return fmt.Errorf("failed to parse edited board item: %w", err)
	}

	if err := storage.SaveBoardItem(cfg.DataDir, updatedItem); err != nil {
		return fmt.Errorf("failed to save board item: %w", err)
	}

	fmt.Println("Board item updated.")
	return nil
}
