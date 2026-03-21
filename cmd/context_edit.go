package cmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/external"
	"github.com/sglavoie/tt/internal/tterrors"
)

var contextEditCmd = &cobra.Command{
	Use:   "edit <name>",
	Short: "Edit a custom context's project list",
	Long: `Edit the projects in a custom context by re-selecting from available projects.

Uses fzf for multi-select if available (with current projects pre-selected),
otherwise prompts for project slugs.

Examples:
  tt context edit Work
  tt ctx edit "Side Projects"`,
	RunE: runContextEdit,
}

func init() {
	contextCmd.AddCommand(contextEditCmd)
	addOutputFlags(contextEditCmd)
}

func runContextEdit(cmd *cobra.Command, args []string) error {
	if len(args) < 1 {
		return tterrors.NewRequiredFieldError("context name")
	}

	name := args[0]

	cfg, mgr, err := LoadConfig()
	if err != nil {
		return err
	}

	out, err := getOutputOpts(cmd)
	if err != nil {
		return err
	}

	// Check if custom context exists
	currentIDs := cfg.GetCustomContext(name)
	if currentIDs == nil {
		return fmt.Errorf("custom context '%s' not found", name)
	}

	projects := cfg.GetActiveProjects()
	if len(projects) == 0 {
		return fmt.Errorf("no active projects available")
	}

	// Build set of currently-selected project IDs for pre-selection
	currentSet := make(map[string]bool, len(currentIDs))
	for _, id := range currentIDs {
		currentSet[id] = true
	}

	var selectedIDs []string

	if cfg.FzfEnabled && external.FzfAvailable() {
		selected, err := external.FzfMultiSelectProjects(projects, "Edit projects for "+name, currentSet)
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
		// Fallback: prompt for project slugs
		fmt.Println("Available projects:")
		for _, p := range projects {
			marker := "  "
			if currentSet[p.ID] {
				marker = "* "
			}
			fmt.Printf("  %s%s (%s)\n", marker, p.Name, p.Slug)
		}
		fmt.Println()
		fmt.Println("Projects marked with * are currently in this context.")
		fmt.Print("Enter project slugs (comma-separated): ")
		reader := bufio.NewReader(os.Stdin)
		input, err := reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("failed to read input: %w", err)
		}
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
				fmt.Fprintf(os.Stderr, "warning: project '%s' not found, skipping\n", slug)
			}
		}
	}

	if len(selectedIDs) == 0 {
		return fmt.Errorf("no valid projects selected")
	}

	cfg.SetCustomContext(name, selectedIDs)
	if err := saveConfig(mgr, cfg); err != nil {
		return err
	}

	if out.Quiet {
		return nil
	}
	if out.JSON {
		data, _ := json.MarshalIndent(struct {
			Context  string `json:"context"`
			Projects int    `json:"projects"`
		}{Context: name, Projects: len(selectedIDs)}, "", "  ")
		fmt.Println(string(data))
		return nil
	}
	fmt.Printf("Updated custom context '%s' with %d project(s)\n", name, len(selectedIDs))
	return nil
}
