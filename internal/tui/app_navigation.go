package tui

import (
	"image/color"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
)

// GetOperationRegistry returns the authoritative operation catalog.
func (m *AppModel) GetOperationRegistry() *OperationRegistry {
	return m.operationRegistry
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
	var viewType PresentationID
	switch cv {
	case ViewNotes:
		viewType = ViewNotes
	case ViewGlobalNotes:
		viewType = ViewGlobalNotes
	default:
		// Check if either renderer is present in the derived navigation path.
		path := m.presentationPath()
		for i := len(path) - 1; i >= 0; i-- {
			if path[i] == ViewNotes || path[i] == ViewGlobalNotes {
				viewType = path[i]
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
// stale pointer and resets to the global Outliner root with a status message.
// Returns handled=true with every command needed to present and load the
// fallback Outliner.
func (m *AppModel) validateCurrentProject() (handled bool, cmd tea.Cmd) {
	if m.currentProject == nil {
		return false, nil
	}
	if m.projectByID(m.currentProject.ID) != nil {
		return false, nil
	}
	m.exitAskView()
	m.currentProject = nil
	presentationCmd := m.prepareCrossProjectOutliner()
	m.navigation.ResetRoot()
	routeCmd := m.activateTaskWorkspaceRoute(outlinerRootRoute())
	cmd = tea.Batch(presentationCmd, routeCmd)
	m.setInfoStatus("Project was removed — returning to the Outliner")
	return true, cmd
}

// currentView returns the current view type.
func (m *AppModel) currentView() PresentationID {
	return m.currentPresentationView()
}

// baseProjectView walks the view stack from the top to find the underlying
// project-level Outliner beneath transient overlays such as Finder. This is
// needed by operation handlers that execute while an overlay is current but
// still need the owning task-workspace renderer.
func (m *AppModel) baseProjectView() PresentationID {
	path := m.presentationPath()
	for i := len(path) - 1; i >= 0; i-- {
		switch path[i] {
		case ViewCrossProjectOutliner:
			return path[i]
		default:
		}
	}
	return ViewCrossProjectOutliner
}

// pushView installs a renderer on the authoritative route or overlay stack.
func (m *AppModel) pushView(view PresentationID) {
	if m.installTypedPresentation(view) {
		m.installPendingStartupRoute(view)
	}
}

// popView removes the top view from the stack. It returns any commands needed
// by the newly revealed view, including eager reload and resume hooks.
func (m *AppModel) popView() tea.Cmd {
	hadOverlay := m.navigation.Overlays.Active()
	hadPrimaryChild := !hadOverlay && m.navigation.Primary.Depth() > 1
	popped := m.popTypedPresentation()
	cv := m.currentView()
	// Re-evaluate alerts when a primary child returns to the Outliner root.
	// Closing a transient overlay must not reload project data: picker models
	// commit or reconcile their mutations after the overlay has been dismissed.
	if hadPrimaryChild && m.navigation.Primary.Depth() == 1 && !m.navigation.Overlays.Active() {
		m.reevaluateAlerts()
	}
	// Update accent and resume the now-top view.
	var resumeCmd tea.Cmd
	if v := m.views.Get(cv); v != nil {
		m.setAccentOnView(v)
		if handler, ok := v.(ViewResumeHandler); popped && ok {
			resumeCmd = handler.OnResume()
		}
	}
	// Notify the cross-project outliner that it has become visible again.
	if cv == ViewCrossProjectOutliner {
		return tea.Batch(func() tea.Msg { return ViewDidAppearMsg{} }, resumeCmd)
	}
	return resumeCmd
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
	if caps := m.currentViewCapabilities(); caps != nil && caps.ShouldCaptureEsc() {
		return false, nil
	}
	// The Outliner is the sole root: a clean root Esc has no child or overlay to
	// unwind and must fall through to the view. There is something to close only
	// when a primary child (Task Detail, secondary tool) is stacked above the
	// root or a transient overlay is open.
	primaryCanClose := m.navigation.Primary.Depth() > 1 || m.navigation.Overlays.Active()
	if primaryCanClose {
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
		cmd := m.popView()
		if closingLabel != "" && closingView != ViewFinder {
			m.setInfoStatus("Closed " + closingLabel)
			return true, tea.Batch(cmd, defaultStatusTimeout())
		}
		return true, cmd
	}
	return false, nil
}

// jumpToOutlinerRoot is the preemptive Ctrl+Esc reset. It bypasses view-local
// Esc ownership so it remains a reliable global escape hatch: it closes every
// overlay, editor, child, and preview, clears project scope, search, filters,
// task zoom, selections, and transient modes, then activates
// the clean global All Outliner with the cursor safely reset.
func (m *AppModel) jumpToOutlinerRoot() tea.Cmd {
	path := m.presentationPath()
	for i := len(path) - 1; i >= 0; i-- {
		viewType := path[i]
		if view := m.views.Get(viewType); view != nil {
			if resetter, ok := view.(ViewExitResetter); ok {
				resetter.OnExit()
			}
		}
		m.saveStateForClosingView(viewType)
	}

	m.clearTransientNavigationState()
	// Dismiss the primary children (Task Detail, secondary tools) that live
	// outside the view registry so the reset lands on a bare Outliner root.
	m.taskDetail = nil
	m.taskComposer = nil

	presentationCmd := m.prepareCrossProjectOutliner()
	m.navigation.ResetRoot()
	if cmd := m.activateTaskWorkspaceRoute(outlinerRootRoute()); cmd != nil {
		// A reset never produces an invalid root route; surface any error.
		return cmd
	}
	m.projectStack = nil
	if m.crossOutliner != nil {
		m.crossOutliner.resetToGlobalAll()
		m.setAccentOnView(m.crossOutliner)
	}
	m.reevaluateAlerts()
	return presentationCmd
}

func (m *AppModel) saveStateForClosingView(viewType PresentationID) {
	switch viewType {
	case ViewCrossProjectOutliner:
		m.saveOutlinerFilterState()
	default:
		// Other views do not keep close-time navigation state.
	}
}

func (m *AppModel) clearTransientNavigationState() {
	m.dismissPeek()
	m.pendingChord = nil
	m.pendingSandboxReset = false
	m.pendingSandboxProjectID = ""
	m.pendingSandboxTaskCount = 0
	m.clearPendingProjectArchive()
	m.clearPendingTaskDelete()

	m.Overlays().ClearAll()

	m.pendingBulkTasks = nil
	m.pendingBulkSectionTasks = nil
	m.pendingBulkSectionProject = nil
	m.pendingBulkNotes = nil
	m.pendingGlobalNoteMove = nil
	m.pendingCreateTaskParent = nil
	m.pendingCreateTaskProject = nil
	m.pendingVarFillerTemplate = nil
	m.pendingDescEdit = nil
	m.pendingParkNav = nil

}

// popToAnchorView dismisses the current overlay or primary child back toward
// the Outliner root.
// Returns a tea.Cmd when popping reveals ViewCrossProjectOutliner so the
// outliner can trigger an eager reload (mirrors popView's behavior).
func (m *AppModel) popToAnchorView() tea.Cmd {
	return m.popView()
}

// currentAccentColor returns the accent color for the topmost mapped view in
// the stack, falling back to ColorPrimary when no mapping is found.
func (m *AppModel) currentAccentColor() color.Color {
	path := m.presentationPath()
	for i := len(path) - 1; i >= 0; i-- {
		if c := ViewAccentColor(path[i]); c != nil {
			return c
		}
	}
	if c := m.navigation.Primary.Current().Accent(); c != nil {
		return c
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
// bulkWireable is implemented by every view that routes its multiselect
// mutations through the application-layer BulkCoordinator. The shell wires them
// all from the single showView/replaceView site so no construction path can
// forget to inject the coordinator (a nil mutator would panic in a bulk handler).
type bulkWireable interface {
	SetBulkCoordinator(BulkCoordinator)
}

// taskItemWireable is implemented by every view that routes its single-task
// mutations through the application-layer TaskItemCoordinator, parallel to
// bulkWireable. The shell wires them all from the single showView/replaceView
// site so no construction path forgets to inject the coordinator.
type taskItemWireable interface {
	SetTaskItemCoordinator(TaskItemCoordinator)
}

// linkWireable is implemented by every view that routes its task-link mutations
// through the application-layer LinkCoordinator, parallel to bulkWireable. The
// shell wires them all from the single showView/replaceView site so no
// construction path forgets to inject the coordinator.
type linkWireable interface {
	SetLinkCoordinator(LinkCoordinator)
}

// captureWireable is implemented by every capture-creation view that routes its
// task creation through the application-layer CaptureCoordinator. The shell wires
// them from the single showView/replaceView site so no construction path can
// forget to inject the coordinator (a nil mutator would panic in a create
// handler).
type captureWireable interface {
	SetCaptureCoordinator(CaptureCoordinator)
}

// taskFormWireable is implemented by the create/edit task form, which routes its
// create and edit mutations through the application-layer TaskFormCoordinator.
type taskFormWireable interface {
	SetTaskFormCoordinator(TaskFormCoordinator)
}

// wireSecondaryCoordinators injects the shell's shared coordinators into a view
// as it is registered, so a view's mutation handlers can commit through the
// application layer without every construction site threading the coordinator.
func (m *AppModel) wireSecondaryCoordinators(view View) {
	if w, ok := view.(bulkWireable); ok {
		w.SetBulkCoordinator(m.secondaryFeatures.Bulk)
	}
	if w, ok := view.(taskItemWireable); ok {
		w.SetTaskItemCoordinator(m.secondaryFeatures.TaskItem)
	}
	if w, ok := view.(linkWireable); ok {
		w.SetLinkCoordinator(m.secondaryFeatures.Link)
	}
	if w, ok := view.(captureWireable); ok {
		w.SetCaptureCoordinator(m.features.Capture)
	}
	if w, ok := view.(taskFormWireable); ok {
		w.SetTaskFormCoordinator(m.features.TaskForm)
	}
	if outliner, ok := view.(*OutlinerModel); ok {
		m.wireOutlinerOperationExecutor(outliner)
	}
}

func (m *AppModel) wireOutlinerOperationExecutor(outliner *OutlinerModel) {
	if outliner == nil {
		return
	}
	outliner.operationRegistry = m.operationRegistry
}

func (m *AppModel) showView(viewType PresentationID, view View) {
	// Ask stream messages are routed only to the current view. Cancel an Ask
	// request before covering that model, or before replacing the registry's
	// existing Ask instance, so its provider cannot outlive the view lifecycle.
	if m.currentView() == ViewAsk || viewType == ViewAsk {
		m.exitAskView()
	}
	// All views (including full-height ones) are sized to height-2. Register
	// re-applies the registry's stored height-2, so a full-height SetSize here
	// would be immediately clobbered anyway; this also matches handleWindowSize.
	view.SetSize(m.width, m.height-2)
	m.wireSecondaryCoordinators(view)
	m.views.Register(viewType, view)
	if viewType == ViewCrossProjectOutliner {
		if outliner, ok := view.(*OutlinerModel); ok {
			m.crossOutliner = outliner
		}
	}
	m.pushView(viewType)
	m.setAccentOnView(view)
}

// showViewInit is showView + return true, view.Init(). Used by handlers that
// create a view, push it, and immediately return its Init command.
func (m *AppModel) showViewInit(viewType PresentationID, view View) (bool, tea.Cmd) {
	m.showView(viewType, view)
	return true, view.Init()
}

func (m *AppModel) exitAskView() {
	if m.views == nil {
		return
	}
	if ask, ok := m.views.Get(ViewAsk).(*AskViewModel); ok {
		ask.OnExit()
	}
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
	target := TaskIdentity{}
	if task != nil && project != nil {
		target = taskIdentityFor(task, project)
	}
	cmd, _ := m.maybePromptPark(target, func() tea.Cmd {
		return m.navigateToTaskInProjectNow(task, project)
	})
	return cmd
}

func (m *AppModel) navigateToTaskInProjectNow(task *models.Task, project *models.Project) tea.Cmd {
	m.switchToProject(project, true)

	// Re-resolve task in the freshly loaded project so parent pointers are valid.
	if task != nil {
		if resolved := project.GetTaskByID(task.ID); resolved != nil {
			task = resolved
		}
	}
	// Task deep links push a project scope onto the single authoritative Outliner
	// and reveal the task, rather than constructing a separate project root.
	return m.enterProjectScopeOutliner(project, task, OutlinerOpenOptions{})
}
