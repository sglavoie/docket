package tui

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/workspace"
)

// A parked project stays reachable by explicit request only. These tests pin
// the whole lazy-focus lifecycle: the load admits exactly one extra project,
// the scope appears only once that load lands, a failure cancels without
// disturbing the base snapshot, and leaving restores the base with no parked
// task or selection left behind.

// parkedFocusOutliner returns an Outliner holding the base workspace alone,
// with a parked project available on request.
func parkedFocusOutliner(t *testing.T) (*models.Project, *models.Project, *OutlinerModel) {
	t.Helper()
	runtime, active, parked := parkedWorkspace(t)
	outliner := baseWorkspaceOutliner(runtime, active, parked)
	outliner.selectOutlinerMode(ModeAll)
	if containsProject(taskRowProjectIDs(outliner), parked.ID) {
		t.Fatal("the base workspace already projects the parked project")
	}
	return active, parked, outliner
}

// adoptParkedFocus drives the real request/land sequence so a test starts from
// the state a user reaches by opening a parked project.
func adoptParkedFocus(t *testing.T, outliner *OutlinerModel, projects ...*models.Project) {
	t.Helper()
	parked := projects[len(projects)-1]
	outliner.beginParkedProjectFocus(parked.ID)
	outliner.handleCrossProjectsLoaded(crossProjectsLoadedMsg{
		projects:       projects,
		generation:     outliner.crossLoadGen,
		focusProjectID: parked.ID,
	})
	if outliner.scopedProjectID != parked.ID {
		t.Fatalf("scoped project = %q, want the focused parked project", outliner.scopedProjectID)
	}
}

func parkedFocusDiskProject(t *testing.T, dataDir, name string) *models.Project {
	t.Helper()
	task := primaryRowsTask(name+"-task", models.StateBacklog)
	task.ShortID = 1
	task.CreatedAt = task.UpdatedAt
	project := primaryRowsProject(name, task)
	project.NextShortID = 2
	project.Path = filepath.Join(dataDir, "projects", project.Slug)
	if err := os.MkdirAll(project.Path, 0o755); err != nil {
		t.Fatalf("MkdirAll(%q): %v", project.Path, err)
	}
	for _, task := range project.Tasks {
		task.Project = project
	}
	if err := storage.SaveProject(project); err != nil {
		t.Fatalf("SaveProject(%q): %v", project.ID, err)
	}
	return project
}

func TestParkedFocusSelectionAdmitsExactlyTheRequestedProject(t *testing.T) {
	runtime, active, parked := parkedWorkspace(t)

	selectionIDs := func(opts CrossScopeOpts) []string {
		var ids []string
		for _, project := range selectCrossScopeProjects(runtime, opts) {
			ids = append(ids, project.ID)
		}
		return ids
	}

	assertIDs(t, selectionIDs(CrossScopeOpts{ActiveOnly: true}), active.ID)
	assertIDs(t, selectionIDs(CrossScopeOpts{ActiveOnly: true, FocusProjectID: parked.ID}), active.ID, parked.ID)
	assertIDs(t, selectionIDs(CrossScopeOpts{ActiveOnly: true, FocusProjectID: active.ID}), active.ID)
	assertIDs(t, selectionIDs(CrossScopeOpts{ActiveOnly: true, FocusProjectID: "absent"}), active.ID)

	parked.Archived = true
	if err := runtime.Catalog().UpsertProject(parked); err != nil {
		t.Fatalf("archive %q: %v", parked.ID, err)
	}
	assertIDs(t, selectionIDs(CrossScopeOpts{ActiveOnly: true, FocusProjectID: parked.ID}), active.ID)
}

