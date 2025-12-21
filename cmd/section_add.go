package cmd

import (
	"fmt"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/spf13/cobra"
)

var sectionAddCmd = &cobra.Command{
	Use:   "add <name>",
	Short: "Add a new section to the current project",
	Long: `Add a new section to organize tasks.

Examples:
  tt section add "In Progress"
  tt section add Review
  tt sec add Done`,
	Args: cobra.ExactArgs(1),
	RunE: runSectionAdd,
}

func init() {
	sectionCmd.AddCommand(sectionAddCmd)
}

func runSectionAdd(cmd *cobra.Command, args []string) error {
	sectionName := args[0]
	if sectionName == "" {
		return fmt.Errorf("section name cannot be empty")
	}

	configManager := config.NewManager(GetConfigPath())
	cfg, err := configManager.LoadOrCreate()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	project := cfg.GetCurrentProject()
	if project == nil {
		return fmt.Errorf("no project selected. Use 'tt select' to choose a project")
	}

	if err := storage.LoadProject(project); err != nil {
		return fmt.Errorf("failed to load project: %w", err)
	}

	if project.HasSection(sectionName) {
		return fmt.Errorf("section '%s' already exists", sectionName)
	}

	project.EnsureSection(sectionName)

	if err := storage.SaveProject(project); err != nil {
		return fmt.Errorf("failed to save project: %w", err)
	}

	fmt.Printf("Added section: %s\n", sectionName)
	return nil
}
