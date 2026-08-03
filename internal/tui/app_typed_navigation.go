package tui

import (
	tea "charm.land/bubbletea/v2"
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
// stacks. It deliberately has no synthetic root sentinel.
func (m *AppModel) presentationPath() []PresentationID {
	routes := m.navigation.Primary.Routes()
	path := make([]PresentationID, 0, len(routes)+m.navigation.Overlays.Depth())
	for _, route := range routes {
		if view, ok := m.presentationForRoute(route); ok {
			path = append(path, view)
		}
	}
	for _, overlay := range m.navigation.Overlays.Overlays() {
		if view, ok := overlay.Presentation(); ok && view != ViewUnknown {
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
	if view, ok := route.Presentation(); ok && view != ViewUnknown {
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
		case ManageParams:
			if m.manageHub != nil && manageParamsEqual(params, m.manageHub.Params()) {
				if view, ok := m.manageHub.PresentationView(); ok {
					return view, true
				}
			}
			return manageModulePresentation(params)
		}
	case Reference:
		params, ok := route.Params().(ReferenceParams)
		if !ok {
			return 0, false
		}
		switch params.Kind {
		case ReferenceHelp:
			return ViewHelp, true
		}
	}
	return 0, false
}

// outlinerPrimary resolves the canonical Task Workspace model beneath shell
// overlays.
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

// activeManageHub reports whether the typed Manage secondary tool is the live
// renderer and is not covered by a legacy-rendered overlay.
func (m *AppModel) activeManageHub() bool {
	if _, _, ok := m.manageHubPrimary(); !ok {
		return false
	}
	if overlay, ok := m.navigation.Overlays.Current(); ok && overlayCoversPrimary(overlay) {
		return false
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

// overlayCoversPrimary distinguishes a real presentation layer from shell
// state markers such as a hub's embedded view menu. Reference overlays select
// their presentation through a typed Reference route instead of the legacy
// Presentation field, so both forms must cover the primary child for rendering,
// input dispatch, and one-layer Esc ownership.
func overlayCoversPrimary(overlay Overlay) bool {
	if _, hasPresentation := overlay.Presentation(); hasPresentation {
		return true
	}
	_, hasReference := overlay.ReferenceRoute()
	return hasReference
}

func (m *AppModel) activeView() View {
	if m.activeTaskComposer() {
		return m.taskComposer
	}
	if m.activeManageHub() {
		return m.manageHub
	}
	if m.views == nil {
		return nil
	}
	return m.views.Get(m.currentView())
}

// installTypedPresentation projects a renderer onto the live typed stacks. It
// returns false for presentation-only views without a route or overlay class.
func (m *AppModel) installTypedPresentation(view PresentationID) bool {
	if view == ViewUnknown {
		return false
	}
	if overlay, ok, err := overlayForPresentation(view); err == nil && ok {
		m.navigation.Overlays.Push(overlay, m.currentFocusTarget(), nil)
		return true
	}

	return false
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
		if overlay.Kind() == OverlayComposer && overlay.Key() == taskComposerOverlayKey {
			m.taskComposer = nil
		}
		return true
	}

	current := m.navigation.Primary.Current()
	if current.ID() == SecondaryTool {
		// Secondary tools are hierarchical children: a single Esc
		// dismisses the child directly back to the launching destination without an
		// intermediate landing retreat.
		m.navigation.Primary.Pop()
		return true
	}
	// The Outliner destination is the sole root. A clean root Esc has nothing to
	// unwind and is a no-op, letting the key fall through to the view.
	return false
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
	default:
	}
}
