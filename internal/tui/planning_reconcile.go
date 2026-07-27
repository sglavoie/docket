package tui

import (
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

// focusQueueContains reports whether the Next order index already lists the
// project-qualified task.
func focusQueueContains(planning *config.PlanningState, identity TaskIdentity) bool {
	for _, item := range planning.GetFocusQueue() {
		if item.TaskID == identity.TaskID && item.ProjectID == identity.ProjectID {
			return true
		}
	}
	return false
}

// dailyPlanContains reports whether the Daily Plan (Today) already lists the
// project-qualified task.
func dailyPlanContains(planning *config.PlanningState, identity TaskIdentity) bool {
	for _, item := range planning.GetDailyPlan() {
		if item.TaskID == identity.TaskID && item.ProjectID == identity.ProjectID {
			return true
		}
	}
	return false
}

// applyTaskPlanningAction drives production mutations through Service.ApplyPlanning
// and gives isolated in-memory UI tests the same app-owned state/invariant logic
// without manufacturing a filesystem repository.
func (m *AppModel) applyTaskPlanningAction(taskID, projectID string, action app.PlanningAction) (*models.Task, *models.Project, error) {
	triagingInbox := false
	if m.activeInboxTriage() {
		if project := m.projectByID(projectID); project != nil {
			if task := project.GetTaskByID(taskID); task != nil {
				triagingInbox = task.State() == models.StateInbox
			}
		}
	}
	if m.appService != nil && m.configManager != nil {
		stateChanged := false
		if project := m.projectByID(projectID); project != nil {
			if task := project.GetTaskByID(taskID); task != nil {
				switch action {
				case app.PlanningAddToday:
					stateChanged = task.State() == models.StateInbox || task.State() == models.StateNext
				case app.PlanningAddNext:
					stateChanged = task.State() != models.StateNext
				case app.PlanningRemoveToday:
				case app.PlanningRemoveNext:
					stateChanged = task.State() == models.StateNext
				case app.PlanningBacklog:
					stateChanged = task.State() != models.StateReady
				}
			}
		}
		builder := runtimePlanningReconcileArtifactBuilder(m.runtime)
		if builder == nil {
			builder = planningReconcileArtifactBuilder(m.configManager)
		}
		m.appService.WithPlanningArtifacts(builder)
		res, err := m.appService.ApplyPlanning(app.PlanningRequest{
			Locator: app.TaskLocator{ProjectID: projectID, TaskID: taskID},
			Action:  action,
		})
		if err != nil {
			return nil, m.projectByID(projectID), err
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
			return nil, nil, nil
		}
		task := project.GetTaskByID(taskID)
		if triagingInbox && task != nil && task.State() != models.StateInbox {
			m.markInboxTriaged()
		}
		return task, project, nil
	}

	project := m.projectByID(projectID)
	if project == nil {
		project = m.currentProject
	}
	if project == nil {
		return nil, nil, fmt.Errorf("planning project %s is unavailable", projectID)
	}
	task := project.GetTaskByID(taskID)
	if task == nil {
		return nil, project, fmt.Errorf("planning task %s is unavailable", taskID)
	}
	target, _, err := app.ApplyPlanningActionToTask(
		task, project.ID, action, time.Now().Format("2006-01-02"), time.Now(),
	)
	if err != nil {
		return nil, project, err
	}
	if m.runtime != nil {
		app.ReconcilePlanningState(m.runtime.Planning(), app.PlanningReconcileRequest{
			Targets: []app.PlanningReconcileTarget{target},
		})
		m.rebuildPlanningFromRuntime()
	}
	if triagingInbox && task.State() != models.StateInbox {
		m.markInboxTriaged()
	}
	return task, project, nil
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
	m.focusQueue = m.focusQueue[:0]
	if m.focusQueueIDs == nil {
		m.focusQueueIDs = make(map[TaskIdentity]bool)
	} else {
		clear(m.focusQueueIDs)
	}
	for _, item := range planning.GetFocusQueue() {
		p := m.projectByID(item.ProjectID)
		if p == nil {
			continue
		}
		if t := p.GetTaskByID(item.TaskID); t != nil {
			m.focusQueue = append(m.focusQueue, FocusTask{Task: t, Project: p})
			m.focusQueueIDs[TaskIdentity{ProjectID: item.ProjectID, TaskID: item.TaskID}] = true
		}
	}
	m.dailyPlan = m.dailyPlan[:0]
	if m.dailyPlanIDs == nil {
		m.dailyPlanIDs = make(map[TaskIdentity]bool)
	} else {
		clear(m.dailyPlanIDs)
	}
	for _, item := range planning.GetDailyPlan() {
		p := m.projectByID(item.ProjectID)
		if p == nil {
			continue
		}
		if t := p.GetTaskByID(item.TaskID); t != nil {
			m.dailyPlan = append(m.dailyPlan, FocusTask{Task: t, Project: p})
			m.dailyPlanIDs[TaskIdentity{ProjectID: item.ProjectID, TaskID: item.TaskID}] = true
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
		// An exhausted ledger is not an error here: fall through so u/ctrl+r can
		// defer to the per-project history fallback rather than toasting.
		if strings.Contains(err.Error(), "history available") {
			return nil
		}
		return errorCmd("%s", err)
	}
	for _, id := range entry.ProjectIDs {
		m.reloadProjectByID(id)
	}
	m.refreshRuntime()
	if changed, reconcileErr := m.appService.ReconcilePlanning(); reconcileErr != nil {
		return errorCmd("%s", reconcileErr)
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
		return planningDomainArtifact(mgr, planning)
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
