package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/template"
)

var templateInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize default task templates",
	Long: `Add a set of default task templates for common workflows.

Default templates are created DISABLED by default. Review them with
'tt template list' and enable the ones you want with 'tt template toggle'.

DEFAULT TEMPLATES

  Bug Report (bug)
    Bug with checklist: Steps to Reproduce, Expected/Actual Behavior, Environment

  Production Incident (bug)
    P1 Bug with checklist: Impact Assessment, Root Cause, Mitigation, Post-mortem

  Feature Request (feature)
    Story with checklist: User Story, Acceptance Criteria, Technical Notes

  API Endpoint (feature)
    Task with checklist: Define Contract, Implement Handler, Add Tests, Update Docs

  UI Component (feature)
    Task with checklist: Design Review, Implementation, Unit Tests, Accessibility

  Code Review Checklist (process)
    Task with checklist: Logic, Tests, Edge Cases, Performance, Docs

  Release Checklist (process)
    Task with checklist: Run Tests, Changelog, Tag, Deploy Staging/Prod

  Sprint Planning (process)
    Epic with checklist: Review Backlog, Estimate, Assign, Define Goals

FLAGS

  --list              Preview default templates without adding them
  --force             Add defaults even if templates already exist
  --enable            Create default templates in enabled state (default: disabled)
  --changed-defaults  Reset edited/deleted default templates to shipped values (requires --force)

EXAMPLES

  Preview default templates:
    tt template init --list

  Add default templates (disabled):
    tt template init

  Add and enable default templates:
    tt template init --enable

  Re-add deleted defaults:
    tt template init --force`,
	RunE: runTemplateInit,
}

var (
	templateInitList            bool
	templateInitForce           bool
	templateInitEnable          bool
	templateInitChangedDefaults bool
)

func init() {
	templateCmd.AddCommand(templateInitCmd)

	templateInitCmd.Flags().BoolVar(&templateInitList, "list", false, "preview default templates without adding them")
	templateInitCmd.Flags().BoolVarP(&templateInitForce, "force", "f", false, "add defaults even if templates already exist")
	templateInitCmd.Flags().BoolVar(&templateInitEnable, "enable", false, "create default templates in enabled state")
	templateInitCmd.Flags().BoolVar(&templateInitChangedDefaults, "changed-defaults", false, "reset edited/deleted default templates to shipped values (requires --force)")
	addOutputFlags(templateInitCmd)
}

func runTemplateInit(cmd *cobra.Command, args []string) error {
	out, err := getOutputOpts(cmd)
	if err != nil {
		return err
	}

	defaults := template.GetDefaultTemplates()

	if templateInitList {
		if out.JSON {
			type templateEntry struct {
				ID       string `json:"id"`
				Name     string `json:"name"`
				Category string `json:"category"`
				Summary  string `json:"summary"`
			}
			entries := make([]templateEntry, 0, len(defaults))
			for _, t := range defaults {
				entries = append(entries, templateEntry{ID: t.ID, Name: t.Name, Category: t.Category, Summary: t.Summary()})
			}
			data, err := json.MarshalIndent(entries, "", "  ")
			if err != nil {
				return fmt.Errorf("marshal JSON: %w", err)
			}
			fmt.Println(string(data))
			return nil
		}
		fmt.Println("Default task templates:")
		fmt.Println()
		for _, t := range defaults {
			fmt.Printf("  [%s] %s (%s)\n", t.ID, t.Name, t.Category)
			fmt.Printf("      %s\n", t.Summary())
			if len(t.Children) > 0 {
				fmt.Printf("      Items: ")
				for i, child := range t.Children {
					if i > 0 {
						fmt.Printf(", ")
					}
					fmt.Printf("%s", child.Title)
					if child.Optional {
						fmt.Printf("?")
					}
				}
				fmt.Println()
			}
			fmt.Println()
		}
		fmt.Println("Run 'tt template init' to add these templates (disabled by default).")
		return nil
	}

	cfg, configManager, err := LoadConfigStrict()
	if err != nil {
		return err
	}

	// Handle --changed-defaults flag (requires --force for safety)
	if templateInitChangedDefaults {
		if !templateInitForce {
			return fmt.Errorf("--changed-defaults requires --force for safety")
		}

		updated, readded, unchanged, affectedIDs := cfg.ResetChangedDefaultTemplates(templateInitEnable)

		if err := saveConfig(configManager, cfg); err != nil {
			return err
		}

		if out.Quiet {
			return nil
		}
		if out.JSON {
			data, err := json.MarshalIndent(struct {
				Updated   int      `json:"updated"`
				Readded   int      `json:"readded"`
				Unchanged int      `json:"unchanged"`
				Affected  []string `json:"affected_ids"`
			}{Updated: updated, Readded: readded, Unchanged: unchanged, Affected: affectedIDs}, "", "  ")
			if err != nil {
				return fmt.Errorf("marshal JSON: %w", err)
			}
			fmt.Println(string(data))
			return nil
		}
		fmt.Println("Reset changed default templates:")
		fmt.Printf("  Updated: %d\n", updated)
		fmt.Printf("  Re-added (deleted): %d\n", readded)
		fmt.Printf("  Unchanged: %d\n", unchanged)

		if len(affectedIDs) > 0 {
			fmt.Println()
			fmt.Println("Affected templates:")
			for _, id := range affectedIDs {
				t := cfg.GetTaskTemplate(id)
				if t != nil {
					fmt.Printf("  [%s] %s\n", t.ID, t.Name)
				}
			}
		}

		fmt.Println()
		fmt.Println("Use 'tt template list' to review.")
		return nil
	}

	if len(cfg.TaskTemplates) > 0 && !templateInitForce && !cfg.DefaultTemplatesInit {
		fmt.Printf("You have %d existing task template(s).\n", len(cfg.TaskTemplates))
		fmt.Println()
		fmt.Println("To add defaults alongside existing templates, use: tt template init --force")
		return nil
	}

	added := cfg.AddDefaultTemplates(templateInitEnable)

	if err := saveConfig(configManager, cfg); err != nil {
		return err
	}

	if out.Quiet {
		return nil
	}

	state := "disabled"
	if templateInitEnable {
		state = "enabled"
	}

	if added == 0 {
		if out.JSON {
			data, err := json.MarshalIndent(struct {
				Added int    `json:"added"`
				State string `json:"state"`
			}{Added: 0, State: state}, "", "  ")
			if err != nil {
				return fmt.Errorf("marshal JSON: %w", err)
			}
			fmt.Println(string(data))
			return nil
		}
		fmt.Println("All default templates already exist.")
		return nil
	}

	if out.JSON {
		data, err := json.MarshalIndent(struct {
			Added int    `json:"added"`
			State string `json:"state"`
		}{Added: added, State: state}, "", "  ")
		if err != nil {
			return fmt.Errorf("marshal JSON: %w", err)
		}
		fmt.Println(string(data))
		return nil
	}

	fmt.Printf("Added %d default task template(s) (%s):\n", added, state)
	fmt.Println()
	for _, t := range defaults {
		if cfg.HasTemplateWithID(t.ID) {
			fmt.Printf("  [%s] %s\n", t.ID, t.Name)
		}
	}
	fmt.Println()
	fmt.Println("Use 'tt template list' to view templates.")
	if !templateInitEnable {
		fmt.Println("Use 'tt template toggle <id>' to enable a template.")
	}

	return nil
}
