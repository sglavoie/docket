package tui

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/testutil"
)

func newSession3CatalogApp(
	t *testing.T,
	dataDir string,
	projects ...*models.Project,
) (*AppModel, *OutlinerModel) {
	t.Helper()

	catalog := config.NewWorkspaceCatalog()
	for _, project := range projects {
		catalog.UpsertProject(project)
	}
	manager := config.NewManager(filepath.Join(dataDir, ".tt.json"))
	if err := manager.CommitCatalog(catalog); err != nil {
		t.Fatalf("CommitCatalog: %v", err)
	}
	runtime, err := config.LoadRuntimeState(dataDir)
	if err != nil {
		t.Fatalf("LoadRuntimeState: %v", err)
	}

	app := newTestAppModel()
	app.configManager = manager
	app.runtime = runtime
	app.deps = newShellDeps(runtime, nil, manager, nil)
	app.projects = make(map[string]*models.Project, len(projects))
	app.syncProjectCache()

	cross := NewOutlinerModel(nil, manager, runtime)
	cross.loadedProjects = app.activeProjects()
	for _, project := range cross.loadedProjects {
		if err := app.persister.Load(project); err != nil {
			t.Fatalf("load project %q: %v", project.ID, err)
		}
	}
	cross.selectOutlinerMode(ModeAll)
	cross.crossForceRebuild = true
	cross.crossProjectRebuildRows()
	app.crossOutliner = cross
	app.views.Register(ViewCrossProjectOutliner, cross)
	app.viewStack = []PresentationID{ViewDashboard, ViewCrossProjectOutliner}
	return app, cross
}

func session3ProjectByID(t *testing.T, projects []*models.Project, projectID string) *models.Project {
	t.Helper()
	for _, project := range projects {
		if project != nil && project.ID == projectID {
			return project
		}
	}
	t.Fatalf("project %q not found", projectID)
	return nil
}

func finishSession3CrossReload(t *testing.T, app *AppModel, cross *OutlinerModel) {
	t.Helper()
	if !cross.crossLoading {
		t.Fatal("cross-project reload was not armed")
	}
	_, followup := app.Update(cross.loadCrossProjectsCmd()())
	if followup != nil {
		runSession2AppCommand(t, app, followup)
	}
	if cross.crossLoading {
		t.Fatal("cross-project reload did not settle")
	}
}

