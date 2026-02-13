package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

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
	Short:   "List board items",
	Long: `List board items. By default shows only open items.

Examples:
  tt board list
  tt board list --all
  tt board list --source slack
  tt board list --done`,
	RunE: runBoardList,
}

func init() {
	boardCmd.AddCommand(boardListCmd)
	boardListCmd.Flags().BoolVarP(&boardListAll, "all", "a", false, "show all items including done/dismissed")
	boardListCmd.Flags().StringVarP(&boardListSource, "source", "s", "", "filter by source")
	boardListCmd.Flags().BoolVarP(&boardListDone, "done", "d", false, "show only done items")
}

func runBoardList(cmd *cobra.Command, args []string) error {
	cfg, _, err := LoadConfig()
	if err != nil {
		return err
	}

	bd, err := storage.LoadBoard(cfg.DataDir)
	if err != nil {
		return fmt.Errorf("failed to load board: %w", err)
	}

	if len(bd.Items) == 0 {
		fmt.Println("No board items. Add one with: tt board add \"Title\"")
		return nil
	}

	var shown int
	for _, item := range bd.Items {
		// Filter by status
		if boardListDone {
			if item.Status != "done" {
				continue
			}
		} else if !boardListAll && item.IsClosed() {
			continue
		}

		// Filter by source
		if boardListSource != "" && item.Source != boardListSource {
			continue
		}

		statusIcon := " "
		switch item.Status {
		case "done":
			statusIcon = "x"
		case "dismissed":
			statusIcon = "-"
		}

		source := ""
		if item.Source != "" {
			source = fmt.Sprintf("[%s] ", item.Source)
		}

		url := ""
		if item.URL != "" {
			url = " ↗"
		}

		tags := ""
		if len(item.Tags) > 0 {
			tags = " [" + strings.Join(item.Tags, ", ") + "]"
		}

		fmt.Printf("%s P%d #%-3d %s%s%s%s\n", statusIcon, item.Priority, item.ShortID, source, item.Title, url, tags)
		shown++
	}

	if shown == 0 {
		if boardListDone {
			fmt.Println("No completed board items.")
		} else if boardListSource != "" {
			fmt.Printf("No board items from source '%s'.\n", boardListSource)
		} else {
			fmt.Println("No open board items.")
		}
	}

	return nil
}
