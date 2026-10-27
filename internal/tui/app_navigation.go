package tui

import (
	"image/color"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/docket/internal/tui/chrome"
	"github.com/sglavoie/docket/internal/tui/coordinator"
	"github.com/sglavoie/docket/internal/tui/notes"
	"github.com/sglavoie/docket/internal/tui/operation"
	"github.com/sglavoie/docket/internal/tui/outliner"
	"github.com/sglavoie/docket/internal/tui/presentation"
	"github.com/sglavoie/docket/internal/tui/route"
	"github.com/sglavoie/docket/internal/tui/status"
	"github.com/sglavoie/docket/internal/tui/taskdetail"
	"github.com/sglavoie/docket/internal/tui/theme"
)

// GetOperationRegistry returns the authoritative operation catalog.
func (m *AppModel) GetOperationRegistry() *operation.OperationRegistry {
	return m.operations.Registry
}

// getActiveNotesView returns the currently active NotesViewModel (project or global).
func (m *AppModel) getActiveNotesView() *notes.NotesViewModel {
	cv := m.currentView()
	var viewType presentation.PresentationID
	switch cv {
	case presentation.ViewNotes:
		viewType = presentation.ViewNotes
	case presentation.ViewGlobalNotes:
		viewType = presentation.ViewGlobalNotes
	default:
		// Check if either renderer is present in the derived navigation path.
		path := m.presentationPath()
		for i := len(path) - 1; i >= 0; i-- {
			if path[i] == presentation.ViewNotes || path[i] == presentation.ViewGlobalNotes {
				viewType = path[i]
				break
			}
		}
		if viewType == 0 {
			return nil
		}
	}
	if v := m.viewNav.views.Get(viewType); v != nil {
		if nv, ok := v.(*notes.NotesViewModel); ok {
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
//
// An open Comments composer defers that reset, because the reset is a route
// unwind: it drops the panel without the discard question the composer owes,
// and the buffer it drops is the only copy of what the user typed. Waiting
// costs nothing, since every comment mutation re-resolves its own owner and
// refuses a gone one, so a deferred reset can never let a write through; the
// next keystroke after the composer closes performs it.
func (m *AppModel) validateCurrentProject() (handled bool, cmd tea.Cmd) {
	if m.currentProject == nil {
		return false, nil
	}
	if m.projectByID(m.currentProject.ID) != nil {
		return false, nil
	}
	if m.commentComposerHoldsUnsavedText() {
		return false, nil
	}
	m.currentProject = nil
	presentationCmd := m.prepareCrossProjectOutliner()
	m.viewNav.navigation.ResetRoot()
	routeCmd := m.activateTaskWorkspaceRoute(route.OutlinerRootRoute())
	cmd = tea.Batch(presentationCmd, routeCmd)
	m.setInfoStatus("Project was removed — returning to the Outliner")
	return true, cmd
}

// commentComposerHoldsUnsavedText reports whether the view on screen is the
// Comments panel with an add or edit draft open. It names that one surface
// rather than asking every view whether it holds text: a form whose save could
// still reach storage must not outlive the project it was opened on, while this
// panel's every write is already refused by its own owner check.
func (m *AppModel) commentComposerHoldsUnsavedText() bool {
	if m.currentView() != presentation.ViewTaskComments {
		return false
	}
	panel, ok := m.activeView().(*taskdetail.CommentsModel)
	return ok && panel.HasTextInput()
}

// currentView returns the current view type.
func (m *AppModel) currentView() presentation.PresentationID {
	return m.currentPresentationView()
}

// baseProjectView walks the view stack from the top to find the underlying
// project-level Outliner beneath transient overlays. This is
// needed by operation handlers that execute while an overlay is current but
// still need the owning task-workspace renderer.
func (m *AppModel) baseProjectView() presentation.PresentationID {
	path := m.presentationPath()
	for i := len(path) - 1; i >= 0; i-- {
		switch path[i] {
		case presentation.ViewCrossProjectOutliner:
			return path[i]
		default:
		}
	}
	return presentation.ViewCrossProjectOutliner
}

// pushView installs a renderer on the authoritative route or overlay stack.
func (m *AppModel) pushView(view presentation.PresentationID) {
	m.installTypedPresentation(view)
}

// popView removes the top view from the stack. It returns any commands needed
// by the newly revealed view, including eager reload and resume hooks.
func (m *AppModel) popView() tea.Cmd {
	popped := m.popTypedPresentation()
	cv := m.currentView()
	// Closing a transient overlay must not reload project data: picker models
	// commit or reconcile their mutations after the overlay has been dismissed.
	// Update accent and resume the now-top view.
	var resumeCmd tea.Cmd
	if v := m.viewNav.views.Get(cv); v != nil {
		m.setAccentOnView(v)
		if handler, ok := v.(route.ViewResumeHandler); popped && ok {
			resumeCmd = handler.OnResume()
		}
	}
	// Notify the cross-project outliner that it has become visible again.
	if cv == presentation.ViewCrossProjectOutliner {
		return tea.Batch(func() tea.Msg { return status.ViewDidAppearMsg{} }, resumeCmd)
	}
	return resumeCmd
}

// handleEscKey handles the escape key for view navigation.
// Returns (handled, cmd): handled=true if the key was consumed (should not
// propagate to views); cmd is non-nil when the pop triggers a side-effect
// (e.g. ViewDidAppearMsg for the cross-project outliner).
func (m *AppModel) handleEscKey() (bool, tea.Cmd) {
	if caps := m.currentViewCapabilities(); caps != nil && caps.ShouldCaptureEsc() {
		return false, nil
	}
	// The Outliner is the sole root: a clean root Esc has no child or overlay to
	// unwind and must fall through to the view. There is something to close only
	// when a primary secondary-tool child is stacked above the
	// root or a transient overlay is open.
	if !m.viewNav.atRoot() {
		// Leaving is not an outcome worth a status line: the view underneath is
		// what the press reveals, and announcing "Closed …" would replace the
		// feedback the closed view left behind — an undo offer included.
		if view := m.viewNav.views.Get(m.currentView()); view != nil {
			if resetter, ok := view.(route.ViewExitResetter); ok {
				resetter.OnExit()
			}
		}
		return true, m.popView()
	}
	return false, nil
}

func (m *AppModel) clearTransientNavigationState() {
	m.operations.Chord = nil
	m.project.ClearArchive()
	m.taskMutation.clearDelete()

	m.Overlays().ClearAll()

	m.pendingTaskTextEdit = nil

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
		if c := route.ViewAccentColor(path[i]); c != nil {
			return c
		}
	}
	if c := m.viewNav.navigation.Primary.Current().Accent(); c != nil {
		return c
	}
	return theme.ColorPrimary
}

// setAccentOnView injects the accent color for the current view stack top into
// any view that implements AccentColorSetter.
func (m *AppModel) setAccentOnView(view route.View) {
	if v, ok := view.(route.AccentColorSetter); ok {
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
	SetBulkCoordinator(coordinator.BulkCoordinator)
}

// taskItemWireable is implemented by every view that routes its single-task
// mutations through the application-layer TaskItemCoordinator, parallel to
// bulkWireable. The shell wires them all from the single showView/replaceView
// site so no construction path forgets to inject the coordinator.
type taskItemWireable interface {
	SetTaskItemCoordinator(coordinator.TaskItemCoordinator)
}

// captureWireable is implemented by every capture-creation view that routes its
// task creation through the application-layer CaptureCoordinator. The shell wires
// them from the single showView/replaceView site so no construction path can
// forget to inject the coordinator (a nil mutator would panic in a create
// handler).
type captureWireable interface {
	SetCaptureCoordinator(coordinator.CaptureCoordinator)
}

// taskFormWireable is implemented by the expanded create form, which routes its
// mutation through the application-layer TaskFormCoordinator.
type taskFormWireable interface {
	SetTaskFormCoordinator(coordinator.TaskFormCoordinator)
}

// wireSecondaryCoordinators injects the shell's shared coordinators into a view
// as it is registered, so a view's mutation handlers can commit through the
// application layer without every construction site threading the coordinator.
func (m *AppModel) wireSecondaryCoordinators(view route.View) {
	if w, ok := view.(bulkWireable); ok {
		w.SetBulkCoordinator(m.secondaryFeatures.Bulk)
	}
	if w, ok := view.(taskItemWireable); ok {
		w.SetTaskItemCoordinator(m.secondaryFeatures.TaskItem)
	}
	if w, ok := view.(captureWireable); ok {
		w.SetCaptureCoordinator(m.features.Capture)
	}
	if w, ok := view.(taskFormWireable); ok {
		w.SetTaskFormCoordinator(m.features.TaskForm)
	}
	if outlinerView, ok := view.(*outliner.OutlinerModel); ok {
		m.wireOutlinerOperationExecutor(outlinerView)
	}
}

// wireOutlinerOperationExecutor injects the shell seams an Outliner cannot build
// for itself: the operation registry it projects keys and help from, the
// machine-local write that remembers which lens is live, the workspace conflict
// count its status strip reports, and whether that strip carries ambient
// advisories at all.
func (m *AppModel) wireOutlinerOperationExecutor(outlinerView *outliner.OutlinerModel) {
	if outlinerView == nil {
		return
	}
	outlinerView.OperationRegistry = m.operations.Registry
	outlinerView.HistoryPreview = m.workspaceHistoryPreview
	m.lenses.WireOutliner(outlinerView, m.commitActiveLensPointer)
	if machine := m.Machine(); machine != nil {
		outlinerView.ConfigureSearchHistory(machine.SearchHistory, m.commitSearchHistory)
	}
	outlinerView.SetOutstandingConflicts(m.feedback.outstandingConflictCount)
	outlinerView.SetSyncAdvisory(m.feedback.syncAdvisoryLine)
	outlinerView.SetAdvisoriesHidden(m.feedback.advisoriesHidden)
	outlinerView.SetMouseDisabled(m.mouseDisabled)
}

func (m *AppModel) showView(viewType presentation.PresentationID, view route.View) {
	// All views (including full-height ones) are sized to height-2. Register
	// re-applies the registry's stored height-2, so a full-height SetSize here
	// would be immediately clobbered anyway; this also matches handleWindowSize.
	view.SetSize(m.viewNav.viewSize())
	m.wireSecondaryCoordinators(view)
	m.viewNav.views.Register(viewType, view)
	if viewType == presentation.ViewCrossProjectOutliner {
		if outlinerView, ok := view.(*outliner.OutlinerModel); ok {
			m.crossOutliner = outlinerView
		}
	}
	m.pushView(viewType)
	m.setAccentOnView(view)
}

// showViewInit is showView + return true, view.Init(). Used by handlers that
// create a view, push it, and immediately return its Init command.
func (m *AppModel) showViewInit(viewType presentation.PresentationID, view route.View) (bool, tea.Cmd) {
	m.showView(viewType, view)
	return true, view.Init()
}

// viewNavigationController is the shell's view and navigation cluster: the
// authoritative route and overlay stacks, the registry every renderer is
// installed in, and the terminal geometry both are sized against. It grows the
// NavigationState precedent — that field keeps its own type and stays named,
// with the registry and geometry it was already paired with joining it rather
// than replacing it.
//
// It owns the machinery, not the renderers. The live view instances
// (crossOutliner, helpView, manageHub, taskComposer, lenses.picker,
// conflictBrowser) stay AppModel fields: foundation_invariants_test.go pins
// "AppModel has exactly one field mentioning OutlinerModel" against the
// AppModel struct itself, and a pointer to a view is not navigation state.
//
// Registration remains a separate path: showView is the single site that sizes,
// wires, registers, caches, pushes, and sets accent, and the paint-time
// injection in View() is its own pass over the active renderer. This type is
// deliberately not a seam where those two could merge.
type viewNavigationController struct {
	// navigation is the sole live shell authority. PresentationID values attached
	// to routes and overlays select renderers; they never form a second
	// navigation stack.
	navigation route.NavigationState

	// views is the centralized registry of installed renderers.
	views *route.ViewRegistry

	// width and height are the terminal dimensions the shell composes its frame
	// against. They are distinct from the InteractiveViewState/BaseView fields of
	// the same name on views: those are per-renderer budgets, these are the
	// terminal's own size.
	width  int
	height int

	// tooNarrow and tooShort record that the terminal cannot fit the minimum
	// frame, which the shell renders instead of any view.
	tooNarrow bool
	tooShort  bool
}

// newViewNavigationController builds the cluster at its startup defaults, which
// stand in until the first WindowSizeMsg arrives.
func newViewNavigationController() viewNavigationController {
	return viewNavigationController{
		navigation: route.NewNavigationState(),
		views:      route.NewViewRegistry(),
		width:      80,
		height:     24,
	}
}

// size returns the terminal dimensions.
func (c *viewNavigationController) size() (int, int) { return c.width, c.height }

// viewSize returns the dimensions every renderer is sized to. All views —
// full-height ones included — get height-2, because ViewRegistry.Register
// re-applies its stored height-2 and would clobber anything else anyway.
func (c *viewNavigationController) viewSize() (int, int) { return c.width, c.height - 2 }

// tooSmall reports whether the terminal is below the minimum frame in either
// axis, so the shell renders the size notice instead of a view.
func (c *viewNavigationController) tooSmall() bool { return c.tooNarrow || c.tooShort }

// resize records new terminal dimensions and re-sizes every registered
// renderer to the view budget.
func (c *viewNavigationController) resize(width, height int) {
	c.width = width
	c.height = height
	c.tooNarrow = width < chrome.MinTerminalWidth
	c.tooShort = height < chrome.MinTerminalHeight
	c.views.SetSize(c.viewSize())
}

// atRoot reports whether navigation is the bare Outliner root: one primary
// frame and no overlay. Its negation is what makes an Esc have something to
// unwind.
func (c *viewNavigationController) atRoot() bool {
	return c.navigation.Primary.Depth() == 1 && !c.navigation.Overlays.Active()
}

// This file hosts the per-project primary-view (Outliner) lifecycle helpers.
// They replace the retired ProjectTree equivalents; the Outliner is now the
// sole per-project primary view, so every former ProjectTree entry point
// (navigation, project cycling, task-link jumps, section/tag filters) routes
// through here.

// saveProjectViewState caches the current outliner's fold state for this session.
func (m *AppModel) saveProjectViewState() {
	m.saveOutlinerFilterState()
}

// refreshProjectView rebuilds the current outliner's rows from the (already
// reloaded) project, preserving the cursor. It never reloads from disk.
func (m *AppModel) refreshProjectView() {
	if m.crossOutliner == nil {
		return
	}
	outliner.RebuildOutlinerPreservingCursor(m.crossOutliner)
}

// refreshProjectViewInMemory is the in-memory-only refresh used as the safe
// fallback after a runtime-domain reload failure.
func (m *AppModel) refreshProjectViewInMemory() {
	m.refreshProjectView()
}
