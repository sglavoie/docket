package tui

import (
	tea "charm.land/bubbletea/v2"
	"github.com/sglavoie/tt/internal/tui/outliner"
	"github.com/sglavoie/tt/internal/tui/presentation"
	"github.com/sglavoie/tt/internal/tui/projecthub"
	"github.com/sglavoie/tt/internal/tui/route"
	"github.com/sglavoie/tt/internal/tui/status"
)

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
