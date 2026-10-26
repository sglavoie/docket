package tui

import (
	"errors"
	"fmt"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/docket/internal/app"
	"github.com/sglavoie/docket/internal/config"
	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/storage"
	"github.com/sglavoie/docket/internal/tui/coordinator"
	"github.com/sglavoie/docket/internal/tui/notes"
	"github.com/sglavoie/docket/internal/tui/outliner"
	"github.com/sglavoie/docket/internal/tui/presentation"
	"github.com/sglavoie/docket/internal/tui/status"
	"github.com/sglavoie/docket/internal/tui/taskref"
)

// nextOrderContains reports whether the Next order index already lists the
// project-qualified task.
// todayContains reports whether the Daily Plan (Today) already lists the
// project-qualified task.
// applyTaskPlanningAction drives production mutations through Service.ApplyPlanning
// and gives isolated in-memory UI tests the same app-owned state/invariant logic
// without manufacturing a filesystem repository.
func (m *AppModel) applyTaskPlanningAction(taskID, projectID string, action app.PlanningAction) (*models.Task, *models.Project, string, error) {
	if m.domain.service != nil && m.domain.manager != nil {
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
		builder := coordinator.PlanningArtifacts(m.domain.runtime, m.domain.manager)
		m.domain.service.WithPlanningArtifacts(builder)
		res, err := m.domain.service.ApplyPlanning(app.PlanningRequest{
			Locator: app.TaskLocator{ProjectID: projectID, TaskID: taskID},
			Action:  action,
		})
		if err != nil {
			return nil, m.projectByID(projectID), "", err
		}
		// A planning move is a single-task result and names its own task, which
		// it leaves at its address, so undoing a triage returns the cursor to
		// the task it restored (inbox-triage-undo-returns-to-task).
		locator := app.TaskLocator{ProjectID: projectID, TaskID: taskID}
		m.undoFocus.ObserveCommit(res.TransactionID, coordinator.CommittedFocusTarget{Before: locator, After: locator})
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
	if m.domain.runtime != nil {
		app.ReconcilePlanningState(m.domain.runtime.Planning(), app.PlanningReconcileRequest{
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
	if m.domain.runtime == nil {
		return
	}
	planning := m.domain.runtime.Planning()
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
// reconciliation into one ledger transaction. The restore itself is the one
// every ledger undo takes, restoreLedgerEntry, so the wording, the exhausted and
// retired-entry handling, the due-clock supersession, and the focus correlation
// cannot drift from it; the planning reconcile is the only step added on top.
// Because it reads the durable ledger rather than in-memory toggle history, the
// restore survives a restart.
//
// restoreLedgerEntry puts the toggle history down, as any ledger restore must;
// the toggle's own undo and redo just wrote the history that keeps u and Ctrl+R
// walking the planning chain, so it is carried across the restore.
func (m *AppModel) planningLedgerRestore(redo bool) tea.Cmd {
	history := m.toggleHistorySnapshot()
	cmd, _, restored := m.restoreLedgerEntry(redo, "")
	if !restored {
		return cmd
	}
	m.restoreToggleHistory(history)
	changed, err := m.domain.service.ReconcilePlanning()
	if err != nil {
		return tea.Batch(cmd, status.ErrorCmd("%s", err))
	}
	if changed && m.refreshRuntime() {
		m.rebuildPlanningFromRuntime()
		m.refreshLoadedTaskWorkspace()
	}
	return cmd
}

// reconcileNoteLedgerRestore advances every shell cache affected by the exact
// workspace transaction restored from a notes view. The ledger operation has
// already completed before this runs, so failures are accumulated while safe
// refresh work continues and are reported separately from restore failures.
func (m *AppModel) reconcileNoteLedgerRestore(entry storage.LedgerEntry) ([]tea.Cmd, error) {
	var refreshErrs []error
	runtimeReloaded := true
	if m.domain.runtime != nil && m.domain.runtime.DataDir() != "" {
		if err := m.domain.runtime.RefreshAfterCommit(); err != nil {
			runtimeReloaded = false
			refreshErrs = append(refreshErrs, fmt.Errorf("reload runtime configuration: %w", err))
		} else {
			m.syncProjectCache()
		}
	} else {
		m.syncProjectCache()
	}

	reloadedProjects, projectErrs := m.reloadLedgerProjects(entry.ProjectIDs)
	refreshErrs = append(refreshErrs, projectErrs...)
	if runtimeReloaded {
		if err := m.repairTaskProjectRefsAfterRestore(reloadedProjects); err != nil {
			refreshErrs = append(refreshErrs, fmt.Errorf("repair task references: %w", err))
		}
	}

	cmds := m.refreshTaskViewsAfterRestore(runtimeReloaded)
	// A few established view refreshers reload their own project pointer from
	// disk. Rebind the shell-owned planning/visit pointers once more after that
	// fan-out without repeating any persistent runtime repair.
	if runtimeReloaded {
		m.rewireTaskProjectRefsAfterRestore(reloadedProjects)
	}
	return cmds, errors.Join(refreshErrs...)
}

// reloadLedgerProjects reloads only the projects named by the restored entry
// and returns the successful set. Reference repair must never use a project
// whose disk reload failed, because its task pointers still describe the state
// from before the ledger restore.
func (m *AppModel) reloadLedgerProjects(projectIDs []string) ([]*models.Project, []error) {
	projects := make([]*models.Project, 0, len(projectIDs))
	var refreshErrs []error
	seen := make(map[string]bool, len(projectIDs))
	for _, projectID := range projectIDs {
		if projectID == "" || seen[projectID] {
			continue
		}
		seen[projectID] = true
		project := m.ledgerProjectByID(projectID)
		if project == nil {
			refreshErrs = append(refreshErrs, fmt.Errorf("reload project %q: project is not in the runtime catalog", projectID))
			continue
		}
		if err := storage.LoadProject(project); err != nil {
			refreshErrs = append(refreshErrs, fmt.Errorf("reload project %q: %w", projectID, err))
			continue
		}
		projects = append(projects, project)
	}
	return projects, refreshErrs
}

func (m *AppModel) ledgerProjectByID(projectID string) *models.Project {
	if project := m.projectByID(projectID); project != nil {
		return project
	}
	if m.currentProject != nil && tuiUndoProjectID(m.currentProject) == projectID {
		return m.currentProject
	}
	if m.viewNav.views != nil {
		if view, ok := m.viewNav.views.Get(presentation.ViewNotes).(*notes.NotesViewModel); ok {
			if project := view.ProjectByID(projectID); project != nil {
				return project
			}
		}
	}
	return nil
}

func (m *AppModel) reloadActiveNotesAfterLedgerRestore() *notes.NotesViewModel {
	view := m.getActiveNotesView()
	if view == nil {
		return nil
	}
	view.Reload()
	return view
}

func tuiUndoProjectID(project *models.Project) string {
	if project == nil {
		return ""
	}
	if project.ID != "" {
		return project.ID
	}
	return project.Slug
}

func repairRuntimeTaskRefsAfterRestore(runtime *config.RuntimeState) func([]*models.Project) error {
	return func(projects []*models.Project) error {
		if runtime == nil || len(projects) == 0 {
			return nil
		}
		taskProjects := make(map[string][]string)
		for _, project := range projects {
			projectID := tuiUndoProjectID(project)
			for _, task := range project.Tasks {
				if task != nil && task.ID != "" {
					taskProjects[task.ID] = append(taskProjects[task.ID], projectID)
				}
			}
		}
		planning := coordinator.ClonePlanningState(runtime.Planning())
		planningChanged := false
		for i := range planning.Next {
			if id, ok := restoredTaskProject(taskProjects, planning.Next[i].TaskID, planning.Next[i].ProjectID); ok && planning.Next[i].ProjectID != id {
				planning.Next[i].ProjectID = id
				planningChanged = true
			}
		}
		planning.SetNext(planning.Next)
		for i := range planning.Today {
			if id, ok := restoredTaskProject(taskProjects, planning.Today[i].TaskID, planning.Today[i].ProjectID); ok && planning.Today[i].ProjectID != id {
				planning.Today[i].ProjectID = id
				planningChanged = true
			}
		}
		if runtime.DataDir() == "" {
			if planningChanged {
				*runtime.Planning() = *planning
			}
			return nil
		}
		if planningChanged {
			if err := runtime.CommitPlanning(planning); err != nil {
				return err
			}
		}
		return nil
	}
}

func restoredTaskProject(taskProjects map[string][]string, taskID, currentProjectID string) (string, bool) {
	projects := taskProjects[taskID]
	for _, projectID := range projects {
		if projectID == currentProjectID {
			return currentProjectID, true
		}
	}
	if len(projects) == 1 {
		return projects[0], true
	}
	return "", false
}
