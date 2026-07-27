package tui

import (
	"errors"
	"fmt"
)

var ErrInvalidRouteTransition = errors.New("invalid primary route transition")

// FocusTarget is an implementation-independent bookmark restored after a
// child route or overlay closes. EntityID keeps selection stable across data
// refreshes; ControlID identifies the control that should regain keyboard focus.
type FocusTarget struct {
	ControlID string
	EntityID  string
}

type primaryRouteFrame struct {
	route        Route
	restoreFocus FocusTarget
}

// outlinerRootRoute is the sole primary root: the global (All Projects)
// Outliner. Every navigation stack bottoms out here; there is no Home root.
func outlinerRootRoute() Route {
	route, err := NewRoute(TaskWorkspace, TaskWorkspaceParams{Scope: AppEntityScope()})
	if err != nil {
		panic(err)
	}
	return route
}

// PrimaryRouteStack always contains the Outliner root at index zero. A
// destination switch replaces that root frame; Task Detail and secondary tools
// are pushed as child routes above it.
type PrimaryRouteStack struct{ frames []primaryRouteFrame }

func NewPrimaryRouteStack() PrimaryRouteStack {
	return PrimaryRouteStack{frames: []primaryRouteFrame{{route: outlinerRootRoute()}}}
}

func (s *PrimaryRouteStack) ensureRoot() {
	if len(s.frames) == 0 {
		*s = NewPrimaryRouteStack()
	}
}

func (s *PrimaryRouteStack) Current() Route {
	s.ensureRoot()
	return s.frames[len(s.frames)-1].route
}

func (s *PrimaryRouteStack) Routes() []Route {
	s.ensureRoot()
	routes := make([]Route, len(s.frames))
	for i := range s.frames {
		routes[i] = s.frames[i].route
	}
	return routes
}

func (s *PrimaryRouteStack) Depth() int {
	s.ensureRoot()
	return len(s.frames)
}

// ReplaceDestination swaps the primary root. Only the Outliner (TaskWorkspace)
// is a destination, so this replaces the single root frame and drops any child
// routes above it; scope variations (All Projects vs. a single project) all
// resolve to the same Outliner root.
func (s *PrimaryRouteStack) ReplaceDestination(route Route) error {
	s.ensureRoot()
	if !route.IsDestination() {
		return fmt.Errorf("%w: %s is not a root destination", ErrInvalidRouteTransition, route.ID())
	}
	s.frames = []primaryRouteFrame{{route: route}}
	return nil
}

// PushTaskDetail installs Task Detail as the one primary child and records the
// exact stable focus bookmark to restore when it is popped.
func (s *PrimaryRouteStack) PushTaskDetail(route Route, restore FocusTarget) error {
	if route.ID() != TaskDetail {
		return fmt.Errorf("%w: only TaskDetail can be pushed", ErrInvalidRouteTransition)
	}
	return s.PushChild(route, restore)
}

// PushChild installs the one child route above the Outliner root and records the
// parent focus bookmark to restore on pop. Task Detail and secondary tools never
// nest; switching child kinds or modules replaces the existing child so the
// primary stack remains bounded to root + one child.
func (s *PrimaryRouteStack) PushChild(route Route, restore FocusTarget) error {
	s.ensureRoot()
	switch route.ID() {
	case TaskDetail, SecondaryTool:
	default:
		return fmt.Errorf("%w: %s is not a child route", ErrInvalidRouteTransition, route.ID())
	}
	if len(s.frames) > 1 {
		s.frames[len(s.frames)-1].route = route
		return nil
	}
	s.frames = append(s.frames, primaryRouteFrame{route: route, restoreFocus: restore})
	return nil
}