// The Active base must not be reparsed to admit one parked project, which is
// what keeps an explicit focus cheap on a large workspace.
func TestParkedFocusReusesTheLoaderCacheForTheActiveWorkspace(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	dataDir := t.TempDir()
	active := parkedFocusDiskProject(t, dataDir, "active")
	parked := parkedFocusDiskProject(t, dataDir, "parked")

	runtime := newTestConfigAt(dataDir)
	setTestProjects(runtime, []*models.Project{active, parked})
	parkProject(t, runtime, "On Hold", parked.ID)

	loader := workspace.NewLoader()
	_, base, failed := loadCrossScopeSelection(
		selectCrossScopeProjects(runtime, CrossScopeOpts{ActiveOnly: true}), loader)
	if len(failed) != 0 {
		t.Fatalf("base load reported failures: %+v", failed)
	}
	if len(base) != 1 || loader.ParseCount() != 1 {
		t.Fatalf("base load = %d projects with ParseCount %d, want 1 and 1", len(base), loader.ParseCount())
	}

	_, focused, failed := loadCrossScopeSelection(
		selectCrossScopeProjects(runtime, CrossScopeOpts{ActiveOnly: true, FocusProjectID: parked.ID}), loader)
	if len(failed) != 0 {
		t.Fatalf("focus load reported failures: %+v", failed)
	}
	if len(focused) != 2 {
		t.Fatalf("focus load = %d projects, want the base plus the parked project", len(focused))
	}
	if loader.ParseCount() != 2 {
		t.Fatalf("focus load ParseCount = %d, want only the parked project parsed", loader.ParseCount())
	}
	if loader.CacheHits() != 1 {
		t.Fatalf("focus load CacheHits = %d, want the Active workspace reused", loader.CacheHits())
	}
}

func TestParkedFocusPushesScopeOnlyOnceTheLoadLands(t *testing.T) {
	active, parked, outliner := parkedFocusOutliner(t)

	if cmd := outliner.beginParkedProjectFocus(parked.ID); cmd == nil {
		t.Fatal("beginParkedProjectFocus started no load")
	}
	if outliner.isProjectScoped() {
		t.Fatal("the scope was pushed before the parked project was loaded")
	}
	if outliner.pendingParkedFocusID != parked.ID {
		t.Fatalf("pending focus = %q, want the requested parked project", outliner.pendingParkedFocusID)
	}
	if got := outliner.focusRequestForLoad(); got != parked.ID {
		t.Fatalf("focus request = %q, want the pending parked project", got)
	}

	outliner.handleCrossProjectsLoaded(crossProjectsLoadedMsg{
		projects:       []*models.Project{active, parked},
		generation:     outliner.crossLoadGen,
		focusProjectID: parked.ID,
	})

	if outliner.scopedProjectID != parked.ID {
		t.Fatalf("scoped project = %q, want the parked project", outliner.scopedProjectID)
	}
	if outliner.pendingParkedFocusID != "" {
		t.Fatalf("pending focus survived the landed load: %q", outliner.pendingParkedFocusID)
	}
	if got := taskRowProjectIDs(outliner); len(got) != 1 || got[0] != parked.ID {
		t.Fatalf("focused rows project %v, want only the parked project", got)
	}
	// Every later reload must keep asking for the adopted focus.
	if got := outliner.focusRequestForLoad(); got != parked.ID {
		t.Fatalf("focus request after adoption = %q, want the parked project retained", got)
	}
}

