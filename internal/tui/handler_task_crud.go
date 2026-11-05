package tui

import (
	"cmp"
	"fmt"
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/docket/internal/app"
	"github.com/sglavoie/docket/internal/icons"
	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/tui/coordinator"
	"github.com/sglavoie/docket/internal/tui/taskform"
	"github.com/sglavoie/docket/internal/tui/taskref"
	"github.com/sglavoie/docket/internal/tui/theme"
	"github.com/sglavoie/docket/internal/util"
)

// handleTaskCRUDMessages handles task CRUD messages (create, edit, delete).
// Returns true if the message was handled.
func (m *AppModel) handleTaskCRUDMessages(msg tea.Msg) (bool, tea.Cmd) {
	switch msg := msg.(type) {
	case taskform.CreateTaskMsg:
		return m.handleCreateTask(msg)

	case coordinator.TaskCreatedMsg:
		return m.handleTaskCreated(msg)

	case TaskDeleteRequestedMsg:
		m.handleTaskDeleteRequested(msg)
		return true, nil

	}
	return false, nil
}

// handleCreateTask handles the CreateTaskMsg.
func (m *AppModel) handleCreateTask(msg taskform.CreateTaskMsg) (bool, tea.Cmd) {
	project := msg.Project
	if project == nil {
		project = m.currentProject
	}
	if project == nil {
		return true, nil
	}

	return true, m.openTaskComposer(m.createComposerPreset(project, msg.ParentTask))
}

// handleTaskCreated handles the coordinator.TaskCreatedMsg.
func (m *AppModel) handleTaskCreated(msg coordinator.TaskCreatedMsg) (bool, tea.Cmd) {
	// Capture whether this creation originated from the Quick Capture view and the
	// pre-capture task count before handleWithRefresh pops the view / reloads.
	// Coordinator path: the capture committed one atomic ChangeSet through the
	// application layer, so reload the committed project from disk and re-resolve
	// the task by ID. The shared navigation/status/open-after logic below then runs
	// off the reloaded pointers exactly as it did for the legacy in-place save.
	planningRefreshed := false
	// A capture that created its projects changed the catalog too, and the
	// reload below can only find a project the runtime already knows.
	if msg.PlanningChanged || msg.ProjectsCreated {
		planningRefreshed = m.refreshRuntime()
	}
	if msg.ProjectsCreated {
		m.refreshProjectHubIfActive()
	}
	// A multi-line capture can span projects, and the planning rebuild below
	// resolves membership against the loaded tasks, so every project the commit
	// touched is reloaded before it, not just the first result's.
	reloaded := make(map[string]bool, len(msg.Results)+1)
	reload := func(projectID string) {
		if projectID != "" && !reloaded[projectID] {
			reloaded[projectID] = true
			m.reloadProjectByID(projectID)
		}
	}
	for _, result := range msg.Results {
		reload(result.ProjectID)
	}
	if msg.Result != nil {
		reload(msg.Result.ProjectID)
		if project := m.projectByID(msg.Result.ProjectID); project != nil {
			msg.Project = project
			// OpenTaskID overrides the navigation/open target (task-form
			// open-after-subtask targets a created child, not the parent).
			targetID := msg.Result.TaskID
			if msg.OpenTaskID != "" {
				targetID = msg.OpenTaskID
			}
			msg.Task = project.GetTaskByID(targetID)
		}
	}
	if planningRefreshed {
		m.rebuildPlanningFromRuntime()
	}

	refreshCmd := m.handleWithRefresh(!msg.KeepComposerOpen, true)

	var landingCmd tea.Cmd
	if msg.Task != nil && !msg.KeepComposerOpen {
		project := msg.Project
		if project == nil && msg.Result != nil {
			project = m.projectByID(msg.Result.ProjectID)
		}
		if m.crossOutliner != nil && project != nil {
			m.crossOutliner.NavigateToTaskInProject(msg.Task, project)
			landingCmd = m.crossOutliner.LightLanding(msg.Task.ID)
		}
	}

	// Set status message showing section
	if msg.Task != nil {
		if bucket := m.createdTaskSectionLabel(msg.Task, msg.Project); bucket != "" {
			if parked := m.parkedOwnerLabel(msg.Project); parked != "" {
				m.setSuccessStatus(fmt.Sprintf("Created in %s — %s", bucket, parked))
			} else {
				m.setSuccessStatus(fmt.Sprintf("Created in %s", bucket))
			}
		}
	}
	if msg.CreatedCount > 1 {
		m.setSuccessStatus(fmt.Sprintf("Created %d tasks", msg.CreatedCount))
	}
	successMessage := m.feedback.message
	if successMessage == "" {
		successMessage = "Created task"
	}
	transactionID := ""
	if msg.Result != nil {
		transactionID = msg.Result.TransactionID
	}
	// A creation has no before address, so undoing it correlates to nothing and
	// leaves the view alone; redoing it puts the cursor back on the task it mints
	// again. A multiline capture created several tasks and names none of them.
	created := coordinator.CommittedFocusTarget{}
	if msg.Result != nil && msg.CreatedCount <= 1 {
		created.After = app.TaskLocator{ProjectID: msg.Result.ProjectID, TaskID: msg.Result.TaskID}
	}
	m.undoFocus.ObserveCommit(transactionID, created)
	successMessage = cmp.Or(msg.RecoveryWarning, successMessage)
	feedbackCmd := m.setRecoveryFeedbackWithContext(successMessage, transactionID,
		recoveryWords{Headline: m.capturedTasksHeadline(msg)}, false)

	// Delegate to the current view so it can reload its state
	cmd := m.viewNav.views.Update(m.currentView(), msg)

	// If open-after requested, re-lookup the task (stale after reload) and open
	if msg.Task != nil && msg.OpenContent {
		project := msg.Project
		if project == nil {
			project = m.currentProject
		}
		if project != nil {
			if freshTask := project.GetTaskByID(msg.Task.ID); freshTask != nil {
				openMsg := EditTaskContentMsg{Task: freshTask, Project: project}
				return true, tea.Batch(refreshCmd, cmd, feedbackCmd, landingCmd, func() tea.Msg {
					return openMsg
				})
			}
		}
	}

	return true, tea.Batch(refreshCmd, cmd, feedbackCmd, landingCmd)
}

