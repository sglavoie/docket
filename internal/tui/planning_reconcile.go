package tui

import (
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/tui/coordinator"
	"github.com/sglavoie/tt/internal/tui/outliner"
	"github.com/sglavoie/tt/internal/tui/status"
	"github.com/sglavoie/tt/internal/tui/taskref"
)

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
		builder := coordinator.PlanningArtifacts(m.runtime, m.configManager)
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
			m.crossOutliner.Population.Invalidate()
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
			m.nextOrder = append(m.nextOrder, outliner.PlanningTask{Task: t, Project: p})
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
			m.today = append(m.today, outliner.PlanningTask{Task: t, Project: p})
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
