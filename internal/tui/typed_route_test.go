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
	want := []string{"Home", "Planner", "TaskWorkspace", "ReviewHub", "ManageHub", "TaskDetail", "Reference"}
	for id, name := range want {
		routeID := RouteID(id)
		if routeID.String() != name {
			t.Errorf("RouteID(%d) = %q, want %q", id, routeID, name)
		}
	}
	if RouteID(len(want)).String() == "" {
		t.Error("invalid RouteID should have a diagnostic string")
	}
	if TaskDetail.IsDestination() || Reference.IsDestination() {
		t.Error("TaskDetail and Reference must not be Ctrl+G destinations")
	}
	for _, id := range []RouteID{Home, Planner, TaskWorkspace, ReviewHub, ManageHub} {
		if !id.IsDestination() {
			t.Errorf("%s should be a destination", id)
		}
	}
}

func TestNewRouteValidatesAndCanonicalizesParams(t *testing.T) {
	if _, err := NewRoute(Home, PlannerParams{}); !errors.Is(err, ErrRouteParamsType) {
		t.Fatalf("Home with PlannerParams error = %v", err)
	}
	if _, err := NewRoute(RouteID(99), HomeParams{}); !errors.Is(err, ErrInvalidRouteID) {
		t.Fatalf("unknown ID error = %v", err)
	}

	workspace := mustRoute(t, TaskWorkspace, TaskWorkspaceParams{
		Scope: EntityScope{Kind: ScopeApp, ProjectID: "irrelevant"},
	}).Params().(TaskWorkspaceParams)
	if workspace.Scope != AppEntityScope() {
		t.Errorf("workspace params not canonical: %+v", workspace)
	}
	if _, err := NewRoute(TaskWorkspace, TaskWorkspaceParams{
		Scope: EntityScope{Kind: ScopeProject},
	}); !errors.Is(err, ErrProjectRequired) {
		t.Fatalf("project workspace without project error = %v", err)
	}

	manage := mustRoute(t, ManageHub, ManageParams{
		Module:           ManageSettings,
		Scope:            EntityScope{Kind: ScopeProject, ProjectID: "p1"},
		NotificationMode: NotificationLog,
	}).Params().(ManageParams)
	if manage.Scope != AppEntityScope() || manage.NotificationMode != NotificationAlerts {
		t.Errorf("irrelevant manage params not canonical: %+v", manage)
	}

	if _, err := NewRoute(TaskDetail, TaskDetailParams{ProjectID: "p1"}); !errors.Is(err, ErrTaskRequired) {
		t.Fatalf("detail without task error = %v", err)
	}
	if _, err := NewRoute(TaskDetail, TaskDetailParams{TaskID: "t1"}); !errors.Is(err, ErrProjectRequired) {
		t.Fatalf("detail without project error = %v", err)
	}
}
