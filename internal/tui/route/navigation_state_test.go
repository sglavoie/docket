package route

import (
	"errors"
	"reflect"
	"testing"
)

func TestPrimaryRouteStackPushReplacePopReset(t *testing.T) {
	stack := NewPrimaryRouteStack()
	if stack.Depth() != 1 || stack.Current().ID() != TaskWorkspace {
		t.Fatalf("initial stack = %+v", stack.Routes())
	}

	// The Outliner (TaskWorkspace) is the sole destination and is app-scoped;
	// replacing the destination swaps the single root frame rather than stacking.
	// Project selection is an Outliner scope frame, not a project-rooted
	replacement := mustRoute(t, TaskWorkspace, TaskWorkspaceParams{Scope: AppEntityScope()})
	if err := stack.ReplaceDestination(replacement); err != nil {
		t.Fatal(err)
	}
	if stack.Depth() != 1 || stack.Current().ID() != TaskWorkspace {
		t.Fatalf("destination replacement = %+v", stack.Routes())
	}
	if err := stack.ReplaceDestination(mustRoute(t, Reference, ReferenceParams{})); err == nil {
		t.Fatal("Reference unexpectedly accepted as a destination")
	}

	secondary := mustRoute(t, SecondaryTool, ManageParams{Module: ManageProjects})
	if err := stack.PushChild(secondary); err != nil {
		t.Fatal(err)
	}
	popped, ok := stack.Pop()
	if !ok || popped.ID() != SecondaryTool || stack.Current().ID() != TaskWorkspace {
		t.Fatalf("pop = %s ok=%v current=%s", popped.ID(), ok, stack.Current().ID())
	}
	stack.ResetRoot()
	if stack.Depth() != 1 || stack.Current().ID() != TaskWorkspace {
		t.Fatalf("reset stack = %+v", stack.Routes())
	}
	if _, ok := stack.Pop(); ok {
		t.Fatal("Outliner root must not be popped")
	}
}

func TestPrimaryRouteStackBoundsChildren(t *testing.T) {
	stack := NewPrimaryRouteStack()
	first := mustRoute(t, SecondaryTool, ManageParams{Module: ManageProjects})
	if err := stack.PushChild(first); err != nil {
		t.Fatal(err)
	}

	secondary := mustRoute(t, SecondaryTool, ManageParams{Module: ManageProjects})
	if err := stack.PushChild(secondary); err != nil {
		t.Fatal(err)
	}
	if stack.Depth() != 2 || stack.Current().ID() != SecondaryTool {
		t.Fatalf("replaced child stack = %+v, want root + SecondaryTool", stack.Routes())
	}

	popped, ok := stack.Pop()
	if !ok || popped.ID() != SecondaryTool {
		t.Fatalf("pop = %s ok=%v, want SecondaryTool", popped.ID(), ok)
	}
	if stack.Depth() != 1 || stack.Current().ID() != TaskWorkspace {
		t.Fatalf("stack after child pop = %+v, want sole Outliner root", stack.Routes())
	}
}

// TestTaskWorkspaceRejectsProjectRouteScope locks the FR15-001 contract: the
// Outliner root is app-scoped only. A project-scoped TaskWorkspace is rejected
// at construction (project selection is an Outliner scope frame, not a route),
// so the route stack can never replace its root with a second project root.
func TestTaskWorkspaceRejectsProjectRouteScope(t *testing.T) {
	if _, err := NewRoute(TaskWorkspace, TaskWorkspaceParams{Scope: ProjectEntityScope("p1")}); !errors.Is(err, ErrTaskWorkspaceProjectScope) {
		t.Fatalf("project-scoped TaskWorkspace error = %v, want ErrTaskWorkspaceProjectScope", err)
	}

	// App scope remains the one valid destination scope.
	app, err := NewRoute(TaskWorkspace, TaskWorkspaceParams{Scope: AppEntityScope()})
	if err != nil {
		t.Fatalf("app-scoped TaskWorkspace unexpectedly rejected: %v", err)
	}
	params, ok := app.Params().(TaskWorkspaceParams)
	if !ok || params.Scope.Kind != ScopeApp || params.Scope.ProjectID != "" {
		t.Fatalf("app-scoped params = %+v, want canonical app scope", app.Params())
	}

	// The route stack cannot be pointed at a project root because no such route
	// can be built; ReplaceDestination only ever receives the app-scoped root.
	stack := NewPrimaryRouteStack()
	if err := stack.ReplaceDestination(app); err != nil {
		t.Fatalf("app-scoped destination replacement failed: %v", err)
	}
	if stack.Depth() != 1 || stack.Current().ID() != TaskWorkspace {
		t.Fatalf("stack after replacement = %+v", stack.Routes())
	}
	if got := stack.Current().Params().(TaskWorkspaceParams); got.Scope.Kind != ScopeApp {
		t.Fatalf("root scope = %+v, want app-scoped root", got.Scope)
	}
}

func TestOverlayStackNestingAndCancellation(t *testing.T) {
	var canceled []string
	first, _ := NewOverlay(OverlayForm, "search")
	second, _ := NewOverlay(OverlayPicker, "status")
	stack := OverlayStack{}
	stack.Push(first, func() {
		canceled = append(canceled, "search")
	})
	stack.Push(second, func() {
		canceled = append(canceled, "status")
	})

	closed, ok := stack.CloseTop()
	if !ok || closed.Kind() != OverlayPicker {
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
	state := NewNavigationState()
	overlay, _ := NewOverlay(OverlayComposer, "new-task")
	canceled := false
	state.Overlays.Push(overlay, func() { canceled = true })
	workspace := mustRoute(t, TaskWorkspace, TaskWorkspaceParams{Scope: AppEntityScope()})
	if err := state.ReplaceDestination(workspace); err != nil {
		t.Fatal(err)
	}
	if !canceled || state.Overlays.Active() || state.Primary.Current().ID() != TaskWorkspace {
		t.Fatalf("destination transition canceled=%v overlay=%v current=%s", canceled, state.Overlays.Active(), state.Primary.Current().ID())
	}
}
