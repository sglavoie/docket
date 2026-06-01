package tui

import (
	"image/color"

	tea "charm.land/bubbletea/v2"

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

// validateCurrentProject checks that the current project still exists in config.
// If the project was deleted (e.g., by an external CLI command), it clears the
// stale pointer and pops back to the dashboard with a status message.
// Returns true if the project was invalidated.
func (m *AppModel) validateCurrentProject() bool {
	if m.currentProject == nil {
		return false
	}
	if m.config.GetProject(m.currentProject.ID) != nil {
		return false
	}
	m.currentProject = nil
	m.viewStack = m.viewStack[:1]
	m.setInfoStatus("Project was removed — returning to dashboard")
	if m.dashboard != nil {
		m.dashboard.Refresh(m.config)
	}
	return true
}

// currentView returns the current view type.
func (m *AppModel) currentView() ViewType {
	if len(m.viewStack) == 0 {
		return ViewDashboard
	}
	return m.viewStack[len(m.viewStack)-1]
}

// baseProjectView walks the view stack from the top to find the underlying
// project-level view (ProjectTree, Outliner, or CrossProjectOutliner), skipping overlays
// like the command palette. This is needed by command handlers that run inside
// the palette's Cmd — at that point currentView() returns ViewCommandPalette,
// but the handler needs to know which project view it was invoked from.
func (m *AppModel) baseProjectView() ViewType {
	for i := len(m.viewStack) - 1; i >= 0; i-- {
		switch m.viewStack[i] {
		case ViewProjectTree, ViewOutliner, ViewCrossProjectOutliner:
			return m.viewStack[i]
		default:
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
		if curr, ok := m.navHistory.Current(); ok {
			m.lastNavEntry = curr
		}
		entry := NavEntry{ViewType: view}
		if m.currentProject != nil && !isGlobalNavView(view) {
			entry.ProjectID = m.currentProject.ID
		}
		if isTaskScopedView(view) {
			entry.TaskID = m.lastNavigatedTaskID
		}
		m.navHistory.Record(entry)
	}
}

// popView removes the top view from the stack. It returns a Cmd when popping
// TO ViewCrossProjectOutliner so the outliner can trigger an eager reload.
func (m *AppModel) popView() tea.Cmd {
	if len(m.viewStack) > 1 {
		m.viewStack = m.viewStack[:len(m.viewStack)-1]
	}
	cv := m.currentView()
	// Re-evaluate alerts when returning to dashboard
	if cv == ViewDashboard {
		m.reevaluateAlerts()
	}
	// Update accent on the now-top view.
	if v := m.views.Get(cv); v != nil {
		m.setAccentOnView(v)
	}
	// Notify the cross-project outliner that it has become visible again.
	if cv == ViewCrossProjectOutliner {
		return func() tea.Msg { return ViewDidAppearMsg{} }
	}
	return nil
}

// handleEscKey handles the escape key for view navigation.
// Returns (handled, cmd): handled=true if the key was consumed (should not
// propagate to views); cmd is non-nil when the pop triggers a side-effect
// (e.g. ViewDidAppearMsg for the cross-project outliner).
func (m *AppModel) handleEscKey() (bool, tea.Cmd) {
	if m.peekActive {
		m.dismissPeek()
		return true, nil
	}
	if m.pendingSemicolonPrefix {
		m.pendingSemicolonPrefix = false
		return true, nil
	}
	if m.pendingSemicolonTPrefix {
		m.pendingSemicolonTPrefix = false
		return true, nil
	}
	if caps := m.currentViewCapabilities(); caps != nil && caps.ShouldCaptureEsc() {
		return false, nil
	}
	if m.currentView() != ViewDashboard {
		closingView := m.currentView()
		// Capture the view label for the Esc undo hint before any state changes.
		var closingLabel string
		if view := m.views.Get(closingView); view != nil {
			if resetter, ok := view.(ViewExitResetter); ok {
				resetter.OnExit()
			}
			if labeler, ok := view.(StatusBarLabeler); ok {
				closingLabel = labeler.StatusBarLabel()
			}
		}
		// Save outliner filter state so it persists across navigation.
		if closingView == ViewOutliner {
			m.saveOutlinerToggleState()
			m.saveOutlinerFilterState()
		}
		// Save kanban view state (mode, cursor, scroll) so it persists across navigation.
		if closingView == ViewKanban || closingView == ViewCrossProjectKanban {
			m.saveKanbanState()
		}
		// Save temporary outliner's filter state so subsequent temporary
		// outliners for the same project inherit any filter changes.
		if closingView == ViewTemporaryOutliner {
			if tempOutliner, ok := m.views.Get(ViewTemporaryOutliner).(*OutlinerModel); ok {
				m.saveOutlinerFilterStateFrom(tempOutliner)
			}
		}
		var cmd tea.Cmd
		if isGlobalOverlayView(closingView) {
			cmd = m.popToAnchorView()
		} else {
			cmd = m.popView()
		}
		// Rebuild outliner rows when returning from cross-outliner so the
		// display reflects the current in-memory filter state (e.g. after Fx).
		if closingView == ViewCrossProjectOutliner && m.outliner != nil {
			m.outliner.rebuildRows()
		}
		// Restore previous project when popping a stacked project tree
		statusAlreadySet := false
		if closingView == ViewProjectTree && m.currentView() == ViewProjectTree && len(m.projectStack) > 0 {
			prevID := m.projectStack[len(m.projectStack)-1]
			m.projectStack = m.projectStack[:len(m.projectStack)-1]
			if prev := m.config.GetProject(prevID); prev != nil {
				m.switchToProject(prev, true)
			} else {
				m.setInfoStatus("Previous project no longer exists")
				statusAlreadySet = true
			}
		}
		if !statusAlreadySet && closingLabel != "" && closingView != ViewCommandPalette {
			m.setInfoStatus("Closed " + closingLabel + " — Alt+O to reopen")
			return true, tea.Batch(cmd, defaultStatusTimeout())
		}
		return true, cmd
	}
	return false, nil
}

// isGlobalOverlayView returns true for views launched via the global ";" overlay prefix.
// These views can be opened from any context and should return to an anchor view when closed.
func isGlobalOverlayView(v ViewType) bool {
	switch v {
	case ViewFocusView, ViewFocusBoard, ViewGlobalInboxProcessor,
		ViewGlobalNotes, ViewRecentActivity, ViewGlobalBlockers,
		ViewMetricsDashboard, ViewAllTasks, ViewWorkflowRules,
		ViewPipelineHub, ViewRoadmap:
		return true
	default:
	}
	return false
}

// popToAnchorView pops the view stack until the top is a project-level view
// (ProjectTree, Outliner) or the Dashboard, or the stack has only one entry.
// Returns a tea.Cmd when popping reveals ViewCrossProjectOutliner so the
// outliner can trigger an eager reload (mirrors popView's behavior).
func (m *AppModel) popToAnchorView() tea.Cmd {
	for len(m.viewStack) > 1 {
		top := m.viewStack[len(m.viewStack)-1]
		switch top {
		case ViewDashboard, ViewProjectTree, ViewOutliner, ViewCrossProjectOutliner:
			if top == ViewDashboard {
				m.reevaluateAlerts()
			}
			if v := m.views.Get(top); v != nil {
				m.setAccentOnView(v)
			}
			if top == ViewCrossProjectOutliner {
				return func() tea.Msg { return ViewDidAppearMsg{} }
			}
			return nil
		default:
		}
		m.viewStack = m.viewStack[:len(m.viewStack)-1]
	}
	// Stack has 1 entry — re-evaluate if it's the dashboard
	cv := m.currentView()
	if cv == ViewDashboard {
		m.reevaluateAlerts()
	}
	if v := m.views.Get(cv); v != nil {
		m.setAccentOnView(v)
	}
	if cv == ViewCrossProjectOutliner {
		return func() tea.Msg { return ViewDidAppearMsg{} }
	}
	return nil
}

// currentAccentColor returns the accent color for the topmost mapped view in
// the stack, falling back to ColorPrimary when no mapping is found.
func (m *AppModel) currentAccentColor() color.Color {
	for i := len(m.viewStack) - 1; i >= 0; i-- {
		if c := ViewAccentColor(m.viewStack[i]); c != nil {
			return c
		}
	}
	return ColorPrimary
}

// setAccentOnView injects the accent color for the current view stack top into
// any view that implements AccentColorSetter.
func (m *AppModel) setAccentOnView(view View) {
	if v, ok := view.(AccentColorSetter); ok {
		v.SetAccentColor(m.currentAccentColor())
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
	m.setAccentOnView(view)
}

// showViewInit is showView + return true, view.Init(). Used by handlers that
// create a view, push it, and immediately return its Init command.
func (m *AppModel) showViewInit(viewType ViewType, view View) (bool, tea.Cmd) {
	m.showView(viewType, view)
	return true, view.Init()
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
		if curr, ok := m.navHistory.Current(); ok {
			m.lastNavEntry = curr
		}
		entry := NavEntry{ViewType: viewType}
		if m.currentProject != nil && !isGlobalNavView(viewType) {
			entry.ProjectID = m.currentProject.ID
		}
		if isTaskScopedView(viewType) {
			entry.TaskID = m.lastNavigatedTaskID
		}
		m.navHistory.Record(entry)
	}
	m.setAccentOnView(view)
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

const maxProjectLastViewEntries = 50

// saveCurrentProjectView records the current project-level view for the active project.
func (m *AppModel) saveCurrentProjectView() {
	if m.currentProject == nil {
		return
	}
	cv := m.currentView()
	if cv == ViewProjectTree || cv == ViewOutliner {
		m.projectLastView[m.currentProject.ID] = cv
		// Evict oldest entries when over cap (simple eviction: delete arbitrary entries)
		for len(m.projectLastView) > maxProjectLastViewEntries {
			for id := range m.projectLastView {
				if id != m.currentProject.ID {
					delete(m.projectLastView, id)
					break
				}
			}
		}
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

// popAndNavigateToTask pops the current view and navigates to a task in its project.
// If either task or project is nil, it just pops and returns. This encapsulates the
// common "select from overview" pattern used by many *SelectMsg handlers.
func (m *AppModel) popAndNavigateToTask(task *models.Task, project *models.Project) (bool, tea.Cmd) {
	popCmd := m.popView()
	if task != nil && project != nil {
		return true, m.navigateToTaskInProject(task, project)
	}
	return true, popCmd
}

// navigateToTaskInProject switches to the given project using the user's
// configured default view (or last-used view) and positions the cursor on
// the specified task. When task is nil it just opens the project view.
// Returns a tea.Cmd if any notification needs to be shown (e.g. filters cleared).
func (m *AppModel) navigateToTaskInProject(task *models.Task, project *models.Project) tea.Cmd {
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
		var navCmd tea.Cmd
		if task != nil {
			navCmd = m.projectTree.NavigateToTask(task)
		}
		m.showView(ViewProjectTree, m.projectTree)
		return navCmd
	default:
		m.saveOutlinerToggleState()
		m.saveOutlinerFilterState()
		m.outliner = NewOutlinerModel(project, m.config, m.configManager)
		m.outliner.queueIDs = m.focusQueueIDs
		m.outliner.dailyPlanIDs = m.dailyPlanIDs
		m.restoreOutlinerToggleState()
		m.restoreOutlinerFilterState()
		if task != nil {
			m.outliner.NavigateToSidebarTask(task)
		}
		m.showView(ViewOutliner, m.outliner)
		return nil
	}
}