// A focus that cannot be read is canceled outright rather than opening an
// empty scope, and the workspace the user is still looking at is untouched.
func TestParkedFocusParseFailureCancelsWithoutDisturbingTheBaseSnapshot(t *testing.T) {
	active, parked, outliner := parkedFocusOutliner(t)
	baseIdentity := taskIdentityFor(active.Tasks[0], active)
	outliner.ReplaceSelection([]TaskIdentity{baseIdentity})
	baseSnapshot := outliner.loadedProjects
	baseRows := len(outliner.rows)

	outliner.beginParkedProjectFocus(parked.ID)
	cmd := outliner.handleCrossProjectsLoaded(crossProjectsLoadedMsg{
		projects:   []*models.Project{active},
		generation: outliner.crossLoadGen,
		failedLoads: []projectLoadFailure{
			{ProjectID: parked.ID, Detail: "failed to load project parked: bad heading"},
		},
		focusProjectID: parked.ID,
	})

	if outliner.isProjectScoped() {
		t.Fatal("a failed focus load still pushed a scope")
	}
	if outliner.pendingParkedFocusID != "" || outliner.parkedFocusProjectID != "" {
		t.Fatalf("a failed focus left state behind: pending=%q adopted=%q",
			outliner.pendingParkedFocusID, outliner.parkedFocusProjectID)
	}
	if &outliner.loadedProjects[0] != &baseSnapshot[0] || len(outliner.loadedProjects) != len(baseSnapshot) {
		t.Fatal("a failed focus replaced the previous base snapshot")
	}
	if len(outliner.rows) != baseRows {
		t.Fatalf("rows = %d, want the %d base rows left alone", len(outliner.rows), baseRows)
	}
	if !outliner.IsTaskIdentitySelected(baseIdentity) {
		t.Fatal("a failed focus discarded a selection the base workspace still admits")
	}
	if outliner.crossLoading {
		t.Fatal("a failed focus left the Outliner loading")
	}

	if cmd == nil {
		t.Fatal("a failed focus reported nothing")
	}
	failure, ok := cmd().(ErrorMsg)
	if !ok || failure.Err == nil || !strings.Contains(failure.Err.Error(), "bad heading") {
		t.Fatalf("failed focus reported %#v, want the project-specific parse error", cmd())
	}
}

// Archival or removal between the request and the load leaves the project
// simply absent from the snapshot, which must cancel the focus too.
func TestParkedFocusCancelsWhenTheProjectNeverArrives(t *testing.T) {
	active, parked, outliner := parkedFocusOutliner(t)

	outliner.beginParkedProjectFocus(parked.ID)
	cmd := outliner.handleCrossProjectsLoaded(crossProjectsLoadedMsg{
		projects:       []*models.Project{active},
		generation:     outliner.crossLoadGen,
		focusProjectID: parked.ID,
	})

	if outliner.isProjectScoped() || outliner.parkedFocusProjectID != "" {
		t.Fatal("a focus whose project never arrived still opened a scope")
	}
	if cmd == nil {
		t.Fatal("a vanished focus target reported nothing")
	}
	if _, ok := cmd().(ErrorMsg); !ok {
		t.Fatalf("a vanished focus target reported %#v, want an error", cmd())
	}
}

func TestSupersededParkedFocusResultIsDiscarded(t *testing.T) {
	active, parked, outliner := parkedFocusOutliner(t)

	outliner.beginParkedProjectFocus(parked.ID)
	stale := outliner.crossLoadGen
	outliner.beginParkedProjectFocus(active.ID)

	if cmd := outliner.handleCrossProjectsLoaded(crossProjectsLoadedMsg{
		projects:       []*models.Project{active, parked},
		generation:     stale,
		focusProjectID: parked.ID,
	}); cmd != nil {
		t.Fatalf("a superseded focus result produced %#v", cmd())
	}
	if outliner.isProjectScoped() {
		t.Fatal("a superseded focus result pushed a scope")
	}
	if outliner.pendingParkedFocusID != active.ID {
		t.Fatalf("pending focus = %q, want the newest request preserved", outliner.pendingParkedFocusID)
	}
}

