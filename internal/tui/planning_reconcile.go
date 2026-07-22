package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

// reconcilePlanningExclusivity applies Today/Next exclusivity to a planning
// snapshot for exactly the transitioned tasks: a task now in StateNext leaves the
// Daily Plan (Today), and a task that is no longer StateNext drops its stale Next
// order-index entry. It returns true when any entry changed. It is the shared
// reconciliation every TUI transition folds into the same durable ledger
// transaction as the task write, so undo restores task state and planning
// membership together.
func reconcilePlanningExclusivity(planning *config.PlanningState, targets []app.PlanningReconcileTarget) bool {
	changed := false
	for _, t := range targets {
		if t.State == models.StateNext {
			if planning.RemoveFromDailyPlanInProject(t.TaskID, t.ProjectID) {
				changed = true
			}
			continue
		}
		if planning.RemoveFromFocusQueueInProject(t.TaskID, t.ProjectID) {
			changed = true
		}
	}
	return changed
}

// focusQueueContains reports whether the Next order index already lists taskID.
func focusQueueContains(planning *config.PlanningState, taskID string) bool {
	for _, item := range planning.GetFocusQueue() {
		if item.TaskID == taskID {
			return true
		}
	}
	return false
}

// focusQueueAddArtifactBuilder renders the planning.json artifact that rides a
// Next-queue promotion: it reconciles Today/Next exclusivity for the transitioned
// task (dropping any Daily Plan entry) and appends the task's Next order entry, so
// the workflow-state change, the Today drop, and the order entry all land in one
// durable ledger transaction and undo together.
func focusQueueAddArtifactBuilder(mgr *config.Manager, taskID, projectID string) app.PlanningArtifactBuilder {
	if mgr == nil {
		return nil
	}
	return func(targets []app.PlanningReconcileTarget) ([]storage.ArtifactChange, error) {
		coord := mgr.DomainCoordinator()
		planning, err := coord.LoadPlanning()
		if err != nil {
			return nil, err
		}
		reconcilePlanningExclusivity(planning, targets)
		if !focusQueueContains(planning, taskID) {
			planning.SetFocusQueue(append(planning.GetFocusQueue(), config.FocusQueueItem{TaskID: taskID, ProjectID: projectID}))
		}
		return planningDomainArtifact(mgr, planning)
	}
}

// dailyPlanContains reports whether the Daily Plan (Today) already lists taskID.
func dailyPlanContains(planning *config.PlanningState, taskID string) bool {
	for _, item := range planning.GetDailyPlan() {
		if item.TaskID == taskID {
			return true
		}
	}
	return false
}

// dailyPlanAddArtifactBuilder renders the planning.json artifact that rides a
// Next-to-Today move: it reconciles Today/Next exclusivity for the transitioned
// task (dropping the stale Next order entry now that the task has left StateNext)
// and appends the task's Today (Daily Plan) entry, so the workflow-state change,
// the Next order drop, and the Today insertion all land in one durable ledger
// transaction and undo together. This is the Today-side analog of
// focusQueueAddArtifactBuilder.
func dailyPlanAddArtifactBuilder(mgr *config.Manager, taskID, projectID, plannedDate string) app.PlanningArtifactBuilder {
	if mgr == nil {
		return nil
	}
	return func(targets []app.PlanningReconcileTarget) ([]storage.ArtifactChange, error) {
		coord := mgr.DomainCoordinator()
		planning, err := coord.LoadPlanning()
		if err != nil {
			return nil, err
		}
		reconcilePlanningExclusivity(planning, targets)
		if !dailyPlanContains(planning, taskID) {
			planning.SetDailyPlan(append(planning.GetDailyPlan(), config.DailyPlanItem{
				TaskID:      taskID,
				ProjectID:   projectID,
				PlannedDate: plannedDate,
			}))
		}
		return planningDomainArtifact(mgr, planning)
	}
}

// plannerCaptureArtifactBuilder builds the planning side of a tab-preset task
// creation. The application layer supplies the freshly allocated task identity,
// allowing the project write and Today/Next membership to share one ChangeSet.
func plannerCaptureArtifactBuilder(mgr *config.Manager, tab int, plannedDate string) app.PlanningArtifactBuilder {
	if mgr == nil || (tab != boardTabToday && tab != boardTabNext) {
		return nil
	}
	return func(targets []app.PlanningReconcileTarget) ([]storage.ArtifactChange, error) {
		coord := mgr.DomainCoordinator()
		planning, err := coord.LoadPlanning()
		if err != nil {
			return nil, err
		}
		reconcilePlanningExclusivity(planning, targets)
		for _, target := range targets {
			switch tab {
			case boardTabToday:
				if !dailyPlanContains(planning, target.TaskID) {
					planning.SetDailyPlan(append(planning.GetDailyPlan(), config.DailyPlanItem{
						TaskID: target.TaskID, ProjectID: target.ProjectID, PlannedDate: plannedDate,
					}))
				}
			case boardTabNext:
				if !focusQueueContains(planning, target.TaskID) {
					planning.SetFocusQueue(append(planning.GetFocusQueue(), config.FocusQueueItem{
						TaskID: target.TaskID, ProjectID: target.ProjectID,
					}))
				}
			}
		}
		return planningDomainArtifact(mgr, planning)
	}
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
		m.focusQueueIDs = make(map[string]bool)
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
			m.focusQueueIDs[item.TaskID] = true
		}
	}
	m.dailyPlan = m.dailyPlan[:0]
	if m.dailyPlanIDs == nil {
		m.dailyPlanIDs = make(map[string]bool)
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
			m.dailyPlanIDs[item.TaskID] = true
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
	return func(targets []app.PlanningReconcileTarget) ([]storage.ArtifactChange, error) {
		coord := mgr.DomainCoordinator()
		planning, err := coord.LoadPlanning()
		if err != nil {
			return nil, err
		}
		if !reconcilePlanningExclusivity(planning, targets) {
			return nil, nil
		}
		return planningDomainArtifact(mgr, planning)
	}
}
