package tui

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/tui/status"
	"github.com/sglavoie/tt/internal/tui/taskref"
)

// shellPlanningReconcileArtifactBuilder binds application-owned invariant
// mutations to the shell's revision-bound planning snapshot. A mismatched
// manager/runtime pair is rejected before mutation instead of writing planning
// through an unrelated workspace root.
func shellPlanningReconcileArtifactBuilder(runtime *config.RuntimeState, mgr *config.Manager) app.PlanningArtifactBuilder {
	if runtime != nil {
		if planningWorkspaceMismatch(runtime, mgr) {
			return func(app.PlanningReconcileRequest) ([]storage.ArtifactChange, error) {
				return nil, fmt.Errorf("planning manager and runtime refer to different workspaces")
			}
		}
		return runtimePlanningReconcileArtifactBuilder(runtime)
	}
	return planningReconcileArtifactBuilder(mgr)
}

// shellStructuralPlanningArtifactBuilder is used for delete and project move.
// Their request contains the complete qualified remove/reassign delta, so it can
// reconcile the newest on-disk planning state without adopting stale shell
// intent. The resulting expected revision still detects a writer racing the
// combined project-plus-planning commit.
func shellStructuralPlanningArtifactBuilder(runtime *config.RuntimeState, mgr *config.Manager) app.PlanningArtifactBuilder {
	if planningWorkspaceMismatch(runtime, mgr) {
		return func(app.PlanningReconcileRequest) ([]storage.ArtifactChange, error) {
			return nil, fmt.Errorf("planning manager and runtime refer to different workspaces")
		}
	}
	if mgr != nil {
		return planningReconcileArtifactBuilder(mgr)
	}
	return runtimePlanningReconcileArtifactBuilder(runtime)
}

func planningWorkspaceMismatch(runtime *config.RuntimeState, mgr *config.Manager) bool {
	return runtime != nil && mgr != nil && runtime.DataDir() != "" &&
		filepath.Clean(config.WorkspaceRootFromConfigPath(mgr.GetConfigPath())) != filepath.Clean(runtime.DataDir())
}

// nextOrderContains reports whether the Next order index already lists the
// project-qualified task.
// todayContains reports whether the Daily Plan (Today) already lists the
// project-qualified task.
// applyTaskPlanningAction drives production mutations through Service.ApplyPlanning
// and gives isolated in-memory UI tests the same app-owned state/invariant logic
// without manufacturing a filesystem repository.
func (m *AppModel) applyTaskPlanningAction(taskID, projectID string, action app.PlanningAction) (*models.Task, *models.Project, string, error) {
	if m.appService != nil && m.configManager != nil {
		stateChanged := false
		if project := m.projectByID(projectID); project != nil {
			if task := project.GetTaskByID(taskID); task != nil {
				switch action {
				case app.PlanningAddToday:
					stateChanged = (task.State() != models.StateDoing && task.State() != models.StateBacklog) || task.DeferredUntil != nil
				case app.PlanningAddNext:
					stateChanged = task.State() != models.StateBacklog || task.DeferredUntil != nil
				case app.PlanningRemoveToday:
				case app.PlanningRemoveNext:
				case app.PlanningBacklog:
					stateChanged = task.State() != models.StateBacklog
				}
			}
		}
		builder := shellPlanningReconcileArtifactBuilder(m.runtime, m.configManager)
		m.appService.WithPlanningArtifacts(builder)
		res, err := m.appService.ApplyPlanning(app.PlanningRequest{
			Locator: app.TaskLocator{ProjectID: projectID, TaskID: taskID},
			Action:  action,
		})
		if err != nil {
			return nil, m.projectByID(projectID), "", err
		}
		if stateChanged {
			m.reloadProjectByID(res.ProjectID)
			m.reloadCrossProjectOutlinerProject(res.ProjectID)
		}
		m.refreshRuntime()
		m.rebuildPlanningFromRuntime()
		if m.crossOutliner != nil {
			m.crossOutliner.crossForceRebuild = true
		}
		project := m.projectByID(res.ProjectID)
		if project == nil {
			return nil, nil, res.TransactionID, nil
		}
		task := project.GetTaskByID(taskID)
		return task, project, res.TransactionID, nil
	}

	project := m.projectByID(projectID)
	if project == nil {
		project = m.currentProject
	}
	if project == nil {
		return nil, nil, "", fmt.Errorf("planning project %s is unavailable", projectID)
	}
	task := project.GetTaskByID(taskID)
	if task == nil {
		return nil, project, "", fmt.Errorf("planning task %s is unavailable", taskID)
	}
	target, _, err := app.ApplyPlanningActionToTask(
		task, project.ID, action, time.Now(),
	)
	if err != nil {
		return nil, project, "", err
	}
	if m.runtime != nil {
		app.ReconcilePlanningState(m.runtime.Planning(), app.PlanningReconcileRequest{
			Targets: []app.PlanningReconcileTarget{target},
		})
		m.rebuildPlanningFromRuntime()
	}
	return task, project, "", nil
}