//nolint:gocyclo // This regression keeps both entry points in one persistence-and-ordering matrix.
func TestProjectAdminAndPalettePinRequestsPersistAndReorder(t *testing.T) {
	dataDir := t.TempDir()
	alpha := newSession2Project(t, dataDir, "Alpha")
	zulu := newSession2Project(t, dataDir, "Zulu")
	app, cross := newSession3CatalogApp(t, dataDir, alpha, zulu)
	liveZulu := app.projectByID(zulu.ID)
	originalUpdatedAt := liveZulu.UpdatedAt

	admin := newProjectAdminMenu(liveZulu)
	for i, item := range admin.items {
		if item.action == adminActionPin {
			admin.cursor = i
			break
		}
	}
	raw := admin.dispatch()
	pin, ok := raw.(ProjectPinRequestMsg)
	if !ok || pin.Project != liveZulu || !pin.Pinned {
		t.Fatalf("admin pin action = %#v, want explicit pin request", raw)
	}
	if liveZulu.Pinned || liveZulu.PinnedAt != nil || !liveZulu.UpdatedAt.Equal(originalUpdatedAt) {
		t.Fatal("admin pin action mutated live metadata before the app commit")
	}

	handled, cmd := app.handleProjectMessages(pin)
	if !handled || cmd == nil {
		t.Fatalf("pin request = handled %v cmd %v, want handled reload/status batch", handled, cmd)
	}
	if !liveZulu.Pinned || liveZulu.PinnedAt == nil || !liveZulu.UpdatedAt.After(originalUpdatedAt) {
		t.Fatalf("live pin metadata not updated after commit: %#v", liveZulu)
	}
	if app.statusMessage != "Pinned" {
		t.Fatalf("pin status = %q, want Pinned", app.statusMessage)
	}
	finishSession3CrossReload(t, app, cross)
	if len(cross.loadedProjects) != 2 || cross.loadedProjects[0].ID != liveZulu.ID {
		t.Fatalf("pinned-first loaded order = %#v, want Zulu first", cross.loadedProjects)
	}

	disk, err := config.LoadRuntimeState(dataDir)
	if err != nil {
		t.Fatalf("reload pinned catalog: %v", err)
	}
	diskZulu := session3ProjectByID(t, disk.AllProjects(), liveZulu.ID)
	if !diskZulu.Pinned || diskZulu.PinnedAt == nil || !diskZulu.UpdatedAt.Equal(liveZulu.UpdatedAt) {
		t.Fatalf("durable pin metadata = %#v, want live committed metadata", diskZulu)
	}

	pinnedAt := *liveZulu.PinnedAt
	pinnedUpdatedAt := liveZulu.UpdatedAt
	palette := &CommandPaletteModel{}
	unpinAction := findPaletteAction(t, palette.getProjectActions(PaletteItem{Project: liveZulu}), "Unpin")
	raw = unpinAction.Handler()
	unpin, ok := raw.(ProjectPinRequestMsg)
	if !ok || unpin.Project != liveZulu || unpin.Pinned {
		t.Fatalf("palette unpin action = %#v, want explicit unpin request", raw)
	}
	if !liveZulu.Pinned || liveZulu.PinnedAt == nil ||
		!liveZulu.PinnedAt.Equal(pinnedAt) || !liveZulu.UpdatedAt.Equal(pinnedUpdatedAt) {
		t.Fatal("palette unpin action mutated live metadata before the app commit")
	}

	handled, cmd = app.handleProjectMessages(unpin)
	if !handled || cmd == nil {
		t.Fatalf("unpin request = handled %v cmd %v, want handled reload/status batch", handled, cmd)
	}
	if liveZulu.Pinned || liveZulu.PinnedAt != nil || !liveZulu.UpdatedAt.After(pinnedUpdatedAt) {
		t.Fatalf("live unpin metadata not updated after commit: %#v", liveZulu)
	}
	if app.statusMessage != "Unpinned" {
		t.Fatalf("unpin status = %q, want Unpinned", app.statusMessage)
	}
	finishSession3CrossReload(t, app, cross)
	if len(cross.loadedProjects) != 2 || cross.loadedProjects[0].ID != alpha.ID {
		t.Fatalf("unpinned loaded order = %#v, want alphabetical Alpha first", cross.loadedProjects)
	}

	disk, err = config.LoadRuntimeState(dataDir)
	if err != nil {
		t.Fatalf("reload unpinned catalog: %v", err)
	}
	diskZulu = session3ProjectByID(t, disk.AllProjects(), liveZulu.ID)
	if diskZulu.Pinned || diskZulu.PinnedAt != nil || !diskZulu.UpdatedAt.Equal(liveZulu.UpdatedAt) {
		t.Fatalf("durable unpin metadata = %#v, want live committed metadata", diskZulu)
	}
}

