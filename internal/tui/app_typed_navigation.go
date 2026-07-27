package tui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
)

func (m *AppModel) currentFocusTarget() FocusTarget {
	target := FocusTarget{ControlID: m.currentPresentationView().String()}
	if task, _ := m.getCurrentCursorTaskAndProject(); task != nil {
		target.EntityID = task.ID
	}
	return target
}

// currentPresentationView resolves the renderer selected by the authoritative
// route and overlay stacks. Overlays without a registered renderer intentionally
// leave primary dispatch active.
func (m *AppModel) currentPresentationView() PresentationID {
	path := m.presentationPath()
	if len(path) > 0 {
		return path[len(path)-1]
	}
	return ViewCrossProjectOutliner
}

func taskWorkspacePresentation(_ TaskWorkspaceParams) PresentationID {
	// TaskWorkspace routes are app-scoped only, so the single authoritative cross
	// Outliner always renders them; project scopes are Outliner scope frames
	// rather than a second presentation identity.
	return ViewCrossProjectOutliner
}

// presentationPath derives renderer-local identities from the live navigation
// stacks. It deliberately has no dashboard or synthetic root sentinel.
func (m *AppModel) presentationPath() []PresentationID {
	routes := m.navigation.Primary.Routes()
	path := make([]PresentationID, 0, len(routes)+m.navigation.Overlays.Depth())
	for _, route := range routes {
		if view, ok := m.presentationForRoute(route); ok {
			path = append(path, view)
		}
	}
	for _, overlay := range m.navigation.Overlays.Overlays() {
		if view, ok := overlay.Presentation(); ok && view != ViewDashboard {
			path = append(path, view)
			continue
		}
		if route, ok := overlay.ReferenceRoute(); ok {
			if view, mapped := m.presentationForRoute(route); mapped {
				path = append(path, view)
				continue
			}
		}
		if overlay.Kind() == OverlayComposer && overlay.Key() == taskComposerOverlayKey && m.taskComposer != nil {
			path = append(path, ViewTaskComposer)
		}
	}
	return path
}

func (m *AppModel) presentationForRoute(route Route) (PresentationID, bool) {
	if view, ok := route.Presentation(); ok && view != ViewDashboard {
		return view, true
	}
	switch route.ID() {
	case TaskWorkspace:
		params, ok := route.Params().(TaskWorkspaceParams)
		if !ok {
			return 0, false
		}
		return taskWorkspacePresentation(params), true
	case SecondaryTool:
		switch params := route.Params().(type) {
		case ReviewParams:
			project := (*models.Project)(nil)
			if m.reviewHub != nil {
				project = m.reviewHub.ProjectScope()
			}
			return reviewModulePresentation(params.Module, project)
		case ManageParams:
			if m.manageHub != nil && manageParamsEqual(params, m.manageHub.Params()) {
				if view, ok := m.manageHub.PresentationView(); ok {
					return view, true
				}
			}
			return manageModulePresentation(params)
		}
	case TaskDetail:
		params, ok := route.Params().(TaskDetailParams)
		if !ok {
			return 0, false
		}
		return taskDetailPresentation(params.Tab), true
	case Reference:
		params, ok := route.Params().(ReferenceParams)
		if !ok {
			return 0, false
		}
		switch params.Kind {
		case ReferenceHelp:
			return ViewHelp, true
		case ReferenceAsk:
			return ViewAsk, true
		case ReferenceTutor:
			return ViewTutor, true
		}
	}
	return 0, false
}

// outlinerPrimary resolves the canonical Task Workspace model beneath shell
// overlays such as Finder.
func (m *AppModel) outlinerPrimary() (*OutlinerModel, TaskWorkspaceParams, bool) {
	route := m.navigation.Primary.Current()
	if route.ID() != TaskWorkspace {
		return nil, TaskWorkspaceParams{}, false
	}
	params, ok := route.Params().(TaskWorkspaceParams)
	if !ok {
		return nil, TaskWorkspaceParams{}, false
	}
	// The canonical renderer owns both global and project-scoped work. Project
	// selection is a scope frame, so the root route remains app-scoped.
	return m.crossOutliner, params, m.crossOutliner != nil
}

