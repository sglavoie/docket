package tui

import (
	"fmt"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
)

// dailyPlanToggleAction records a single w-key toggle for undo support. It is
// only used for the planning-only cases (adding a non-Next task to Today, or
// removing a task from Today), which are pure config changes with no ledger
// transaction to ride. The durable Next-to-Today move (a workflow-state change)
// is committed as one workspace-ledger transaction instead and undoes/redoes
// through the ledger (undoQueueToggle/redoQueueToggle), so it is never recorded
// here.
type dailyPlanToggleAction struct {
	Task    *models.Task
	Project *models.Project
	WasAdd  bool // true = task was added to daily plan
}

// toggleDailyPlan adds or removes the focused task from the daily plan.
// Returns a status timeout cmd on success, or nil if no task is available.
func (m *AppModel) toggleDailyPlan() tea.Cmd {
	view := m.activeView()
	if view == nil {
		return nil
	}
	qt, ok := view.(QueueToggleable)
	if !ok {
		return nil
	}
	task, project := qt.getTaskRef()
	if task == nil || project == nil {
		return nil
	}
	taskID, projectID := task.ID, project.ID

	if m.dailyPlanIDs[taskID] {
		// Remove from daily plan (planning-only; workflow state is untouched).
		m.removeFromDailyPlan(taskID)
		m.setInfoStatus(fmt.Sprintf("Removed from today's plan: %s", task.Title))
		return m.recordPlanningOnlyDailyPlanToggle(&dailyPlanToggleAction{Task: task, Project: project, WasAdd: false})
	}

	if taskIsNext(task) {
		// Mutual exclusivity: a Today task must not stay in Next. Fold the workflow
		// state change (StateNext -> StateReady), the Next order drop, and the Today
		// insertion into ONE workspace-ledger transaction so a crash cannot land a
		// partial move and durable undo/redo restore all three together.
		fresh, _, err := m.dailyPlanLedgerTransition(taskID, projectID)
		if err != nil {
			return errorCmd("%s", err)
		}
		m.setInfoStatus(fmt.Sprintf("Added to today's plan: %s", taskTitleOrFallback(fresh, task)))
		// Route undo/redo through the durable ledger LIFO, exactly like the Next
		// queue toggle, so consecutive u/ctrl+r presses walk the ledger stack.
		m.lastToggleUndoKind = toggleUndoQueue
		m.lastQueueToggleAt = time.Now()
		m.clearToggleRedoHistory()
		return m.queueToggleResultCmd()
	}

	// Add a non-Next task to Today: a pure planning-domain change with a bespoke
	// in-memory undo (there is no ledger transaction to ride).
	m.dailyPlan = append(m.dailyPlan, FocusTask{Task: task, Project: project})
	m.dailyPlanIDs[taskID] = true
	m.setInfoStatus(fmt.Sprintf("Added to today's plan: %s", task.Title))
	return m.recordPlanningOnlyDailyPlanToggle(&dailyPlanToggleAction{Task: task, Project: project, WasAdd: true})
}

// recordPlanningOnlyDailyPlanToggle records a bespoke in-memory undo entry for a
// planning-only Today toggle and persists the plan.
func (m *AppModel) recordPlanningOnlyDailyPlanToggle(action *dailyPlanToggleAction) tea.Cmd {
	m.lastDailyPlanToggle = action
	m.lastToggleUndoKind = toggleUndoDailyPlan
	m.lastDailyPlanToggleAt = time.Now()
	m.clearToggleRedoHistory()
	m.persistDailyPlan()
	return defaultStatusTimeout()
}