func TestProjectPinRequestFailureRollsBackLiveMetadata(t *testing.T) {
	blockedRoot := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(blockedRoot, []byte("blocked"), 0o600); err != nil {
		t.Fatalf("create blocked workspace root: %v", err)
	}

	project := testutil.NewTestProjectWithTasks("Rollback")
	catalog := config.NewWorkspaceCatalog()
	catalog.UpsertProject(project)
	domains := config.NewDomains()
	domains.Catalog = catalog
	runtime := config.NewRuntimeState(blockedRoot, domains)
	app := newTestAppModel()
	app.runtime = runtime
	app.projects = map[string]*models.Project{project.ID: project}
	app.currentProject = project
	app.statusMessage = "unchanged"
	originalUpdatedAt := project.UpdatedAt

	handled, cmd := app.handleProjectMessages(ProjectPinRequestMsg{Project: project, Pinned: true})

	if !handled || cmd != nil {
		t.Fatalf("failed pin request = handled %v cmd %v, want handled with no commands", handled, cmd)
	}
	if app.err == nil {
		t.Fatal("failed pin commit should surface an error")
	}
	if project.Pinned || project.PinnedAt != nil || !project.UpdatedAt.Equal(originalUpdatedAt) {
		t.Fatalf("failed pin commit changed live metadata: %#v", project)
	}
	if app.statusMessage != "unchanged" {
		t.Fatalf("failed pin commit changed status to %q", app.statusMessage)
	}
	runtimeProject := session3ProjectByID(t, app.runtime.AllProjects(), project.ID)
	if runtimeProject.Pinned || runtimeProject.PinnedAt != nil ||
		!runtimeProject.UpdatedAt.Equal(originalUpdatedAt) {
		t.Fatalf("failed pin commit changed runtime metadata: %#v", runtimeProject)
	}
}

func newBlockedProjectMutationApp(t *testing.T, archived bool) (*AppModel, *models.Project) {
	t.Helper()

	blockedRoot := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(blockedRoot, []byte("blocked"), 0o600); err != nil {
		t.Fatalf("create blocked workspace root: %v", err)
	}

	project := testutil.NewTestProjectWithTasks("Blocked Mutation")
	if archived {
		project.Archive()
	}
	catalog := config.NewWorkspaceCatalog()
	catalog.UpsertProject(project)
	domains := config.NewDomains()
	domains.Catalog = catalog
	domains.Machine.CurrentProject = project.ID
	runtime := config.NewRuntimeState(blockedRoot, domains)
	manager := config.NewManager(filepath.Join(blockedRoot, ".tt.json"))
	ledger := workspaceLedgerFromConfig(runtime)
	service := newTUIAppService(blockedRoot, manager.RuntimeDependencies(), ledger)

	app := newTestAppModel()
	app.configManager = manager
	app.runtime = runtime
	app.appService = service
	app.deps = newShellDeps(runtime, service, manager, ledger)
	app.projects = map[string]*models.Project{project.ID: project}
	app.currentProject = project
	app.statusMessage = "unchanged"
	return app, project
}

func TestProjectArchiveFailureDoesNotPublishLiveMetadata(t *testing.T) {
	app, project := newBlockedProjectMutationApp(t, false)
	originalUpdatedAt := project.UpdatedAt
	originalCurrent := app.currentProject

	handled, cmd := app.handleProjectMessages(ProjectArchivedMsg{Project: project})

	if !handled || cmd != nil {
		t.Fatalf("failed archive = handled %v cmd %v, want handled with no commands", handled, cmd)
	}
	if app.err == nil {
		t.Fatal("failed archive commit should surface an error")
	}
	if project.Archived || project.ArchivedAt != nil || !project.UpdatedAt.Equal(originalUpdatedAt) {
		t.Fatalf("failed archive published live metadata: %#v", project)
	}
	if app.currentProject != originalCurrent {
		t.Fatal("failed archive cleared the live current project")
	}
	if app.runtime.Machine().CurrentProject != project.ID {
		t.Fatalf("failed archive cleared runtime current project: %q", app.runtime.Machine().CurrentProject)
	}
	if app.statusMessage != "unchanged" {
		t.Fatalf("failed archive changed status to %q", app.statusMessage)
	}
	runtimeProject := session3ProjectByID(t, app.runtime.AllProjects(), project.ID)
	if runtimeProject.Archived || runtimeProject.ArchivedAt != nil ||
		!runtimeProject.UpdatedAt.Equal(originalUpdatedAt) {
		t.Fatalf("failed archive changed runtime metadata: %#v", runtimeProject)
	}
}

