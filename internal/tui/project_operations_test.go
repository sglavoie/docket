package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/docket/internal/tui/coordinator"
	"github.com/sglavoie/docket/internal/tui/notes"
	"github.com/sglavoie/docket/internal/tui/operation"
	"github.com/sglavoie/docket/internal/tui/outliner"
	"github.com/sglavoie/docket/internal/tui/presentation"
	"github.com/sglavoie/docket/internal/tui/projecthub"
	"github.com/sglavoie/docket/internal/tui/route"
)

// projectOperationContext mimics a non-Project-Hub host naming one frozen
// project.
func projectOperationContext(projectID string) operation.CommandContext {
	ctx := route.PresentationCommandContext(presentation.ViewCrossProjectOutliner)
	ctx.CurrentEntity = operation.ProjectEntity(projectID)
	return ctx
}

func executeProjectOperation(app *AppModel, id operation.OperationID, ctx operation.CommandContext) operation.OperationExecutionResult {
	return app.operations.Executor.Execute(operation.OperationExecutionRequest{
		ID:      id,
		Context: ctx,
	})
}

// The Hub reaches the project handler by pressing its own key rather than by
// building a context, so the test fails if the Hub's dispatch stops resolving
// the row's project the way every other host resolves a frozen one.
func TestProjectOperationsShareOneHandlerAcrossHosts(t *testing.T) {
	app := projectHubApp(t)
	hub := openProjectHub(t, app)
	focusProjectHubRow(t, hub, "Beta")

	_, cmd := hub.Update(syntheticCtrlKey('n'))
	if cmd == nil {
		t.Fatal("Ctrl+N on a project row emitted no notes request")
	}
	hubMsg, ok := cmd().(notes.ShowNotesManagerMsg)
	if !ok || hubMsg.Scope != route.ProjectNotesScope("beta") {
		t.Fatalf("Project Hub notes emitted %#v, want beta", cmd())
	}

	fromOtherHost := executeProjectOperation(app, operation.OperationScopeOpenProjectNote, projectOperationContext("beta"))
	otherHostMsg, ok := fromOtherHost.Message.(notes.ShowNotesManagerMsg)
	if !ok || otherHostMsg != hubMsg {
		t.Fatalf("other host notes emitted %#v, want the same project handler path", fromOtherHost.Message)
	}
}

func TestProjectOperationsResolveTheProjectFreshAtExecution(t *testing.T) {
	app := projectHubApp(t)
	hub := openProjectHub(t, app)
	focusProjectHubRow(t, hub, "Beta")

	// Pin beta after the row was built: the press freezes only the project ID, so
	// the toggle must read the live project rather than the row's metadata.
	beta := app.projectByID("beta")
	beta.Pin()

	_, cmd := hub.Update(syntheticKey('*'))
	if cmd == nil {
		t.Fatal("* emitted no pin request")
	}
	pin, ok := cmd().(projecthub.ProjectPinRequestMsg)
	if !ok || pin.Project == nil || pin.Project.ID != "beta" {
		t.Fatalf("* emitted %#v, want a beta pin request", cmd())
	}
	if pin.Pinned {
		t.Fatal("pin toggle used the frozen row state instead of the freshly resolved project")
	}
}

func TestProjectOperationsRejectArchivedAndImmutableTargets(t *testing.T) {
	app := projectHubApp(t)
	archived := scopeTestProject("archived", "Archived", "1")
	archived.Archived = true
	addTestAppProject(app, archived)

	catalog := coordinator.CloneWorkspaceCatalog(app.domain.runtime.Catalog())
	catalog.SystemUnfiledProjectID = "alpha"
	app.domain.runtime.SetCatalogInMemory(catalog)

	cases := []struct {
		name string
		id   operation.OperationID
		proj string
	}{
		{"open archived", operation.OperationProjectOpen, "archived"},
		{"archive the system Inbox", operation.OperationScopeArchiveProject, "alpha"},
		{"unarchive an active project", operation.OperationScopeRestoreProject, "beta"},
		{"act on a removed project", operation.OperationScopeRenameProject, "ghost"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result := executeProjectOperation(app, tc.id, projectOperationContext(tc.proj))
			if result.ConfirmationRequired != nil {
				result = app.operations.Executor.Execute(result.ConfirmationRequired.Request)
			}
			if result.Err == nil {
				t.Fatalf("%s succeeded with %#v, want an unavailable result", tc.name, result.Message)
			}
		})
	}
}

