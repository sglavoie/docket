package cmd

import (
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

var (
	boardAddURL      string
	boardAddSource   string
	boardAddPriority int
	boardAddTags     []string
)

var boardAddCmd = &cobra.Command{
	Use:     "add <title>",
	Aliases: []string{"a", "new"},
	Short:   "Add a new board item",
	Long: `Add a new item to the focus board.

Examples:
  tt board add "Review PR #432" --url https://github.com/org/repo/pull/432 --source github
  tt board add "Reply to Sarah's thread" --source slack --priority 2
  tt board add "Follow up on Q2 roadmap" --source meeting --tag planning`,
	Args: cobra.MinimumNArgs(1),
	RunE: runBoardAdd,
}

func init() {
	boardCmd.AddCommand(boardAddCmd)
	boardAddCmd.Flags().StringVarP(&boardAddURL, "url", "u", "", "URL for the external resource")
	boardAddCmd.Flags().StringVarP(&boardAddSource, "source", "s", "", "source of the item (e.g., slack, github, email)")
	boardAddCmd.Flags().IntVarP(&boardAddPriority, "priority", "p", 0, "priority 1-5 (default 3)")
	boardAddCmd.Flags().StringSliceVarP(&boardAddTags, "tag", "t", nil, "tags for the item")
}

func runBoardAdd(cmd *cobra.Command, args []string) error {
	title := strings.Join(args, " ")
	if title == "" {
		return fmt.Errorf("item title is required")
	}

	cfg, _, err := LoadConfig()
	if err != nil {
		return err
	}

	bd, err := storage.LoadBoard(cfg.DataDir)
	if err != nil {
		return fmt.Errorf("failed to load board: %w", err)
	}

	item := models.NewBoardItem(uuid.New().String()[:8], bd.NextShortID, title)

	if boardAddURL != "" {
		item.URL = boardAddURL
	}
	if boardAddSource != "" {
		item.Source = boardAddSource
	}
	if boardAddPriority >= 1 && boardAddPriority <= 5 {
		item.Priority = boardAddPriority
	}
	if len(boardAddTags) > 0 {
		item.Tags = boardAddTags
	}

	item.Filename = models.GenerateBoardItemFilename(item.ShortID, item.Title)

	if err := storage.SaveBoardItem(cfg.DataDir, item); err != nil {
		return fmt.Errorf("failed to save board item: %w", err)
	}
	if err := storage.SaveBoardIndex(cfg.DataDir, bd.NextShortID+1); err != nil {
		return fmt.Errorf("failed to save board index: %w", err)
	}

	fmt.Printf("Added board item #%d: %s\n", item.ShortID, item.Title)
	return nil
}
