package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/tterrors"
	"github.com/sglavoie/tt/internal/workflow"
)

var (
	evalAll     bool
	evalForce   bool
	evalDryRun  bool
	evalQuiet   bool
	evalProject string
)

var workflowEvalCmd = &cobra.Command{
	Use:   "eval",
	Short: "Evaluate workflow rules for tasks",
	Long: `Evaluate workflow rules against tasks and apply matching actions.

This command is useful for time-based rules (due_days, updated_days, created_days)
that need periodic evaluation. Rules are normally only evaluated when tasks change,
so this command allows running them on a schedule or manually.

By default, evaluates only the current project. Use --all to evaluate all projects.
Evaluation is skipped if recently run (configurable interval). Use --force to override.

EXAMPLES

  Evaluate current project:
    tt workflow eval

  Evaluate all projects:
    tt workflow eval --all

  Preview changes without saving (dry-run):
    tt workflow eval --dry-run

  Force evaluation even if recently run:
    tt workflow eval --force

  Silent mode for cron jobs:
    tt workflow eval --all --quiet

CRON EXAMPLE

  # Evaluate all projects daily at 7am
  0 7 * * * /usr/local/bin/tt workflow eval --all --quiet`,
	RunE: runWorkflowEval,
}

func init() {
	workflowCmd.AddCommand(workflowEvalCmd)

	workflowEvalCmd.Flags().BoolVar(&evalAll, "all", false, "Evaluate all projects")
	workflowEvalCmd.Flags().BoolVar(&evalForce, "force", false, "Force evaluation even if recently run")
	workflowEvalCmd.Flags().BoolVar(&evalDryRun, "dry-run", false, "Preview changes without saving")
	workflowEvalCmd.Flags().BoolVar(&evalQuiet, "quiet", false, "Suppress output (for cron jobs)")
	workflowEvalCmd.Flags().StringVar(&evalProject, "project", "", "Specific project to evaluate (name or ID)")
}

func runWorkflowEval(cmd *cobra.Command, args []string) error {
	cfg, configManager, err := LoadConfig()
	if err != nil {
		return err
	}

	rules := cfg.GetEnabledWorkflowRules()
	if len(rules) == 0 {
		if !evalQuiet {
			fmt.Println("No enabled workflow rules. Use 'tt workflow init' or 'tt workflow toggle' to enable rules.")
		}
		return nil
	}

	var projects []*models.Project

	if evalProject != "" {
		// Specific project
		p := cfg.GetProject(evalProject)
		if p == nil {
			return tterrors.NewProjectNotFoundError(evalProject)
		}
		projects = append(projects, p)
	} else if evalAll {
		// All projects
		for _, p := range cfg.Projects {
			if !p.Archived {
				projects = append(projects, p)
			}
		}
	} else {
		// Current project
		p := cfg.GetCurrentProject()
		if p == nil {
			return fmt.Errorf("%w. Use --all or --project, or select a project with: tt select", tterrors.ErrNoCurrentProject)
		}
		projects = append(projects, p)
	}

	if len(projects) == 0 {
		if !evalQuiet {
			fmt.Println("No projects to evaluate.")
		}
		return nil
	}

	type evalResultItem struct {
		project           *models.Project
		modified          int
		rulesApplied      map[string]int
		skipped           bool
		projectStatusSet  bool
		projectStatusIcon string
		projectRuleName   string
		err               error
	}

	results := make([]evalResultItem, len(projects))
	totalModified := 0
	totalSkipped := 0
	totalErrors := 0

	for i, p := range projects {
		results[i].project = p

		// Load project tasks
		if err := storage.LoadProject(p); err != nil {
			results[i].err = fmt.Errorf("failed to load: %w", err)
			totalErrors++
			continue
		}

		if evalDryRun {
			// Dry-run mode: evaluate but don't save
			// Skip CheckAndAutoUnblock to avoid mutating state
			evalResult := workflow.EvaluateRules(p.Tasks, rules)
			results[i].modified = len(evalResult.ModifiedTasks)
			results[i].rulesApplied = evalResult.RuleTriggerCounts

			// Dry-run project-level eval
			if p.StatusIconSource != "user" {
				projCtx := &workflow.ProjectEvaluationContext{
					Project: p,
					Tasks:   p.Tasks,
				}
				projResult := workflow.EvaluateProjectRules(projCtx, rules)
				if projResult.NewStatusIcon != "" {
					results[i].projectStatusSet = true
					results[i].projectStatusIcon = projResult.NewStatusIcon
					results[i].projectRuleName = projResult.AppliedRuleName
				}
			}
		} else {
			// Normal evaluation
			result := EvaluateProjectWorkflowRules(p, cfg, configManager, evalForce)
			results[i].modified = result.ModifiedTasks
			results[i].rulesApplied = result.RulesApplied
			results[i].skipped = result.Skipped
			results[i].projectStatusSet = result.ProjectStatusSet
			results[i].projectStatusIcon = result.ProjectStatusIcon
			results[i].projectRuleName = result.ProjectRuleName
			results[i].err = result.Error
		}

		if results[i].err != nil {
			totalErrors++
		} else if results[i].skipped {
			totalSkipped++
		} else {
			totalModified += results[i].modified
		}
	}

	// Output results
	if !evalQuiet {
		if evalDryRun {
			fmt.Println("Dry-run results (no changes saved):")
		} else {
			fmt.Println("Workflow evaluation results:")
		}
		fmt.Println()

		for _, r := range results {
			name := r.project.Name
			if r.err != nil {
				fmt.Printf("  %s: ERROR - %v\n", name, r.err)
			} else if r.skipped {
				fmt.Printf("  %s: skipped (recently evaluated)\n", name)
			} else if r.modified == 0 && !r.projectStatusSet {
				fmt.Printf("  %s: no changes\n", name)
			} else {
				if r.modified > 0 {
					ruleCount := len(r.rulesApplied)
					fmt.Printf("  %s: %d task(s) modified by %d rule(s)\n", name, r.modified, ruleCount)
				}
				if r.projectStatusSet {
					if icon := models.ProjectStatusIconByKey(r.projectStatusIcon); icon != nil {
						fmt.Printf("  %s: project status -> %s %s (rule: %s)\n", name, icon.Emoji, icon.Label, r.projectRuleName)
					}
				}
			}
		}

		fmt.Println()
		if totalErrors > 0 {
			fmt.Printf("Summary: %d error(s), %d skipped, %d task(s) modified\n", totalErrors, totalSkipped, totalModified)
		} else if totalModified > 0 {
			fmt.Printf("Summary: %d task(s) modified across %d project(s)\n", totalModified, len(projects)-totalSkipped)
		} else if totalSkipped == len(projects) {
			fmt.Println("All projects were recently evaluated. Use --force to re-evaluate.")
		} else {
			fmt.Println("No tasks matched any rules.")
		}
	}

	if totalErrors > 0 {
		return fmt.Errorf("%d project(s) had errors", totalErrors)
	}

	return nil
}
