package tui

// The storage-backed scenario environment the feature-coordinator tests mutate
// through: a real CommitService with the undo ledger attached, plus the seeding
// and reload steps a scenario repeats. Both coordinator scenario files read it.

import (
	"path/filepath"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/google/uuid"

	"github.com/sglavoie/docket/internal/app"
	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/storage"
	"github.com/sglavoie/docket/internal/tui/coordinator"
)

// scenarioRepo is a storage-backed app.Repository that drives coordinator
// mutations through the transactional commit service with the workspace undo
// ledger attached, mirroring the CLI/TUI production repository. It tracks each
// project's revision from the commit results so optimistic concurrency holds.
type scenarioRepo struct {
	svc       *storage.CommitService
	projects  []*models.Project
	revisions map[string]string
}

func newScenarioRepo(svc *storage.CommitService, projects ...*models.Project) *scenarioRepo {
	return &scenarioRepo{svc: svc, projects: projects, revisions: make(map[string]string)}
}

func (r *scenarioRepo) seed(t *testing.T) {
	t.Helper()
	changesets := make([]storage.ChangeSet, len(r.projects))
	for i, p := range r.projects {
		changesets[i] = storage.ChangeSet{Project: p, UndoAction: "seed"}
	}
	results, err := r.svc.CommitMulti(changesets)
	if err != nil {
		t.Fatalf("seed commit: %v", err)
	}
	for i, p := range r.projects {
		r.revisions[p.ID] = results[i].Revision
	}
}

// Snapshot serves the in-memory project pointers with each project's revision
// read from DISK, exactly like the production cli/tui repositories. Reusing the
// cached commit-result revision is not safe across chained mutations: the
// result revision is computed from the in-memory model, whose canonical form
// can differ from a reparse of its own written file (a task edited within the
// minute of its creation serializes `updated:` from the in-memory model but
// parses back as updated==created, which the canonical renderer omits).
func (r *scenarioRepo) Snapshot() (app.Snapshot, error) {
	revs := make(map[string]string, len(r.projects))
	docs := make(map[string]*storage.ProjectDocument, len(r.projects))
	for _, p := range r.projects {
		doc, err := storage.LoadProjectDocument(p)
		if err != nil {
			return nil, err
		}
		revs[p.ID] = doc.Revision
		docs[p.ID] = doc
	}
	return app.NewPartialSnapshot(r.projects, revs, docs, nil), nil
}

func (r *scenarioRepo) Commit(changesets ...storage.ChangeSet) ([]*storage.CommitResult, error) {
	results, err := r.svc.CommitMulti(changesets)
	if err != nil {
		return nil, err
	}
	for i := range changesets {
		r.revisions[changesets[i].Project.ID] = results[i].Revision
	}
	return results, nil
}

// scenarioEnv bundles the wired application service, its coordinators, the undo
// ledger, and the seeded project so each scenario reads as mutation → assert.
type scenarioEnv struct {
	dataDir  string
	ledger   *storage.WorkspaceLedger
	svc      *app.Service
	features coordinator.FeatureCoordinators
	project  *models.Project
}

// newScenarioEnv seeds a single project (with the given tasks) on disk and wires
// the coordinators over the real transactional stack.
func newScenarioEnv(t *testing.T, slug string, tasks ...*models.Task) *scenarioEnv {
	t.Helper()
	dataDir := t.TempDir()
	ledger := storage.NewWorkspaceLedger(filepath.Join(dataDir, ".docket", "undo"), []string{dataDir})
	commitSvc := storage.NewCommitServiceWithLedger(filepath.Join(dataDir, ".docket", "transactions"), []string{dataDir}, ledger)

	project, err := models.NewProject(uuid.New().String(), slug, "")
	if err != nil {
		t.Fatalf("NewProject: %v", err)
	}
	project.Slug = slug
	project.Path = filepath.Join(dataDir, "projects", slug)
	project.Tasks = tasks
	next := 1
	for _, task := range tasks {
		if task.ShortID >= next {
			next = task.ShortID + 1
		}
	}
	project.NextShortID = next
	project.BuildTaskTree()

	repo := newScenarioRepo(commitSvc, project)
	repo.seed(t)
	svc := app.NewServiceWithArtifacts(repo, nil, nil, nil)
	return &scenarioEnv{
		dataDir:  dataDir,
		ledger:   ledger,
		svc:      svc,
		features: coordinator.NewFeatureCoordinators(svc),
		project:  project,
	}
}

// reload reads the project fresh from disk so an assertion sees exactly the
// committed bytes, not the in-memory pointer the use case mutated.
func (e *scenarioEnv) reload(t *testing.T) *models.Project {
	t.Helper()
	fresh := &models.Project{ID: e.project.ID, Name: e.project.Name, Slug: e.project.Slug, Path: e.project.Path}
	if err := storage.LoadProject(fresh); err != nil {
		t.Fatalf("reload project: %v", err)
	}
	return fresh
}

// undo reverts the most recent committed transaction through the workspace
// ledger, restoring the on-disk state the mutation replaced.
func (e *scenarioEnv) undo(t *testing.T) {
	t.Helper()
	if _, err := e.ledger.Undo(storage.GlobalScope()); err != nil {
		t.Fatalf("ledger undo: %v", err)
	}
}

func seedTask(t *testing.T, shortID int, title string, state models.WorkflowState) *models.Task {
	t.Helper()
	task, err := models.NewGenericTask(uuid.New().String(), shortID, title)
	if err != nil {
		t.Fatalf("NewTask: %v", err)
	}
	task.ApplyWorkflowState(state)
	return task
}

// runResult executes a coordinator command and asserts it produced a committed
// transaction-result message, returning the application outcome.
func runResult(t *testing.T, cmd tea.Cmd) app.TaskResult {
	t.Helper()
	if cmd == nil {
		t.Fatal("coordinator returned a nil command")
	}
	switch msg := cmd().(type) {
	case coordinator.TransactionResultMsg:
		return msg.Result
	case coordinator.TransactionErrorMsg:
		t.Fatalf("expected a committed transaction, got error: %v", msg.Err)
	default:
		t.Fatalf("expected coordinator.TransactionResultMsg, got %T", msg)
	}
	return app.TaskResult{}
}
