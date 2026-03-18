package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/logging"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/util"
	"github.com/sglavoie/tt/internal/workflow"
)

// StartupEvalResult holds the result of workflow evaluation at TUI startup.
type StartupEvalResult struct {
	Evaluated     bool
	ModifiedTasks int
	RulesApplied  int
}

// RunApp starts the TUI application.
func RunApp(cfg *config.Config, manager *config.Manager, opts ...AppOption) error {
	lockPath := filepath.Join(cfg.DataDir, "pid.lock")
	if err := util.AcquirePIDLock(lockPath); err != nil {
		return err
	}
	defer util.ReleasePIDLock(lockPath)

	// Evaluate workflow rules at startup if needed
	var startupResult *StartupEvalResult
	if current := cfg.GetCurrentProject(); current != nil {
		startupResult = evaluateWorkflowAtStartup(cfg, manager, current)
	}

	// Apply config-driven storage settings
	if cfg.MaxActivityEntries != nil {
		storage.MaxActivityEntries = *cfg.MaxActivityEntries
	}

	model := NewAppModel(cfg, manager, opts...)

	// Evaluate proactive alerts at startup
	model.reevaluateAlerts()

	// Pass startup eval result to model for status display
	if startupResult != nil && startupResult.Evaluated && startupResult.ModifiedTasks > 0 {
		model.statusMessage = fmt.Sprintf("%d task(s) updated by %d workflow rule(s)",
			startupResult.ModifiedTasks, startupResult.RulesApplied)
	}

	// Show data repair notification for current project
	if model.statusMessage == "" {
		if current := cfg.GetCurrentProject(); current != nil {
			// Ensure project is loaded (may have been loaded by workflow eval already)
			if len(current.Tasks) == 0 && current.Path != "" {
				_ = storage.LoadProject(current)
			}
			if n := len(current.Warnings); n > 0 {
				model.statusMessage = fmt.Sprintf("%d data issue(s) auto-repaired (run tt doctor for details)", n)
				model.statusIsInfo = true
			}
		}
	}

	p := tea.NewProgram(model, tea.WithAltScreen())

	// Recover from panics to restore terminal state before crashing.
	defer func() {
		if r := recover(); r != nil {
			p.Kill()
			_ = p.ReleaseTerminal()
			fmt.Fprintf(os.Stderr, "\ntt: unexpected panic: %v\n", r)
			fmt.Fprintf(os.Stderr, "Please report this at https://github.com/sglavoie/tt/issues\n")
			os.Exit(1)
		}
	}()

	_, err := p.Run()
	return err
}

// evaluateWorkflowAtStartup evaluates workflow rules for the current project if needed.
func evaluateWorkflowAtStartup(cfg *config.Config, manager *config.Manager, project *models.Project) *StartupEvalResult {
	result := &StartupEvalResult{}

	// Check if evaluation is needed
	if !cfg.ShouldEvaluateWorkflow(project.ID) {
		return result
	}

	rules := cfg.GetEnabledWorkflowRules()
	if len(rules) == 0 {
		return result
	}

	// Load project tasks
	if err := storage.LoadProject(project); err != nil {
		return result
	}

	// Run auto-unblock
	project.CheckAndAutoUnblock()

	// Evaluate rules
	evalResult := workflow.EvaluateRules(project.Tasks, rules)

	result.Evaluated = true
	result.ModifiedTasks = len(evalResult.ModifiedTasks)
	result.RulesApplied = len(evalResult.RuleTriggerCounts)

	// Only save if there were actual modifications
	if result.ModifiedTasks > 0 {
		project.ReorderRootTasksByCompletion()

		// Create backup and save (log errors instead of ignoring)
		if err := storage.BackupProject(project); err != nil {
			logging.Warn("failed to backup project: %v", err)
		}
		if err := storage.SaveProject(project); err != nil {
			logging.Warn("failed to save project: %v", err)
		}

		// Update trigger counts
		if len(evalResult.RuleTriggerCounts) > 0 {
			cfg.IncrementRuleTriggerCounts(evalResult.RuleTriggerCounts)
		}
	}

	// Evaluate project-level rules against aggregate task data
	if project.StatusIconSource != "user" {
		projCtx := &workflow.ProjectEvaluationContext{
			Project: project,
			Tasks:   project.Tasks,
		}
		projResult := workflow.EvaluateProjectRules(projCtx, rules)
		if projResult.NewStatusIcon != "" {
			project.StatusIcon = projResult.NewStatusIcon
			project.StatusIconSource = "auto"
			// Count triggered project rules
			for _, id := range projResult.TriggeredRuleIDs {
				cfg.IncrementRuleTriggerCounts(map[string]int{id: 1})
			}
		}
	}

	// Update eval state regardless of changes
	cfg.SetWorkflowEvalState(project.ID, config.WorkflowEvalState{
		LastEvalTime: time.Now(),
		RulesHash:    cfg.ComputeWorkflowRulesHash(),
	})

	// Persist config changes
	if err := manager.Save(cfg); err != nil {
		logging.Warn("failed to save config: %v", err)
	}

	return result
}
