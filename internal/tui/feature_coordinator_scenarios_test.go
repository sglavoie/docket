package tui

import (
	"path/filepath"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/google/uuid"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

// This file exercises the primary-view feature coordinators end to end through
// the real transactional storage stack: each scenario runs a coordinator
// mutation, reloads the affected project from disk, renders it through the
// query surface, and undoes the mutation through the workspace ledger — the
// mutation→reload→render→undo cycle every migrated primary workflow must honor.

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
	return app.NewSnapshot(r.projects, revs, docs), nil
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
	features FeatureCoordinators
	project  *models.Project
}

// newScenarioEnv seeds a single project (with the given tasks) on disk and wires
// the coordinators over the real transactional stack.
func newScenarioEnv(t *testing.T, slug string, tasks ...*models.Task) *scenarioEnv {
	t.Helper()
	dataDir := t.TempDir()
	ledger := storage.NewWorkspaceLedger(filepath.Join(dataDir, ".tt", "undo"), []string{dataDir})
	commitSvc := storage.NewCommitServiceWithLedger(filepath.Join(dataDir, ".tt", "transactions"), []string{dataDir}, ledger)

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
		features: newFeatureCoordinators(svc, svc),
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

// runResult executes a coordinator command and asserts it produced a committed
// transaction-result message, returning the application outcome.
func runResult(t *testing.T, cmd tea.Cmd) app.TaskResult {
	t.Helper()
	if cmd == nil {
		t.Fatal("coordinator returned a nil command")
	}
	switch msg := cmd().(type) {
	case TransactionResultMsg:
		return msg.Result
	case TransactionErrorMsg:
		t.Fatalf("expected a committed transaction, got error: %v", msg.Err)
	default:
		t.Fatalf("expected TransactionResultMsg, got %T", msg)
	}
	return app.TaskResult{}
}

// runError executes a coordinator command and asserts it produced a
// transaction-error message, returning the error.
func runError(t *testing.T, cmd tea.Cmd) error {
	t.Helper()
	if cmd == nil {
		t.Fatal("coordinator returned a nil command")
	}
	switch msg := cmd().(type) {
	case TransactionErrorMsg:
		if msg.Err == nil {
			t.Fatal("TransactionErrorMsg carried a nil error")
		}
		return msg.Err
	case TransactionResultMsg:
		t.Fatalf("expected a rejected mutation, got committed result for %q", msg.Result.Title)
	default:
		t.Fatalf("expected TransactionErrorMsg, got %T", msg)
	}
	return nil
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

// TestCaptureScenario covers the Quick Capture / task-form primary workflow:
// capture commits a new task, the reloaded project shows it, the query surface
// renders the higher task count, and undo removes it.
func TestCaptureScenario(t *testing.T) {
	env := newScenarioEnv(t, "capture")

	res := runResult(t, env.features.Capture.Capture(app.CaptureRequest{
		ProjectID: env.project.ID,
		Title:     "Write the migration notes",
	}))
	if res.Title != "Write the migration notes" {
		t.Fatalf("captured title = %q", res.Title)
	}
	if res.Revision == "" {
		t.Fatal("capture result carried no committed revision")
	}

	// reload + render: the committed task is on disk and visible to the query.
	if got := env.reload(t); got.GetTaskByID(res.TaskID) == nil {
		t.Fatal("captured task absent from reloaded project")
	}
	summaries, err := env.features.ProjectSelection.Projects()
	if err != nil {
		t.Fatalf("project selection query: %v", err)
	}
	if got := projectTaskCount(t, summaries, env.project.ID); got != 1 {
		t.Fatalf("rendered task count = %d, want 1", got)
	}

	// undo: the capture is reversed on disk.
	env.undo(t)
	if got := env.reload(t); got.GetTaskByID(res.TaskID) != nil {
		t.Fatal("captured task survived undo")
	}
}

// TestInboxTriageScenario covers the system Inbox triage workflow: an inbox task
// transitions to next, the change persists and renders, and undo restores inbox.
func TestInboxTriageScenario(t *testing.T) {
	task := seedTask(t, 1, "Triage me", models.StateInbox)
	env := newScenarioEnv(t, "inbox", task)

	res := runResult(t, env.features.Inbox.Triage(task.ID, models.StateBacklog))
	if res.State != models.StateBacklog {
		t.Fatalf("triaged state = %q, want next", res.State)
	}
	if got := env.reload(t).GetTaskByID(task.ID); got == nil || got.State() != models.StateBacklog {
		t.Fatalf("reloaded state = %v, want next", got)
	}

	env.undo(t)
	if got := env.reload(t).GetTaskByID(task.ID); got == nil || got.State() != models.StateInbox {
		t.Fatalf("undo state = %v, want inbox", got)
	}
}

// TestCompleteScenario covers the project-tree/outliner/inbox completion
// workflow: completing a ready task terminates it, and undo reopens it.
func TestCompleteScenario(t *testing.T) {
	task := seedTask(t, 1, "Finish me", models.StateBacklog)
	env := newScenarioEnv(t, "complete", task)

	res := runResult(t, env.features.ProjectTree.Complete(task.ID))
	if res.State != models.StateDone {
		t.Fatalf("completed state = %q, want done", res.State)
	}
	if got := env.reload(t).GetTaskByID(task.ID); got == nil || !got.IsTerminalState() {
		t.Fatalf("reloaded task not terminal after complete: %v", got)
	}

	env.undo(t)
	if got := env.reload(t).GetTaskByID(task.ID); got == nil || got.IsTerminalState() {
		t.Fatalf("undo did not reopen the completed task: %v", got)
	}
}

// TestSetPriorityScenario covers the project-tree/outliner priority workflow:
// the outliner coordinator sets a priority that persists, and undo restores it.
func TestSetPriorityScenario(t *testing.T) {
	task := seedTask(t, 1, "Prioritize me", models.StateBacklog)
	original := task.Priority
	env := newScenarioEnv(t, "priority", task)

	runResult(t, env.features.Outliner.SetPriority(task.ID, 1))
	if got := env.reload(t).GetTaskByID(task.ID); got == nil || got.Priority != 1 {
		t.Fatalf("reloaded priority = %v, want 1", got)
	}

	env.undo(t)
	if got := env.reload(t).GetTaskByID(task.ID); got == nil || got.Priority != original {
		t.Fatalf("undo priority = %v, want %d", got, original)
	}
}

// TestProjectSelectionQueryScenario covers the query-only project-selection
// surface: it lists and resolves projects through the read model.
func TestProjectSelectionQueryScenario(t *testing.T) {
	task := seedTask(t, 1, "Only task", models.StateBacklog)
	env := newScenarioEnv(t, "selection", task)

	summaries, err := env.features.ProjectSelection.Projects()
	if err != nil {
		t.Fatalf("projects query: %v", err)
	}
	if len(summaries) != 1 {
		t.Fatalf("selection projects = %d, want 1", len(summaries))
	}
	summary, err := env.features.ProjectSelection.Project(env.project.ID)
	if err != nil {
		t.Fatalf("project query: %v", err)
	}
	if summary.Slug != "selection" || summary.TaskCount != 1 {
		t.Fatalf("project summary = %+v", summary)
	}
}

// TestCoordinatorRejectionScenario covers the error leg: a mutation the
// application layer rejects surfaces a TransactionErrorMsg and commits nothing.
func TestCoordinatorRejectionScenario(t *testing.T) {
	env := newScenarioEnv(t, "reject")

	// Triaging a task that does not exist is rejected before any commit.
	if err := runError(t, env.features.Inbox.Triage("no-such-task", models.StateBacklog)); err == nil {
		t.Fatal("expected an error for an unknown task")
	}
}

func projectTaskCount(t *testing.T, summaries []app.ProjectSummary, projectID string) int {
	t.Helper()
	for _, s := range summaries {
		if s.ID == projectID {
			return s.TaskCount
		}
	}
	t.Fatalf("project %s absent from summaries", projectID)
	return 0
}
