package tui

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
	// Project selection is an Outliner scope frame, not a project-rooted route.
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

	detail := mustRoute(t, TaskDetail, TaskDetailParams{ProjectID: "p1", TaskID: "t1"})
	focus := FocusTarget{ControlID: "task-list", EntityID: "t1"}
	if err := stack.PushTaskDetail(detail, focus); err != nil {
		t.Fatal(err)
	}
	popped, restored, ok := stack.Pop()
	if !ok || popped.ID() != TaskDetail || restored != focus || stack.Current().ID() != TaskWorkspace {
		t.Fatalf("pop = %s restore=%+v ok=%v current=%s", popped.ID(), restored, ok, stack.Current().ID())
	}
	stack.ResetRoot()
	if stack.Depth() != 1 || stack.Current().ID() != TaskWorkspace {
		t.Fatalf("reset stack = %+v", stack.Routes())
	}
	if _, _, ok := stack.Pop(); ok {
		t.Fatal("Outliner root must not be popped")
	}
}

func TestPrimaryRouteStackBoundsChildrenAndPreservesRootFocus(t *testing.T) {
	stack := NewPrimaryRouteStack()
	rootFocus := FocusTarget{ControlID: "outliner", EntityID: "task-1"}
	detail := mustRoute(t, TaskDetail, TaskDetailParams{ProjectID: "p1", TaskID: "task-1"})
	if err := stack.PushTaskDetail(detail, rootFocus); err != nil {
		t.Fatal(err)
	}

	secondary := mustRoute(t, SecondaryTool, ManageParams{Module: ManageContexts})
	if err := stack.PushChild(secondary, FocusTarget{ControlID: "detail-tab"}); err != nil {
		t.Fatal(err)
	}
	if stack.Depth() != 2 || stack.Current().ID() != SecondaryTool {
		t.Fatalf("replaced child stack = %+v, want root + SecondaryTool", stack.Routes())
	}

	popped, restored, ok := stack.Pop()
	if !ok || popped.ID() != SecondaryTool || restored != rootFocus {
		t.Fatalf("pop = %s restore=%+v ok=%v, want SecondaryTool and original root focus %+v",
			popped.ID(), restored, ok, rootFocus)
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

func TestOverlayStackNestingFocusAndCancellation(t *testing.T) {
	var canceled []string
	first, _ := NewOverlay(OverlaySearch, "search")
	second, _ := NewOverlay(OverlayPicker, "status")
	stack := OverlayStack{}
	stack.Push(first, FocusTarget{ControlID: "workspace", EntityID: "t1"}, func() {
		canceled = append(canceled, "search")
	})
	stack.Push(second, FocusTarget{ControlID: "search-query"}, func() {
		canceled = append(canceled, "status")
	})

	closed, restored, ok := stack.CloseTop()
	if !ok || closed.Kind() != OverlayPicker || restored.ControlID != "search-query" {
		t.Fatalf("close top = %s restore=%+v ok=%v", closed.Kind(), restored, ok)
	}
	if len(canceled) != 0 {
		t.Errorf("successful close invoked cancellation: %v", canceled)
	}
	stack.Push(second, FocusTarget{}, func() { canceled = append(canceled, "status") })
	stack.CancelAll()
	if stack.Active() || !reflect.DeepEqual(canceled, []string{"status", "search"}) {
		t.Errorf("cancel order = %v active=%v", canceled, stack.Active())
	}
}

func TestNavigationDestinationChangeCancelsOverlays(t *testing.T) {
	state := NewNavigationState()
	overlay, _ := NewOverlay(OverlayComposer, "new-task")
	canceled := false
	state.Overlays.Push(overlay, FocusTarget{}, func() { canceled = true })
	workspace := mustRoute(t, TaskWorkspace, TaskWorkspaceParams{Scope: AppEntityScope()})
	if err := state.ReplaceDestination(workspace); err != nil {
		t.Fatal(err)
	}
	if !canceled || state.Overlays.Active() || state.Primary.Current().ID() != TaskWorkspace {
		t.Fatalf("destination transition canceled=%v overlay=%v current=%s", canceled, state.Overlays.Active(), state.Primary.Current().ID())
	}
}

type fakeRouteLookup struct {
	projects map[string]bool
	tasks    map[string]bool
}

func (f fakeRouteLookup) ProjectExists(id string) bool { return f.projects[id] }
func (f fakeRouteLookup) TaskExists(projectID, taskID string) bool {
	return f.tasks[projectID+"/"+taskID]
}

func TestResolveTaskDetailDeepLinkFallbacks(t *testing.T) {
	lookup := fakeRouteLookup{
		projects: map[string]bool{"p1": true},
		tasks:    map[string]bool{"p1/t1": true},
	}

	t.Run("missing project opens picker", func(t *testing.T) {
		resolution, err := ResolveTaskDetailDeepLink(TaskDetailParams{ProjectID: "gone", TaskID: "t1"}, lookup)
		if err != nil {
			t.Fatal(err)
		}
		if resolution.Parent.ID() != TaskWorkspace || resolution.Overlay == nil || resolution.Overlay.Kind() != OverlayProjectPicker || resolution.Status == "" {
			t.Fatalf("resolution = %+v", resolution)
		}
	})

	t.Run("missing task returns workspace parent", func(t *testing.T) {
		resolution, err := ResolveTaskDetailDeepLink(TaskDetailParams{ProjectID: "p1", TaskID: "gone"}, lookup)
		if err != nil {
			t.Fatal(err)
		}
		if resolution.Parent.ID() != TaskWorkspace || resolution.Detail != nil || resolution.Overlay != nil || resolution.Status == "" {
			t.Fatalf("resolution = %+v", resolution)
		}
	})

	t.Run("valid link synthesizes parent and detail", func(t *testing.T) {
		resolution, err := ResolveTaskDetailDeepLink(TaskDetailParams{ProjectID: "p1", TaskID: "t1", Tab: TaskComments}, lookup)
		if err != nil {
			t.Fatal(err)
		}
		if resolution.Parent.ID() != TaskWorkspace || resolution.Detail == nil || resolution.Detail.ID() != TaskDetail || resolution.Status != "" {
			t.Fatalf("resolution = %+v", resolution)
		}
		state := NewNavigationState()
		focus := FocusTarget{ControlID: "task-list", EntityID: "t1"}
		if err := state.ApplyDeepLink(resolution, focus); err != nil {
			t.Fatal(err)
		}
		if state.Primary.Depth() != 2 || state.Primary.Current().ID() != TaskDetail {
			t.Fatalf("applied routes = %+v", state.Primary.Routes())
		}
	})

	t.Run("invalid tab returns safe parent", func(t *testing.T) {
		resolution, err := ResolveTaskDetailDeepLink(TaskDetailParams{
			ProjectID: "p1", TaskID: "t1", Tab: TaskDetailTab(99),
		}, lookup)
		if err != nil {
			t.Fatal(err)
		}
		if resolution.Parent.ID() != TaskWorkspace || resolution.Detail != nil || resolution.Status == "" {
			t.Fatalf("resolution = %+v", resolution)
		}
	})
}

func TestResolveInvalidAndReferenceDeepLinks(t *testing.T) {
	invalid, err := ResolveDeepLink(SecondaryTool, TaskDetailParams{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if invalid.Parent.ID() != TaskWorkspace || invalid.Status == "" || invalid.Overlay != nil {
		t.Fatalf("invalid resolution = %+v", invalid)
	}

	reference, err := ResolveDeepLink(Reference, ReferenceParams{Kind: ReferenceHelp}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if reference.Parent.ID() != TaskWorkspace || reference.Overlay == nil || reference.Overlay.Kind() != OverlayReference {
		t.Fatalf("reference resolution = %+v", reference)
	}
}
