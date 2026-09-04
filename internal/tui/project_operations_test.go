package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/sglavoie/tt/internal/tui/notes"
	"github.com/sglavoie/tt/internal/tui/operation"
	"github.com/sglavoie/tt/internal/tui/presentation"
	"github.com/sglavoie/tt/internal/tui/route"
)

// projectOperationContext mimics a non-Project-Hub host naming one frozen
// project.
func projectOperationContext(projectID string) operation.CommandContext {
	ctx := route.PresentationCommandContext(presentation.ViewCrossProjectOutliner)
	ctx.CurrentEntity = operation.ProjectEntity(projectID)
	return ctx
}

func containsOperationID(ids []operation.OperationID, want operation.OperationID) bool {
	for _, id := range ids {
		if id == want {
			return true
		}
	}
	return false
}

func executeProjectOperation(app *AppModel, id operation.OperationID, ctx operation.CommandContext) operation.OperationExecutionResult {
	return app.operationExecutor.Execute(operation.OperationExecutionRequest{
		ID:      id,
		Context: ctx,
	})
}

func TestProjectOperationsShareOneHandlerAcrossHosts(t *testing.T) {
	app := projectHubApp(t)
	hub := openProjectHub(t, app)

	fromHub := executeProjectOperation(
		app,
		operation.OperationScopeOpenProjectNote,
		hub.projectHubCommandContext("beta"),
	)
	fromOtherHost := executeProjectOperation(app, operation.OperationScopeOpenProjectNote, projectOperationContext("beta"))

	hubMsg, ok := fromHub.Message.(notes.ShowNotesManagerMsg)
	if !ok || hubMsg.Scope != route.ProjectNotesScope("beta") {
		t.Fatalf("Project Hub notes emitted %#v, want beta", fromHub.Message)
	}
	otherHostMsg, ok := fromOtherHost.Message.(notes.ShowNotesManagerMsg)
	if !ok || otherHostMsg != hubMsg {
		t.Fatalf("other host notes emitted %#v, want the same project handler path", fromOtherHost.Message)
	}
}

func TestProjectOperationsResolveTheProjectFreshAtExecution(t *testing.T) {
	app := projectHubApp(t)
	hub := openProjectHub(t, app)
	focusProjectHubItem(t, hub, "project-hub:project:beta")

	// Pin beta after the row was built: the press freezes only the project ID, so
	// the toggle must read the live project rather than the row's metadata.
	beta := app.projectByID("beta")
	beta.Pin()

	_, cmd := hub.Update(syntheticKey('*'))
	if cmd == nil {
		t.Fatal("* emitted no pin request")
	}
	pin, ok := cmd().(ProjectPinRequestMsg)
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

	catalog := cloneWorkspaceCatalog(app.runtime.Catalog())
	catalog.SystemUnfiledProjectID = "alpha"
	app.runtime.SetCatalogInMemory(catalog)

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
				result = app.operationExecutor.Execute(result.ConfirmationRequired.Request)
			}
			if result.Err == nil {
				t.Fatalf("%s succeeded with %#v, want an unavailable result", tc.name, result.Message)
			}
		})
	}
}

func TestOpenProjectNotesRejectsArchivedProject(t *testing.T) {
	app := projectHubApp(t)
	archived := scopeTestProject("archived", "Archived", "1")
	archived.Archived = true
	addTestAppProject(app, archived)

	hub := openProjectHub(t, app)
	focusProjectHubItem(t, hub, "project-hub:archived:archived")
	hub.Update(syntheticKey(' '))
	if got := projectHubActionIDs(hub); containsOperationID(got, operation.OperationScopeOpenProjectNote) {
		t.Fatalf("archived project offered Open project notes: %v", got)
	}

	result := executeProjectOperation(app, operation.OperationScopeOpenProjectNote, projectOperationContext("archived"))
	if result.Err == nil {
		t.Fatalf("Open project notes emitted %#v for an archived project", result.Message)
	}
	if _, ok := result.Message.(notes.ShowNotesManagerMsg); ok {
		t.Fatalf("Open project notes emitted a Notes manager message for an archived project: %#v", result.Message)
	}

	descriptor, ok := app.operationRegistry.Descriptor(operation.OperationScopeOpenProjectNote)
	if !ok {
		t.Fatal("Open project notes operation is not registered")
	}
	handlerResult := app.executeProjectOperation(
		operation.OperationInvocation{Descriptor: descriptor},
		operation.OperationTargetFor(operation.ProjectEntity("archived")),
	)
	if handlerResult.Err == nil || handlerResult.Err.Error() != "Open project notes is unavailable while Archived is archived" {
		t.Fatalf("archived handler error = %v, want the shared lifecycle refusal", handlerResult.Err)
	}
	if _, ok := handlerResult.Message.(notes.ShowNotesManagerMsg); ok {
		t.Fatalf("archived handler emitted a Notes manager message: %#v", handlerResult.Message)
	}
}

