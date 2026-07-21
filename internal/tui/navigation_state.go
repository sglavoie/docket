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

// PrimaryRouteStack always contains Home at index zero. Destination switches
// replace index one; Task Detail alone may be pushed as a child route.
type PrimaryRouteStack struct{ frames []primaryRouteFrame }

func NewPrimaryRouteStack() PrimaryRouteStack {
	home, err := NewRoute(Home, HomeParams{})
	if err != nil {
		panic(err)
	}
	return PrimaryRouteStack{frames: []primaryRouteFrame{{route: home}}}
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

// ReplaceDestination implements Ctrl+G stack replacement. Reference and Task
// Detail are rejected because neither is a primary destination.
func (s *PrimaryRouteStack) ReplaceDestination(route Route) error {
	s.ensureRoot()
	if !route.IsDestination() {
		return fmt.Errorf("%w: %s is not a Ctrl+G destination", ErrInvalidRouteTransition, route.ID())
	}
	if route.ID() == Home {
		s.frames = s.frames[:1]
		return nil
	}
	s.frames = append(s.frames[:1], primaryRouteFrame{route: route})
	return nil
}

// PushTaskDetail adds a hierarchical child and records the exact stable focus
// bookmark to restore when it is popped.
func (s *PrimaryRouteStack) PushTaskDetail(route Route, restore FocusTarget) error {
	s.ensureRoot()
	if route.ID() != TaskDetail {
		return fmt.Errorf("%w: only TaskDetail can be pushed", ErrInvalidRouteTransition)
	}
	if s.Current().ID() == TaskDetail {
		return fmt.Errorf("%w: TaskDetail cannot parent TaskDetail", ErrInvalidRouteTransition)
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

func (s *PrimaryRouteStack) ResetHome() { s.ensureRoot(); s.frames = s.frames[:1] }

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

func (s *NavigationState) ResetHome() {
	s.Overlays.CancelAll()
	s.Primary.ResetHome()
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

// ResolveTaskDetailDeepLink synthesizes a Task Workspace parent for direct
// task links. Missing projects request the project picker; missing tasks remain
// on that parent and return an error status.
func ResolveTaskDetailDeepLink(params TaskDetailParams, lookup RouteEntityLookup) (DeepLinkResolution, error) {
	parent, err := NewRoute(TaskWorkspace, TaskWorkspaceParams{
		Scope: ProjectEntityScope(params.ProjectID), Layout: WorkspaceOutline, Focus: WorkspaceActive,
	})
	if err != nil || lookup == nil || !lookup.ProjectExists(params.ProjectID) {
		home, homeErr := NewRoute(Home, HomeParams{})
		if homeErr != nil {
			return DeepLinkResolution{}, homeErr
		}
		picker, pickerErr := NewOverlay(OverlayProjectPicker, params.ProjectID)
		if pickerErr != nil {
			return DeepLinkResolution{}, pickerErr
		}
		return DeepLinkResolution{
			Parent: home, Overlay: &picker, Status: fmt.Sprintf("Project %q was not found", params.ProjectID),
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
	home, homeErr := NewRoute(Home, HomeParams{})
	if homeErr != nil {
		return DeepLinkResolution{}, homeErr
	}
	overlay, overlayErr := NewReferenceOverlay(route)
	if overlayErr != nil {
		return DeepLinkResolution{}, overlayErr
	}
	return DeepLinkResolution{Parent: home, Overlay: &overlay}, nil
}

func invalidDeepLinkResolution(cause error) (DeepLinkResolution, error) {
	home, err := NewRoute(Home, HomeParams{})
	if err != nil {
		return DeepLinkResolution{}, err
	}
	return DeepLinkResolution{Parent: home, Status: "Invalid deep link: " + cause.Error()}, nil
}

// ApplyDeepLink makes a resolved deep link atomic with overlay cancellation.
func (s *NavigationState) ApplyDeepLink(resolution DeepLinkResolution, restore FocusTarget) error {
	if resolution.Overlay != nil {
		s.ResetHome()
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