// Dismissing the surface that asked for the focus abandons the request; the
// user is back at a base Outliner that never gained the parked project.
func TestParkedFocusIsCancelledWhenTheOutlinerReappears(t *testing.T) {
	active, parked, outliner := parkedFocusOutliner(t)

	outliner.beginParkedProjectFocus(parked.ID)
	abandoned := outliner.crossLoadGen
	if cmd := outliner.handleViewDidAppear(); cmd == nil {
		t.Fatal("returning to the Outliner started no base reload")
	}
	if outliner.pendingParkedFocusID != "" {
		t.Fatalf("pending focus survived the return to the Outliner: %q", outliner.pendingParkedFocusID)
	}

	if cmd := outliner.handleCrossProjectsLoaded(crossProjectsLoadedMsg{
		projects:       []*models.Project{active, parked},
		generation:     abandoned,
		focusProjectID: parked.ID,
	}); cmd != nil {
		t.Fatalf("an abandoned focus result produced %#v", cmd())
	}
	if outliner.isProjectScoped() {
		t.Fatal("an abandoned focus result pushed a scope")
	}
}

// One Esc restores the exact base scope, with no parked row left visible and no
// later reload still requesting the parked project.
func TestEscFromParkedFocusRestoresTheBaseWorkspace(t *testing.T) {
	active, parked, outliner := parkedFocusOutliner(t)
	baseMode := outliner.primaryMode
	baseRows := len(outliner.rows)
	adoptParkedFocus(t, outliner, active, parked)

	if !RunEscCascade(outliner.escSteps()) {
		t.Fatal("Esc did nothing while focused on a parked project")
	}

	if outliner.isProjectScoped() {
		t.Fatalf("Esc left the focus in place: %q", outliner.scopedProjectID)
	}
	if outliner.primaryMode != baseMode {
		t.Fatalf("Esc restored mode %q, want the captured base mode %q", outliner.primaryMode, baseMode)
	}
	if outliner.projectIsLoaded(parked.ID) {
		t.Fatal("Esc left the parked project in the snapshot")
	}
	if containsProject(taskRowProjectIDs(outliner), parked.ID) {
		t.Fatal("Esc left a parked task row visible")
	}
	if len(outliner.rows) != baseRows {
		t.Fatalf("Esc rebuilt %d rows, want the %d base rows back without an empty frame",
			len(outliner.rows), baseRows)
	}
	if outliner.focusRequestForLoad() != "" {
		t.Fatalf("a later reload still requests %q", outliner.focusRequestForLoad())
	}
	if outliner.takeParkedFocusReload() == nil {
		t.Fatal("leaving a parked focus owed no reload of the base workspace")
	}
	if outliner.takeParkedFocusReload() != nil {
		t.Fatal("the owed reload was claimed twice")
	}
}

// A selection that outlives the focus keeps only what the restored base
// admits. Esc clears a visual selection before it unwinds scope, so the case
// that matters is a focus left by any other route.
func TestLeavingAParkedFocusDropsSelectionsTheBaseCannotAdmit(t *testing.T) {
	active, parked, outliner := parkedFocusOutliner(t)
	adoptParkedFocus(t, outliner, active, parked)

	baseIdentity := taskIdentityFor(active.Tasks[0], active)
	parkedIdentity := taskIdentityFor(parked.Tasks[0], parked)
	outliner.ReplaceSelection([]TaskIdentity{baseIdentity, parkedIdentity})

	if !outliner.clearProjectScope() {
		t.Fatal("clearing the scope reported no change")
	}

	if outliner.IsTaskIdentitySelected(parkedIdentity) {
		t.Fatal("a parked task stayed selected after the base workspace was restored")
	}
	if !outliner.IsTaskIdentitySelected(baseIdentity) {
		t.Fatal("leaving the focus dropped a selection the base workspace still admits")
	}
}

// Focusing somewhere else ends the exception rather than accumulating parked
// projects in the snapshot behind the visible scope.
func TestFocusingAnotherProjectDropsTheParkedException(t *testing.T) {
	active, parked, outliner := parkedFocusOutliner(t)
	adoptParkedFocus(t, outliner, active, parked)

	outliner.pushProjectScope(active.ID)

	if outliner.parkedFocusProjectID != "" {
		t.Fatalf("the parked exception survived a different focus: %q", outliner.parkedFocusProjectID)
	}
	if outliner.projectIsLoaded(parked.ID) {
		t.Fatal("the parked project stayed loaded behind another project's focus")
	}
	if outliner.focusRequestForLoad() != "" {
		t.Fatalf("a later reload still requests %q", outliner.focusRequestForLoad())
	}
}