func TestProjectOperationsRefuseAmbiguousMultiProjectScope(t *testing.T) {
	ctx := route.PresentationCommandContext(presentation.ViewCrossProjectOutliner)
	ctx.ScopeProjectIDs = []string{"alpha", "beta"}
	if projectOperationApplicable(ctx) {
		t.Fatal("a multi-project scope must not supply a project operation target")
	}
	ctx.ScopeProjectIDs = []string{"beta"}
	if !projectOperationApplicable(ctx) {
		t.Fatal("a single-project scope names one unambiguous project")
	}
}

func TestProjectArchiveDelegatesToTheShellConfirmation(t *testing.T) {
	app := projectHubApp(t)
	result := executeProjectOperation(app, operation.OperationScopeArchiveProject, projectOperationContext("beta"))
	if result.ConfirmationRequired != nil {
		t.Fatal("project archive raised a second confirmation over the shell prompt")
	}
	request, ok := result.Message.(ProjectArchiveRequestMsg)
	if !ok || request.Project == nil || request.Project.ID != "beta" || !request.Archive {
		t.Fatalf("project archive emitted %#v, want a beta archive request", result.Message)
	}

	handled, cmd := app.handleProjectMessages(request)
	if !handled || cmd != nil {
		t.Fatalf("archive request produced cmd %v instead of arming the confirmation", cmd)
	}
	if !app.projectArchiveConfirm.IsActive() {
		t.Fatal("project archive did not arm the shell confirmation")
	}
}

// TestProjectHubActiveProjectOffersOnlyArchive holds the Space menu to what the
// direct grammar cannot reach. Opening, editing, pinning, tagging and notes are
// all one press away, so a single active row offers archiving alone; the four
// bulk tag actions need a selection and drop out of a one-row sheet.
func TestProjectHubActiveProjectOffersOnlyArchive(t *testing.T) {
	hub := openProjectHub(t, projectHubApp(t))
	focusProjectHubItem(t, hub, "project-hub:project:beta")
	hub.Update(syntheticKey(' '))

	got := projectHubActionIDs(hub)
	if len(got) != 1 || !containsOperationID(got, operation.OperationScopeArchiveProject) {
		t.Fatalf("active project actions = %v, want archive alone", got)
	}
}

func TestProjectHubEnterSharesTheOpenProjectOperation(t *testing.T) {
	app := projectHubApp(t)
	archived := scopeTestProject("stale", "Stale", "1")
	archived.Archived = true
	addTestAppProject(app, archived)

	hub := openProjectHub(t, app)
	focusProjectHubItem(t, hub, "project-hub:project:beta")

	_, cmd := hub.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("Enter emitted no focus request")
	}
	focus, ok := cmd().(ApplyProjectScopeMsg)
	if !ok || focus.ProjectID != "beta" {
		t.Fatalf("Enter emitted %#v, want a beta focus request", cmd())
	}

	focusProjectHubItem(t, hub, "project-hub:archived:stale")
	if _, cmd := hub.Update(tea.KeyPressMsg{Code: tea.KeyEnter}); cmd != nil {
		t.Fatalf("opening an archived project emitted %#v", cmd())
	}
	if hub.notice == "" {
		t.Fatal("opening an archived project gave no unavailable notice")
	}
}
