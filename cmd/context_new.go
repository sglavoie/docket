package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/external"
	"github.com/sglavoie/tt/internal/tterrors"
)

var contextNewCmd = &cobra.Command{
	Use:   "new <name>",
	Short: "Create a new custom context",
	Long: `Create a new custom context by selecting projects to include.

Uses fzf for multi-select if available, otherwise prompts for project names.

Examples:
  tt context new Work
  tt ctx new "Side Projects"`,
	Args: cobra.ExactArgs(1),
	RunE: runContextNew,
}

func init() {
	contextCmd.AddCommand(contextNewCmd)
}

func runContextNew(cmd *cobra.Command, args []string) error {
	name := args[0]
	if name == "" {
		return fmt.Errorf("custom context name cannot be empty")
	}

	cfg, mgr, err := LoadConfig()
	if err != nil {
		return err
	}

	// Check if custom context already exists (case-insensitive)
	if cfg.CustomContextExists(name) {
		return fmt.Errorf("custom context '%s' already exists", name)
	}

	projects := cfg.GetActiveProjects()
	if len(projects) == 0 {
		return fmt.Errorf("no active projects available. Create one first with: tt project new <name>")
	}

	var selectedIDs []string

	// Use fzf if available
	if cfg.FzfEnabled && external.FzfAvailable() {
		selected, err := external.FzfMultiSelectProjects(projects, "Select projects for "+name, nil)
		if err != nil {
			if strings.Contains(err.Error(), "canceled") {
				fmt.Println("Canceled")
				return nil
			}
			return fmt.Errorf("selection failed: %w", err)
		}
		for _, p := range selected {
			selectedIDs = append(selectedIDs, p.ID)
		}
	} else {
		// Fallback: prompt for project names
		fmt.Println("Available projects:")
		for _, p := range projects {
			fmt.Printf("  - %s (%s)\n", p.Name, p.Slug)
		}
		fmt.Println()
		fmt.Print("Enter project slugs (comma-separated): ")
		reader := bufio.NewReader(os.Stdin)
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)
		if input == "" {
			return fmt.Errorf("no projects selected")
		}
		slugs := strings.SplitSeq(input, ",")
		for slug := range slugs {
			slug = strings.TrimSpace(slug)
			if p := cfg.GetProject(slug); p != nil {
				selectedIDs = append(selectedIDs, p.ID)
			} else {
				fmt.Printf("Warning: project '%s' not found, skipping\n", slug)
			}
		}
	}

	if len(selectedIDs) == 0 {
		return fmt.Errorf("no valid projects selected")
	}

	cfg.SetCustomContext(name, selectedIDs)
	if err := mgr.Save(cfg); err != nil {
		return tterrors.NewSaveConfigError(err)
	}

	fmt.Printf("Created custom context '%s' with %d project(s)\n", name, len(selectedIDs))
	return nil
}
