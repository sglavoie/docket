package tui

import (
	"errors"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/google/uuid"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

// boundaryEnv wires a real workspace, ledger, and shell so a boundary mutation
// is exercised through the same checked transaction the product uses. commitErr
// makes every artifact commit fail, which is how the speculative-state
// assertions distinguish "not published" from "never prepared".
type boundaryEnv struct {
	dataDir string
	ledger  *storage.WorkspaceLedger
	shell   *AppModel
	planned *models.Project
	other   *models.Project
	task    *models.Task
}

type failingArtifactRepo struct {
	*scenarioRepo
	err error
}

func (r *failingArtifactRepo) CommitArtifactTransaction(storage.ArtifactCommitRequest) (string, error) {
	return "", r.err
}

func newBoundaryEnv(t *testing.T, commitErr error) *boundaryEnv {
	t.Helper()
	dataDir := t.TempDir()
	ledger := storage.NewWorkspaceLedger(filepath.Join(dataDir, ".tt", "undo"), []string{dataDir})
	commitSvc := storage.NewCommitServiceWithLedger(
		filepath.Join(dataDir, ".tt", "transactions"), []string{dataDir}, ledger,
	)

	task, err := models.NewGenericTask(uuid.New().String(), 1, "Planned work")
	if err != nil {
		t.Fatalf("NewGenericTask: %v", err)
	}
	planned := newBoundaryTestProject(t, dataDir, "planned", "Planned", []*models.Task{task})
	other := newBoundaryTestProject(t, dataDir, "other", "Other", nil)

	repo := newScenarioRepo(commitSvc, planned, other)
	repo.seed(t)
	var svc *app.Service
	if commitErr != nil {
		svc = app.NewServiceWithArtifacts(&failingArtifactRepo{scenarioRepo: repo, err: commitErr}, nil, nil, nil)
	} else {
		svc = app.NewServiceWithArtifacts(repo, nil, nil, nil)
	}

	mgr := config.NewManager(filepath.Join(dataDir, ".tt.json"))
	cfg := newTestConfigAt(dataDir)
	setTestProjects(cfg, []*models.Project{planned, other})
	locator := config.ProjectTaskLocator{ProjectID: planned.ID, TaskID: task.ID}
	cfg.Planning().Today = []config.ProjectTaskLocator{locator}
	cfg.Planning().SetNext([]config.ProjectTaskLocator{locator})
	if err = saveTestRuntime(mgr, cfg); err != nil {
		t.Fatalf("seed workspace domains: %v", err)
	}

	shell := &AppModel{
		configManager: mgr,
		runtime:       testRuntimeAt(dataDir, cfg),
		projects:      map[string]*models.Project{planned.ID: planned, other.ID: other},
		persister:     NewProjectService(),
		appService:    svc,
		views:         NewViewRegistry(),
		width:         80,
		height:        24,
	}
	shell.rebuildPlanningFromRuntime()
	return &boundaryEnv{
		dataDir: dataDir, ledger: ledger, shell: shell,
		planned: planned, other: other, task: task,
	}
}

func newBoundaryTestProject(t *testing.T, dataDir, slug, name string, tasks []*models.Task) *models.Project {
	t.Helper()
	project, err := models.NewProject(uuid.New().String(), slug, "")
	if err != nil {
		t.Fatalf("NewProject: %v", err)
	}
	project.Name = name
	project.Slug = slug
	project.Path = filepath.Join(dataDir, "projects", slug)
	project.Tasks = tasks
	project.NextShortID = len(tasks) + 1
	project.BuildTaskTree()
	return project
}

// createBoundary drives the shell's own create path so the tests never
// hand-build a boundary the product could not have produced.
func (e *boundaryEnv) createBoundary(t *testing.T, name string) config.ProjectBoundary {
	t.Helper()
	handled, cmd := e.shell.handleBoundaryMessages(CreateBoundaryMsg{Name: name})
	if !handled {
		t.Fatal("CreateBoundaryMsg was not handled")
	}
	assertNoBoundaryError(t, cmd)
	for _, boundary := range e.shell.runtime.Catalog().Boundaries {
		if boundary.Name == name {
			return boundary
		}
	}
	t.Fatalf("boundary %q was not published to the runtime catalog", name)
	return config.ProjectBoundary{}
}

func (e *boundaryEnv) diskCatalog(t *testing.T) *config.WorkspaceCatalog {
	t.Helper()
	catalog, err := config.NewDomainFileStore(e.dataDir).LoadCatalog()
	if err != nil {
		t.Fatalf("load catalog from disk: %v", err)
	}
	return catalog
}

func (e *boundaryEnv) diskPlanning(t *testing.T) *config.PlanningState {
	t.Helper()
	planning, err := config.NewDomainFileStore(e.dataDir).LoadPlanning()
	if err != nil {
		t.Fatalf("load planning from disk: %v", err)
	}
	return planning
}

func assertNoBoundaryError(t *testing.T, cmd tea.Cmd) {
	t.Helper()
	for _, msg := range drainTUITestCmd(t, cmd) {
		if errMsg, ok := msg.(ErrorMsg); ok {
			t.Fatalf("boundary mutation failed: %v", errMsg.Err)
		}
	}
}

// writeExternalBoundary commits a real catalog change through a second manager,
// standing in for another process that advanced workspace.json after the shell
// captured its runtime snapshot.
func (e *boundaryEnv) writeExternalBoundary(t *testing.T, name string) config.ProjectBoundary {
	t.Helper()
	catalog, err := config.NewDomainFileStore(e.dataDir).LoadCatalog()
	if err != nil {
		t.Fatalf("load catalog for the concurrent write: %v", err)
	}
	created, err := catalog.CreateBoundary(name)
	if err != nil {
		t.Fatalf("prepare the concurrent boundary: %v", err)
	}
	if err = config.NewManager(filepath.Join(e.dataDir, ".tt.json")).CommitCatalog(catalog); err != nil {
		t.Fatalf("commit the concurrent catalog write: %v", err)
	}
	return created
}

// writeExternalToday is the planning-domain counterpart, returning the Today
// membership the shell must not erase.
func (e *boundaryEnv) writeExternalToday(t *testing.T) []config.ProjectTaskLocator {
	t.Helper()
	planning, err := config.NewDomainFileStore(e.dataDir).LoadPlanning()
	if err != nil {
		t.Fatalf("load planning for the concurrent write: %v", err)
	}
	today := []config.ProjectTaskLocator{{ProjectID: e.other.ID, TaskID: uuid.New().String()}}
	planning.Today = today
	if err = config.NewManager(filepath.Join(e.dataDir, ".tt.json")).CommitPlanning(planning); err != nil {
		t.Fatalf("commit the concurrent planning write: %v", err)
	}
	return today
}

func assertBoundaryError(t *testing.T, cmd tea.Cmd) error {
	t.Helper()
	for _, msg := range drainTUITestCmd(t, cmd) {
		if errMsg, ok := msg.(ErrorMsg); ok {
			return errMsg.Err
		}
	}
	t.Fatal("expected the boundary mutation to report an error")
	return nil
}

// The shell reports a rejected mutation as rendered text rather than a wrapped
// sentinel, so the cause is asserted on the message the user actually sees.
func assertBoundaryErrorMentions(t *testing.T, cmd tea.Cmd, cause error) {
	t.Helper()
	if err := assertBoundaryError(t, cmd); !strings.Contains(err.Error(), cause.Error()) {
		t.Fatalf("error = %v, want it to report %v", err, cause)
	}
}

// A shell without a writable workspace still has to observe its own mutations,
// because the in-memory path is what isolated views and tests run on.
func TestBoundaryMutationsApplyInMemoryWithoutWorkspace(t *testing.T) {
	project, err := models.NewProject(uuid.New().String(), "solo", "")
	if err != nil {
		t.Fatal(err)
	}
	runtime := testConfigWithProjects(project)
	runtime.Planning().Today = []config.ProjectTaskLocator{{ProjectID: project.ID, TaskID: "t1"}}
	shell := &AppModel{
		runtime:  runtime,
		projects: map[string]*models.Project{project.ID: project},
		views:    NewViewRegistry(),
	}

	// Update, not the handler, so the shell's message routing is covered too.
	shell.Update(CreateBoundaryMsg{Name: "On Hold"})
	boundaries := shell.runtime.Catalog().Boundaries
	if len(boundaries) != 2 {
		t.Fatalf("in-memory create produced %d boundaries, want 2", len(boundaries))
	}
	parked := boundaries[1]

	handled, cmd := shell.handleBoundaryMessages(AssignProjectBoundaryMsg{
		ProjectID: project.ID, BoundaryID: parked.ID,
	})
	if !handled {
		t.Fatal("AssignProjectBoundaryMsg was not handled")
	}
	assertNoBoundaryError(t, cmd)
	if got := shell.runtime.Catalog().ProjectBoundaryID(project.ID); got != parked.ID {
		t.Fatalf("in-memory boundary = %q, want %q", got, parked.ID)
	}
	if len(shell.runtime.Planning().Today) != 0 {
		t.Fatal("in-memory parking must clear Today")
	}
	if got := shell.projects[project.ID].BoundaryID; got != parked.ID {
		t.Fatalf("cached project boundary = %q, want %q", got, parked.ID)
	}
}

// Parking a project is the only assignment that spans two domains, so its
// catalog change and planning cleanup must be one ledger entry: undo restores
// both or the workspace is left describing a project that is parked yet still
// planned for today.
func TestParkingProjectCommitsCatalogAndPlanningInOneUndoableTransaction(t *testing.T) {
	env := newBoundaryEnv(t, nil)
	parked := env.createBoundary(t, "On Hold")

	handled, cmd := env.shell.handleBoundaryMessages(AssignProjectBoundaryMsg{
		ProjectID: env.planned.ID, BoundaryID: parked.ID,
	})
	if !handled {
		t.Fatal("AssignProjectBoundaryMsg was not handled")
	}
	assertNoBoundaryError(t, cmd)

	if got := env.diskCatalog(t).ProjectBoundaryID(env.planned.ID); got != parked.ID {
		t.Fatalf("committed boundary = %q, want %q", got, parked.ID)
	}
	planning := env.diskPlanning(t)
	if len(planning.Today) != 0 || len(planning.GetNext()) != 0 {
		t.Fatalf("parking left planning memberships behind: today=%v next=%v", planning.Today, planning.GetNext())
	}
	if len(env.shell.today) != 0 || len(env.shell.nextOrder) != 0 {
		t.Fatal("shell planning caches still list the parked project")
	}
	if got := env.shell.projects[env.planned.ID].BoundaryID; got != parked.ID {
		t.Fatalf("published project boundary = %q, want %q", got, parked.ID)
	}

	if _, err := env.ledger.Undo(storage.GlobalScope()); err != nil {
		t.Fatalf("ledger undo: %v", err)
	}
	restored := env.diskCatalog(t)
	if got := restored.ProjectBoundaryID(env.planned.ID); got != models.ActiveBoundaryID {
		t.Fatalf("undo left the project in %q, want %q", got, models.ActiveBoundaryID)
	}
	if _, ok := restored.Boundary(parked.ID); !ok {
		t.Fatal("undo removed the boundary the assignment did not create")
	}
	planning = env.diskPlanning(t)
	if len(planning.Today) != 1 || len(planning.GetNext()) != 1 {
		t.Fatalf("undo did not restore planning memberships: today=%v next=%v", planning.Today, planning.GetNext())
	}

	if _, err := env.ledger.Redo(storage.GlobalScope()); err != nil {
		t.Fatalf("ledger redo: %v", err)
	}
	if got := env.diskCatalog(t).ProjectBoundaryID(env.planned.ID); got != parked.ID {
		t.Fatalf("redo boundary = %q, want %q", got, parked.ID)
	}
	if planning = env.diskPlanning(t); len(planning.Today) != 0 || len(planning.GetNext()) != 0 {
		t.Fatal("redo did not re-clear planning memberships")
	}
}

// Only a live project leaving Active can strand a planning membership. Every
// other assignment stays a catalog-only change, so it must not rewrite planning.
func TestUnparkingKeepsPlanningUntouched(t *testing.T) {
	env := newBoundaryEnv(t, nil)
	parked := env.createBoundary(t, "On Hold")

	_, cmd := env.shell.handleBoundaryMessages(AssignProjectBoundaryMsg{
		ProjectID: env.other.ID, BoundaryID: parked.ID,
	})
	assertNoBoundaryError(t, cmd)
	_, cmd = env.shell.handleBoundaryMessages(AssignProjectBoundaryMsg{
		ProjectID: env.other.ID, BoundaryID: models.ActiveBoundaryID,
	})
	assertNoBoundaryError(t, cmd)

	if got := env.diskCatalog(t).ProjectBoundaryID(env.other.ID); got != models.ActiveBoundaryID {
		t.Fatalf("unparked boundary = %q, want %q", got, models.ActiveBoundaryID)
	}
	planning := env.diskPlanning(t)
	if len(planning.Today) != 1 || len(planning.GetNext()) != 1 {
		t.Fatalf("a catalog-only assignment rewrote planning: today=%v next=%v", planning.Today, planning.GetNext())
	}
}

// Deleting a boundary moves its members in the same transaction that removes the
// definition, so undo can never restore a boundary without its membership.
func TestDeleteBoundaryReassignsMembersInOneUndoableTransaction(t *testing.T) {
	env := newBoundaryEnv(t, nil)
	source := env.createBoundary(t, "On Hold")
	destination := env.createBoundary(t, "Someday")

	for _, id := range []string{env.planned.ID, env.other.ID} {
		_, cmd := env.shell.handleBoundaryMessages(AssignProjectBoundaryMsg{ProjectID: id, BoundaryID: source.ID})
		assertNoBoundaryError(t, cmd)
	}

	handled, cmd := env.shell.handleBoundaryMessages(DeleteBoundaryMsg{
		BoundaryID: source.ID, DestinationID: destination.ID,
	})
	if !handled {
		t.Fatal("DeleteBoundaryMsg was not handled")
	}
	assertNoBoundaryError(t, cmd)

	catalog := env.diskCatalog(t)
	if _, ok := catalog.Boundary(source.ID); ok {
		t.Fatal("deleted boundary survived the commit")
	}
	for _, id := range []string{env.planned.ID, env.other.ID} {
		if got := catalog.ProjectBoundaryID(id); got != destination.ID {
			t.Fatalf("member %q landed in %q, want %q", id, got, destination.ID)
		}
	}

	if _, err := env.ledger.Undo(storage.GlobalScope()); err != nil {
		t.Fatalf("ledger undo: %v", err)
	}
	catalog = env.diskCatalog(t)
	if _, ok := catalog.Boundary(source.ID); !ok {
		t.Fatal("undo did not restore the deleted boundary")
	}
	if got := catalog.ProjectBoundaryID(env.planned.ID); got != source.ID {
		t.Fatalf("undo left the member in %q, want %q", got, source.ID)
	}
}

func TestBoundaryRenameKeepsMembershipAndCommits(t *testing.T) {
	env := newBoundaryEnv(t, nil)
	parked := env.createBoundary(t, "On Hold")
	_, cmd := env.shell.handleBoundaryMessages(AssignProjectBoundaryMsg{
		ProjectID: env.other.ID, BoundaryID: parked.ID,
	})
	assertNoBoundaryError(t, cmd)

	handled, cmd := env.shell.handleBoundaryMessages(RenameBoundaryMsg{BoundaryID: parked.ID, Name: "  Paused  "})
	if !handled {
		t.Fatal("RenameBoundaryMsg was not handled")
	}
	assertNoBoundaryError(t, cmd)

	catalog := env.diskCatalog(t)
	renamed, ok := catalog.Boundary(parked.ID)
	if !ok || renamed.Name != "Paused" {
		t.Fatalf("renamed boundary = %+v ok=%v, want name %q", renamed, ok, "Paused")
	}
	if got := catalog.ProjectBoundaryID(env.other.ID); got != parked.ID {
		t.Fatalf("rename moved a member to %q", got)
	}
}

// A rejected commit — a stale revision or any other failure — must leave the
// durable artifacts, the runtime snapshot, and the shell caches exactly as they
// were, because the mutation was only ever applied to an abandoned clone.
func TestBoundaryCommitFailureLeavesNoSpeculativeState(t *testing.T) {
	failures := map[string]error{
		"stale revision": storage.ErrStaleRevision,
		"commit failure": errors.New("workspace is read-only"),
	}
	for label, commitErr := range failures {
		t.Run(label, func(t *testing.T) {
			env := newBoundaryEnv(t, commitErr)

			handled, cmd := env.shell.handleBoundaryMessages(CreateBoundaryMsg{Name: "On Hold"})
			if !handled {
				t.Fatal("CreateBoundaryMsg was not handled")
			}
			assertBoundaryErrorMentions(t, cmd, commitErr)
			if got := len(env.shell.runtime.Catalog().Boundaries); got != 1 {
				t.Fatalf("failed create published %d boundaries, want 1", got)
			}
			if got := len(env.diskCatalog(t).Boundaries); got != 1 {
				t.Fatalf("failed create wrote %d boundaries to disk, want 1", got)
			}

			// A parking assignment fails across both domains, so neither the
			// catalog membership nor the planning cleanup may survive.
			env.shell.runtime.Catalog().Boundaries = append(
				env.shell.runtime.Catalog().Boundaries,
				config.ProjectBoundary{ID: uuid.New().String(), Name: "On Hold"},
			)
			parked := env.shell.runtime.Catalog().Boundaries[1]
			handled, cmd = env.shell.handleBoundaryMessages(AssignProjectBoundaryMsg{
				ProjectID: env.planned.ID, BoundaryID: parked.ID,
			})
			if !handled {
				t.Fatal("AssignProjectBoundaryMsg was not handled")
			}
			assertBoundaryErrorMentions(t, cmd, commitErr)
			if got := env.shell.runtime.Catalog().ProjectBoundaryID(env.planned.ID); got != models.ActiveBoundaryID {
				t.Fatalf("failed assignment published boundary %q", got)
			}
			if len(env.shell.runtime.Planning().Today) != 1 {
				t.Fatal("failed assignment published the planning cleanup")
			}
			if len(env.shell.today) != 1 {
				t.Fatal("failed assignment cleared the shell planning cache")
			}
			planning := env.diskPlanning(t)
			if len(planning.Today) != 1 || len(planning.GetNext()) != 1 {
				t.Fatalf("failed assignment rewrote durable planning: today=%v next=%v",
					planning.Today, planning.GetNext())
			}
		})
	}
}

// A concurrent writer is what optimistic concurrency exists for. The shell
// mutates domains cloned from the snapshot it loaded, so encoding those
// candidates against the revision on disk at commit time would adopt the
// intervening write as the expected baseline and silently replace it. Both
// boundary write sets — catalog-only and catalog-plus-planning — must instead
// be stamped with the snapshot's own revisions and fail closed.
func TestBoundaryRejectsConcurrentCatalogAndPlanningWrites(t *testing.T) {
	t.Run("catalog-only mutation", func(t *testing.T) {
		env := newBoundaryEnv(t, nil)
		external := env.writeExternalBoundary(t, "Written Elsewhere")

		handled, cmd := env.shell.handleBoundaryMessages(CreateBoundaryMsg{Name: "On Hold"})
		if !handled {
			t.Fatal("CreateBoundaryMsg was not handled")
		}
		assertConcurrentWriteRejected(t, cmd, "workspace.json")

		catalog := env.diskCatalog(t)
		if _, ok := catalog.Boundary(external.ID); !ok {
			t.Fatal("the rejected create erased the concurrent boundary")
		}
		if len(catalog.Boundaries) != 2 {
			t.Fatalf("durable boundaries = %v, want Active plus the concurrent one", catalog.Boundaries)
		}
		if got := len(env.shell.runtime.Catalog().Boundaries); got != 1 {
			t.Fatalf("rejected create published %d boundaries to the runtime, want 1", got)
		}
	})

	t.Run("assignment spanning catalog and planning", func(t *testing.T) {
		env := newBoundaryEnv(t, nil)
		parked := env.createBoundary(t, "On Hold")
		external := env.writeExternalToday(t)

		handled, cmd := env.shell.handleBoundaryMessages(AssignProjectBoundaryMsg{
			ProjectID: env.planned.ID, BoundaryID: parked.ID,
		})
		if !handled {
			t.Fatal("AssignProjectBoundaryMsg was not handled")
		}
		assertConcurrentWriteRejected(t, cmd, "planning.json")

		// The catalog artifact's own revision still matched, so only an all-or-
		// nothing transaction leaves workspace.json untouched here.
		if got := env.diskCatalog(t).ProjectBoundaryID(env.planned.ID); got != models.ActiveBoundaryID {
			t.Fatalf("rejected assignment wrote boundary %q to disk", got)
		}
		if got := env.diskPlanning(t).Today; !slices.Equal(got, external) {
			t.Fatalf("rejected assignment rewrote durable Today to %v, want %v", got, external)
		}
		if got := env.shell.runtime.Catalog().ProjectBoundaryID(env.planned.ID); got != models.ActiveBoundaryID {
			t.Fatalf("rejected assignment published boundary %q", got)
		}
		if len(env.shell.runtime.Planning().Today) != 1 || len(env.shell.today) != 1 {
			t.Fatal("rejected assignment published the planning cleanup")
		}
	})
}

// A concurrent write must be reported as the specific artifact that moved, not
// as a generic failure, because that is what tells a user to reload.
func assertConcurrentWriteRejected(t *testing.T, cmd tea.Cmd, artifact string) {
	t.Helper()
	err := assertBoundaryError(t, cmd)
	if !strings.Contains(err.Error(), artifact) || !strings.Contains(err.Error(), "changed on disk") {
		t.Fatalf("error = %v, want it to report that %s changed on disk", err, artifact)
	}
}

func TestBoundaryMutationRejectionsSurfaceErrors(t *testing.T) {
	env := newBoundaryEnv(t, nil)
	parked := env.createBoundary(t, "On Hold")

	rejections := map[string]struct {
		msg  tea.Msg
		want error
	}{
		"duplicate name": {
			msg: CreateBoundaryMsg{Name: "on hold"}, want: config.ErrBoundaryNameConflict,
		},
		"empty name": {
			msg: CreateBoundaryMsg{Name: "   "}, want: config.ErrInvalidBoundaryName,
		},
		"rename active": {
			msg:  RenameBoundaryMsg{BoundaryID: models.ActiveBoundaryID, Name: "Live"},
			want: config.ErrProtectedActiveBoundary,
		},
		"rename unknown": {
			msg: RenameBoundaryMsg{BoundaryID: "no-such-boundary", Name: "Fine"}, want: config.ErrBoundaryNotFound,
		},
		"delete active": {
			msg:  DeleteBoundaryMsg{BoundaryID: models.ActiveBoundaryID, DestinationID: parked.ID},
			want: config.ErrProtectedActiveBoundary,
		},
		"delete into itself": {
			msg:  DeleteBoundaryMsg{BoundaryID: parked.ID, DestinationID: parked.ID},
			want: config.ErrBoundaryDestinationInvalid,
		},
		"assign unknown boundary": {
			msg:  AssignProjectBoundaryMsg{ProjectID: env.planned.ID, BoundaryID: "no-such-boundary"},
			want: config.ErrBoundaryNotFound,
		},
	}
	for label, tc := range rejections {
		t.Run(label, func(t *testing.T) {
			handled, cmd := env.shell.handleBoundaryMessages(tc.msg)
			if !handled {
				t.Fatalf("%T was not handled", tc.msg)
			}
			assertBoundaryErrorMentions(t, cmd, tc.want)
			catalog := env.diskCatalog(t)
			if len(catalog.Boundaries) != 2 {
				t.Fatalf("a rejected mutation changed the durable boundaries: %v", catalog.Boundaries)
			}
			if got := catalog.ProjectBoundaryID(env.planned.ID); got != models.ActiveBoundaryID {
				t.Fatalf("a rejected mutation moved a project to %q", got)
			}
		})
	}

	handled, cmd := env.shell.handleBoundaryMessages(AssignProjectBoundaryMsg{
		ProjectID: "no-such-project", BoundaryID: parked.ID,
	})
	if !handled {
		t.Fatal("AssignProjectBoundaryMsg was not handled")
	}
	if err := assertBoundaryError(t, cmd); !strings.Contains(err.Error(), "no-such-project") {
		t.Fatalf("unknown project error = %v, want it to name the missing project", err)
	}
}

// An assignment that changes nothing must not open a transaction at all, so the
// durable revision the next mutation checks against cannot drift.
func TestNoOpBoundaryAssignmentSkipsTheTransaction(t *testing.T) {
	env := newBoundaryEnv(t, errors.New("no commit may be attempted"))

	handled, cmd := env.shell.handleBoundaryMessages(AssignProjectBoundaryMsg{
		ProjectID: env.planned.ID, BoundaryID: models.ActiveBoundaryID,
	})
	if !handled {
		t.Fatal("AssignProjectBoundaryMsg was not handled")
	}
	assertNoBoundaryError(t, cmd)
	if env.shell.statusMessage == "" || !env.shell.statusIsInfo {
		t.Fatalf("a no-op assignment must report an informational status, got %q", env.shell.statusMessage)
	}
	if today := env.diskPlanning(t).Today; len(today) != 1 {
		t.Fatalf("a no-op assignment touched planning: %v", today)
	}
}