func TestProjectUnarchiveFailureDoesNotPublishLiveMetadata(t *testing.T) {
	app, project := newBlockedProjectMutationApp(t, true)
	originalArchivedAt := *project.ArchivedAt
	originalUpdatedAt := project.UpdatedAt

	handled, cmd := app.handleProjectMessages(ProjectRestoredMsg{Project: project})

	if !handled || cmd != nil {
		t.Fatalf("failed unarchive = handled %v cmd %v, want handled with no commands", handled, cmd)
	}
	if app.err == nil {
		t.Fatal("failed unarchive commit should surface an error")
	}
	if !project.Archived || project.ArchivedAt == nil ||
		!project.ArchivedAt.Equal(originalArchivedAt) || !project.UpdatedAt.Equal(originalUpdatedAt) {
		t.Fatalf("failed unarchive published live metadata: %#v", project)
	}
	if app.statusMessage != "unchanged" {
		t.Fatalf("failed unarchive changed status to %q", app.statusMessage)
	}
	runtimeProject := session3ProjectByID(t, app.runtime.AllProjects(), project.ID)
	if !runtimeProject.Archived || runtimeProject.ArchivedAt == nil ||
		!runtimeProject.ArchivedAt.Equal(originalArchivedAt) ||
		!runtimeProject.UpdatedAt.Equal(originalUpdatedAt) {
		t.Fatalf("failed unarchive changed runtime metadata: %#v", runtimeProject)
	}
}

func TestProjectStatusIconFailureDoesNotPublishLiveMetadata(t *testing.T) {
	app, project := newBlockedProjectMutationApp(t, false)
	project.StatusIcon = "on-track"
	project.StatusIconSource = "user"
	originalUpdatedAt := project.UpdatedAt
	app.viewStack = []PresentationID{ViewDashboard, ViewStatusIconPicker}

	handled, cmd := app.handleViewSelectionMessages(StatusIconSelectedMsg{
		Project: project,
		Key:     "at-risk",
	})

	if !handled || cmd != nil {
		t.Fatalf("failed status icon = handled %v cmd %v, want handled with no commands", handled, cmd)
	}
	if app.err == nil {
		t.Fatal("failed status-icon commit should surface an error")
	}
	if project.StatusIcon != "on-track" || project.StatusIconSource != "user" ||
		!project.UpdatedAt.Equal(originalUpdatedAt) {
		t.Fatalf("failed status-icon commit published live metadata: %#v", project)
	}
	if app.currentView() != ViewStatusInput {
		t.Fatalf("failed status-icon commit returned to %s, want status input", app.currentView())
	}
	if app.statusMessage != "unchanged" {
		t.Fatalf("failed status-icon commit changed status to %q", app.statusMessage)
	}
	runtimeProject := session3ProjectByID(t, app.runtime.AllProjects(), project.ID)
	if runtimeProject.StatusIcon != "" || !runtimeProject.UpdatedAt.Equal(originalUpdatedAt) {
		t.Fatalf("failed status-icon commit changed runtime metadata: %#v", runtimeProject)
	}
}

