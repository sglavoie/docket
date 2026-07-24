package tui

import tea "charm.land/bubbletea/v2"

// enableTypedNavigation initializes the live route/overlay stacks from the
// current renderer identity. Production enables this in NewAppModel; small
// headless models may use viewStack-only behavior.
func (m *AppModel) enableTypedNavigation() {
	m.navigation = NewNavigationState()
	m.typedNavigation = true
	if len(m.viewStack) == 0 {
		m.viewStack = []PresentationID{ViewDashboard}
	}
	stack := append([]PresentationID(nil), m.viewStack...)
	m.viewStack = []PresentationID{ViewDashboard}
	for _, view := range stack {
		if view != ViewDashboard {
			m.installTypedPresentation(view)
		}
	}
}

func (m *AppModel) currentFocusTarget() FocusTarget {
	target := FocusTarget{ControlID: m.currentPresentationView().String()}
	if task, _ := m.getCurrentCursorTaskAndProject(); task != nil {
		target.EntityID = task.ID
	}
	return target
}

// currentPresentationView resolves the PresentationID renderer selected by
// the typed overlay/primary stacks. Overlays without a registered renderer (for
// example the Ctrl+G switcher) intentionally leave primary dispatch active.
func (m *AppModel) currentPresentationView() PresentationID {
	if !m.typedNavigation {
		if len(m.viewStack) == 0 {
			return ViewDashboard
		}
		return m.viewStack[len(m.viewStack)-1]
	}
	if overlay, ok := m.navigation.Overlays.Current(); ok {
		if view, hasPresentation := overlay.Presentation(); hasPresentation {
			return view
		}
		if overlay.Kind() == OverlayComposer && overlay.Key() == taskComposerOverlayKey && m.taskComposer != nil {
			return m.taskComposer.presentation
		}
	}
	if view, ok := m.navigation.Primary.Current().Presentation(); ok {
		return view
	}
	// A Task Workspace route is rendered exclusively by the mature Outliner for
	// its scope, even when a legacy route instance lacks presentation metadata.
	if route := m.navigation.Primary.Current(); route.ID() == TaskWorkspace {
		if params, ok := route.Params().(TaskWorkspaceParams); ok {
			return taskWorkspacePresentation(params)
		}
	}
	if route := m.navigation.Primary.Current(); route.ID() == SecondaryTool {
		switch params := route.Params().(type) {
		case ReviewParams:
			if m.reviewHub != nil {
				if presentation, found := reviewModulePresentation(params.Module, m.reviewHub.ProjectScope()); found {
					return presentation
				}
			}
		case ManageParams:
			if m.manageHub != nil {
				if presentation, found := m.manageHub.PresentationView(); found && params == m.manageHub.Params() {
					return presentation
				}
			}
		}
	}
	if route := m.navigation.Primary.Current(); route.ID() == TaskDetail && m.taskDetail != nil {
		if params, ok := route.Params().(TaskDetailParams); ok && m.taskDetail.Matches(params) {
			return taskDetailPresentation(params.Tab)
		}
	}
	return ViewDashboard
}

func taskWorkspacePresentation(_ TaskWorkspaceParams) PresentationID {
	// TaskWorkspace routes are app-scoped only, so the single authoritative cross
	// Outliner always renders them; project scopes are Outliner scope frames
	// rather than a second presentation identity.
	return ViewCrossProjectOutliner
}

// outlinerPrimary resolves the canonical Task Workspace model beneath shell
// overlays such as the command palette.
func (m *AppModel) outlinerPrimary() (*OutlinerModel, TaskWorkspaceParams, bool) {
	if !m.typedNavigation {
		return nil, TaskWorkspaceParams{}, false
	}
	route := m.navigation.Primary.Current()
	if route.ID() != TaskWorkspace {
		return nil, TaskWorkspaceParams{}, false
	}
	params, ok := route.Params().(TaskWorkspaceParams)
	if !ok {
		return nil, TaskWorkspaceParams{}, false
	}
	// The single authoritative Outliner owns both global and project-scoped work.
	// Project selection is a scope frame, so the root TaskWorkspace route is always
	// app-scoped and resolves to the one cross-project instance.
	return m.crossOutliner, params, m.crossOutliner != nil
}

