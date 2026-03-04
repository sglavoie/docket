package tui

import (
	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
)

// GetCommandRegistry returns the command registry for the application.
// This is used by the command palette and for testing.
func (m *AppModel) GetCommandRegistry() *CommandRegistry {
	return m.commandRegistry
}

// isNotesFullscreen returns true when the notes view is in fullscreen pane mode.
func (m *AppModel) isNotesFullscreen() bool {
	cv := m.currentView()
	if cv != ViewNotes && cv != ViewGlobalNotes {
		return false
	}
	if nv := m.getActiveNotesView(); nv != nil {
		return nv.fullscreenPane
	}
	return false
}

// getActiveNotesView returns the currently active NotesViewModel (project or global).
func (m *AppModel) getActiveNotesView() *NotesViewModel {
	cv := m.currentView()
	var viewType ViewType
	switch cv {
	case ViewNotes:
		viewType = ViewNotes
	case ViewGlobalNotes:
		viewType = ViewGlobalNotes
	default:
		// Check if either is on the stack
		for i := len(m.viewStack) - 1; i >= 0; i-- {
			if m.viewStack[i] == ViewNotes || m.viewStack[i] == ViewGlobalNotes {
				viewType = m.viewStack[i]
				break
			}
		}
		if viewType == 0 {
			return nil
		}
	}
	if v := m.views.Get(viewType); v != nil {
		if nv, ok := v.(*NotesViewModel); ok {
			return nv
		}
	}
	return nil
}

// currentView returns the current view type.
func (m *AppModel) currentView() ViewType {
	if len(m.viewStack) == 0 {
		return ViewDashboard
	}
	return m.viewStack[len(m.viewStack)-1]
}

// baseProjectView walks the view stack from the top to find the underlying
// project-level view (ProjectTree or Outliner), skipping overlays
// like the command palette. This is needed by command handlers that run inside
// the palette's Cmd — at that point currentView() returns ViewCommandPalette,
// but the handler needs to know which project view it was invoked from.
func (m *AppModel) baseProjectView() ViewType {
	for i := len(m.viewStack) - 1; i >= 0; i-- {
		switch m.viewStack[i] {
		case ViewProjectTree, ViewOutliner:
			return m.viewStack[i]
		}
	}
	return ViewDashboard
}

// pushView pushes a new view onto the stack and records it in navigation history
// if it is a tracked view and history is not locked.
func (m *AppModel) pushView(view ViewType) {
	m.viewStack = append(m.viewStack, view)
	if view == ViewProjectTree || view == ViewOutliner {
		m.lastProjectView = view
	}
	if !m.navHistory.IsLocked() && isTrackedView(view) {
		entry := NavEntry{ViewType: view}
		if m.currentProject != nil {
			entry.ProjectID = m.currentProject.ID
		}
		if isTaskScopedView(view) {
			entry.TaskID = m.lastNavigatedTaskID
		}
		m.navHistory.Record(entry)
	}
}

// popView removes the top view from the stack.
func (m *AppModel) popView() {
	if len(m.viewStack) > 1 {
		m.viewStack = m.viewStack[:len(m.viewStack)-1]
	}
	// Re-evaluate alerts when returning to dashboard
	if m.currentView() == ViewDashboard {
		m.reevaluateAlerts()
	}
}

// isGlobalOverlayView returns true for views launched via the global ";" overlay prefix.
// These views can be opened from any context and should return to an anchor view when closed.
func isGlobalOverlayView(v ViewType) bool {
	switch v {
	case ViewFocusView, ViewFocusBoard, ViewGlobalInboxProcessor,
		ViewGlobalNotes, ViewRecentActivity, ViewGlobalBlockers,
		ViewMetricsDashboard, ViewAllTasks:
		return true
	}
	return false
}

// popToAnchorView pops the view stack until the top is a project-level view
// (ProjectTree, Outliner) or the Dashboard, or the stack has only one entry.
func (m *AppModel) popToAnchorView() {
	for len(m.viewStack) > 1 {
		top := m.viewStack[len(m.viewStack)-1]
		switch top {
		case ViewDashboard, ViewProjectTree, ViewOutliner:
			if top == ViewDashboard {
				m.reevaluateAlerts()
			}
			return
		}
		m.viewStack = m.viewStack[:len(m.viewStack)-1]
	}
	// Stack has 1 entry — re-evaluate if it's the dashboard
	if m.currentView() == ViewDashboard {
		m.reevaluateAlerts()
	}
}