//nolint:gocyclo // The end-to-end regression deliberately asserts every confirmation transition.
func TestCrossPaletteArchivedProjectOpenConfirmsWithoutChangingScope(t *testing.T) {
	dataDir := t.TempDir()
	active := newSession2Project(t, dataDir, "Active")
	archived := newSession2Project(t, dataDir, "Archived")
	archived.Archive()
	app, cross := newSession3CatalogApp(t, dataDir, active, archived)
	liveActive := app.projectByID(active.ID)
	liveArchived := app.projectByID(archived.ID)
	app.currentProject = liveActive
	cross.pushProjectScope(liveActive.ID)
	cross.cursor = session2TaskRow(t, cross, liveActive.ID, active.Tasks[0].ID)

	originalProject := app.currentProject
	originalScope := cross.scopedProjectID
	originalMode := cross.activeOutlinerMode()
	originalCursor := cross.cursor
	originalArchivedAt := *liveArchived.ArchivedAt
	originalUpdatedAt := liveArchived.UpdatedAt

	app.viewStack = append(app.viewStack, ViewCommandPalette)
	_, paletteCmd := app.Update(CommandPaletteExecuteMsg{
		ResultMsg: SelectProjectMsg{Project: liveArchived},
	})
	if app.currentView() != ViewCrossProjectOutliner {
		t.Fatalf("archived Open left palette on %s, want cross-project Outliner", app.currentView())
	}
	if !app.projectArchiveConfirm.IsActive() || app.pendingProjectArchive != liveArchived ||
		app.pendingProjectArchiveArchive {
		t.Fatal("archived Open did not enter the unarchive confirmation flow")
	}
	if app.currentProject != originalProject || cross.scopedProjectID != originalScope ||
		cross.activeOutlinerMode() != originalMode || cross.cursor != originalCursor {
		t.Fatal("archived Open changed project navigation before confirmation")
	}
	if !liveArchived.Archived || liveArchived.ArchivedAt == nil ||
		!liveArchived.ArchivedAt.Equal(originalArchivedAt) || !liveArchived.UpdatedAt.Equal(originalUpdatedAt) {
		t.Fatal("archived Open mutated project metadata before confirmation")
	}
	runSession2AppCommand(t, app, paletteCmd)

	_, cancelCmd := app.Update(syntheticKey('n'))
	if cancelCmd != nil || app.projectArchiveConfirm.IsActive() {
		t.Fatal("cancel should dismiss unarchive confirmation without commands")
	}
	if app.currentProject != originalProject || cross.scopedProjectID != originalScope ||
		cross.activeOutlinerMode() != originalMode || cross.cursor != originalCursor ||
		!liveArchived.Archived {
		t.Fatal("canceling archived Open changed navigation or project state")
	}

	app.viewStack = append(app.viewStack, ViewCommandPalette)
	_, paletteCmd = app.Update(CommandPaletteExecuteMsg{
		ResultMsg: SelectProjectMsg{Project: liveArchived},
	})
	runSession2AppCommand(t, app, paletteCmd)
	_, confirmCmd := app.Update(syntheticKey('y'))
	if confirmCmd == nil {
		t.Fatal("confirming archived Open returned no command")
	}
	_, restoreCmd := app.Update(confirmCmd())
	if restoreCmd == nil {
		t.Fatal("confirmed archived Open returned no restore command")
	}
	_, reloadCmd := app.Update(restoreCmd())
	if reloadCmd == nil {
		t.Fatal("restored project did not return reload/status commands")
	}
	if liveArchived.Archived || liveArchived.ArchivedAt != nil {
		t.Fatal("confirmed archived Open did not restore the project")
	}
	if app.currentProject != originalProject || cross.scopedProjectID != originalScope ||
		cross.activeOutlinerMode() != originalMode {
		t.Fatal("restoring archived Open automatically changed the active project scope")
	}
	if app.statusMessage != "Unarchived Archived" {
		t.Fatalf("restore status = %q, want Unarchived Archived", app.statusMessage)
	}

	finishSession3CrossReload(t, app, cross)
	if session3ProjectByID(t, cross.loadedProjects, liveArchived.ID).Archived {
		t.Fatal("restored project was not reloaded into the active-project set")
	}
	if cross.scopedProjectID != originalScope {
		t.Fatalf("restored project changed scope to %q, want %q", cross.scopedProjectID, originalScope)
	}
	row := cross.rows[cross.cursor]
	if !row.isTaskRow() || row.ProjectID != liveActive.ID || row.task == nil ||
		row.task.ID != active.Tasks[0].ID {
		t.Fatalf("restore reload did not preserve cursor identity: %#v", row)
	}

	disk, err := config.LoadRuntimeState(dataDir)
	if err != nil {
		t.Fatalf("reload restored catalog: %v", err)
	}
	diskArchived := session3ProjectByID(t, disk.AllProjects(), liveArchived.ID)
	if diskArchived.Archived || diskArchived.ArchivedAt != nil {
		t.Fatalf("restored project was not durably active: %#v", diskArchived)
	}
	if !diskArchived.UpdatedAt.After(originalUpdatedAt) {
		t.Fatalf("restored project update timestamp = %s, want after %s", diskArchived.UpdatedAt, originalUpdatedAt)
	}
}
