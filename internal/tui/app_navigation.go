package tui

import (
	"image/color"

	tea "charm.land/bubbletea/v2"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/coordinator"
	"github.com/sglavoie/tt/internal/tui/notes"
	"github.com/sglavoie/tt/internal/tui/operation"
	"github.com/sglavoie/tt/internal/tui/outliner"
	"github.com/sglavoie/tt/internal/tui/presentation"
	"github.com/sglavoie/tt/internal/tui/projecthub"
	"github.com/sglavoie/tt/internal/tui/route"
	"github.com/sglavoie/tt/internal/tui/status"
	"github.com/sglavoie/tt/internal/tui/theme"
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
func (m *AppModel) validateCurrentProject() (handled bool, cmd tea.Cmd) {
	if m.currentProject == nil {
		return false, nil
	}
	if m.projectByID(m.currentProject.ID) != nil {
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
		closingView := m.currentView()
		// Capture the view label for the Esc undo hint before any state changes.
		var closingLabel string
		if view := m.viewNav.views.Get(closingView); view != nil {
			if resetter, ok := view.(route.ViewExitResetter); ok {
				resetter.OnExit()
			}
			if labeler, ok := view.(route.StatusBarLabeler); ok {
				closingLabel = labeler.StatusBarLabel()
			}
		}
		cmd := m.popView()
		if closingLabel != "" {
			m.setInfoStatus("Closed " + closingLabel)
			return true, tea.Batch(cmd, status.DefaultStatusTimeout())
		}
		return true, cmd
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
	m.lenses.WireOutliner(outlinerView, m.commitActiveLensPointer)
	outlinerView.SetOutstandingConflicts(m.feedback.outstandingConflictCount)
	outlinerView.SetAdvisoriesHidden(m.feedback.advisoriesHidden)
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

// currentPresentationView resolves the renderer selected by the authoritative
// route and overlay stacks. Overlays without a registered renderer intentionally
// leave primary dispatch active.
func (m *AppModel) currentPresentationView() presentation.PresentationID {
	path := m.presentationPath()
	if len(path) > 0 {
		return path[len(path)-1]
	}
	return presentation.ViewCrossProjectOutliner
}

func taskWorkspacePresentation(_ route.TaskWorkspaceParams) presentation.PresentationID {
	// TaskWorkspace routes are app-scoped only, so the single authoritative cross
	// Outliner always renders them; project scopes are Outliner scope frames
	// rather than a second presentation identity.
	return presentation.ViewCrossProjectOutliner
}

// presentationPath derives renderer-local identities from the live navigation
// stacks. It deliberately has no synthetic root sentinel.
func (m *AppModel) presentationPath() []presentation.PresentationID {
	routes := m.viewNav.navigation.Primary.Routes()
	path := make([]presentation.PresentationID, 0, len(routes)+m.viewNav.navigation.Overlays.Depth())
	for _, primary := range routes {
		if view, ok := m.presentationForRoute(primary); ok {
			path = append(path, view)
		}
	}
	for _, overlay := range m.viewNav.navigation.Overlays.Overlays() {
		if view, ok := overlay.Presentation(); ok && view != presentation.ViewUnknown {
			path = append(path, view)
			continue
		}
		if reference, ok := overlay.ReferenceRoute(); ok {
			if view, mapped := m.presentationForRoute(reference); mapped {
				path = append(path, view)
				continue
			}
		}
		if overlay.Kind() == route.OverlayComposer && overlay.Key() == taskComposerOverlayKey && m.taskComposer != nil {
			path = append(path, presentation.ViewTaskComposer)
		}
	}
	return path
}

func (m *AppModel) presentationForRoute(target route.Route) (presentation.PresentationID, bool) {
	if view, ok := target.Presentation(); ok && view != presentation.ViewUnknown {
		return view, true
	}
	switch target.ID() {
	case route.TaskWorkspace:
		params, ok := target.Params().(route.TaskWorkspaceParams)
		if !ok {
			return 0, false
		}
		return taskWorkspacePresentation(params), true
	case route.SecondaryTool:
		switch params := target.Params().(type) {
		case route.ManageParams:
			if m.manageHub != nil && route.ManageParamsEqual(params, m.manageHub.Params()) {
				if view, ok := m.manageHub.PresentationView(); ok {
					return view, true
				}
			}
			return projecthub.ManageModulePresentation(params)
		}
	case route.Reference:
		params, ok := target.Params().(route.ReferenceParams)
		if !ok {
			return 0, false
		}
		switch params.Kind {
		case route.ReferenceHelp:
			return presentation.ViewHelp, true
		case route.ReferenceOutlineErrors:
			return presentation.ViewOutlineErrors, true
		}
	}
	return 0, false
}

// outlinerPrimary resolves the canonical Task Workspace model beneath shell
// overlays.
func (m *AppModel) outlinerPrimary() (*outliner.OutlinerModel, bool) {
	current := m.viewNav.navigation.Primary.Current()
	if current.ID() != route.TaskWorkspace {
		return nil, false
	}
	_, ok := current.Params().(route.TaskWorkspaceParams)
	if !ok {
		return nil, false
	}
	// The canonical renderer owns both global and project-scoped work. Project
	// selection is a scope frame, so the root route remains app-scoped.
	return m.crossOutliner, m.crossOutliner != nil
}

func (m *AppModel) activeOutlinerRoute() bool {
	if _, ok := m.outlinerPrimary(); !ok {
		return false
	}
	return !m.viewNav.navigation.Overlays.Active()
}

// activeManageHub reports whether the typed Manage secondary tool is the live
// renderer and is not covered by a legacy-rendered overlay.
func (m *AppModel) activeManageHub() bool {
	if _, _, ok := m.manageHubPrimary(); !ok {
		return false
	}
	if overlay, ok := m.viewNav.navigation.Overlays.Current(); ok && overlayCoversPrimary(overlay) {
		return false
	}
	return true
}

// manageHubPrimary returns Manage beneath shell overlays so palette and
// collection result handlers resolve the same embedded module as direct keys.
func (m *AppModel) manageHubPrimary() (*projecthub.ManageHubModel, route.ManageParams, bool) {
	if m.manageHub == nil {
		return nil, route.ManageParams{}, false
	}
	current := m.viewNav.navigation.Primary.Current()
	if current.ID() != route.SecondaryTool {
		return nil, route.ManageParams{}, false
	}
	if _, hasPresentation := current.Presentation(); hasPresentation {
		return nil, route.ManageParams{}, false
	}
	params, ok := current.Params().(route.ManageParams)
	if !ok || !route.ManageParamsEqual(params, m.manageHub.Params()) {
		return nil, route.ManageParams{}, false
	}
	return m.manageHub, params, true
}

// overlayCoversPrimary distinguishes a real presentation layer from shell
// state markers such as a hub's embedded view menu. Reference overlays select
// their presentation through a typed Reference route instead of the legacy
// Presentation field, so both forms must cover the primary child for rendering,
// input dispatch, and one-layer Esc ownership.
func overlayCoversPrimary(overlay route.Overlay) bool {
	if _, hasPresentation := overlay.Presentation(); hasPresentation {
		return true
	}
	_, hasReference := overlay.ReferenceRoute()
	return hasReference
}

func (m *AppModel) activeView() route.View {
	if m.activeTaskComposer() {
		return m.taskComposer
	}
	if m.activeManageHub() {
		return m.manageHub
	}
	if m.viewNav.views == nil {
		return nil
	}
	return m.viewNav.views.Get(m.currentView())
}

// installTypedPresentation projects a renderer onto the live typed stacks. It
// returns false for presentation-only views without a route or overlay class.
func (m *AppModel) installTypedPresentation(view presentation.PresentationID) bool {
	if view == presentation.ViewUnknown {
		return false
	}
	if overlay, ok, err := route.OverlayForPresentation(view); err == nil && ok {
		m.viewNav.navigation.Overlays.Push(overlay, nil)
		return true
	}

	return false
}

func (m *AppModel) activateTaskWorkspaceRoute(target route.Route) tea.Cmd {
	if target.ID() != route.TaskWorkspace {
		return nil
	}
	if _, ok := target.Params().(route.TaskWorkspaceParams); !ok {
		return nil
	}
	if err := m.viewNav.navigation.ReplaceDestination(target); err != nil {
		return func() tea.Msg { return status.ErrorMsg{Err: err} }
	}
	return nil
}

func (m *AppModel) popTypedPresentation() bool {
	if overlay, ok := m.viewNav.navigation.Overlays.CloseTop(); ok {
		if overlay.Kind() == route.OverlayComposer && overlay.Key() == taskComposerOverlayKey {
			m.taskComposer = nil
		}
		return true
	}

	current := m.viewNav.navigation.Primary.Current()
	if current.ID() == route.SecondaryTool {
		// Secondary tools are hierarchical children: a single Esc
		// dismisses the child directly back to the launching destination without an
		// intermediate landing retreat.
		m.viewNav.navigation.Primary.Pop()
		return true
	}
	// The Outliner destination is the sole root. A clean root Esc has nothing to
	// unwind and is a no-op, letting the key fall through to the view.
	return false
}

func (m *AppModel) pushTypedOverlay(kind route.OverlayKind, key string) {
	if current, ok := m.viewNav.navigation.Overlays.Current(); ok && current.Kind() == kind && current.Key() == key {
		return
	}
	overlay, err := route.NewOverlay(kind, key)
	if err == nil {
		m.viewNav.navigation.Overlays.Push(overlay, func() {
			m.cancelTypedOverlayPresentation(kind, key)
		})
	}
}

// cancelTypedOverlayPresentation clears presentation state when a destination
// switch or Home reset cancels the typed overlay stack. Successful closes use
// their owning close methods; cancellation must not recursively mutate the
// stack it is currently unwinding.
func (m *AppModel) cancelTypedOverlayPresentation(kind route.OverlayKind, key string) {
	switch kind {
	case route.OverlayComposer:
		if key == taskComposerOverlayKey {
			m.taskComposer = nil
		}
	default:
	}
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