// ReplaceCurrentMode changes the mode/parameters of the active destination or
// Task Detail without changing its parentage. Consolidated hub modules and
// Task Detail tabs are modes, not child routes.
func (s *PrimaryRouteStack) ReplaceCurrentMode(route Route) error {
	s.ensureRoot()
	current := s.Current()
	if current.ID() != route.ID() {
		return fmt.Errorf("%w: cannot replace %s with %s as a mode", ErrInvalidRouteTransition, current.ID(), route.ID())
	}
	s.frames[len(s.frames)-1].route = route
	return nil
}

// Pop removes a child route and returns the focus bookmark of its parent.
func (s *PrimaryRouteStack) Pop() (popped Route, restore FocusTarget, ok bool) {
	s.ensureRoot()
	if len(s.frames) == 1 {
		return Route{}, FocusTarget{}, false
	}
	frame := s.frames[len(s.frames)-1]
	s.frames = s.frames[:len(s.frames)-1]
	return frame.route, frame.restoreFocus, true
}

// ResetRoot collapses the stack back to the sole Outliner root, dropping every
// child route above it.
func (s *PrimaryRouteStack) ResetRoot() { s.ensureRoot(); s.frames = s.frames[:1] }

type overlayFrame struct {
	overlay      Overlay
	restoreFocus FocusTarget
	cancel       func()
}

// OverlayStack owns nested transient presentations. Close is a successful
// dismissal; Cancel invokes the overlay's cancellation callback exactly once.
type OverlayStack struct{ frames []overlayFrame }

func (s *OverlayStack) Push(overlay Overlay, restore FocusTarget, cancel func()) {
	s.frames = append(s.frames, overlayFrame{overlay: overlay, restoreFocus: restore, cancel: cancel})
}

func (s *OverlayStack) Active() bool { return len(s.frames) > 0 }

func (s *OverlayStack) Depth() int { return len(s.frames) }

func (s *OverlayStack) Current() (Overlay, bool) {
	if len(s.frames) == 0 {
		return Overlay{}, false
	}
	return s.frames[len(s.frames)-1].overlay, true
}

func (s *OverlayStack) Overlays() []Overlay {
	overlays := make([]Overlay, len(s.frames))
	for i := range s.frames {
		overlays[i] = s.frames[i].overlay
	}
	return overlays
}

func (s *OverlayStack) CloseTop() (Overlay, FocusTarget, bool) {
	return s.removeTop(false)
}

func (s *OverlayStack) CancelTop() (Overlay, FocusTarget, bool) {
	return s.removeTop(true)
}

func (s *OverlayStack) removeTop(cancel bool) (Overlay, FocusTarget, bool) {
	if len(s.frames) == 0 {
		return Overlay{}, FocusTarget{}, false
	}
	index := len(s.frames) - 1
	frame := s.frames[index]
	s.frames = s.frames[:index]
	if cancel && frame.cancel != nil {
		frame.cancel()
	}
	return frame.overlay, frame.restoreFocus, true
}

// CancelAll cancels overlays from top to bottom, matching visible nesting.
func (s *OverlayStack) CancelAll() {
	for len(s.frames) > 0 {
		s.CancelTop()
	}
}

// NavigationState coordinates route and overlay stacks so destination changes
// cannot leave transient state attached to an abandoned route.
type NavigationState struct {
	Primary  PrimaryRouteStack
	Overlays OverlayStack
}

func NewNavigationState() NavigationState {
	return NavigationState{Primary: NewPrimaryRouteStack()}
}

func (s *NavigationState) ReplaceDestination(route Route) error {
	if err := s.Primary.ReplaceDestination(route); err != nil {
		return err
	}
	s.Overlays.CancelAll()
	return nil
}

func (s *NavigationState) ResetRoot() {
	s.Overlays.CancelAll()
	s.Primary.ResetRoot()
}

// PushChild launches a hierarchical child route (a secondary tool or Task
// Detail), abandoning any transient overlay that triggered it so the child is
// the live primary and dismisses back to its launching destination.
func (s *NavigationState) PushChild(route Route, restore FocusTarget) error {
	s.Overlays.CancelAll()
	return s.Primary.PushChild(route, restore)
}