// capturedTasksHeadline is the toast a committed capture answers with. A
// single task names the project and lane it landed in, the lane read the way
// createdTaskSectionLabel reads it; several name their count and where they
// went. A legacy form result carrying no titles keeps the ledger's words, and
// a capture that owes a recovery warning says that instead.
func (m *AppModel) capturedTasksHeadline(msg coordinator.TaskCreatedMsg) string {
	if msg.RecoveryWarning != "" {
		return ""
	}
	results := msg.Results
	if len(results) == 0 && msg.Result != nil {
		results = []app.TaskResult{*msg.Result}
	}
	titles := make([]string, 0, len(results))
	projects := make([]*models.Project, 0, 1)
	seen := make(map[string]bool, 1)
	for _, result := range results {
		titles = append(titles, result.Title)
		if !seen[result.ProjectID] {
			seen[result.ProjectID] = true
			projects = append(projects, m.projectByID(result.ProjectID))
		}
	}
	lane := ""
	if len(results) == 1 && msg.Task != nil {
		lane = m.createdTaskSectionLabel(msg.Task, msg.Project)
	}
	return captureHeadline(titles, projects, lane)
}

func (m *AppModel) createdTaskSectionLabel(task *models.Task, project *models.Project) string {
	if task == nil {
		return ""
	}
	if project != nil {
		identity := taskref.For(task, project)
		if m.todayIDs[identity] {
			return models.StateGroupToday.Label()
		}
		if m.nextOrderIDs[identity] {
			return models.StateGroupNext.Label()
		}
	}
	return theme.TaskStateGroupLabel(task)
}

// The ledger records each transaction in the words Change History lists it
// by — an action and the context it acted on — and the recovery toast reads
// them back verbatim. For the two gestures made dozens of times a day, finishing
// a task and capturing one, that reads as a log line, so the toast says what
// happened the way a person would instead. Change History keeps the ledger's
// words; only the toast is rephrased, and the `u undo` ending is unchanged.

// statusChangeHeadline phrases a status change that finished work: the done
// glyph, the title, and the day's tally the pulse already counts. Any other
// destination keeps the ledger's own sentence, which is what an empty answer
// means.
func (m *AppModel) statusChangeHeadline(action, context string) string {
	done := models.WorkflowStateGroupForState(models.StateDone).Label()
	var headline string
	switch action {
	case app.StateChangeUndoAction:
		title, lane, ok := app.ParseStateChangeUndoContext(context)
		if !ok || lane != done || strings.TrimSpace(title) == "" {
			return ""
		}
		headline = icons.Get("status.done") + " Done: " + title
	case app.BulkStateChangeUndoAction:
		if strings.TrimSpace(context) != done {
			return ""
		}
		headline = icons.Get("status.done") + " Done"
	default:
		return ""
	}
	if count := m.workspacePulse.Done; count > 0 {
		headline += " · " + doneTodayTally(count)
	}
	return headline
}

// doneTodayTally is the day's completion count as the toast spells it.
func doneTodayTally(count int) string {
	if count == 1 {
		return "first done today"
	}
	return strconv.Itoa(count) + " done today"
}

// captureHeadline names where a capture landed before what it was, because a
// toast wider than its row keeps its start: the destination is what a capture
// most often leaves the screen with, and the title is already on the row the
// cursor landed on. A multiline capture names its count and where it went.
func captureHeadline(titles []string, projects []*models.Project, lane string) string {
	switch {
	case len(titles) == 0:
		return ""
	case len(projects) > 1:
		return "Captured " + util.Count(len(titles), "task") + " across " + util.Count(len(projects), "project")
	}
	where := ""
	if len(projects) == 1 && projects[0] != nil {
		where = cmpNonEmpty(projects[0].Slug, projects[0].Name)
	}
	if len(titles) > 1 {
		if where == "" {
			return "Captured " + util.Count(len(titles), "task")
		}
		return "Captured " + util.Count(len(titles), "task") + " to " + where
	}
	destination := strings.Join(nonEmpty(where, lane), " · ")
	if destination == "" {
		return "Captured: " + titles[0]
	}
	return "Captured to " + destination + ": " + titles[0]
}

func cmpNonEmpty(values ...string) string {
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return ""
}

func nonEmpty(values ...string) []string {
	kept := make([]string, 0, len(values))
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			kept = append(kept, value)
		}
	}
	return kept
}
