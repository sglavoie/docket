package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/external"
	"github.com/sglavoie/tt/internal/storage"
)

var scratchpadEditCmd = &cobra.Command{
	Use:   "edit",
	Short: "Edit the project scratchpad",
	Long: `Edit the project scratchpad in an external editor.

The editor can be configured via:
- --editor flag
- 'editor' field in ~/.tt.json
- EDITOR environment variable
- Falls back to 'code' (VS Code)

Examples:
  tt scratchpad edit
  tt scratchpad edit --editor vim`,
	RunE: runScratchpadEdit,
}

var scratchpadEditEditor string

func init() {
	scratchpadCmd.AddCommand(scratchpadEditCmd)
	scratchpadEditCmd.Flags().StringVar(&scratchpadEditEditor, "editor", "", "editor to use")
}

func runScratchpadEdit(cmd *cobra.Command, args []string) error {
	project, cfg, _, err := LoadCurrentProject()
	if err != nil {
		return err
	}

	// Load existing content
	content, err := storage.LoadScratchpad(project)
	if err != nil {
		return fmt.Errorf("failed to load scratchpad: %w", err)
	}

	// If empty, provide a template
	if content == "" {
		content = storage.DefaultScratchpadTemplate
	}

	// Determine editor
	editor := scratchpadEditEditor
	if editor == "" {
		editor = cfg.Editor
	}

	// Edit in external editor
	modified, err := external.EditInExternalEditor(editor, content, ".md")
	if err != nil {
		return fmt.Errorf("failed to edit: %w", err)
	}

	// Check if changed
	if modified == content {
		fmt.Println("No changes made.")
		return nil
	}

	// Save
	if err := storage.SaveScratchpad(project, modified); err != nil {
		return fmt.Errorf("failed to save scratchpad: %w", err)
	}

	fmt.Println("Scratchpad updated.")
	return nil
}
