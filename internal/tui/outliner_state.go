package tui

import (
	"fmt"
	"maps"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/workflow"
)

// saveAndRefreshBulk saves the project with workflow rules and rebuilds rows.
func (m *OutlinerModel) saveAndRefreshBulk(prevStates map[string]*models.Task) tea.Cmd {
	return m.taskItem.CommitProjectWorkflow(m.project, m.commitWorkflowOpts(prevStates), "")
}

// setStatusMessage sets the status bar text.
func (m *OutlinerModel) setStatusMessage(msg string) {
	m.setStatus(msg)
}

// commitWorkflowOpts constructs the project-scoped workflow options for the
// application-layer commit, draining the pending undo context. Workflow rules come
// from config (nil-guarded for isolated tests); callers may supply an already-
// captured snapshot or let the service infer it from storage.
func (m *OutlinerModel) commitWorkflowOpts(prevStates map[string]*models.Task) app.ProjectWorkflowOptions {
	var rules []workflow.WorkflowRule
	if m.runtime != nil {
		rules = m.runtime.Automation().GetEnabledWorkflowRules()
	}
	action, context := m.undo.Action, m.undo.Context
	// The pending undo TaskIDs name the tasks this mutation targeted; forward them
	// as the planning-reconcile set so a whole-project commit that changed a task's
	// workflow state (quick edit, section move, paste) folds Today/Next exclusivity
	// into the same transaction. A task whose state did not change reconciles to no
	// planning artifact, so this is harmless for structural-only mutations.
	planningTaskIDs := append([]string(nil), m.undo.TaskIDs...)
	m.undo.Clear()
	return app.ProjectWorkflowOptions{
		Rules:           rules,
		PrevStates:      prevStates,
		UndoAction:      action,
		UndoContext:     context,
		PlanningTaskIDs: planningTaskIDs,
	}
}

func (m *OutlinerModel) singleWorkflowOpts(task *models.Task, action string) app.SingleTaskWorkflowOptions {
	var rules []workflow.WorkflowRule
	if m.runtime != nil {
		rules = m.runtime.Automation().GetEnabledWorkflowRules()
	}
	context := ""
	if task != nil {
		context = fmt.Sprintf("#%d %s", task.ShortID, task.Title)
	}
	return app.SingleTaskWorkflowOptions{Rules: rules, UndoAction: action, UndoContext: context}
}

// singleTaskCoordinator exposes the application-layer single-task coordinator so
// the shared primary-view helpers route single-task mutations through the
// coordinator (bulk paths remain on saveAndRefreshBulk).
func (m *OutlinerModel) singleTaskCoordinator() TaskItemCoordinator {
	return m.taskItem
}

// commitWorkflow commits the project's current in-memory state (already mutated by
// the caller) through the application layer with the workflow cascade, labeling
// the ledger entry from the pending undo context and setting summary as the
// status-line text. It is the single-project bulk-status/delete analog of
// saveAndRefreshBulk that carries an explicit success message.
func (m *OutlinerModel) commitWorkflow(prevStates map[string]*models.Task, summary string) tea.Cmd {
	return m.taskItem.CommitProjectWorkflow(m.project, m.commitWorkflowOpts(prevStates), summary)
}

// setUndoContext sets the action context for the next save operation.
func (m *OutlinerModel) setUndoContext(action, context string) {
	m.undo.Set(action, context)
}

// setUndoTaskIDs sets the targeted task IDs for the next undo snapshot.
func (m *OutlinerModel) setUndoTaskIDs(ids []string) {
	m.undo.TaskIDs = ids
}

// performUndo restores the project from the most recent undo snapshot.
func (m *OutlinerModel) performUndo() tea.Cmd {
	return performUndoForProjectsCmd(m.projectService, m.project, activeUndoProjectsFromRuntime(m.runtime), repairRuntimeTaskRefsAfterRestore(m.runtime))
}

// performRedo restores the project from the most recent redo snapshot.
func (m *OutlinerModel) performRedo() tea.Cmd {
	return performRedoForProjectsCmd(m.projectService, m.project, activeUndoProjectsFromRuntime(m.runtime), repairRuntimeTaskRefsAfterRestore(m.runtime))
}

// OutlinerToggleState captures section/fold state so it can survive outliner
// recreation within the same TUI session.
type OutlinerToggleState struct {
	CollapsedSections map[string]bool       // Section collapse state for All perspective
	ExpandedNodes     map[TaskIdentity]bool // Project-qualified node fold state (empty = all expanded)
	CursorTaskID      string                // Task ID at cursor position (session-only)
	ZoomStack         []ZoomEntry           // Zoom stack (session-only)
}