// A focused project that disappears out from under the session unwinds to the
// base scope and stops being requested.
func TestParkedFocusClearsWhenTheFocusedProjectVanishes(t *testing.T) {
	active, parked, outliner := parkedFocusOutliner(t)
	adoptParkedFocus(t, outliner, active, parked)

	outliner.handleCrossProjectsLoaded(crossProjectsLoadedMsg{
		projects:   []*models.Project{active},
		generation: outliner.crossLoadGen,
	})

	if outliner.isProjectScoped() {
		t.Fatalf("the scope survived the focused project's removal: %q", outliner.scopedProjectID)
	}
	if outliner.parkedFocusProjectID != "" || outliner.focusRequestForLoad() != "" {
		t.Fatalf("a removed focus is still requested: %q", outliner.focusRequestForLoad())
	}
}

// Chrome must say why these tasks are absent from the unscoped Outliner.
func TestParkedFocusChromeNamesTheBoundary(t *testing.T) {
	active, parked, outliner := parkedFocusOutliner(t)
	outliner.SetSize(120, 30)
	adoptParkedFocus(t, outliner, active, parked)

	want := parked.Name + " · On Hold"
	if got := outliner.scopedProjectChromeLabel(parked.ID); got != want {
		t.Fatalf("scope label = %q, want %q", got, want)
	}
	assertIDs(t, outliner.scopeChainNames(), want)
	if got := outliner.crossStatusBarLabel(); !strings.Contains(got, want) {
		t.Fatalf("status bar = %q, want it to name %q", got, want)
	}
	if got := outliner.crossProjectBreadcrumb(120); !strings.Contains(got, "["+want+"]") {
		t.Fatalf("breadcrumb = %q, want the bracketed boundary anchor", got)
	}
	if got := outliner.renderOutlinerTitle(120); !strings.Contains(got, want) {
		t.Fatalf("title = %q, want it to name %q", got, want)
	}

	outliner.pushProjectScope(active.ID)
	if got := outliner.scopedProjectChromeLabel(active.ID); got != active.Name {
		t.Fatalf("base-workspace scope label = %q, want the bare name %q", got, active.Name)
	}
}

// Mutation and planning operations stay available while focused: the operation
// scope names the focused project, which is what resolves their targets.
func TestParkedFocusKeepsTaskOperationsInScope(t *testing.T) {
	active, parked, outliner := parkedFocusOutliner(t)
	adoptParkedFocus(t, outliner, active, parked)

	assertIDs(t, outliner.operationCommandContext().ScopeProjectIDs, parked.ID)

	RunEscCascade(outliner.escSteps())
	assertIDs(t, outliner.operationCommandContext().ScopeProjectIDs, active.ID)
}

