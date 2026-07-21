package tui

import (
	"reflect"
	"testing"
)

func TestPrimaryRouteStackPushReplacePopReset(t *testing.T) {
	stack := NewPrimaryRouteStack()
	if stack.Depth() != 1 || stack.Current().ID() != Home {
		t.Fatalf("initial stack = %+v", stack.Routes())
	}

	planner := mustRoute(t, Planner, PlannerParams{Tab: PlannerNext})
	if err := stack.ReplaceDestination(planner); err != nil {
		t.Fatal(err)
	}
	if stack.Depth() != 2 || stack.Current().ID() != Planner {
		t.Fatalf("planner replacement = %+v", stack.Routes())
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
	if !ok || popped.ID() != TaskDetail || restored != focus || stack.Current().ID() != Planner {
		t.Fatalf("pop = %s restore=%+v ok=%v current=%s", popped.ID(), restored, ok, stack.Current().ID())
	}
	stack.ResetHome()
	if stack.Depth() != 1 || stack.Current().ID() != Home {
		t.Fatalf("reset stack = %+v", stack.Routes())
	}
	if _, _, ok := stack.Pop(); ok {
		t.Fatal("Home root must not be popped")
	}
}

func TestOverlayStackNestingFocusAndCancellation(t *testing.T) {
	var canceled []string
	first, _ := NewOverlay(OverlayPalette, "palette")
	second, _ := NewOverlay(OverlayPicker, "status")
	stack := OverlayStack{}
	stack.Push(first, FocusTarget{ControlID: "workspace", EntityID: "t1"}, func() {
		canceled = append(canceled, "palette")
	})
	stack.Push(second, FocusTarget{ControlID: "palette-query"}, func() {
		canceled = append(canceled, "status")
	})

	closed, restored, ok := stack.CloseTop()
	if !ok || closed.Kind() != OverlayPicker || restored.ControlID != "palette-query" {
		t.Fatalf("close top = %s restore=%+v ok=%v", closed.Kind(), restored, ok)
	}
	if len(canceled) != 0 {
		t.Errorf("successful close invoked cancellation: %v", canceled)
	}
	stack.Push(second, FocusTarget{}, func() { canceled = append(canceled, "status") })
	stack.CancelAll()
	if stack.Active() || !reflect.DeepEqual(canceled, []string{"status", "palette"}) {
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
		if resolution.Parent.ID() != Home || resolution.Overlay == nil || resolution.Overlay.Kind() != OverlayProjectPicker || resolution.Status == "" {
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
		if state.Primary.Depth() != 3 || state.Primary.Current().ID() != TaskDetail {
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
	invalid, err := ResolveDeepLink(Planner, TaskDetailParams{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if invalid.Parent.ID() != Home || invalid.Status == "" || invalid.Overlay != nil {
		t.Fatalf("invalid resolution = %+v", invalid)
	}

	reference, err := ResolveDeepLink(Reference, ReferenceParams{Kind: ReferenceHelp}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if reference.Parent.ID() != Home || reference.Overlay == nil || reference.Overlay.Kind() != OverlayReference {
		t.Fatalf("reference resolution = %+v", reference)
	}
}
