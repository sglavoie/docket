package outliner

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/google/uuid"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/tui/coordinator"
)

func newTestConfig() *config.RuntimeState {
	return newTestConfigAt("")
}

func newTestConfigAt(dataDir string) *config.RuntimeState {
	return config.NewRuntimeState(dataDir, config.NewDomains())
}

func testConfigWithProjects(projects ...*models.Project) *config.RuntimeState {
	runtime := newTestConfig()
	setTestProjects(runtime, projects)
	return runtime
}

func setTestProjects(runtime *config.RuntimeState, projects []*models.Project) {
	catalog := config.NewWorkspaceCatalog()
	for _, project := range projects {
		if err := catalog.UpsertProject(project); err != nil {
			panic(err)
		}
	}
	runtime.SetCatalogInMemory(catalog)
}

func testRuntimeAt(dataDir string, runtime *config.RuntimeState) *config.RuntimeState {
	if runtime == nil {
		return config.EmptyRuntimeState(dataDir)
	}
	domains := &config.Domains{
		Catalog:  runtime.Catalog(),
		Planning: runtime.Planning(),
		Machine:  runtime.Machine(),
	}
	if dataDir != "" {
		// Checked non-catalog commits now require the authoritative catalog
		// identity. Persist that one foundation artifact for real-directory test
		// runtimes instead of letting each test mint an unrelated domain ID.
		manager := config.NewManager(filepath.Join(dataDir, ".tt.json"))
		if err := manager.CommitCatalog(domains.Catalog); err != nil {
			panic(fmt.Sprintf("seed test workspace catalog: %v", err))
		}
	}
	return config.NewRuntimeState(dataDir, domains)
}

func newCrossProjectTestEnv(t *testing.T, taskGroups ...[]*models.Task) *crossProjectTestEnv {
	t.Helper()
	dataDir := t.TempDir()
	ledger := storage.NewWorkspaceLedger(filepath.Join(dataDir, ".tt", "undo"), []string{dataDir})
	commitSvc := storage.NewCommitServiceWithLedger(filepath.Join(dataDir, ".tt", "transactions"), []string{dataDir}, ledger)

	projects := make([]*models.Project, len(taskGroups))
	for i, tasks := range taskGroups {
		slug := fmt.Sprintf("out%d", i)
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
		projects[i] = project
	}

	repo := newScenarioRepo(commitSvc, projects...)
	repo.seed(t)
	svc := app.NewServiceWithArtifacts(repo, nil, nil, nil)
	coords := coordinator.NewSecondaryCoordinators(svc)
	return &crossProjectTestEnv{
		dataDir: dataDir, ledger: ledger, svc: svc,
		bulk: coords.Bulk, taskItem: coords.TaskItem, projects: projects,
	}
}

// crossProjectTestEnv seeds projects on disk and wires the coordinators used
// by cross-project Outliner mutation tests.

// crossProjectTestEnv seeds projects on disk and wires the coordinators used
// by cross-project Outliner mutation tests.
type crossProjectTestEnv struct {
	dataDir  string
	ledger   *storage.WorkspaceLedger
	svc      *app.Service
	bulk     coordinator.BulkCoordinator
	taskItem coordinator.TaskItemCoordinator
	projects []*models.Project
}

func (e *crossProjectTestEnv) reload(t *testing.T, idx int) *models.Project {
	t.Helper()
	project := e.projects[idx]
	fresh := &models.Project{ID: project.ID, Name: project.Name, Slug: project.Slug, Path: project.Path}
	if err := storage.LoadProject(fresh); err != nil {
		t.Fatalf("reload project %d: %v", idx, err)
	}
	return fresh
}

func (e *crossProjectTestEnv) undo(t *testing.T) {
	t.Helper()
	if _, err := e.ledger.Undo(storage.GlobalScope()); err != nil {
		t.Fatalf("ledger undo: %v", err)
	}
}

// runResult executes a coordinator command and asserts it produced a committed
// transaction-result message, returning the application outcome.

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

// legacyStateForStatusSection collapses a legacy (status, section) pair onto a
// single canonical WorkflowState, applying the migration precedence used across
// the refactored tests: canceled > done/Completed > in_progress >
// blocked/Waiting > standby/Standby > maybe/Someday > Inbox > Next > ready.
// Earlier rules win when multiple legacy signals are present.

// legacyStateForStatusSection collapses a legacy (status, section) pair onto a
// single canonical WorkflowState, applying the migration precedence used across
// the refactored tests: canceled > done/Completed > in_progress >
// blocked/Waiting > standby/Standby > maybe/Someday > Inbox > Next > ready.
// Earlier rules win when multiple legacy signals are present.
func legacyStateForStatusSection(status models.WorkflowState, section string) models.WorkflowState {
	switch {
	case status == models.StateCanceled:
		return models.StateCanceled
	case status == models.StateDone || section == string(models.StateDone):
		return models.StateDone
	case status == models.StateDoing:
		return models.StateDoing
	case status == models.StateWaiting || section == string(models.StateWaiting):
		return models.StateWaiting
	case status == models.StatePaused || section == string(models.StatePaused):
		return models.StatePaused
	case status == models.StateSomeday || section == string(models.StateSomeday):
		return models.StateSomeday
	case section == string(models.StateInbox):
		return models.StateInbox
	case section == string(models.StateBacklog):
		return models.StateBacklog
	default:
		return models.StateBacklog
	}
}

func newScenarioRepo(svc *storage.CommitService, projects ...*models.Project) *scenarioRepo {
	return &scenarioRepo{svc: svc, projects: projects, revisions: make(map[string]string)}
}

func isFormattingDirective(value string) bool {
	if len(value) != 2 || value[0] != '%' {
		return false
	}
	return strings.ContainsRune("vTtbcdoOqxXUeEfFgGsp", rune(value[1]))
}

// scenarioRepo is a storage-backed app.Repository that drives coordinator
// mutations through the transactional commit service with the workspace undo
// ledger attached, mirroring the CLI/TUI production repository. It tracks each
// project's revision from the commit results so optimistic concurrency holds.
type scenarioRepo struct {
	svc       *storage.CommitService
	projects  []*models.Project
	revisions map[string]string
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

// CommitArtifactTransaction makes the scenario repository exercise the same
// note-only commit and ledger boundary as the production TUI repository.

// CommitArtifactTransaction makes the scenario repository exercise the same
// note-only commit and ledger boundary as the production TUI repository.
func (r *scenarioRepo) CommitArtifactTransaction(request storage.ArtifactCommitRequest) (string, error) {
	return r.svc.CommitArtifactTransaction(request)
}
