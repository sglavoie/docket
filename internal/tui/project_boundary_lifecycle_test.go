package tui

import (
	"testing"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

// A boundary is durable project identity, so every lifecycle mutation that
// copies a project and writes the copy back — archive, restore, pin, a context
// relabel, undo, redo — has to carry it through untouched. The catalog defaults
// an absent BoundaryID to Active, which is precisely how a parked project could
// be silently readmitted to the task workspace by a mutation that never
// mentioned boundaries at all. These tests exist to make that impossible.

// parkedLifecycleFixture builds a real workspace holding one Active project and
// one parked project, both parked through the shell's own assignment path so no
// test hand-builds a catalog the product could not have produced.
func parkedLifecycleFixture(t *testing.T) (*session4LifecycleFixture, *models.Project, config.ProjectBoundary) {
	t.Helper()
	active := newSession4Project(t, "Active")
	parkedProject := newSession4Project(t, "Parked")
	fixture := newSession4LifecycleFixture(t, active, parkedProject)
	boundary := createParkedBoundary(t, fixture.app)

	live := fixture.app.projectByID(parkedProject.ID)
	handled, cmd := fixture.app.handleBoundaryMessages(AssignProjectBoundaryMsg{
		ProjectID:  live.ID,
		BoundaryID: boundary.ID,
	})
	if !handled {
		t.Fatal("AssignProjectBoundaryMsg was not handled")
	}
	assertNoBoundaryError(t, cmd)
	if got := fixture.app.runtime.Catalog().ProjectBoundaryID(live.ID); got != boundary.ID {
		t.Fatalf("fixture boundary = %q, want %q", got, boundary.ID)
	}
	return fixture, live, boundary
}

func lifecycleBoundaryOnDisk(t *testing.T, fixture *session4LifecycleFixture, projectID string) string {
	t.Helper()
	return reloadLifecycleRuntime(t, fixture).Catalog().ProjectBoundaryID(projectID)
}

func lifecyclePopulationHas(projects []*models.Project, projectID string) bool {
	for _, project := range projects {
		if project != nil && project.ID == projectID {
			return true
		}
	}
	return false
}

// Archival and the boundary answer different questions, so a parked project that
// is archived and later restored has to come back parked. Restoring it into
// Active would readmit tasks the user deliberately moved out of the workspace.
func TestArchiveAndRestoreKeepTheParkedBoundary(t *testing.T) {
	fixture, parkedProject, boundary := parkedLifecycleFixture(t)

	_, cmd := fixture.app.handleProjectMessages(ProjectArchivedMsg{Project: parkedProject})
	runSession2AppCommand(t, fixture.app, cmd)

	archived := reloadLifecycleRuntime(t, fixture)
	if !testProjectByID(t, archived.AllProjects(), parkedProject.ID).Archived {
		t.Fatal("archiving a parked project was not durable")
	}
	if got := archived.Catalog().ProjectBoundaryID(parkedProject.ID); got != boundary.ID {
		t.Fatalf("archived boundary = %q, want the retained %q", got, boundary.ID)
	}

	_, cmd = fixture.app.handleProjectMessages(ProjectRestoredMsg{Project: parkedProject})
	runSession2AppCommand(t, fixture.app, cmd)

	restored := reloadLifecycleRuntime(t, fixture)
	if testProjectByID(t, restored.AllProjects(), parkedProject.ID).Archived {
		t.Fatal("restoring a parked project was not durable")
	}
	if got := restored.Catalog().ProjectBoundaryID(parkedProject.ID); got != boundary.ID {
		t.Fatalf("restored boundary = %q, want the same %q it was archived from", got, boundary.ID)
	}
	if !lifecyclePopulationHas(restored.AvailableProjects(), parkedProject.ID) {
		t.Fatal("a restored parked project left the availability population")
	}
	if lifecyclePopulationHas(restored.TaskWorkspaceProjects(), parkedProject.ID) {
		t.Fatal("restoring a parked project readmitted it to the base task workspace")
	}
}

// Pinning is a presentation preference. It orders a project within whichever
// population already admits it and can never be a second way into the task
// workspace, or pinning would silently undo a park.
func TestPinningAParkedProjectDoesNotAdmitItToTheTaskWorkspace(t *testing.T) {
	fixture, parkedProject, boundary := parkedLifecycleFixture(t)

	_, cmd := fixture.app.handleProjectMessages(ProjectPinRequestMsg{Project: parkedProject, Pinned: true})
	runSession2AppCommand(t, fixture.app, cmd)

	pinned := reloadLifecycleRuntime(t, fixture)
	if !testProjectByID(t, pinned.AllProjects(), parkedProject.ID).Pinned {
		t.Fatal("pinning a parked project was not durable")
	}
	if got := pinned.Catalog().ProjectBoundaryID(parkedProject.ID); got != boundary.ID {
		t.Fatalf("pinned boundary = %q, want the retained %q", got, boundary.ID)
	}
	if lifecyclePopulationHas(pinned.TaskWorkspaceProjects(), parkedProject.ID) {
		t.Fatal("pinning admitted a parked project to the base task workspace")
	}
	available := pinned.AvailableProjects()
	if len(available) == 0 || available[0].ID != parkedProject.ID {
		t.Fatalf("pinned parked project did not sort first in availability: %#v", available)
	}

	_, cmd = fixture.app.handleProjectMessages(ProjectPinRequestMsg{Project: parkedProject, Pinned: false})
	runSession2AppCommand(t, fixture.app, cmd)

	unpinned := reloadLifecycleRuntime(t, fixture)
	if testProjectByID(t, unpinned.AllProjects(), parkedProject.ID).Pinned {
		t.Fatal("unpinning a parked project was not durable")
	}
	if got := unpinned.Catalog().ProjectBoundaryID(parkedProject.ID); got != boundary.ID {
		t.Fatalf("unpinned boundary = %q, want the retained %q", got, boundary.ID)
	}
}

// Contexts and boundaries are independent axes of the same catalog record. A
// context rename rewrites one field across many projects and a membership edit
// rewrites it across two more, so both are the classic place a whole-record
// rewrite would reset the boundary of every project it touched.
func TestContextRenameAndMembershipKeepBoundaries(t *testing.T) {
	fixture, parkedProject, boundary := parkedLifecycleFixture(t)
	activeID := "Active-project"
	if fixture.app.projectByID(activeID) == nil {
		t.Fatalf("fixture is missing the Active project %q", activeID)
	}
	mutator := NewContextMutator(fixture.app.runtime, fixture.app.configManager, fixture.app.appService)

	if _, err := mutator.SaveRegularContext(RegularContextSaveRequest{
		Name:       "shared",
		ProjectIDs: []string{activeID, parkedProject.ID},
	}); err != nil {
		t.Fatalf("create context across both boundaries: %v", err)
	}
	if _, err := mutator.SaveRegularContext(RegularContextSaveRequest{
		OriginalName: "shared",
		Name:         "renamed",
		ProjectIDs:   []string{activeID, parkedProject.ID},
	}); err != nil {
		t.Fatalf("rename context: %v", err)
	}
	if _, err := mutator.SaveRegularContext(RegularContextSaveRequest{
		OriginalName: "renamed",
		Name:         "renamed",
		ProjectIDs:   []string{activeID},
	}); err != nil {
		t.Fatalf("drop the parked member: %v", err)
	}

	committed := reloadLifecycleRuntime(t, fixture)
	if got := committed.Catalog().ProjectBoundaryID(parkedProject.ID); got != boundary.ID {
		t.Fatalf("context edits moved the parked project to %q, want %q", got, boundary.ID)
	}
	if got := committed.Catalog().ProjectBoundaryID(activeID); got != models.ActiveBoundaryID {
		t.Fatalf("context edits moved the Active project to %q", got)
	}
	dropped := testProjectByID(t, committed.AllProjects(), parkedProject.ID)
	if dropped.Context == "renamed" {
		t.Fatal("the parked project kept a context it was removed from")
	}
}

// A parked project's archive is one undoable transaction like any other. Undo
// has to restore the project to the boundary it was archived from, and redo has
// to reapply the archive without disturbing that boundary either.
func TestUndoAndRedoOfAParkedArchiveKeepTheBoundary(t *testing.T) {
	fixture, parkedProject, boundary := parkedLifecycleFixture(t)

	_, cmd := fixture.app.handleProjectMessages(ProjectArchivedMsg{Project: parkedProject})
	runSession2AppCommand(t, fixture.app, cmd)

	runSession2AppCommand(t, fixture.app, fixture.app.handleLedgerTaskUndoRedo(false))
	undone := reloadLifecycleRuntime(t, fixture)
	if testProjectByID(t, undone.AllProjects(), parkedProject.ID).Archived {
		t.Fatal("undo did not restore the archived parked project")
	}
	if got := undone.Catalog().ProjectBoundaryID(parkedProject.ID); got != boundary.ID {
		t.Fatalf("undo left the project in %q, want the parked %q", got, boundary.ID)
	}

	runSession2AppCommand(t, fixture.app, fixture.app.handleLedgerTaskUndoRedo(true))
	redone := reloadLifecycleRuntime(t, fixture)
	if !testProjectByID(t, redone.AllProjects(), parkedProject.ID).Archived {
		t.Fatal("redo did not reapply the archive")
	}
	if got := redone.Catalog().ProjectBoundaryID(parkedProject.ID); got != boundary.ID {
		t.Fatalf("redo left the project in %q, want the parked %q", got, boundary.ID)
	}
}

// Undo reverses a catalog transaction the same way the forward mutation applied
// it, so every surface that groups or filters by boundary has to be rebuilt from
// the restored catalog. The Project Hub is the strictest case: it is the one
// place a parked project is listed, so a stale Hub shows membership the
// workspace no longer has.
func TestUndoRefreshesTheBoundaryGroupedProjectHub(t *testing.T) {
	fixture, parkedProject, boundary := parkedLifecycleFixture(t)
	hub := openProjectHub(t, fixture.app)
	parkedRow := ListItemID(projectHubItemPrefix + parkedProject.ID)
	parkedCategory := projectHubBoundaryCategory(boundary.ID)
	if got := projectHubItemByID(t, hub, parkedRow).CategoryID; got != parkedCategory {
		t.Fatalf("Hub grouped the parked project under %q, want %q", got, parkedCategory)
	}

	entry, ok, err := workspaceLedgerFromConfig(fixture.app.runtime).SelectNewestUndo(storage.GlobalScope())
	if err != nil || !ok || entry.Action != "Set Project Boundary" {
		t.Fatalf("newest undo entry = %#v/%v/%v, want the boundary assignment", entry, ok, err)
	}
	runSession2AppCommand(t, fixture.app, fixture.app.handleLedgerTaskUndoRedo(false))

	if got := fixture.app.runtime.Catalog().ProjectBoundaryID(parkedProject.ID); got != models.ActiveBoundaryID {
		t.Fatalf("undo left the project in %q, want %q", got, models.ActiveBoundaryID)
	}
	if got := projectHubItemByID(t, hub, parkedRow).CategoryID; got != projectHubCategoryActive {
		t.Fatalf("undo left the Hub grouping the project under %q, want the Active group", got)
	}
}

// The Boundary Manager reports member counts read from the same catalog, so an
// undo that moves a project between boundaries has to be visible in the open
// administration surface rather than only in the listing behind it.
func TestUndoRefreshesTheOpenBoundaryManagerCounts(t *testing.T) {
	fixture, _, boundary := parkedLifecycleFixture(t)
	_, manager := openBoundaryManager(t, fixture.app)

	const parkedCounts = "1 project · 1 unarchived · 0 archived"
	if got := boundaryManagerRow(t, manager, boundary.Name).SecondaryText; got != parkedCounts {
		t.Fatalf("%s counts = %q, want %q", boundary.Name, got, parkedCounts)
	}

	runSession2AppCommand(t, fixture.app, fixture.app.handleLedgerTaskUndoRedo(false))

	const emptyCounts = "0 projects · 0 unarchived · 0 archived"
	if got := boundaryManagerRow(t, manager, boundary.Name).SecondaryText; got != emptyCounts {
		t.Fatalf("undo left %s reporting %q, want %q", boundary.Name, got, emptyCounts)
	}
}

// A catalog written outside this shell is observed by refreshing the runtime,
// and the refresh has to republish the boundary onto the cached project pointers
// every lifecycle mutation copies. Otherwise the next archive would write back a
// boundary the workspace had already left.
func TestExternalCatalogRefreshRepublishesBoundaryToCachedProjects(t *testing.T) {
	fixture, parkedProject, boundary := parkedLifecycleFixture(t)
	if got := fixture.app.projectByID(parkedProject.ID).BoundaryID; got != boundary.ID {
		t.Fatalf("cached boundary = %q, want %q", got, boundary.ID)
	}

	store := config.NewDomainFileStore(fixture.dataDir)
	catalog, err := store.LoadCatalog()
	if err != nil {
		t.Fatalf("load catalog: %v", err)
	}
	if _, err = catalog.AssignProjectBoundary(parkedProject.ID, models.ActiveBoundaryID); err != nil {
		t.Fatalf("external unpark: %v", err)
	}
	if err = store.SaveCatalog(catalog); err != nil {
		t.Fatalf("save external catalog: %v", err)
	}

	if !fixture.app.refreshRuntime() {
		t.Fatal("the runtime refused to observe the external catalog write")
	}
	if got := fixture.app.projectByID(parkedProject.ID).BoundaryID; got != models.ActiveBoundaryID {
		t.Fatalf("refreshed cached boundary = %q, want %q", got, models.ActiveBoundaryID)
	}

	_, cmd := fixture.app.handleProjectMessages(ProjectArchivedMsg{Project: fixture.app.projectByID(parkedProject.ID)})
	runSession2AppCommand(t, fixture.app, cmd)
	if got := lifecycleBoundaryOnDisk(t, fixture, parkedProject.ID); got != models.ActiveBoundaryID {
		t.Fatalf("archive wrote back the stale boundary %q", got)
	}
}

// Notes ownership is availability, not workspace membership. Parking a project
// removes its tasks from every base projection but must not orphan its notes:
// the owner has to survive the park live, in an already-open Notes manager, and
// disappear only when the project is archived.
func TestParkingKeepsANotesOwnerAndArchivingDropsIt(t *testing.T) {
	active := newSession4Project(t, "Active")
	owner := newSession4Project(t, "Owner")
	fixture := newSession4LifecycleFixture(t, active, owner)
	boundary := createParkedBoundary(t, fixture.app)

	_, _ = fixture.app.handleNotesMessages(ShowNotesManagerMsg{Scope: ProjectNotesScope(owner.ID)})
	notes := fixture.app.getActiveNotesView()
	if notes == nil {
		t.Fatal("Notes manager did not open")
	}
	location := app.ProjectNoteLocation(owner.ID)
	if !notes.locationAvailable(location) {
		t.Fatal("the owner was missing before it was even parked")
	}

	handled, cmd := fixture.app.handleBoundaryMessages(AssignProjectBoundaryMsg{
		ProjectID:  owner.ID,
		BoundaryID: boundary.ID,
	})
	if !handled {
		t.Fatal("AssignProjectBoundaryMsg was not handled")
	}
	assertNoBoundaryError(t, cmd)

	notes = fixture.app.getActiveNotesView()
	if notes == nil || !notes.locationAvailable(location) {
		t.Fatal("parking a project removed it from the open Notes owner catalog")
	}
	if fixture.app.manageHub.params.NotesScope != ProjectNotesScope(owner.ID) {
		t.Fatalf("parking normalized the Notes scope away: %#v", fixture.app.manageHub.params.NotesScope)
	}

	_, cmd = fixture.app.handleProjectMessages(ProjectArchivedMsg{Project: fixture.app.projectByID(owner.ID)})
	runSession2AppCommand(t, fixture.app, cmd)

	notes = fixture.app.getActiveNotesView()
	if notes == nil || notes.locationAvailable(location) {
		t.Fatal("archiving a project left it in the Notes owner catalog")
	}
	if fixture.app.manageHub.params.NotesScope != AllNotesScope() {
		t.Fatalf("archived owner scope = %#v, want All Notes", fixture.app.manageHub.params.NotesScope)
	}
}