func (m *AppModel) activeOutlinerRoute() bool {
	if _, _, ok := m.outlinerPrimary(); !ok {
		return false
	}
	return !m.navigation.Overlays.Active()
}

// activeReviewHub reports whether the typed Review secondary tool is the live
// renderer and is not covered by a legacy-rendered overlay.
func (m *AppModel) activeReviewHub() bool {
	if _, _, ok := m.reviewHubPrimary(); !ok {
		return false
	}
	if overlay, ok := m.navigation.Overlays.Current(); ok {
		if _, hasPresentation := overlay.Presentation(); hasPresentation {
			return false
		}
	}
	return true
}

// reviewHubPrimary returns Review even beneath shell overlays so palette
// actions can resolve the selected task against the same module as direct keys.
func (m *AppModel) reviewHubPrimary() (*ReviewHubModel, ReviewParams, bool) {
	if m.reviewHub == nil {
		return nil, ReviewParams{}, false
	}
	route := m.navigation.Primary.Current()
	if route.ID() != SecondaryTool {
		return nil, ReviewParams{}, false
	}
	if _, hasPresentation := route.Presentation(); hasPresentation {
		return nil, ReviewParams{}, false
	}
	params, ok := route.Params().(ReviewParams)
	if !ok || !reviewParamsEqual(params, m.reviewHub.Params()) {
		return nil, ReviewParams{}, false
	}
	return m.reviewHub, params, true
}

// activeManageHub reports whether the typed Manage secondary tool is the live
// renderer and is not covered by a legacy-rendered overlay.
func (m *AppModel) activeManageHub() bool {
	if _, _, ok := m.manageHubPrimary(); !ok {
		return false
	}
	if overlay, ok := m.navigation.Overlays.Current(); ok {
		if _, hasPresentation := overlay.Presentation(); hasPresentation {
			return false
		}
	}
	return true
}

// manageHubPrimary returns Manage beneath shell overlays so palette and
// collection result handlers resolve the same embedded module as direct keys.
func (m *AppModel) manageHubPrimary() (*ManageHubModel, ManageParams, bool) {
	if m.manageHub == nil {
		return nil, ManageParams{}, false
	}
	route := m.navigation.Primary.Current()
	if route.ID() != SecondaryTool {
		return nil, ManageParams{}, false
	}
	if _, hasPresentation := route.Presentation(); hasPresentation {
		return nil, ManageParams{}, false
	}
	params, ok := route.Params().(ManageParams)
	if !ok || !manageParamsEqual(params, m.manageHub.Params()) {
		return nil, ManageParams{}, false
	}
	return m.manageHub, params, true
}

// activeTaskDetail reports whether the unified Task Detail is the live primary
// child and is not covered by a legacy-rendered overlay.
func (m *AppModel) activeTaskDetail() bool {
	if m.taskDetail == nil {
		return false
	}
	if overlay, ok := m.navigation.Overlays.Current(); ok {
		if _, hasPresentation := overlay.Presentation(); hasPresentation {
			return false
		}
	}
	_, _, ok := m.taskDetailPrimary()
	return ok
}

// taskDetailPrimary returns Task Detail beneath shell overlays so global task
// actions resolve the same task as tab-local controls.
func (m *AppModel) taskDetailPrimary() (*TaskDetailModel, TaskDetailParams, bool) {
	if m.taskDetail == nil {
		return nil, TaskDetailParams{}, false
	}
	route := m.navigation.Primary.Current()
	if route.ID() != TaskDetail {
		return nil, TaskDetailParams{}, false
	}
	if _, hasPresentation := route.Presentation(); hasPresentation {
		return nil, TaskDetailParams{}, false
	}
	params, ok := route.Params().(TaskDetailParams)
	if !ok || !m.taskDetail.Matches(params) {
		return nil, TaskDetailParams{}, false
	}
	return m.taskDetail, params, true
}

func (m *AppModel) activeView() View {
	if m.activeTaskComposer() {
		return m.taskComposer
	}
	if m.activeReviewHub() {
		return m.reviewHub
	}
	if m.activeManageHub() {
		return m.manageHub
	}
	if m.activeTaskDetail() {
		return m.taskDetail
	}
	if m.views == nil {
		return nil
	}
	return m.views.Get(m.currentView())
}

