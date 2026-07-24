package tui

import (
	"errors"
	"testing"
)

func mustRoute(t *testing.T, id RouteID, params RouteParams) Route {
	t.Helper()
	route, err := NewRoute(id, params)
	if err != nil {
		t.Fatalf("NewRoute(%s): %v", id, err)
	}
	return route
}

func TestRouteIDSetAndDestinations(t *testing.T) {
	want := []string{"TaskWorkspace", "SecondaryTool", "TaskDetail", "Reference"}
	for id, name := range want {
		routeID := RouteID(id)
		if routeID.String() != name {
			t.Errorf("RouteID(%d) = %q, want %q", id, routeID, name)
		}
	}
	if RouteID(len(want)).String() == "" {
		t.Error("invalid RouteID should have a diagnostic string")
	}
	if SecondaryTool.IsDestination() || TaskDetail.IsDestination() || Reference.IsDestination() {
		t.Error("SecondaryTool, TaskDetail, and Reference must not be destinations")
	}
	// The Outliner (TaskWorkspace) is the sole destination; Home and Planner
	// were deleted.
	if !TaskWorkspace.IsDestination() {
		t.Errorf("%s should be the sole destination", TaskWorkspace)
	}
}

func TestNewRouteValidatesAndCanonicalizesParams(t *testing.T) {
	if _, err := NewRoute(TaskWorkspace, ReviewParams{Module: ReviewDaily}); !errors.Is(err, ErrRouteParamsType) {
		t.Fatalf("TaskWorkspace with ReviewParams error = %v", err)
	}
	if _, err := NewRoute(RouteID(99), TaskWorkspaceParams{}); !errors.Is(err, ErrInvalidRouteID) {
		t.Fatalf("unknown ID error = %v", err)
	}

	workspace := mustRoute(t, TaskWorkspace, TaskWorkspaceParams{
		Scope: EntityScope{Kind: ScopeApp, ProjectID: "irrelevant"},
	}).Params().(TaskWorkspaceParams)
	if workspace.Scope != AppEntityScope() {
		t.Errorf("workspace params not canonical: %+v", workspace)
	}
	// TaskWorkspace is the single app-scoped Outliner root; any project scope is
	// rejected because project selection is an Outliner scope frame, not a route.
	if _, err := NewRoute(TaskWorkspace, TaskWorkspaceParams{
		Scope: EntityScope{Kind: ScopeProject},
	}); !errors.Is(err, ErrTaskWorkspaceProjectScope) {
		t.Fatalf("project workspace without project error = %v", err)
	}
	if _, err := NewRoute(TaskWorkspace, TaskWorkspaceParams{
		Scope: ProjectEntityScope("p1"),
	}); !errors.Is(err, ErrTaskWorkspaceProjectScope) {
		t.Fatalf("project-scoped workspace error = %v", err)
	}

	manage := mustRoute(t, SecondaryTool, ManageParams{
		Module:           ManageSettings,
		Scope:            EntityScope{Kind: ScopeProject, ProjectID: "p1"},
		NotificationMode: NotificationLog,
	}).Params().(ManageParams)
	if manage.Scope != AppEntityScope() || manage.NotificationMode != NotificationAlerts {
		t.Errorf("irrelevant manage params not canonical: %+v", manage)
	}
	if _, err := NewRoute(SecondaryTool, ManageParams{Module: ManageLanding}); !errors.Is(err, ErrInvalidRouteParams) {
		t.Fatalf("manage landing must be rejected as a secondary tool: %v", err)
	}
	if _, err := NewRoute(SecondaryTool, ReviewParams{Module: ReviewLanding}); !errors.Is(err, ErrInvalidRouteParams) {
		t.Fatalf("review landing must be rejected as a secondary tool: %v", err)
	}
	if _, err := NewRoute(SecondaryTool, TaskWorkspaceParams{}); !errors.Is(err, ErrRouteParamsType) {
		t.Fatalf("secondary tool with foreign params error = %v", err)
	}

	if _, err := NewRoute(TaskDetail, TaskDetailParams{ProjectID: "p1"}); !errors.Is(err, ErrTaskRequired) {
		t.Fatalf("detail without task error = %v", err)
	}
	if _, err := NewRoute(TaskDetail, TaskDetailParams{TaskID: "t1"}); !errors.Is(err, ErrProjectRequired) {
		t.Fatalf("detail without project error = %v", err)
	}
}