// rebuildPlanningFromRuntime rebuilds the in-memory Next order index and Daily
// Plan from the (freshly reloaded) planning domain, resolving each entry against
// the loaded projects. It is called after a ledger restore repoints the durable
// planning collections, so the open planning views reflect the restored state.
func (m *AppModel) rebuildPlanningFromRuntime() {
	if m.runtime == nil {
		return
	}
	planning := m.runtime.Planning()
	m.nextOrder = m.nextOrder[:0]
	if m.nextOrderIDs == nil {
		m.nextOrderIDs = make(map[taskref.TaskIdentity]bool)
	} else {
		clear(m.nextOrderIDs)
	}
	for _, item := range planning.GetNext() {
		p := m.projectByID(item.ProjectID)
		if p == nil {
			continue
		}
		if t := p.GetTaskByID(item.TaskID); t != nil {
			m.nextOrder = append(m.nextOrder, PlanningTask{Task: t, Project: p})
			m.nextOrderIDs[taskref.TaskIdentity{ProjectID: item.ProjectID, TaskID: item.TaskID}] = true
		}
	}
	m.today = m.today[:0]
	if m.todayIDs == nil {
		m.todayIDs = make(map[taskref.TaskIdentity]bool)
	} else {
		clear(m.todayIDs)
	}
	for _, item := range planning.GetToday() {
		p := m.projectByID(item.ProjectID)
		if p == nil {
			continue
		}
		if t := p.GetTaskByID(item.TaskID); t != nil {
			m.today = append(m.today, PlanningTask{Task: t, Project: p})
			m.todayIDs[taskref.TaskIdentity{ProjectID: item.ProjectID, TaskID: item.TaskID}] = true
		}
	}
}

// planningLedgerRestore drives a durable workspace-ledger undo (redo=false) or
// redo (redo=true) for a planning move that folded its state change and planning
// reconciliation into one ledger transaction. It restores the newest ledger entry
// (so a later mutation is always undone first), reloads the touched projects and
// the config snapshot, then rebuilds the in-memory planning collections. Because
// it reads the durable ledger rather than in-memory toggle history, the restore
// survives a restart.
func (m *AppModel) planningLedgerRestore(redo bool) tea.Cmd {
	ledger := workspaceLedgerFromConfig(m.runtime)
	scope := storage.GlobalScope()
	restore := ledger.UndoWithEntry
	verb := "Undone"
	if redo {
		restore = ledger.RedoWithEntry
		verb = "Redone"
	}
	entry, desc, err := restore(scope)
	if err != nil {
		// An exhausted ledger is not an error here.
		if strings.Contains(err.Error(), "history available") {
			return nil
		}
		return status.ErrorCmd("%s", err)
	}
	for _, id := range entry.ProjectIDs {
		m.reloadProjectByID(id)
	}
	m.refreshRuntime()
	if changed, reconcileErr := m.appService.ReconcilePlanning(); reconcileErr != nil {
		return status.ErrorCmd("%s", reconcileErr)
	} else if changed {
		m.refreshRuntime()
	}
	m.rebuildPlanningFromRuntime()
	m.setInfoStatus(fmt.Sprintf("%s: %s", verb, desc))
	return m.queueToggleResultCmd()
}

// planningReconcileArtifactBuilder returns the planning-artifact builder the
// application layer invokes after a transition (and its rule cascade) settles. It
// loads the current planning domain, reconciles Today/Next exclusivity for the
// transitioned tasks, and renders one revision-checked planning.json artifact for
// the same transaction. A nil manager yields a nil builder (a bare transition).
func planningReconcileArtifactBuilder(mgr *config.Manager) app.PlanningArtifactBuilder {
	if mgr == nil {
		return nil
	}
	return func(req app.PlanningReconcileRequest) ([]storage.ArtifactChange, error) {
		coord := mgr.DomainCoordinator()
		planning, err := coord.LoadPlanning()
		if err != nil {
			return nil, err
		}
		if !app.ReconcilePlanningState(planning, req) {
			return nil, nil
		}
		artifact, err := coord.CheckedPlanningArtifact(planning)
		if err != nil {
			return nil, err
		}
		return []storage.ArtifactChange{artifact.StorageChange()}, nil
	}
}

// runtimePlanningReconcileArtifactBuilder binds an interactive planning action
// to the shell's current runtime snapshot. This keeps injected/failing runtime
// tests honest and prevents a long-lived service from writing through a stale
// manager after the shell switches workspace state.
func runtimePlanningReconcileArtifactBuilder(runtime *config.RuntimeState) app.PlanningArtifactBuilder {
	if runtime == nil {
		return nil
	}
	return func(req app.PlanningReconcileRequest) ([]storage.ArtifactChange, error) {
		planning := clonePlanningState(runtime.Planning())
		if !app.ReconcilePlanningState(planning, req) {
			return nil, nil
		}
		artifact, err := runtime.CheckedPlanningArtifact(planning)
		if err != nil {
			return nil, err
		}
		return []storage.ArtifactChange{artifact.StorageChange()}, nil
	}
}
