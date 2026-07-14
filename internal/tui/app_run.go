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
func RunApp(runtime *config.RuntimeState, manager *config.Manager, opts ...AppOption) error {
	if runtime == nil {
		return fmt.Errorf("runtime state is required")
	}
	var err error
	runtime, err = recoverTUIWorkspace(runtime)
	if err != nil {
		return err
	}
	dataDir := runtime.DataDir()
	// Keep the TUI PID lock machine-local: when the durable data dir lives on
	// iCloud Drive it must never sync, so route it through the local runtime dir.
	runtimeDir := config.LocalRuntimeDirForDataDir(dataDir)
	if err := os.MkdirAll(runtimeDir, 0700); err != nil {
		return fmt.Errorf("create runtime dir: %w", err)
	}
	lockPath := pidLockPath(dataDir)
	lock, err := util.AcquirePIDLock(lockPath)
	if err != nil {
		return err
	}
	defer lock.Release()

	// Evaluate workflow rules at startup if needed
	var startupResult *StartupEvalResult
	if current := runtime.CurrentProject(); current != nil {
		startupResult = evaluateWorkflowAtStartup(runtime, manager, current)
	}

	// Apply config-driven storage settings
	if runtime.Settings().MaxActivityEntries != nil {
		storage.MaxActivityEntries = *runtime.Settings().MaxActivityEntries
	}

	model, err := NewAppModel(runtime, manager, opts...)
	if err != nil {
		return err
	}

	// Evaluate proactive alerts at startup
	model.reevaluateAlerts()

	// Pass startup eval result to model for status display
	if startupResult != nil && startupResult.Evaluated && startupResult.ModifiedTasks > 0 {
		model.statusMessage = fmt.Sprintf("%d task(s) updated by %d workflow rule(s)",
			startupResult.ModifiedTasks, startupResult.RulesApplied)
	}

	// Show data repair notification for current project
	if model.statusMessage == "" {
		if current := model.currentProject; current != nil {
			// Ensure project is loaded (may have been loaded by workflow eval already)
			if len(current.Tasks) == 0 && current.Path != "" {
				_ = storage.LoadProject(current)
			}
			if n := len(current.Warnings); n > 0 {
				model.statusMessage = fmt.Sprintf("%d data issue(s) auto-repaired (run tt doctor for details)", n)
				model.statusIsInfo = true
				if saveErr := storage.SaveProjectLocked(current); saveErr != nil {
					logging.Warn("failed to persist auto-repaired data: %v", saveErr)
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
				if t.State() == models.StateInProgress {
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

// evaluateWorkflowAtStartup evaluates workflow rules for the current project.
// Task mutations use the application transaction boundary; configuration side
// effects are committed only to their owning planning, automation, and catalog
// domains.
func evaluateWorkflowAtStartup(runtime *config.RuntimeState, manager *config.Manager, project *models.Project) *StartupEvalResult {
	result := &StartupEvalResult{}
	if runtime == nil || manager == nil || project == nil {
		return result
	}

	automation := cloneAutomationConfig(runtime.Automation())
	if !automation.ShouldEvaluateWorkflow(project.ID) {
		return result
	}
	rules := automation.GetEnabledWorkflowRules()
	if len(rules) == 0 {
		return result
	}

	svc := newTUIAppService(runtime.DataDir(), manager.RuntimeDependencies())
	wr, err := svc.ApplyWorkflows(project.ID, rules, nil, workflowFocusArtifactBuilder(manager))
	if err != nil {
		logging.Warn("startup workflow evaluation failed: %v", err)
		return result
	}

	result.Evaluated = true
	result.ModifiedTasks = len(wr.ModifiedTaskIDs)
	result.RulesApplied = len(wr.RuleTriggerCounts)

	if len(wr.FocusChanges) > 0 {
		// ApplyWorkflows committed the planning artifact with the task cascade;
		// reload only the runtime snapshot so startup consumers observe it.
		if err := runtime.RefreshAfterCommit(); err != nil {
			logging.Warn("failed to refresh startup workflow planning changes: %v", err)
		}
	}

	automation.IncrementRuleTriggerCounts(wr.RuleTriggerCounts)
	projectChanged := false
	if project.StatusIconSource != "user" {
		if err := storage.LoadProject(project); err == nil {
			projResult := workflow.EvaluateProjectRules(&workflow.ProjectEvaluationContext{
				Project: project,
				Tasks:   project.Tasks,
			}, rules)
			if projResult.NewStatusIcon != "" {
				project.StatusIcon = projResult.NewStatusIcon
				project.StatusIconSource = "auto"
				projectChanged = true
				for _, id := range projResult.TriggeredRuleIDs {
					automation.IncrementRuleTriggerCounts(map[string]int{id: 1})
				}
			}
		}
	}

	automation.SetWorkflowEvalState(project.ID, config.WorkflowEvalState{
		LastEvalTime: time.Now(),
		RulesHash:    automation.ComputeWorkflowRulesHash(),
	})
	if err := runtime.CommitAutomation(automation); err != nil {
		logging.Warn("failed to persist startup workflow automation state: %v", err)
	}
	if projectChanged {
		catalog := cloneWorkspaceCatalog(runtime.Catalog())
		catalog.UpsertProject(project)
		if err := runtime.CommitCatalog(catalog); err != nil {
			logging.Warn("failed to persist startup workflow project status: %v", err)
		}
	}

	return result
}
