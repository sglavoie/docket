package tui

import (
	"errors"
	"slices"
	"testing"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/listsurface"
	"github.com/sglavoie/tt/internal/tui/operation"
)

// projectHeaderCursorApp parks the fixture Outliner's cursor on a project header
// row and returns the app, that row's project, and the context dispatch builds
// from it. Going through the real fixture rather than a hand-built EntityContext
// is the point: HasProjectHeaderCursor, the frozen project identity, and the
// scope IDs all have to agree, and only the production path makes them agree.
func projectHeaderCursorApp(t *testing.T) (*AppModel, *models.Project, operation.CommandContext) {
	t.Helper()
	driver, workspace, err := NewFixtureDriver(FixtureRich, 100, 35)
	if err != nil {
		t.Fatalf("fixture: %v", err)
	}
	t.Cleanup(func() { _ = workspace.Cleanup() })
	app := driver.App()
	outlinerView := app.crossOutliner
	if outlinerView == nil {
		t.Fatal("fixture app has no Outliner")
	}
	for index, row := range outlinerView.Rows {
		if !row.IsProjectHeader {
			continue
		}
		outlinerView.Cursor = index
		context := app.operationCommandContext()
		project := app.projectByID(row.ProjectID)
		if project == nil {
			t.Fatalf("project header row names unknown project %q", row.ProjectID)
		}
		return app, project, context
	}
	t.Fatal("fixture Outliner exposed no project header row")
	return nil, nil, operation.CommandContext{}
}

// TestProjectActionSheetProjectsTheOutlinerProjectManifest pins the exact rows
// Space offers on a project header. The sheet is the only route to any of them,
// so an operation silently added to or dropped from the manifest changes what a
// user can do to a project without changing a key.
func TestProjectActionSheetProjectsTheOutlinerProjectManifest(t *testing.T) {
	app, project, context := projectHeaderCursorApp(t)
	if project.Archived {
		t.Fatalf("fixture project %q is archived; an Outliner header is drawn from the active population", project.Name)
	}
	registry := app.GetOperationRegistry()

	ids := ProjectActionSheetOperationIDs(registry, context)
	wantIDs := []operation.OperationID{
		operation.OperationScopeRenameProject,
		operation.OperationScopeArchiveProject,
		operation.OperationProjectDelete,
	}
	if !slices.Equal(ids, wantIDs) {
		t.Fatalf("project sheet IDs = %v, want exact three %v", ids, wantIDs)
	}
	wantLabels := []string{
		"Edit project", "Archive project", "Delete project",
	}
	for index, id := range ids {
		descriptor, ok := registry.Descriptor(id)
		if !ok {
			t.Fatalf("sheet row %d (%s) is not registered", index, id)
		}
		if descriptor.Label != wantLabels[index] {
			t.Errorf("row %d label = %q, want %q", index, descriptor.Label, wantLabels[index])
		}
	}
	// Danger holds exactly the two acts that end the project's presence, so the
	// list a user arrows through looking for Edit never puts one under the cursor.
	for id, want := range map[operation.OperationID]listsurface.ListCategoryID{
		operation.OperationScopeRenameProject:  outlinerProjectActionCategoryProject,
		operation.OperationScopeArchiveProject: outlinerProjectActionCategoryDanger,
		operation.OperationProjectDelete:       outlinerProjectActionCategoryDanger,
	} {
		descriptor, _ := registry.Descriptor(id)
		if got := outlinerProjectActionCategory(descriptor); got != want {
			t.Errorf("%s category = %q, want %q", id, got, want)
		}
	}
	// The three direct-only manifest entries carry their own keys — `#`, `*` and
	// Space itself — so listing any would be the second path one-path forbids.
	for _, excluded := range []operation.OperationID{
		operation.OperationProjectSetTags,
		operation.OperationProjectActions,
		operation.OperationScopePinProject,
		operation.OperationProjectOpen,
		operation.OperationScopeOpenProjectNote,
		operation.OperationScopeRestoreProject,
	} {
		if slices.Contains(ids, excluded) {
			t.Errorf("project sheet offers %s, which reaches the project another way", excluded)
		}
	}
}

