package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/external"
	"github.com/sglavoie/tt/internal/storage"
)

var boardEditEditor string

var boardEditCmd = &cobra.Command{
	Use:   "edit [id]",
	Short: "Edit an action in an external editor",
	Long: `Open an action's project tasks file in an external editor.

If no ID is provided, an interactive selector is shown.

Examples:
  tt board edit 1
  tt board edit 1 --editor vim
  tt board edit`,
	RunE: runBoardEdit,
}

func init() {
	boardCmd.AddCommand(boardEditCmd)
	boardEditCmd.Flags().StringVar(&boardEditEditor, "editor", "", "editor to use")
	addOutputFlags(boardEditCmd)
}

func runBoardEdit(cmd *cobra.Command, args []string) error {
	cfg, _, err := LoadConfig()
	if err != nil {
		return err
	}

	out, err := getOutputOpts(cmd)
	if err != nil {
		return err
	}

	task, project, err := SelectActionTask(cfg, args, "Select action to edit:", "No actions found. Use 'tt board add' to create one.")
	if err != nil {
		return err
	}

	editor := resolveEditor(boardEditEditor, cfg.Editor)
	tasksFile := storage.GetTasksFilePath(project)

	resolvedEditor, editorErr := external.ResolveEditor(editor)
	if editorErr != nil {
		return editorErr
	}
	c := external.EditorCommand(resolvedEditor, tasksFile)
	if err := c.Run(); err != nil {
		return fmt.Errorf("editor failed: %w", err)
	}

	return out.OutputTask(project.Name, "Updated", task)
}
