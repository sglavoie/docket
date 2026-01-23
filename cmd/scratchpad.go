package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/external"
	"github.com/sglavoie/tt/internal/storage"
)

var scratchpadCmd = &cobra.Command{
	Use:   "scratchpad",
	Short: "View the project scratchpad",
	Long: `View the project scratchpad (free-form notes and ideas).

The scratchpad is a free-form markdown file for quickly dumping ideas
without creating structured tasks. Uses glow for rendering if available.

Examples:
  tt scratchpad             # View scratchpad
  tt scratchpad edit        # Edit scratchpad in external editor
  tt scratchpad --no-glow   # View without glow rendering`,
	RunE: runScratchpadView,
}

var scratchpadNoGlow bool

func init() {
	rootCmd.AddCommand(scratchpadCmd)
	scratchpadCmd.Flags().BoolVar(&scratchpadNoGlow, "no-glow", false, "disable glow rendering")
}

func runScratchpadView(cmd *cobra.Command, args []string) error {
	project, cfg, _, err := LoadCurrentProject()
	if err != nil {
		return err
	}

	content, err := storage.LoadScratchpad(project)
	if err != nil {
		return fmt.Errorf("failed to load scratchpad: %w", err)
	}

	if content == "" {
		fmt.Println("Scratchpad is empty. Use 'tt scratchpad edit' to add content.")
		return nil
	}

	// Add a header for nicer rendering
	displayContent := fmt.Sprintf("# Scratchpad: %s\n\n%s", project.Name, content)

	// Use glow if available and enabled
	useGlow := cfg.GlowEnabled && external.GlowAvailable() && !scratchpadNoGlow
	if useGlow {
		return external.PageMarkdown(displayContent)
	}

	fmt.Println(displayContent)
	return nil
}
