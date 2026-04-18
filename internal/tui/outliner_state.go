package tui

import (
	"fmt"
	"maps"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
)

// refreshView rebuilds the outliner rows.
func (m *OutlinerModel) refreshView() {
	m.rebuildRows()
}

// saveAndRefreshBulk saves the project with workflow rules and rebuilds rows.
func (m *OutlinerModel) saveAndRefreshBulk(prevStates map[string]*models.Task) tea.Cmd {
	opts := m.buildWorkflowOpts(prevStates)
	return func() tea.Msg {
		if err := persistWithWorkflow(opts); err != nil {
			return ErrorMsg{Err: err}
		}
		return outlinerRefreshMsg{}
	}
}

// setStatusMessage sets the status bar text.
func (m *OutlinerModel) setStatusMessage(msg string) {
	m.setStatus(msg)
}

// buildWorkflowOpts constructs a WorkflowSaveOptions from the current model
// state, draining the pending undo context.
func (m *OutlinerModel) buildWorkflowOpts(prevStates map[string]*models.Task) WorkflowSaveOptions {
	opts := BuildWorkflowOpts(m.projectService, m.project, m.config, m.configManager, prevStates, m.undo.Action, m.undo.Context, m.undo.TaskIDs)
	m.undo.Clear()
	return opts
}

// workflowPersistFn returns a persistFn that runs the full save pipeline:
// auto-unblock, recurring, workflow rules, undo snapshots.
func (m *OutlinerModel) workflowPersistFn() persistFn {
	return newWorkflowPersistFn(m)
}

// saveAndRefreshWithPrevStates saves the project with the workflow pipeline
// and rebuilds rows. This is the outliner's equivalent of ProjectTree's method.
func (m *OutlinerModel) saveAndRefreshWithPrevStates(prevStates map[string]*models.Task) error {
	opts := m.buildWorkflowOpts(prevStates)
	if err := persistWithWorkflow(opts); err != nil {
		return err
	}
	m.rebuildRows()
	return nil
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
	return performUndoCmd(m.projectService, m.project)
}

// performRedo restores the project from the most recent redo snapshot.
func (m *OutlinerModel) performRedo() tea.Cmd {
	return performRedoCmd(m.projectService, m.project)
}

// OutlinerToggleState captures section/fold state so it can survive outliner
// recreation within the same TUI session.
type OutlinerToggleState struct {
	CollapsedSections map[string]bool // Section collapse state for All perspective
	ExpandedNodes     map[string]bool // Node fold state (empty = all expanded)
	CursorTaskID      string          // Task ID at cursor position (session-only)
	ZoomStack         []ZoomEntry     // Zoom stack (session-only)
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

// copyBoolMap returns a shallow copy of a map[string]bool.
func copyBoolMap(src map[string]bool) map[string]bool {
	if src == nil {
		return nil
	}
	dst := make(map[string]bool, len(src))
	maps.Copy(dst, src)
	return dst
}

// outlinerRefreshMsg signals that the outliner should refresh its rows.
type outlinerRefreshMsg struct{}

// saveWithWorkflow persists the project using the full workflow pipeline
// and rebuilds rows on success.
func (m *OutlinerModel) saveWithWorkflow() tea.Cmd {
	opts := m.buildWorkflowOpts(nil)
	return func() tea.Msg {
		if err := persistWithWorkflow(opts); err != nil {
			return ErrorMsg{Err: err}
		}
		return outlinerRefreshMsg{}
	}
}

// pasteFromClipboard reads the clipboard, creates a task in the project's
// Inbox, registers an undo snapshot, and saves.
func (m *OutlinerModel) pasteFromClipboard() tea.Cmd {
	task, err := newInboxTaskFromClipboard(m.project)
	if err != nil {
		return func() tea.Msg { return StatusMsg("Clipboard is empty") }
	}
	title := task.Title
	if len([]rune(title)) > 40 {
		title = string([]rune(title)[:37]) + "..."
	}
	m.setUndoContext("Paste Task", task.Title)
	m.undo.TaskIDs = []string{task.ID}
	m.setStatus(fmt.Sprintf("Created \"%s\" in Inbox", title))
	return m.saveWithWorkflow()
}

// archiveProject archives or unarchives the project and saves config.
func (m *OutlinerModel) archiveProject() tea.Cmd {
	if m.project.Archived {
		m.project.Unarchive()
	} else {
		m.project.Archive()
		if m.config.CurrentProject == m.project.ID {
			m.config.CurrentProject = ""
		}
	}

	mgr := m.configManager
	cfg := m.config
	project := m.project
	return func() tea.Msg {
		if err := mgr.Save(cfg); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to save: %w", err)}
		}
		return ProjectArchivedMsg{Project: project}
	}
}