// RouteEntityLookup is the narrow existence check used to resolve deep links.
type RouteEntityLookup interface {
	ProjectExists(projectID string) bool
	TaskExists(projectID, taskID string) bool
}

type DeepLinkResolution struct {
	Parent  Route
	Detail  *Route
	Overlay *Overlay
	Status  string
}

// ResolveTaskDetailDeepLink reveals a direct task link inside the Outliner or
// Task Detail without ever constructing a standalone Project route. Missing
// projects fall back to the Outliner root with a project picker; missing tasks
// remain on the Outliner parent and return an error status.
func ResolveTaskDetailDeepLink(params TaskDetailParams, lookup RouteEntityLookup) (DeepLinkResolution, error) {
	// The parent is always the canonical app-scoped Outliner root. Entering the
	// task's project is an Outliner scope frame applied by the caller, so a deep
	// link never constructs a separate project-scoped TaskWorkspace root.
	parent := outlinerRootRoute()
	if lookup == nil || !lookup.ProjectExists(params.ProjectID) {
		picker, pickerErr := NewOverlay(OverlayProjectPicker, params.ProjectID)
		if pickerErr != nil {
			return DeepLinkResolution{}, pickerErr
		}
		return DeepLinkResolution{
			Parent: outlinerRootRoute(), Overlay: &picker, Status: fmt.Sprintf("Project %q was not found", params.ProjectID),
		}, nil
	}
	if params.TaskID == "" || !lookup.TaskExists(params.ProjectID, params.TaskID) {
		return DeepLinkResolution{
			Parent: parent, Status: fmt.Sprintf("Task %q was not found", params.TaskID),
		}, nil
	}
	detail, err := NewRoute(TaskDetail, params)
	if err != nil {
		// A syntactically valid link with invalid detail parameters degrades to its
		// safe parent route and a user-facing status instead of failing navigation.
		return DeepLinkResolution{Parent: parent, Status: "Invalid task deep link: " + err.Error()}, nil //nolint:nilerr
	}
	return DeepLinkResolution{Parent: parent, Detail: &detail}, nil
}

// ResolveDeepLink validates an arbitrary typed link and converts malformed
// links into a safe Home result with a status message. Reference links become
// typed reference overlays; Task Detail uses the entity-aware fallback above.
func ResolveDeepLink(id RouteID, params RouteParams, lookup RouteEntityLookup) (DeepLinkResolution, error) {
	if id == TaskDetail {
		detailParams, ok := params.(TaskDetailParams)
		if !ok {
			return invalidDeepLinkResolution(paramsTypeError(id, params))
		}
		return ResolveTaskDetailDeepLink(detailParams, lookup)
	}
	route, err := NewRoute(id, params)
	if err != nil {
		return invalidDeepLinkResolution(err)
	}
	if id != Reference {
		return DeepLinkResolution{Parent: route}, nil
	}
	overlay, overlayErr := NewReferenceOverlay(route)
	if overlayErr != nil {
		return DeepLinkResolution{}, overlayErr
	}
	return DeepLinkResolution{Parent: outlinerRootRoute(), Overlay: &overlay}, nil
}

func invalidDeepLinkResolution(cause error) (DeepLinkResolution, error) {
	return DeepLinkResolution{Parent: outlinerRootRoute(), Status: "Invalid deep link: " + cause.Error()}, nil
}

// ApplyDeepLink makes a resolved deep link atomic with overlay cancellation.
func (s *NavigationState) ApplyDeepLink(resolution DeepLinkResolution, restore FocusTarget) error {
	if resolution.Overlay != nil {
		s.ResetRoot()
		s.Overlays.Push(*resolution.Overlay, restore, nil)
		return nil
	}
	if err := s.ReplaceDestination(resolution.Parent); err != nil {
		return err
	}
	if resolution.Detail != nil {
		return s.Primary.PushTaskDetail(*resolution.Detail, restore)
	}
	return nil
}
