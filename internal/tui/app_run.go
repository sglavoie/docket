package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/logging"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/util"
	"github.com/sglavoie/tt/internal/workflow"
)

// AppVersion is the application version string used when writing theme files.
// Set this from the cmd package before calling RunApp.
var AppVersion = "dev"

// StartupEvalResult holds the result of workflow evaluation at TUI startup.
type StartupEvalResult struct {
	Evaluated     bool
	ModifiedTasks int
	RulesApplied  int
}

// RunApp starts the TUI application.
func RunApp(cfg *config.Config, manager *config.Manager, opts ...AppOption) error {
	// Keep the TUI PID lock machine-local: when the durable data dir lives on
	// iCloud Drive it must never sync, so route it through the local runtime dir.
	runtimeDir := config.LocalRuntimeDirForDataDir(cfg.DataDir)
	if err := os.MkdirAll(runtimeDir, 0700); err != nil {
		return fmt.Errorf("create runtime dir: %w", err)
	}
	lockPath := pidLockPath(cfg.DataDir)
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
				if err := storage.SaveProjectLocked(current); err != nil {
					logging.Warn("failed to persist auto-repaired data: %v", err)
				}
			}
		}
	}

	p := tea.NewProgram(model)

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

	finalModel, err := p.Run()
	if app, ok := finalModel.(*AppModel); ok {
		if app.currentProject != nil {
			now := time.Now()
			for _, t := range app.currentProject.Tasks {
				if t.Status == models.StatusInProgress {
					app.sessionStats.tasksInProgress++
				}
				if t.SnoozedUntil != nil && t.SnoozedUntil.After(now) {
					app.sessionStats.tasksSnoozed++
				}
			}
		}
		printSessionSummary(app.sessionStats)
	}
	return err
}

// pidLockPath returns the machine-local PID lock path for a given durable data
// dir. The lock lives under the local runtime dir (never synced) and is keyed by
// data dir so distinct data directories get isolated locks.
func pidLockPath(dataDir string) string {
	return filepath.Join(config.LocalRuntimeDirForDataDir(dataDir), "pid.lock")
}

// printSessionSummary writes a brief session summary to stderr after the TUI exits.
func printSessionSummary(stats sessionStats) {
	if stats.tasksCompleted == 0 && stats.tasksCreated == 0 && stats.tasksInProgress == 0 && stats.tasksSnoozed == 0 {
		return
	}

	greenStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	yellowStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("3"))

	var parts []string
	if stats.tasksCompleted > 0 {
		parts = append(parts, greenStyle.Render(fmt.Sprintf("%d", stats.tasksCompleted))+" completed")
	}
	if stats.tasksCreated > 0 {
		parts = append(parts, fmt.Sprintf("%d created", stats.tasksCreated))
	}
	if stats.tasksSnoozed > 0 {
		parts = append(parts, fmt.Sprintf("%d snoozed", stats.tasksSnoozed))
	}

	header := "Session"
	if dur := sessionDurationStr(stats.sessionStart); dur != "" {
		header = "Session (" + dur + ")"
	}

	var msg string
	if len(parts) > 0 {
		msg = header + ": " + strings.Join(parts, ", ")
	} else {
		msg = header + ":"
	}
	if stats.tasksInProgress > 0 {
		msg += " · " + yellowStyle.Render(fmt.Sprintf("%d", stats.tasksInProgress)) + " still in progress"
	}
	fmt.Fprintln(os.Stderr, msg)
}

// sessionDurationStr formats elapsed time since start as "Xm" or "Xh Ym".
// Returns empty string when elapsed is under one minute.
func sessionDurationStr(start time.Time) string {
	if start.IsZero() {
		return ""
	}
	d := time.Since(start).Round(time.Minute)
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	if h > 0 {
		return fmt.Sprintf("%dh %dm", h, m)
	}
	if m > 0 {
		return fmt.Sprintf("%dm", m)
	}
	return ""
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
		if err := storage.SaveProjectLocked(project); err != nil {
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
	saveConfigWarn(manager, cfg, "TUI shutdown")

	return result
}
