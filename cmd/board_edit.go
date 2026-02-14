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
	Short: "Edit an action in an external editor",
	Long: `Open an action's project tasks file in an external editor.

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

	task, project := FindActionTaskByID(cfg, args[0])
	if task == nil {
		return fmt.Errorf("action not found: %s", args[0])
	}

	editor := resolveEditor(boardEditEditor, cfg.Editor)
	tasksFile := storage.GetTasksFilePath(project)

	c := external.EditorCommand(external.ResolveEditor(editor), tasksFile)
	if err := c.Run(); err != nil {
		return fmt.Errorf("editor failed: %w", err)
	}

	fmt.Println("Action updated.")
	return nil
}