func (m *AppModel) activeOutlinerRoute() bool {
	if _, _, ok := m.outlinerPrimary(); !ok {
		return false
	}
	if overlay, ok := m.navigation.Overlays.Current(); ok {
		_, hasPresentation := overlay.Presentation()
		return !hasPresentation
	}
	return true
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
	if !m.typedNavigation || m.reviewHub == nil {
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
	if !ok || params.Module != m.reviewHub.Module() {
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
	if !m.typedNavigation || m.manageHub == nil {
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
	if !ok || params != m.manageHub.Params() {
		return nil, ManageParams{}, false
	}
	return m.manageHub, params, true
}

// activeTaskDetail reports whether the unified Task Detail is the live primary
// child and is not covered by a legacy-rendered overlay.
func (m *AppModel) activeTaskDetail() bool {
	if !m.typedNavigation || m.taskDetail == nil {
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
	if !m.typedNavigation || m.taskDetail == nil {
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
	if !m.typedNavigation {
		return false
	}
	projectID := ""
	if m.currentProject != nil {
		projectID = m.currentProject.ID
	}
	if overlay, ok, err := overlayForPresentation(view, projectID, m.lastNavigatedTaskID); err == nil && ok {
		var cancel func()
		if view == ViewAsk {
			cancel = m.exitAskView
		}
		m.navigation.Overlays.Push(overlay, m.currentFocusTarget(), cancel)
		m.viewStack = append(m.viewStack, view)
		return true
	}

	route, ok, err := routeForPresentation(view, projectID, m.lastNavigatedTaskID)
	if err != nil || !ok || route.ID() == Reference {
		return false
	}
	// Navigating to a primary route abandons transient presentations first.
	m.navigation.Overlays.CancelAll()
	m.syncPresentationStack()

	switch route.ID() {
	case SecondaryTool:
		if err := m.navigation.Primary.PushChild(route, m.currentFocusTarget()); err != nil {
			return false
		}
		m.syncPresentationStack()
	case TaskDetail:
		if m.navigation.Primary.Current().ID() == TaskDetail {
			if err := m.navigation.Primary.ReplaceCurrentMode(route); err != nil {
				return false
			}
			m.syncPresentationStack()
		} else {
			if err := m.navigation.Primary.PushTaskDetail(route, m.currentFocusTarget()); err != nil {
				return false
			}
			m.syncPresentationStack()
		}
	default:
		if err := m.navigation.ReplaceDestination(route); err != nil {
			return false
		}
		m.syncPresentationStack()
	}
	return true
}

// installPendingStartupRoute replaces inferred renderer defaults with the
// exact typed route loaded from configuration. It runs only
// for the first renderer selected by AppModel.Init.
func (m *AppModel) installPendingStartupRoute(view PresentationID) {
	if !m.typedNavigation || m.startupRoute == nil || view != m.startupView {
		return
	}
	projectID := ""
	if m.currentProject != nil {
		projectID = m.currentProject.ID
	}
	route := withPresentation(*m.startupRoute, view, projectID, "")
	if m.navigation.ReplaceDestination(route) != nil {
		return
	}
	m.syncPresentationStack()
	m.startupRoute = nil
	m.startupView = ViewDashboard
}

func (m *AppModel) activateTaskWorkspaceRoute(route Route) tea.Cmd {
	if !m.typedNavigation || route.ID() != TaskWorkspace {
		return nil
	}
	params, ok := route.Params().(TaskWorkspaceParams)
	if !ok {
		return nil
	}
	presentation := taskWorkspacePresentation(params)
	projectID := params.Scope.ProjectID
	route = withPresentation(route, presentation, projectID, "")
	if err := m.navigation.ReplaceDestination(route); err != nil {
		return func() tea.Msg { return ErrorMsg{Err: err} }
	}
	m.syncPresentationStack()
	return nil
}

func (m *AppModel) syncPresentationStack() {
	stack := []PresentationID{ViewDashboard}
	for _, route := range m.navigation.Primary.Routes() {
		view, ok := route.Presentation()
		if !ok && route.ID() == TaskWorkspace {
			if params, paramsOK := route.Params().(TaskWorkspaceParams); paramsOK {
				view, ok = taskWorkspacePresentation(params), true
			}
		}
		if !ok && route.ID() == SecondaryTool {
			switch params := route.Params().(type) {
			case ReviewParams:
				if m.reviewHub != nil {
					view, ok = reviewModulePresentation(params.Module, m.reviewHub.ProjectScope())
				}
			case ManageParams:
				if m.manageHub != nil {
					if params == m.manageHub.Params() {
						view, ok = m.manageHub.PresentationView()
					} else {
						view, ok = manageModulePresentation(params)
					}
				}
			}
		}
		if !ok && route.ID() == TaskDetail && m.taskDetail != nil {
			if params, paramsOK := route.Params().(TaskDetailParams); paramsOK && m.taskDetail.Matches(params) {
				view, ok = taskDetailPresentation(params.Tab), true
			}
		}
		if !ok || view == ViewDashboard {
			continue
		}
		stack = append(stack, view)
	}
	m.viewStack = stack
}

func (m *AppModel) popTypedPresentation() bool {
	if !m.typedNavigation {
		return false
	}
	if overlay, _, ok := m.navigation.Overlays.CloseTop(); ok {
		if view, hasPresentation := overlay.Presentation(); hasPresentation && view == ViewAsk {
			m.exitAskView()
		}
		if overlay.Kind() == OverlayComposer && overlay.Key() == taskComposerOverlayKey {
			m.taskComposer = nil
		}
		if view, hasPresentation := overlay.Presentation(); hasPresentation && len(m.viewStack) > 1 && m.viewStack[len(m.viewStack)-1] == view {
			m.viewStack = m.viewStack[:len(m.viewStack)-1]
		}
		return true
	}

	current := m.navigation.Primary.Current()
	if current.ID() == TaskDetail || current.ID() == SecondaryTool {
		// Task Detail and secondary tools are hierarchical children: a single Esc
		// dismisses the child directly back to the launching destination without an
		// intermediate landing retreat.
		m.navigation.Primary.Pop()
		m.syncPresentationStack()
		return true
	}
	// The Outliner destination is the sole root. A clean root Esc has nothing to
	// unwind and is a no-op, letting the key fall through to the view.
	return false
}

func (m *AppModel) closeTypedOverlay(kind OverlayKind, key string) {
	if !m.typedNavigation {
		return
	}
	current, ok := m.navigation.Overlays.Current()
	if !ok || current.Kind() != kind || (key != "" && current.Key() != key) {
		return
	}
	m.navigation.Overlays.CloseTop()
}

func (m *AppModel) pushTypedOverlay(kind OverlayKind, key string) {
	if !m.typedNavigation {
		return
	}
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
	case OverlayRecent:
		m.showQuickSwitch = false
		m.quickSwitchFiltering = false
	case OverlayPinned:
		m.showPinnedPopup = false
		m.pinnedPopupFiltering = false
	case OverlayViewMenu:
		if key == manageHubViewMenuOverlayKey && m.manageHub != nil {
			m.manageHub.closePicker()
		}
	default:
	}
}

func (m *AppModel) closeQuickSwitchOverlay() {
	m.showQuickSwitch = false
	m.closeTypedOverlay(OverlayRecent, "recent-tasks")
}

func (m *AppModel) closePinnedTasksOverlay() {
	m.showPinnedPopup = false
	m.closeTypedOverlay(OverlayPinned, "pinned-tasks")
}