// TestOpenProjectNotesRejectsArchivedProject holds the refusal to the handler
// every host shares, so no route to the operation reaches the Notes manager. The
// sheet half -- that the Hub never offers the entry in the first place -- is
// asserted inside internal/tui/projecthub, where the sheet lives.
func TestOpenProjectNotesRejectsArchivedProject(t *testing.T) {
	app := projectHubApp(t)
	archived := scopeTestProject("archived", "Archived", "1")
	archived.Archived = true
	addTestAppProject(app, archived)

	result := executeProjectOperation(app, operation.OperationScopeOpenProjectNote, projectOperationContext("archived"))
	if result.Err == nil {
		t.Fatalf("Open project notes emitted %#v for an archived project", result.Message)
	}
	if _, ok := result.Message.(notes.ShowNotesManagerMsg); ok {
		t.Fatalf("Open project notes emitted a Notes manager message for an archived project: %#v", result.Message)
	}

	descriptor, ok := app.operations.Registry.Descriptor(operation.OperationScopeOpenProjectNote)
	if !ok {
		t.Fatal("Open project notes operation is not registered")
	}
	handlerResult := app.project.ExecuteProjectOperation(
		app,
		operation.OperationInvocation{Descriptor: descriptor},
		operation.OperationTargetFor(operation.ProjectEntity("archived")),
	)
	if handlerResult.Err == nil || handlerResult.Err.Error() != "Open Project Notes is unavailable while Archived is archived" {
		t.Fatalf("archived handler error = %v, want the shared lifecycle refusal", handlerResult.Err)
	}
	if _, ok := handlerResult.Message.(notes.ShowNotesManagerMsg); ok {
		t.Fatalf("archived handler emitted a Notes manager message: %#v", handlerResult.Message)
	}
}

func TestProjectOperationsRefuseAmbiguousMultiProjectScope(t *testing.T) {
	ctx := route.PresentationCommandContext(presentation.ViewCrossProjectOutliner)
	ctx.ScopeProjectIDs = []string{"alpha", "beta"}
	if projecthub.ProjectOperationApplicable(ctx) {
		t.Fatal("a multi-project scope must not supply a project operation target")
	}
	ctx.ScopeProjectIDs = []string{"beta"}
	if !projecthub.ProjectOperationApplicable(ctx) {
		t.Fatal("a single-project scope names one unambiguous project")
	}
}

func TestProjectArchiveDelegatesToTheShellConfirmation(t *testing.T) {
	app := projectHubApp(t)
	result := executeProjectOperation(app, operation.OperationScopeArchiveProject, projectOperationContext("beta"))
	if result.ConfirmationRequired != nil {
		t.Fatal("project archive raised a second confirmation over the shell prompt")
	}
	request, ok := result.Message.(projecthub.ProjectArchiveRequestMsg)
	if !ok || request.Project == nil || request.Project.ID != "beta" || !request.Archive {
		t.Fatalf("project archive emitted %#v, want a beta archive request", result.Message)
	}

	handled, cmd := app.handleProjectMessages(request)
	if !handled || cmd != nil {
		t.Fatalf("archive request produced cmd %v instead of arming the confirmation", cmd)
	}
	if !app.project.ArchiveArmed() {
		t.Fatal("project archive did not arm the shell confirmation")
	}
}

func TestProjectHubEnterSharesTheOpenProjectOperation(t *testing.T) {
	app := projectHubApp(t)
	archived := scopeTestProject("stale", "Stale", "1")
	archived.Archived = true
	addTestAppProject(app, archived)

	hub := openProjectHub(t, app)
	focusProjectHubRow(t, hub, "Beta")

	_, cmd := hub.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("Enter emitted no focus request")
	}
	focus, ok := cmd().(outliner.ApplyProjectScopeMsg)
	if !ok || focus.ProjectID != "beta" {
		t.Fatalf("Enter emitted %#v, want a beta focus request", cmd())
	}

	// An archived row raises no scope at all. The notice the Hub shows instead is
	// its own, and is asserted inside internal/tui/projecthub.
	focusProjectHubRow(t, hub, "Stale")
	if _, cmd := hub.Update(tea.KeyPressMsg{Code: tea.KeyEnter}); cmd != nil {
		t.Fatalf("opening an archived project emitted %#v", cmd())
	}
}
