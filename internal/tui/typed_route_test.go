package tui

import (
	"errors"
	"testing"

	"github.com/sglavoie/tt/internal/tui/route"
)

func mustRoute(t *testing.T, id route.RouteID, params route.RouteParams) route.Route {
	t.Helper()
	target, err := route.NewRoute(id, params)
	if err != nil {
		t.Fatalf("NewRoute(%s): %v", id, err)
	}
	return target
}

func TestRouteIDSetAndDestinations(t *testing.T) {
	want := []string{"TaskWorkspace", "SecondaryTool", "Reference"}
	for id, name := range want {
		routeID := route.RouteID(id)
		if routeID.String() != name {
			t.Errorf("RouteID(%d) = %q, want %q", id, routeID, name)
		}
	}
	if route.RouteID(len(want)).String() == "" {
		t.Error("invalid RouteID should have a diagnostic string")
	}
	if route.SecondaryTool.IsDestination() || route.Reference.IsDestination() {
		t.Error("SecondaryTool and Reference must not be destinations")
	}
	// The Outliner (TaskWorkspace) is the sole destination; Home and Planner
	// were deleted.
	if !route.TaskWorkspace.IsDestination() {
		t.Errorf("%s should be the sole destination", route.TaskWorkspace)
	}
}

func TestNewRouteValidatesAndCanonicalizesParams(t *testing.T) {
	if _, err := route.NewRoute(route.TaskWorkspace, route.ManageParams{Module: route.ManageNotes}); !errors.Is(err, route.ErrRouteParamsType) {
		t.Fatalf("TaskWorkspace with ManageParams error = %v", err)
	}
	if _, err := route.NewRoute(route.RouteID(99), route.TaskWorkspaceParams{}); !errors.Is(err, route.ErrInvalidRouteID) {
		t.Fatalf("unknown ID error = %v", err)
	}

	workspace := mustRoute(t, route.TaskWorkspace, route.TaskWorkspaceParams{
		Scope: route.EntityScope{Kind: route.ScopeApp, ProjectID: "irrelevant"},
	}).Params().(route.TaskWorkspaceParams)
	if workspace.Scope != route.AppEntityScope() {
		t.Errorf("workspace params not canonical: %+v", workspace)
	}
	// TaskWorkspace is the single app-scoped Outliner root; any project scope is
	// rejected because project selection is an Outliner scope frame, not a route.
	if _, err := route.NewRoute(route.TaskWorkspace, route.TaskWorkspaceParams{
		Scope: route.EntityScope{Kind: route.ScopeProject},
	}); !errors.Is(err, route.ErrTaskWorkspaceProjectScope) {
		t.Fatalf("project workspace without project error = %v", err)
	}
	if _, err := route.NewRoute(route.TaskWorkspace, route.TaskWorkspaceParams{
		Scope: route.ProjectEntityScope("p1"),
	}); !errors.Is(err, route.ErrTaskWorkspaceProjectScope) {
		t.Fatalf("project-scoped workspace error = %v", err)
	}

	manage := mustRoute(t, route.SecondaryTool, route.ManageParams{
		Module:     route.ManageProjects,
		NotesScope: route.ProjectNotesScope("p1"),
	}).Params().(route.ManageParams)
	if manage != (route.ManageParams{Module: route.ManageProjects}) {
		t.Errorf("irrelevant manage params not canonical: %+v", manage)
	}
	pinned := mustRoute(t, route.SecondaryTool, route.ManageParams{
		Module:         route.ManageProjects,
		ProjectHubMode: route.ProjectHubModePinned,
	}).Params().(route.ManageParams)
	if pinned != (route.ManageParams{Module: route.ManageProjects, ProjectHubMode: route.ProjectHubModePinned}) {
		t.Errorf("pinned Project Hub params not preserved: %+v", pinned)
	}
	allNotes := mustRoute(t, route.SecondaryTool, route.ManageParams{
		Module:     route.ManageNotes,
		NotesScope: route.NotesScope{Kind: route.NotesScopeAll, ProjectID: "irrelevant"},
	}).Params().(route.ManageParams)
	if allNotes.NotesScope != route.AllNotesScope() {
		t.Errorf("All Notes params not canonical: %+v", allNotes)
	}
	globalNotes := mustRoute(t, route.SecondaryTool, route.ManageParams{
		Module:     route.ManageNotes,
		NotesScope: route.NotesScope{Kind: route.NotesScopeGlobal, ProjectID: "irrelevant"},
	}).Params().(route.ManageParams)
	if globalNotes.NotesScope != route.GlobalNotesScope() {
		t.Errorf("Global Notes params not canonical: %+v", globalNotes)
	}
	projectNotes := mustRoute(t, route.SecondaryTool, route.ManageParams{
		Module:     route.ManageNotes,
		NotesScope: route.ProjectNotesScope("p1"),
	}).Params().(route.ManageParams)
	if projectNotes.NotesScope != route.ProjectNotesScope("p1") {
		t.Errorf("project Notes params not canonical: %+v", projectNotes)
	}
	if _, err := route.NewRoute(route.SecondaryTool, route.ManageParams{
		Module:     route.ManageNotes,
		NotesScope: route.ProjectNotesScope(""),
	}); !errors.Is(err, route.ErrProjectRequired) {
		t.Fatalf("project Notes without project error = %v", err)
	}
	if _, err := route.NewRoute(route.SecondaryTool, route.ManageParams{
		Module:     route.ManageNotes,
		NotesScope: route.NotesScope{Kind: route.NotesScopeKind(99)},
	}); !errors.Is(err, route.ErrInvalidRouteParams) {
		t.Fatalf("invalid Notes scope error = %v", err)
	}
	if _, err := route.NewRoute(route.SecondaryTool, route.ManageParams{
		Module:         route.ManageProjects,
		ProjectHubMode: route.ProjectHubMode(99),
	}); !errors.Is(err, route.ErrInvalidRouteParams) {
		t.Fatalf("invalid Project Hub mode error = %v", err)
	}
	canonicalNotes := mustRoute(t, route.SecondaryTool, route.ManageParams{
		Module:         route.ManageNotes,
		NotesScope:     route.AllNotesScope(),
		ProjectHubMode: route.ProjectHubModePinned,
	}).Params().(route.ManageParams)
	if canonicalNotes.ProjectHubMode != route.ProjectHubModeAll {
		t.Fatalf("Notes retained Project Hub mode: %+v", canonicalNotes)
	}
	if _, err := route.NewRoute(route.SecondaryTool, route.TaskWorkspaceParams{}); !errors.Is(err, route.ErrRouteParamsType) {
		t.Fatalf("secondary tool with foreign params error = %v", err)
	}

}
