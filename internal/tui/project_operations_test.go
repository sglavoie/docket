package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
)

// projectOperationContext mimics a non-Scope-Hub host (Finder) naming one
// frozen project.
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
		Source:  OperationSourceFinder,
	})
}

func TestProjectOperationsShareOneHandlerAcrossScopeHubAndFinder(t *testing.T) {
	app := contextHubApp(t)
	hub := openContextHub(t, app)
	focusScopeHubItem(t, hub, "scope-hub:active:beta")

	fromHub := executeProjectOperation(
		app,
		operationProjectExport,
		hub.scopeHubActionContext(scopeHubActionTarget{ProjectID: "beta"}),
	)
	fromFinder := executeProjectOperation(app, operationProjectExport, projectOperationContext("beta"))

	hubMsg, ok := fromHub.Message.(ExportProjectMsg)
	if !ok || hubMsg.Project == nil || hubMsg.Project.ID != "beta" {
		t.Fatalf("Scope Hub export emitted %#v, want beta", fromHub.Message)
	}
	finderMsg, ok := fromFinder.Message.(ExportProjectMsg)
	if !ok || finderMsg.Project != hubMsg.Project {
		t.Fatalf("Finder export emitted %#v, want the same project handler path", fromFinder.Message)
	}
}

func TestProjectOperationsResolveTheProjectFreshAtExecution(t *testing.T) {
	app := contextHubApp(t)
	hub := openContextHub(t, app)
	focusScopeHubItem(t, hub, "scope-hub:active:beta")
	hub.Update(syntheticKey(' '))
	if hub.actions == nil {
		t.Fatal("Space did not freeze the project action target")
	}

	// Pin beta after the sheet froze its target: the toggle must read the live
	// project, not the metadata the row was built from.
	beta := app.projectByID("beta")
	beta.Pin()

	if !hub.actions.surface.SetCursorToID(ListItemID("scope-hub-action:" + string(operationScopePinProject))) {
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

func TestProjectOperationsRejectArchivedSandboxAndImmutableTargets(t *testing.T) {
	app := contextHubApp(t)
	archived := scopeTestProject("archived", "Archived", "1")
	archived.Archived = true
	sandbox := scopeTestProject("sandbox", "Sandbox", "1")
	sandbox.Sandbox = true
	addTestAppProject(app, archived)
	addTestAppProject(app, sandbox)

	catalog := cloneWorkspaceCatalog(app.runtime.Catalog())
	catalog.SystemInboxProjectID = "alpha"
	app.runtime.SetCatalogInMemory(catalog)

	cases := []struct {
		name string
		id   OperationID
		proj string
	}{
		{"open archived", operationProjectOpen, "archived"},
		{"archive completed on archived", operationProjectArchiveCompleted, "archived"},
		{"archive the sandbox", operationScopeArchiveProject, "sandbox"},
		{"edit the system Inbox", operationScopeRenameProject, "alpha"},
		{"unarchive an active project", operationScopeRestoreProject, "beta"},
		{"act on a removed project", operationProjectMetrics, "ghost"},
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

func TestArchiveCompletedTasksRequiresExplicitConfirmation(t *testing.T) {
	app := contextHubApp(t)
	ctx := projectOperationContext("beta")

	result := executeProjectOperation(app, operationProjectArchiveCompleted, ctx)
	if result.ConfirmationRequired == nil {
		t.Fatalf("archive completed ran without confirmation: %#v", result)
	}
	if result.Message != nil {
		t.Fatalf("archive completed emitted %#v before confirmation", result.Message)
	}

	// The shell prompt must actually open for a host that returns the request
	// as a message rather than hosting a contextual action sheet.
	handled, _ := app.handleOperationOverlayMessage(*result.ConfirmationRequired)
	if !handled || !app.operationConfirm.IsActive() {
		t.Fatal("the destructive confirmation request never reached the shell prompt")
	}

	handled, cmd := app.handleOperationConfirmKey("y")
	if !handled || cmd == nil {
		t.Fatal("confirming archive completed produced no execution")
	}
	archive, ok := cmd().(ArchiveCompletedTasksMsg)
	if !ok || archive.Project == nil || archive.Project.ID != "beta" {
		t.Fatalf("confirmed archive completed emitted %#v, want a beta request", cmd())
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

func TestProjectFolderAndMetricsTargetTheFrozenProject(t *testing.T) {
	app := contextHubApp(t)
	app.currentProject = app.projectByID("alpha")
	ctx := projectOperationContext("beta")

	folder := executeProjectOperation(app, operationProjectOpenFolder, ctx)
	open, ok := folder.Message.(OpenDataFolderMsg)
	if !ok || open.Project == nil || open.Project.ID != "beta" {
		t.Fatalf("open project folder emitted %#v, want beta", folder.Message)
	}

	metrics := executeProjectOperation(app, operationProjectMetrics, ctx)
	dashboard, ok := metrics.Message.(ShowMetricsDashboardMsg)
	if !ok || dashboard.Project == nil || dashboard.Project.ID != "beta" {
		t.Fatalf("project metrics emitted %#v, want beta", metrics.Message)
	}
}

func TestScopeHubActiveProjectOffersEveryMaintenanceOperation(t *testing.T) {
	hub := openContextHub(t, contextHubApp(t))
	focusScopeHubItem(t, hub, "scope-hub:active:beta")
	hub.Update(syntheticKey(' '))

	got := scopeHubActionIDs(hub)
	for _, want := range []OperationID{
		operationProjectOpen,
		operationScopeRenameProject,
		operationScopeStatusProject,
		operationScopePinProject,
		operationScopeArchiveProject,
		operationScopeOpenProjectNote,
		operationProjectMetrics,
		operationProjectExport,
		operationProjectOpenFolder,
		operationProjectArchiveCompleted,
	} {
		if !containsOperationID(got, want) {
			t.Fatalf("active project actions omitted %s: %v", want, got)
		}
	}
	if containsOperationID(got, operationScopeRestoreProject) {
		t.Fatalf("active project offered unarchive: %v", got)
	}
}

func TestScopeHubEnterSharesTheOpenProjectOperation(t *testing.T) {
	app := contextHubApp(t)
	hub := openContextHub(t, app)
	focusScopeHubItem(t, hub, "scope-hub:active:beta")

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
	if cmd := hub.executeScopeHubProjectOperation(operationProjectOpen, scopeHubEntry{
		Kind:    scopeHubEntryActiveProject,
		Project: &models.Project{ID: "stale"},
	}); cmd != nil {
		t.Fatalf("opening an archived project emitted %#v", cmd())
	}
	if hub.notice == "" {
		t.Fatal("opening an archived project gave no unavailable notice")
	}
}