// TestProjectActionSheetDropsRowsTheProjectStateForbids proves the sheet reads
// the live project rather than the metadata the row was built from: archiving
// the project under the frozen context withdraws archiving itself, and leaves
// delete standing because removal survives archiving by design.
func TestProjectActionSheetDropsRowsTheProjectStateForbids(t *testing.T) {
	app, project, context := projectHeaderCursorApp(t)
	project.Archived = true

	ids := ProjectActionSheetOperationIDs(app.GetOperationRegistry(), context)
	wantIDs := []operation.OperationID{operation.OperationScopeRenameProject, operation.OperationProjectDelete}
	if !slices.Equal(ids, wantIDs) {
		t.Fatalf("archived project sheet IDs = %v, want %v", ids, wantIDs)
	}
}

// TestProjectActionSheetRefusesTheSystemUnfiledDelete keeps the storage-layer
// protection visible in the surface that offers the act: Unfiled may be edited,
// but the row that would remove it is never drawn. Only the frozen
// project identity is swapped, because that is the single fact the sheet keeps
// from the row it was raised on.
func TestProjectActionSheetRefusesTheSystemUnfiledDelete(t *testing.T) {
	app, _, context := projectHeaderCursorApp(t)
	unfiled := app.domain.runtime.SystemUnfiledProject()
	if unfiled == nil {
		t.Fatal("fixture workspace has no system Unfiled project")
	}
	context.Entity.ProjectID = unfiled.ID
	context.CurrentEntity = operation.CommandContextCurrentEntity(context.Entity)

	ids := ProjectActionSheetOperationIDs(app.GetOperationRegistry(), context)
	if slices.Contains(ids, operation.OperationProjectDelete) {
		t.Errorf("project sheet offers Delete project for the system Unfiled project: %v", ids)
	}
	if slices.Contains(ids, operation.OperationScopeArchiveProject) {
		t.Errorf("project sheet offers Archive project for the system Unfiled project: %v", ids)
	}
	if !slices.Contains(ids, operation.OperationScopeRenameProject) {
		t.Errorf("project sheet withdrew Edit project from the system Unfiled project: %v", ids)
	}
}

// TestOutlinerProjectOperationManifestIsRegisteredApplicableAndExecutable is the
// project counterpart of the task catalog gate: an entry that reaches the
// manifest but is never registered, never applicable on a header, or never
// reaches a handler would advertise a project action Space cannot perform.
func TestOutlinerProjectOperationManifestIsRegisteredApplicableAndExecutable(t *testing.T) {
	app, _, context := projectHeaderCursorApp(t)

	for _, entry := range outlinerProjectOperationManifest() {
		descriptor, registered := app.operations.Registry.Descriptor(entry.ID)
		if !registered {
			t.Errorf("manifest operation %s is not registered in the production registry", entry.ID)
			continue
		}
		if !descriptor.Applicable(context) {
			t.Errorf("manifest operation %s is not applicable on a project header", entry.ID)
			continue
		}
		result := app.operations.Executor.Execute(operation.OperationExecutionRequest{
			ID:      entry.ID,
			Context: context,
		})
		for _, sentinel := range []error{
			operation.ErrOperationNotRegistered,
			operation.ErrOperationUnavailable,
			operation.ErrOperationHandler,
		} {
			if errors.Is(result.Err, sentinel) {
				t.Errorf("%s on a project header: %v", entry.ID, result.Err)
			}
		}
		if result.Err != nil {
			t.Errorf("%s on a project header: %v", entry.ID, result.Err)
		}
		// Raising the sheet is the whole of what Space does, so it is the one
		// entry whose evidence is the overlay rather than a message.
		if entry.ID == operation.OperationProjectActions {
			if app.operations.ActionSheet == nil {
				t.Errorf("%s executed without raising the project sheet", entry.ID)
			}
			app.operations.ActionSheet = nil
			continue
		}
		// Every other handler either produced work or asked for the value its
		// provider declares. Neither means nothing ran.
		if result.Message == nil && result.ValueRequired == nil {
			t.Errorf("%s on a project header produced no message and requested no value", entry.ID)
		}
	}
}