// installTypedPresentation projects a renderer onto the live typed stacks. It
// returns false for presentation-only views without a route or overlay class.
func (m *AppModel) installTypedPresentation(view PresentationID) bool {
	if view == ViewDashboard {
		return false
	}
	if overlay, ok, err := overlayForPresentation(view); err == nil && ok {
		var cancel func()
		if view == ViewAsk {
			cancel = m.exitAskView
		}
		m.navigation.Overlays.Push(overlay, m.currentFocusTarget(), cancel)
		return true
	}

	return false
}

// installPendingStartupRoute replaces inferred renderer defaults with the
// exact typed route loaded from configuration. It runs only
// for the first renderer selected by AppModel.Init.
func (m *AppModel) installPendingStartupRoute(view PresentationID) {
	if m.startupRoute == nil || view != m.startupView {
		return
	}
	if m.navigation.ReplaceDestination(*m.startupRoute) != nil {
		return
	}
	m.startupRoute = nil
	m.startupView = ViewCrossProjectOutliner
}

func (m *AppModel) activateTaskWorkspaceRoute(route Route) tea.Cmd {
	if route.ID() != TaskWorkspace {
		return nil
	}
	if _, ok := route.Params().(TaskWorkspaceParams); !ok {
		return nil
	}
	if err := m.navigation.ReplaceDestination(route); err != nil {
		return func() tea.Msg { return ErrorMsg{Err: err} }
	}
	return nil
}

func (m *AppModel) popTypedPresentation() bool {
	if overlay, _, ok := m.navigation.Overlays.CloseTop(); ok {
		if view, hasPresentation := overlay.Presentation(); hasPresentation && view == ViewAsk {
			m.exitAskView()
		}
		if overlay.Kind() == OverlayComposer && overlay.Key() == taskComposerOverlayKey {
			m.taskComposer = nil
		}
		return true
	}

	current := m.navigation.Primary.Current()
	if current.ID() == TaskDetail || current.ID() == SecondaryTool {
		// Task Detail and secondary tools are hierarchical children: a single Esc
		// dismisses the child directly back to the launching destination without an
		// intermediate landing retreat.
		m.navigation.Primary.Pop()
		return true
	}
	// The Outliner destination is the sole root. A clean root Esc has nothing to
	// unwind and is a no-op, letting the key fall through to the view.
	return false
}

func (m *AppModel) closeTypedOverlay(kind OverlayKind, key string) {
	current, ok := m.navigation.Overlays.Current()
	if !ok || current.Kind() != kind || (key != "" && current.Key() != key) {
		return
	}
	m.navigation.Overlays.CloseTop()
}

func (m *AppModel) pushTypedOverlay(kind OverlayKind, key string) {
	if current, ok := m.navigation.Overlays.Current(); ok && current.Kind() == kind && current.Key() == key {
		return
	}
	overlay, err := NewOverlay(kind, key)
	if err == nil {
		m.navigation.Overlays.Push(overlay, m.currentFocusTarget(), func() {
			m.cancelTypedOverlayPresentation(kind, key)
		})
	}
}

// cancelTypedOverlayPresentation clears presentation state when a destination
// switch or Home reset cancels the typed overlay stack. Successful closes use
// their owning close methods; cancellation must not recursively mutate the
// stack it is currently unwinding.
func (m *AppModel) cancelTypedOverlayPresentation(kind OverlayKind, key string) {
	switch kind {
	case OverlayComposer:
		if key == taskComposerOverlayKey {
			m.taskComposer = nil
		}
	case OverlayPeek:
		m.peekTask = nil
		m.peekProject = nil
		m.peekActive = false
		m.peekLeftOffset = 0
		m.peekRightOffset = 0
		m.peekFocusRight = false
		m.peekLeftLines = 0
		m.peekRightLines = 0
		m.peekPaneHeight = 0
	case OverlayViewMenu:
		if key == manageHubViewMenuOverlayKey && m.manageHub != nil {
			m.manageHub.closePicker()
		}
	default:
	}
}