// The Project Hub stays open across the whole request so a failure reports
// against the row the user chose, and closes only once the scope is in place.
func TestProjectHubOpensAParkedProjectThroughTheShell(t *testing.T) {
	app := contextHubApp(t)
	parkProject(t, app.runtime, "On Hold", "beta")
	alpha := app.projectByID("alpha")
	beta := app.projectByID("beta")
	app.crossOutliner.loadedProjects = []*models.Project{alpha}
	app.crossOutliner.crossForceRebuild = true
	app.crossOutliner.rebuildRows()

	openProjectHub(t, app)
	app.handleViewActionMessages(ApplyProjectScopeMsg{ProjectID: beta.ID})

	if app.currentView() != ViewProjectHub {
		t.Fatalf("current view = %v, want the Project Hub held open while the load runs", app.currentView())
	}
	if app.crossOutliner.isProjectScoped() {
		t.Fatal("the shell pushed a scope before the parked project loaded")
	}

	failing := crossProjectsLoadedMsg{
		projects:   []*models.Project{alpha},
		generation: app.crossOutliner.crossLoadGen,
		failedLoads: []projectLoadFailure{
			{ProjectID: beta.ID, Detail: "failed to load project Beta: bad heading"},
		},
		focusProjectID: beta.ID,
	}
	_, cmd := app.Update(failing)
	if app.currentView() != ViewProjectHub {
		t.Fatalf("current view = %v, want the Project Hub kept open after a failed load", app.currentView())
	}
	if cmd == nil {
		t.Fatal("the failed shell load reported nothing")
	}
	app.Update(cmd())
	if app.err == nil || !strings.Contains(app.err.Error(), "bad heading") {
		t.Fatalf("shell error = %v, want the project-specific parse failure", app.err)
	}

	app.handleViewActionMessages(ApplyProjectScopeMsg{ProjectID: beta.ID})
	app.Update(crossProjectsLoadedMsg{
		projects:       []*models.Project{alpha, beta},
		generation:     app.crossOutliner.crossLoadGen,
		focusProjectID: beta.ID,
	})

	if app.crossOutliner.scopedProjectID != beta.ID {
		t.Fatalf("scoped project = %q, want the parked project", app.crossOutliner.scopedProjectID)
	}
	if app.CurrentRoute().ID() != TaskWorkspace {
		t.Fatalf("route = %s, want the Outliner reached once the focus landed", app.CurrentRoute().ID())
	}
	if app.projectByID(beta.ID) == nil {
		t.Fatal("the shell cache did not adopt the focused parked project")
	}
}

// A parked project is not a substitute for the base-workspace guidance: a
// project the base admits but the snapshot lacks is still an error, not a load.
func TestUnloadedActiveProjectStillReportsGuidance(t *testing.T) {
	app := contextHubApp(t)
	app.crossOutliner.loadedProjects = []*models.Project{app.projectByID("alpha")}

	if cmd, started := app.startParkedProjectFocus("beta"); started || cmd != nil {
		t.Fatal("an Active-boundary project started a parked focus load")
	}
	if _, started := app.startParkedProjectFocus("absent"); started {
		t.Fatal("an unknown project started a parked focus load")
	}
}

func TestParkedFocusFailureDetailPrefersTheProjectSpecificError(t *testing.T) {
	msg := crossProjectsLoadedMsg{
		projects: []*models.Project{{ID: "other"}},
		failedLoads: []projectLoadFailure{
			{ProjectID: "other", Detail: "failed to load project other: noise"},
			{ProjectID: "wanted", Detail: "failed to load project wanted: signal"},
		},
	}
	detail, failed := parkedFocusFailureDetail(msg, "wanted")
	if !failed || !strings.Contains(detail, "signal") {
		t.Fatalf("detail = %q failed = %v, want the requested project's own error", detail, failed)
	}

	msg.failedLoads = nil
	msg.projects = append(msg.projects, &models.Project{ID: "wanted"})
	if _, failed := parkedFocusFailureDetail(msg, "wanted"); failed {
		t.Fatal("a project present in the snapshot was reported as failed")
	}
}

func TestDiagnosticToLoadFailureCarriesTheProjectIdentity(t *testing.T) {
	failure := diagnosticToLoadFailure(workspace.Diagnostic{
		ProjectID:   "parked",
		ProjectName: "Parked",
		Detail:      errors.New("boom").Error(),
	})
	if failure.ProjectID != "parked" {
		t.Fatalf("failure project = %q, want the diagnostic's project", failure.ProjectID)
	}
	if failure.Detail != "failed to load project Parked: boom" {
		t.Fatalf("failure detail = %q, want the historical format preserved", failure.Detail)
	}
}