// showView is a helper that registers a view, sets its size, and pushes it onto the view stack.
// This consolidates the common pattern used by Show*Msg handlers.
func (m *AppModel) showView(viewType ViewType, view View) {
	if caps, ok := view.(ViewCapabilities); ok && caps.IsFullHeight() {
		view.SetSize(m.width, m.height)
	} else {
		view.SetSize(m.width, m.height-2)
	}
	m.views.Register(viewType, view)
	m.pushView(viewType)
}

// replaceView replaces the top of the view stack instead of pushing.
// Used when switching between peer project views (ProjectTree, Outliner)
// so the stack doesn't accumulate entries.
func (m *AppModel) replaceView(viewType ViewType, view View) {
	if caps, ok := view.(ViewCapabilities); ok && caps.IsFullHeight() {
		view.SetSize(m.width, m.height)
	} else {
		view.SetSize(m.width, m.height-2)
	}
	m.views.Register(viewType, view)
	if len(m.viewStack) > 1 {
		m.viewStack[len(m.viewStack)-1] = viewType
	} else {
		m.viewStack = append(m.viewStack, viewType)
	}
	if viewType == ViewProjectTree || viewType == ViewOutliner {
		m.lastProjectView = viewType
	}
	if !m.navHistory.IsLocked() && isTrackedView(viewType) {
		entry := NavEntry{ViewType: viewType}
		if m.currentProject != nil {
			entry.ProjectID = m.currentProject.ID
		}
		if isTaskScopedView(viewType) {
			entry.TaskID = m.lastNavigatedTaskID
		}
		m.navHistory.Record(entry)
	}
}

// isOnProjectView returns true if the current view is a peer project view.
func (m *AppModel) isOnProjectView() bool {
	cur := m.currentView()
	return cur == ViewProjectTree || cur == ViewOutliner
}

// defaultProjectViewType maps the config's default_project_view string to a ViewType.
func defaultProjectViewType(cfg *config.Config) ViewType {
	switch cfg.GetDefaultProjectView() {
	case "project_tree":
		return ViewProjectTree
	default:
		return ViewOutliner
	}
}

// saveCurrentProjectView records the current project-level view for the active project.
func (m *AppModel) saveCurrentProjectView() {
	if m.currentProject == nil {
		return
	}
	cv := m.currentView()
	if cv == ViewProjectTree || cv == ViewOutliner {
		m.projectLastView[m.currentProject.ID] = cv
	}
}

// getProjectLastView returns the last project-level view for the given project ID,
// falling back to the configured default project view.
func (m *AppModel) getProjectLastView(projectID string) ViewType {
	if v, ok := m.projectLastView[projectID]; ok {
		return v
	}
	return defaultProjectViewType(m.config)
}

// navigateToTaskInProject switches to the given project using the user's
// configured default view (or last-used view) and positions the cursor on
// the specified task. When task is nil it just opens the project view.
func (m *AppModel) navigateToTaskInProject(task *models.Task, project *models.Project) {
	m.saveCurrentProjectView()
	m.switchToProject(project, true)

	// Re-resolve task in the freshly loaded project so parent pointers are valid.
	if task != nil {
		if resolved := project.GetTaskByID(task.ID); resolved != nil {
			task = resolved
		}
	}

	targetView := m.getProjectLastView(project.ID)
	switch targetView {
	case ViewProjectTree:
		if task != nil {
			m.projectTree.NavigateToTask(task)
		}
		m.showView(ViewProjectTree, m.projectTree)
	default:
		m.saveOutlinerToggleState()
		m.saveOutlinerFilterState()
		m.outliner = NewOutlinerModel(project, m.config, m.configManager)
		m.restoreOutlinerToggleState()
		m.restoreOutlinerFilterState()
		if task != nil {
			m.outliner.NavigateToSidebarTask(task)
		}
		m.showView(ViewOutliner, m.outliner)
	}
}
