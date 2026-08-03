package tui

import (
	"errors"
	"fmt"
)

var ErrInvalidRouteTransition = errors.New("invalid primary route transition")

type primaryRouteFrame struct {
	route Route
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
// destination switch replaces that root frame; secondary tools are pushed as
// child routes above it.
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

// PushChild installs the one child route above the Outliner root and records the
// parent focus bookmark to restore on pop. Secondary tools never nest; switching
// modules replaces the existing child so the
// primary stack remains bounded to root + one child.
func (s *PrimaryRouteStack) PushChild(route Route) error {
	s.ensureRoot()
	switch route.ID() {
	case SecondaryTool:
	default:
		return fmt.Errorf("%w: %s is not a child route", ErrInvalidRouteTransition, route.ID())
	}
	if len(s.frames) > 1 {
		s.frames[len(s.frames)-1].route = route
		return nil
	}
	s.frames = append(s.frames, primaryRouteFrame{route: route})
	return nil
}

// ReplaceCurrentMode changes the parameters of the active route without
// changing its parentage. Consolidated hub modules are modes, not child routes.
func (s *PrimaryRouteStack) ReplaceCurrentMode(route Route) error {
	s.ensureRoot()
	current := s.Current()
	if current.ID() != route.ID() {
		return fmt.Errorf("%w: cannot replace %s with %s as a mode", ErrInvalidRouteTransition, current.ID(), route.ID())
	}
	s.frames[len(s.frames)-1].route = route
	return nil
}

// Pop removes a child route.
func (s *PrimaryRouteStack) Pop() (popped Route, ok bool) {
	s.ensureRoot()
	if len(s.frames) == 1 {
		return Route{}, false
	}
	frame := s.frames[len(s.frames)-1]
	s.frames = s.frames[:len(s.frames)-1]
	return frame.route, true
}

// ResetRoot collapses the stack back to the sole Outliner root, dropping every
// child route above it.
func (s *PrimaryRouteStack) ResetRoot() { s.ensureRoot(); s.frames = s.frames[:1] }

type overlayFrame struct {
	overlay Overlay
	cancel  func()
}

// OverlayStack owns nested transient presentations. Close is a successful
// dismissal; Cancel invokes the overlay's cancellation callback exactly once.
type OverlayStack struct{ frames []overlayFrame }

func (s *OverlayStack) Push(overlay Overlay, cancel func()) {
	s.frames = append(s.frames, overlayFrame{overlay: overlay, cancel: cancel})
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

func (s *OverlayStack) CloseTop() (Overlay, bool) {
	return s.removeTop(false)
}

func (s *OverlayStack) CancelTop() (Overlay, bool) {
	return s.removeTop(true)
}

func (s *OverlayStack) removeTop(cancel bool) (Overlay, bool) {
	if len(s.frames) == 0 {
		return Overlay{}, false
	}
	index := len(s.frames) - 1
	frame := s.frames[index]
	s.frames = s.frames[:index]
	if cancel && frame.cancel != nil {
		frame.cancel()
	}
	return frame.overlay, true
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
func (s *NavigationState) PushChild(route Route) error {
	s.Overlays.CancelAll()
	return s.Primary.PushChild(route)
}
