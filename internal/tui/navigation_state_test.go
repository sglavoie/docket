package tui

import (
	"errors"
	"reflect"
	"testing"

	"github.com/sglavoie/tt/internal/tui/route"
)

func TestPrimaryRouteStackPushReplacePopReset(t *testing.T) {
	stack := route.NewPrimaryRouteStack()
	if stack.Depth() != 1 || stack.Current().ID() != route.TaskWorkspace {
		t.Fatalf("initial stack = %+v", stack.Routes())
	}

	// The Outliner (TaskWorkspace) is the sole destination and is app-scoped;
	// replacing the destination swaps the single root frame rather than stacking.
	// Project selection is an Outliner scope frame, not a project-rooted route.
	replacement := mustRoute(t, route.TaskWorkspace, route.TaskWorkspaceParams{Scope: route.AppEntityScope()})
	if err := stack.ReplaceDestination(replacement); err != nil {
		t.Fatal(err)
	}
	if stack.Depth() != 1 || stack.Current().ID() != route.TaskWorkspace {
		t.Fatalf("destination replacement = %+v", stack.Routes())
	}
	if err := stack.ReplaceDestination(mustRoute(t, route.Reference, route.ReferenceParams{})); err == nil {
		t.Fatal("Reference unexpectedly accepted as a destination")
	}

	secondary := mustRoute(t, route.SecondaryTool, route.ManageParams{Module: route.ManageProjects})
	if err := stack.PushChild(secondary); err != nil {
		t.Fatal(err)
	}
	popped, ok := stack.Pop()
	if !ok || popped.ID() != route.SecondaryTool || stack.Current().ID() != route.TaskWorkspace {
		t.Fatalf("pop = %s ok=%v current=%s", popped.ID(), ok, stack.Current().ID())
	}
	stack.ResetRoot()
	if stack.Depth() != 1 || stack.Current().ID() != route.TaskWorkspace {
		t.Fatalf("reset stack = %+v", stack.Routes())
	}
	if _, ok := stack.Pop(); ok {
		t.Fatal("Outliner root must not be popped")
	}
}

func TestPrimaryRouteStackBoundsChildren(t *testing.T) {
	stack := route.NewPrimaryRouteStack()
	first := mustRoute(t, route.SecondaryTool, route.ManageParams{Module: route.ManageProjects})
	if err := stack.PushChild(first); err != nil {
		t.Fatal(err)
	}

	secondary := mustRoute(t, route.SecondaryTool, route.ManageParams{Module: route.ManageProjects})
	if err := stack.PushChild(secondary); err != nil {
		t.Fatal(err)
	}
	if stack.Depth() != 2 || stack.Current().ID() != route.SecondaryTool {
		t.Fatalf("replaced child stack = %+v, want root + SecondaryTool", stack.Routes())
	}

	popped, ok := stack.Pop()
	if !ok || popped.ID() != route.SecondaryTool {
		t.Fatalf("pop = %s ok=%v, want SecondaryTool", popped.ID(), ok)
	}
	if stack.Depth() != 1 || stack.Current().ID() != route.TaskWorkspace {
		t.Fatalf("stack after child pop = %+v, want sole Outliner root", stack.Routes())
	}
}

// TestTaskWorkspaceRejectsProjectRouteScope locks the FR15-001 contract: the
// Outliner root is app-scoped only. A project-scoped TaskWorkspace is rejected
// at construction (project selection is an Outliner scope frame, not a route),
// so the route stack can never replace its root with a second project root.
func TestTaskWorkspaceRejectsProjectRouteScope(t *testing.T) {
	if _, err := route.NewRoute(route.TaskWorkspace, route.TaskWorkspaceParams{Scope: route.ProjectEntityScope("p1")}); !errors.Is(err, route.ErrTaskWorkspaceProjectScope) {
		t.Fatalf("project-scoped TaskWorkspace error = %v, want ErrTaskWorkspaceProjectScope", err)
	}

	// App scope remains the one valid destination scope.
	app, err := route.NewRoute(route.TaskWorkspace, route.TaskWorkspaceParams{Scope: route.AppEntityScope()})
	if err != nil {
		t.Fatalf("app-scoped TaskWorkspace unexpectedly rejected: %v", err)
	}
	params, ok := app.Params().(route.TaskWorkspaceParams)
	if !ok || params.Scope.Kind != route.ScopeApp || params.Scope.ProjectID != "" {
		t.Fatalf("app-scoped params = %+v, want canonical app scope", app.Params())
	}

	// The route stack cannot be pointed at a project root because no such route
	// can be built; ReplaceDestination only ever receives the app-scoped root.
	stack := route.NewPrimaryRouteStack()
	if err := stack.ReplaceDestination(app); err != nil {
		t.Fatalf("app-scoped destination replacement failed: %v", err)
	}
	if stack.Depth() != 1 || stack.Current().ID() != route.TaskWorkspace {
		t.Fatalf("stack after replacement = %+v", stack.Routes())
	}
	if got := stack.Current().Params().(route.TaskWorkspaceParams); got.Scope.Kind != route.ScopeApp {
		t.Fatalf("root scope = %+v, want app-scoped root", got.Scope)
	}
}

func TestOverlayStackNestingAndCancellation(t *testing.T) {
	var canceled []string
	first, _ := route.NewOverlay(route.OverlayForm, "search")
	second, _ := route.NewOverlay(route.OverlayPicker, "status")
	stack := route.OverlayStack{}
	stack.Push(first, func() {
		canceled = append(canceled, "search")
	})
	stack.Push(second, func() {
		canceled = append(canceled, "status")
	})

	closed, ok := stack.CloseTop()
	if !ok || closed.Kind() != route.OverlayPicker {
		t.Fatalf("close top = %s ok=%v", closed.Kind(), ok)
	}
	if len(canceled) != 0 {
		t.Errorf("successful close invoked cancellation: %v", canceled)
	}
	stack.Push(second, func() { canceled = append(canceled, "status") })
	stack.CancelAll()
	if stack.Active() || !reflect.DeepEqual(canceled, []string{"status", "search"}) {
		t.Errorf("cancel order = %v active=%v", canceled, stack.Active())
	}
}

func TestNavigationDestinationChangeCancelsOverlays(t *testing.T) {
	state := route.NewNavigationState()
	overlay, _ := route.NewOverlay(route.OverlayComposer, "new-task")
	canceled := false
	state.Overlays.Push(overlay, func() { canceled = true })
	workspace := mustRoute(t, route.TaskWorkspace, route.TaskWorkspaceParams{Scope: route.AppEntityScope()})
	if err := state.ReplaceDestination(workspace); err != nil {
		t.Fatal(err)
	}
	if !canceled || state.Overlays.Active() || state.Primary.Current().ID() != route.TaskWorkspace {
		t.Fatalf("destination transition canceled=%v overlay=%v current=%s", canceled, state.Overlays.Active(), state.Primary.Current().ID())
	}
}
