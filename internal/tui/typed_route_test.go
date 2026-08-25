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
	want := []string{"TaskWorkspace", "SecondaryTool", "Reference"}
	for id, name := range want {
		routeID := RouteID(id)
		if routeID.String() != name {
			t.Errorf("RouteID(%d) = %q, want %q", id, routeID, name)
		}
	}
	if RouteID(len(want)).String() == "" {
		t.Error("invalid RouteID should have a diagnostic string")
	}
	if SecondaryTool.IsDestination() || Reference.IsDestination() {
		t.Error("SecondaryTool and Reference must not be destinations")
	}
	// The Outliner (TaskWorkspace) is the sole destination; Home and Planner
	// were deleted.
	if !TaskWorkspace.IsDestination() {
		t.Errorf("%s should be the sole destination", TaskWorkspace)
	}
}

func TestNewRouteValidatesAndCanonicalizesParams(t *testing.T) {
	if _, err := NewRoute(TaskWorkspace, ManageParams{Module: ManageNotes}); !errors.Is(err, ErrRouteParamsType) {
		t.Fatalf("TaskWorkspace with ManageParams error = %v", err)
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
		Module:     ManageProjects,
		NotesScope: ProjectNotesScope("p1"),
	}).Params().(ManageParams)
	if manage != (ManageParams{Module: ManageProjects}) {
		t.Errorf("irrelevant manage params not canonical: %+v", manage)
	}
	pinned := mustRoute(t, SecondaryTool, ManageParams{
		Module:         ManageProjects,
		ProjectHubMode: ProjectHubModePinned,
	}).Params().(ManageParams)
	if pinned != (ManageParams{Module: ManageProjects, ProjectHubMode: ProjectHubModePinned}) {
		t.Errorf("pinned Project Hub params not preserved: %+v", pinned)
	}
	allNotes := mustRoute(t, SecondaryTool, ManageParams{
		Module:     ManageNotes,
		NotesScope: NotesScope{Kind: NotesScopeAll, ProjectID: "irrelevant"},
	}).Params().(ManageParams)
	if allNotes.NotesScope != AllNotesScope() {
		t.Errorf("All Notes params not canonical: %+v", allNotes)
	}
	globalNotes := mustRoute(t, SecondaryTool, ManageParams{
		Module:     ManageNotes,
		NotesScope: NotesScope{Kind: NotesScopeGlobal, ProjectID: "irrelevant"},
	}).Params().(ManageParams)
	if globalNotes.NotesScope != GlobalNotesScope() {
		t.Errorf("Global Notes params not canonical: %+v", globalNotes)
	}
	projectNotes := mustRoute(t, SecondaryTool, ManageParams{
		Module:     ManageNotes,
		NotesScope: ProjectNotesScope("p1"),
	}).Params().(ManageParams)
	if projectNotes.NotesScope != ProjectNotesScope("p1") {
		t.Errorf("project Notes params not canonical: %+v", projectNotes)
	}
	if _, err := NewRoute(SecondaryTool, ManageParams{
		Module:     ManageNotes,
		NotesScope: ProjectNotesScope(""),
	}); !errors.Is(err, ErrProjectRequired) {
		t.Fatalf("project Notes without project error = %v", err)
	}
	if _, err := NewRoute(SecondaryTool, ManageParams{
		Module:     ManageNotes,
		NotesScope: NotesScope{Kind: NotesScopeKind(99)},
	}); !errors.Is(err, ErrInvalidRouteParams) {
		t.Fatalf("invalid Notes scope error = %v", err)
	}
	if _, err := NewRoute(SecondaryTool, ManageParams{
		Module:         ManageProjects,
		ProjectHubMode: ProjectHubMode(99),
	}); !errors.Is(err, ErrInvalidRouteParams) {
		t.Fatalf("invalid Project Hub mode error = %v", err)
	}
	canonicalNotes := mustRoute(t, SecondaryTool, ManageParams{
		Module:         ManageNotes,
		NotesScope:     AllNotesScope(),
		ProjectHubMode: ProjectHubModePinned,
	}).Params().(ManageParams)
	if canonicalNotes.ProjectHubMode != ProjectHubModeAll {
		t.Fatalf("Notes retained Project Hub mode: %+v", canonicalNotes)
	}
	if _, err := NewRoute(SecondaryTool, TaskWorkspaceParams{}); !errors.Is(err, ErrRouteParamsType) {
		t.Fatalf("secondary tool with foreign params error = %v", err)
	}

}