// dailyPlanLedgerTransition commits a Next-to-Today move as one ledger
// transaction that folds the workflow-state change (StateNext -> StateReady) and
// the planning change (drop the stale Next order entry, add the Today entry)
// together, then reloads the touched project and config and rebuilds the
// in-memory planning collections from the committed state. It is the Today-side
// analog of queueLedgerTransition.
func (m *AppModel) dailyPlanLedgerTransition(taskID, projectID string) (*models.Task, *models.Project, error) {
	plannedDate := time.Now().Format("2006-01-02")
	builder := dailyPlanAddArtifactBuilder(m.configManager, taskID, projectID, plannedDate)
	res, err := m.appService.Transition(app.StateRequest{TaskID: taskID, Target: models.StateReady, PlanningArtifacts: builder})
	if err != nil {
		return nil, m.projectByID(projectID), err
	}
	m.reloadProjectByID(res.ProjectID)
	m.refreshRuntime()
	m.rebuildPlanningFromRuntime()
	project := m.projectByID(res.ProjectID)
	var fresh *models.Task
	if project != nil {
		fresh = project.GetTaskByID(taskID)
	}
	return fresh, project, nil
}

// undoDailyPlanToggle reverses the last planning-only Today toggle.
func (m *AppModel) undoDailyPlanToggle() tea.Cmd {
	action := m.lastDailyPlanToggle
	if action == nil {
		return nil
	}
	m.lastDailyPlanToggleRedo = action
	m.lastDailyPlanToggle = nil
	m.lastDailyPlanToggleAt = time.Time{}
	m.lastDailyPlanRedoAt = time.Now()
	m.lastToggleRedoKind = toggleUndoDailyPlan
	m.lastToggleUndoKind = m.nextUndoKind(toggleUndoDailyPlan)

	taskID := action.Task.ID
	if action.WasAdd {
		// Reverse add: remove from daily plan.
		m.removeFromDailyPlan(taskID)
	} else if !m.dailyPlanIDs[taskID] {
		// Reverse remove: add back to daily plan.
		m.dailyPlan = append(m.dailyPlan, FocusTask{Task: action.Task, Project: action.Project})
		m.dailyPlanIDs[taskID] = true
	}
	m.persistDailyPlan()
	m.setInfoStatus(fmt.Sprintf("Undone: %s", action.Task.Title))
	return defaultStatusTimeout()
}

// redoDailyPlanToggle replays the last undone planning-only Today toggle.
func (m *AppModel) redoDailyPlanToggle() tea.Cmd {
	action := m.lastDailyPlanToggleRedo
	if action == nil {
		return nil
	}
	m.lastDailyPlanToggleRedo = nil
	m.lastDailyPlanRedoAt = time.Time{}
	m.lastToggleRedoKind = m.nextRedoKind(toggleUndoDailyPlan)

	taskID := action.Task.ID
	if action.WasAdd {
		// Redo add: re-add to daily plan.
		if !m.dailyPlanIDs[taskID] {
			m.dailyPlan = append(m.dailyPlan, FocusTask{Task: action.Task, Project: action.Project})
			m.dailyPlanIDs[taskID] = true
		}
	} else {
		// Redo remove: re-remove from daily plan.
		m.removeFromDailyPlan(taskID)
	}
	m.persistDailyPlan()
	m.setInfoStatus(fmt.Sprintf("Redone: %s", action.Task.Title))
	m.lastDailyPlanToggle = action
	m.lastDailyPlanToggleAt = time.Now()
	m.lastToggleUndoKind = toggleUndoDailyPlan
	return defaultStatusTimeout()
}

// persistDailyPlan saves the current daily plan state through a checked planning
// commit. The Daily Plan is planning-domain state, so it is committed to
// planning.json alone rather than routed through a whole-config save.
func (m *AppModel) persistDailyPlan() {
	today := time.Now().Format("2006-01-02")
	items := make([]config.DailyPlanItem, len(m.dailyPlan))
	for i, ft := range m.dailyPlan {
		items[i] = config.DailyPlanItem{
			TaskID:      ft.Task.ID,
			ProjectID:   ft.Project.ID,
			PlannedDate: today,
		}
	}
	if err := m.commitPlanningMutation(func(p *config.PlanningState) {
		p.SetDailyPlan(items)
	}); err != nil {
		m.err = err
		return
	}
	if len(items) > 0 {
		m.advanceOnboarding(func(progress *config.OnboardingProgress) { progress.TaskPlanned = true })
	}
}
