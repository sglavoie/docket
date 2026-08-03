package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"
)

// projectOperationContext mimics a non-Project-Hub host naming one frozen
// project.
func projectOperationContext(projectID string) CommandContext {
	ctx := PresentationCommandContext(ViewCrossProjectOutliner)
	ctx.CurrentEntity = ProjectEntity(projectID)
	ctx.FocusedProjectID = projectID
	return ctx
}

func executeProjectOperation(app *AppModel, id OperationID, ctx CommandContext) OperationExecutionResult {
	return app.operationExecutor.Execute(OperationExecutionRequest{
		ID:      id,
		Context: ctx,
		Source:  OperationSourceActionSheet,
	})
}

func TestProjectOperationsShareOneHandlerAcrossHosts(t *testing.T) {
	app := contextHubApp(t)
	hub := openProjectHub(t, app)

	fromHub := executeProjectOperation(
		app,
		operationScopeOpenProjectNote,
		hub.projectHubCommandContext("beta"),
	)
	fromOtherHost := executeProjectOperation(app, operationScopeOpenProjectNote, projectOperationContext("beta"))

	hubMsg, ok := fromHub.Message.(ShowNotesManagerMsg)
	if !ok || hubMsg.Scope != ProjectNotesScope("beta") {
		t.Fatalf("Project Hub notes emitted %#v, want beta", fromHub.Message)
	}
	otherHostMsg, ok := fromOtherHost.Message.(ShowNotesManagerMsg)
	if !ok || otherHostMsg != hubMsg {
		t.Fatalf("other host notes emitted %#v, want the same project handler path", fromOtherHost.Message)
	}
}

func TestProjectOperationsResolveTheProjectFreshAtExecution(t *testing.T) {
	app := contextHubApp(t)
	hub := openProjectHub(t, app)
	hub.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	focusProjectHubItem(t, hub, "project-hub:active:beta")
	hub.Update(syntheticKey(' '))
	if hub.actions == nil {
		t.Fatal("Space did not freeze the project action target")
	}

	// Pin beta after the sheet froze its target: the toggle must read the live
	// project, not the metadata the row was built from.
	beta := app.projectByID("beta")
	beta.Pin()

	if !hub.actions.surface.SetCursorToID(ListItemID("project-hub-action:" + string(operationScopePinProject))) {
		t.Fatal("pin action was not selectable")
	}
	_, cmd := hub.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("pin action emitted no request")
	}
	pin, ok := cmd().(ProjectPinRequestMsg)
	if !ok || pin.Project == nil || pin.Project.ID != "beta" {
		t.Fatalf("pin action emitted %#v, want a beta pin request", cmd())
	}
	if pin.Pinned {
		t.Fatal("pin toggle used the frozen row state instead of the freshly resolved project")
	}
}

func TestProjectOperationsRejectArchivedAndImmutableTargets(t *testing.T) {
	app := contextHubApp(t)
	archived := scopeTestProject("archived", "Archived", "1")
	archived.Archived = true
	addTestAppProject(app, archived)

	catalog := cloneWorkspaceCatalog(app.runtime.Catalog())
	catalog.SystemUnfiledProjectID = "alpha"
	app.runtime.SetCatalogInMemory(catalog)

	cases := []struct {
		name string
		id   OperationID
		proj string
	}{
		{"open archived", operationProjectOpen, "archived"},
		{"archive the system Inbox", operationScopeArchiveProject, "alpha"},
		{"unarchive an active project", operationScopeRestoreProject, "beta"},
		{"act on a removed project", operationScopeRenameProject, "ghost"},
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
	app := contextHubApp(t)
	archived := scopeTestProject("archived", "Archived", "1")
	archived.Archived = true
	addTestAppProject(app, archived)

	hub := openProjectHub(t, app)
	hub.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	focusProjectHubItem(t, hub, "project-hub:archived:archived")
	hub.Update(syntheticKey(' '))
	if got := projectHubActionIDs(hub); containsOperationID(got, operationScopeOpenProjectNote) {
		t.Fatalf("archived project offered Open project notes: %v", got)
	}

	result := executeProjectOperation(app, operationScopeOpenProjectNote, projectOperationContext("archived"))
	if result.Err == nil {
		t.Fatalf("Open project notes emitted %#v for an archived project", result.Message)
	}
	if _, ok := result.Message.(ShowNotesManagerMsg); ok {
		t.Fatalf("Open project notes emitted a Notes manager message for an archived project: %#v", result.Message)
	}

	descriptor, ok := app.operationRegistry.Descriptor(operationScopeOpenProjectNote)
	if !ok {
		t.Fatal("Open project notes operation is not registered")
	}
	handlerResult := app.executeProjectOperation(
		OperationInvocation{Descriptor: descriptor},
		operationTargetFor(OperationTargetCurrentEntity, ProjectEntity("archived")),
	)
	if handlerResult.Err == nil || handlerResult.Err.Error() != "Open project notes is unavailable while Archived is archived" {
		t.Fatalf("archived handler error = %v, want the shared lifecycle refusal", handlerResult.Err)
	}
	if _, ok := handlerResult.Message.(ShowNotesManagerMsg); ok {
		t.Fatalf("archived handler emitted a Notes manager message: %#v", handlerResult.Message)
	}
}

func TestProjectOperationsRefuseAmbiguousMultiProjectScope(t *testing.T) {
	ctx := PresentationCommandContext(ViewCrossProjectOutliner)
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
	app := contextHubApp(t)
	result := executeProjectOperation(app, operationScopeArchiveProject, projectOperationContext("beta"))
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

func TestProjectHubActiveProjectOffersOnlyLeanAdministration(t *testing.T) {
	hub := openProjectHub(t, contextHubApp(t))
	hub.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	focusProjectHubItem(t, hub, "project-hub:active:beta")
	hub.Update(syntheticKey(' '))

	got := projectHubActionIDs(hub)
	for _, want := range []OperationID{
		operationProjectOpen,
		operationScopeRenameProject,
		operationScopePinProject,
		operationScopeArchiveProject,
		operationScopeOpenProjectNote,
	} {
		if !containsOperationID(got, want) {
			t.Fatalf("active project actions omitted %s: %v", want, got)
		}
	}
	if containsOperationID(got, operationScopeRestoreProject) {
		t.Fatalf("active project offered unarchive: %v", got)
	}
	if len(got) != 6 {
		t.Fatalf("active project actions = %v, want exactly six lean actions", got)
	}
}

func TestProjectHubEnterSharesTheOpenProjectOperation(t *testing.T) {
	app := contextHubApp(t)
	hub := openProjectHub(t, app)
	hub.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	focusProjectHubItem(t, hub, "project-hub:active:beta")

	_, cmd := hub.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("Enter emitted no focus request")
	}
	focus, ok := cmd().(ApplyProjectScopeMsg)
	if !ok || focus.ProjectID != "beta" {
		t.Fatalf("Enter emitted %#v, want a beta focus request", cmd())
	}

	archived := scopeTestProject("stale", "Stale", "1")
	archived.Archived = true
	addTestAppProject(app, archived)
	if cmd := hub.executeProjectOperation(operationProjectOpen, "stale"); cmd != nil {
		t.Fatalf("opening an archived project emitted %#v", cmd())
	}
	if hub.notice == "" {
		t.Fatal("opening an archived project gave no unavailable notice")
	}
}