// SaveToggleState snapshots the current section/fold state.
func (m *OutlinerModel) SaveToggleState() *OutlinerToggleState {
	var cursorTaskID string
	if task := m.getCursorTask(); task != nil {
		cursorTaskID = task.ID
	}

	var zoomCopy []ZoomEntry
	if len(m.zoomStack) > 0 {
		zoomCopy = make([]ZoomEntry, len(m.zoomStack))
		copy(zoomCopy, m.zoomStack)
	}

	return &OutlinerToggleState{
		CollapsedSections: copyBoolMap(m.collapsedSections),
		ExpandedNodes:     copyBoolMap(m.expandedNodes),
		CursorTaskID:      cursorTaskID,
		ZoomStack:         zoomCopy,
	}
}

// RestoreToggleState applies a previously saved section/fold state.
func (m *OutlinerModel) RestoreToggleState(state *OutlinerToggleState) {
	if state == nil {
		return
	}

	m.collapsedSections = copyBoolMap(state.CollapsedSections)
	m.expandedNodes = copyBoolMap(state.ExpandedNodes)
	if len(state.ZoomStack) > 0 {
		m.zoomStack = make([]ZoomEntry, len(state.ZoomStack))
		copy(m.zoomStack, state.ZoomStack)
	}
	m.rebuildRows()
	if state.CursorTaskID != "" {
		m.moveCursorToTask(state.CursorTaskID)
	}
}

// copyBoolMap returns a shallow copy of a bool map.
func copyBoolMap[K comparable](src map[K]bool) map[K]bool {
	if src == nil {
		return nil
	}
	dst := make(map[K]bool, len(src))
	maps.Copy(dst, src)
	return dst
}

// outlinerRefreshMsg signals that the outliner should refresh its rows.
type outlinerRefreshMsg struct{}

// saveWithWorkflow persists the project's current in-memory state through the
// application layer (workflow cascade + grouped ledger undo) and the shell reloads
// the rows on the resulting BatchTransactionResultMsg. The caller has already
// applied its structural mutation (swap/indent/outdent/section move/paste) to
// m.project in memory.
func (m *OutlinerModel) saveWithWorkflow() tea.Cmd {
	return m.taskItem.CommitProjectWorkflow(m.project, m.commitWorkflowOpts(nil), "")
}

// pasteFromClipboardInto reads the clipboard and creates a task in proj's Inbox.
// When proj is m.project the single-project workflow pipeline runs (an in-memory
// append + save). Otherwise (cross-project mode) the new task is created through
// the app-layer Capture seam, which commits one ledger transaction; the shell
// reloads and toasts. Cross-project paste stores a detected URL as the task's URL
// field rather than as a URL attachment (the single-project path still attaches),
// because the capture use case models URLs as a task field.
func (m *OutlinerModel) pasteFromClipboardInto(proj *models.Project) tea.Cmd {
	if proj == nil {
		return nil
	}

	if proj == m.project {
		task, err := newInboxTaskFromClipboard(proj)
		if err != nil {
			return func() tea.Msg { return StatusMsg("Clipboard is empty") }
		}
		title := task.Title
		if len([]rune(title)) > 40 {
			title = string([]rune(title)[:37]) + "..."
		}
		m.setStatus(fmt.Sprintf("Created \"%s\" in Inbox", title))
		m.setUndoContext("Paste Task", task.Title)
		m.undo.TaskIDs = []string{task.ID}
		return m.saveWithWorkflow()
	}

	pt, err := readPastedClipboard()
	if err != nil {
		return func() tea.Msg { return StatusMsg("Clipboard is empty") }
	}
	return m.taskItem.Capture(app.CaptureRequest{
		ProjectID:   proj.ID,
		Title:       pt.Title,
		Description: pt.Description,
		State:       models.StateInbox,
		URL:         pt.URL,
		Prepend:     true,
	})
}

// archiveProject requests a confirmed archive or unarchive for the project
// under m.project. Use archiveProjectFor when the target may differ from
// m.project (cross-project mode).
func (m *OutlinerModel) archiveProject() tea.Cmd {
	return m.archiveProjectFor(m.project)
}

// archiveProjectFor requests a confirmed archive or unarchive for proj.
func (m *OutlinerModel) archiveProjectFor(proj *models.Project) tea.Cmd {
	if proj == nil {
		return nil
	}
	return projectArchiveRequestCmd(proj, !proj.Archived)
}
