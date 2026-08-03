package tui

import (
	"path/filepath"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/google/uuid"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

// TestTUITaskMoveCommitsAtomicTransactionAndLedgerUndo verifies the migrated
// cross-project move commits the source removal, destination insertion, and the
// durable planning refs (Focus Queue, Daily Plan) as ONE app-layer transaction,
// undoable/redoable through the workspace ledger — replacing the retired
// saveBothProjectsAsync per-project raw-save + .undo orchestration.
func TestTUITaskMoveCommitsAtomicTransactionAndLedgerUndo(t *testing.T) {
	env := newMoveHandlerEnv(t, "move-task-1", "Move me")
	source, dest := env.source, env.dest
	moved := source.GetTaskByID("move-task-1")

	env.cfg.Planning().SetNext([]config.ProjectTaskLocator{{TaskID: moved.ID, ProjectID: source.ID}})
	env.cfg.Planning().Today = []config.ProjectTaskLocator{{TaskID: moved.ID, ProjectID: source.ID}}
	if err := saveTestRuntime(env.mgr, env.cfg); err != nil {
		t.Fatalf("save config: %v", err)
	}

	app := env.appModel()
	app.nextOrder = []PlanningTask{{Task: moved, Project: source}}
	app.nextOrderIDs = map[TaskIdentity]bool{taskIdentityFor(moved, source): true}
	app.today = []PlanningTask{{Task: moved, Project: source}}
	app.todayIDs = map[TaskIdentity]bool{taskIdentityFor(moved, source): true}
	handled, cmd := app.handleMoveTaskToProject(MoveTaskToProjectMsg{
		Task:          moved,
		SourceProject: source,
		DestProjectID: dest.ID,
	})
	if !handled {
		t.Fatal("move message was not handled")
	}
	assertMoveMsg(t, cmd)

	// Task committed to dest, removed from source (on disk).
	if env.reloadProject(t, source).GetTaskByID(moved.ID) != nil {
		t.Fatal("source should no longer contain moved task on disk")
	}
	if env.reloadProject(t, dest).GetTaskByID(moved.ID) == nil {
		t.Fatal("dest should contain moved task on disk")
	}
	// Durable planning refs point to the destination.
	assertDiskConfigRef(t, env.mgr, moved.ID, dest.ID)
	if app.nextOrder[0].Project.ID != dest.ID || app.today[0].Project.ID != dest.ID {
		t.Fatal("in-memory refs should point to dest after move")
	}

	// Ledger undo restores the move and the folded durable planning refs.
	if _, err := env.ledger.Undo(storage.GlobalScope()); err != nil {
		t.Fatalf("ledger undo: %v", err)
	}
	if env.reloadProject(t, source).GetTaskByID(moved.ID) == nil {
		t.Fatal("source should contain task again after undo")
	}
	if env.reloadProject(t, dest).GetTaskByID(moved.ID) != nil {
		t.Fatal("dest should not contain task after undo")
	}
	assertDiskConfigRef(t, env.mgr, moved.ID, source.ID)

	// Ledger redo re-applies the move and durable refs.
	if _, err := env.ledger.Redo(storage.GlobalScope()); err != nil {
		t.Fatalf("ledger redo: %v", err)
	}
	if env.reloadProject(t, dest).GetTaskByID(moved.ID) == nil {
		t.Fatal("dest should contain task after redo")
	}
	assertDiskConfigRef(t, env.mgr, moved.ID, dest.ID)
}

func TestTUITaskMovePreservesDuplicateIDReferencesInOtherProject(t *testing.T) {
	env := newMoveHandlerEnv(t, "shared-id", "Move beta/shared")
	source, dest := env.source, env.dest
	moved := source.GetTaskByID("shared-id")
	other := &models.Project{ID: "alpha", Name: "Alpha"}

	env.cfg.Planning().SetNext([]config.ProjectTaskLocator{
		{TaskID: moved.ID, ProjectID: source.ID},
		{TaskID: moved.ID, ProjectID: other.ID},
	})
	env.cfg.Planning().Today = []config.ProjectTaskLocator{
		{TaskID: moved.ID, ProjectID: source.ID},
		{TaskID: moved.ID, ProjectID: other.ID},
	}
	if err := saveTestRuntime(env.mgr, env.cfg); err != nil {
		t.Fatalf("save config: %v", err)
	}

	m := env.appModel()
	m.nextOrder = []PlanningTask{{Task: moved, Project: source}, {Task: moved, Project: other}}
	m.today = []PlanningTask{{Task: moved, Project: source}, {Task: moved, Project: other}}
	_, cmd := m.handleMoveTaskToProject(MoveTaskToProjectMsg{
		Task: moved, SourceProject: source, DestProjectID: dest.ID,
	})
	assertMoveMsg(t, cmd)

	reloaded, err := config.NewManager(env.mgr.GetConfigPath()).LoadRuntimeState()
	if err != nil {
		t.Fatalf("reload config: %v", err)
	}
	if reloaded.Planning().Next[0].ProjectID != dest.ID || reloaded.Planning().Next[1].ProjectID != other.ID ||
		reloaded.Planning().Today[0].ProjectID != dest.ID || reloaded.Planning().Today[1].ProjectID != other.ID {
		t.Fatalf("qualified move reassigned another project's refs: focus=%+v daily=%+v",
			reloaded.Planning().Next, reloaded.Planning().Today)
	}
	if m.nextOrder[0].Project.ID != dest.ID || m.nextOrder[1].Project.ID != other.ID ||
		m.today[0].Project.ID != dest.ID || m.today[1].Project.ID != other.ID {
		t.Fatalf("qualified move reassigned another project's in-memory refs")
	}
}

// TestTUITaskMoveUsesCanonicalTask verifies the migrated move
// resolves and moves the CANONICAL source task (not a stale view copy passed in
// the message).
func TestTUITaskMoveUsesCanonicalTask(t *testing.T) {
	env := newMoveHandlerEnv(t, "move-task-1", "Move me")
	source, dest := env.source, env.dest
	moved := source.GetTaskByID("move-task-1")
	oldCanonicalShortID := moved.ShortID

	app := env.appModel()
	staleTask, err := models.NewGenericTask(moved.ID, 99, "Stale view copy")
	if err != nil {
		t.Fatalf("new stale task: %v", err)
	}
	handled, cmd := app.handleMoveTaskToProject(MoveTaskToProjectMsg{
		Task:          staleTask,
		SourceProject: source,
		DestProjectID: dest.ID,
	})
	if !handled {
		t.Fatal("move message was not handled")
	}
	movedMsg := assertMoveMsg(t, cmd)
	if movedMsg.OldShortID != oldCanonicalShortID {
		t.Fatalf("OldShortID = %d, want canonical short ID %d", movedMsg.OldShortID, oldCanonicalShortID)
	}
	if staleTask.State() != models.StateBacklog {
		t.Fatalf("stale message task must not be mutated, got state %q", staleTask.State())
	}

	if env.reloadProject(t, source).GetTaskByID(moved.ID) != nil {
		t.Fatal("source should no longer contain moved task")
	}
	destTask := env.reloadProject(t, dest).GetTaskByID(moved.ID)
	if destTask == nil {
		t.Fatal("destination should contain moved task")
	}
	if destTask.Title != "Move me" {
		t.Fatalf("destination task title = %q, want canonical %q", destTask.Title, "Move me")
	}
	if destTask.State() != models.StateBacklog {
		t.Fatalf("destination task state = %q, want ready", destTask.State())
	}
}

// TestTUITaskMoveRollsBackOnCommitFailure verifies the whole move transaction
// aborts with zero writes when the folded config artifact targets a path outside
// the approved data-dir root: the task stays in the source project and the
// destination stays empty.
func TestTUITaskMoveRollsBackOnCommitFailure(t *testing.T) {
	env := newMoveHandlerEnv(t, "move-task-1", "Move me")
	source, dest := env.source, env.dest
	moved := source.GetTaskByID("move-task-1")

	// Point the config manager OUTSIDE the approved data-dir root so the durable
	// planning-ref artifact folded into the move transaction has an out-of-root
	// target the journal rejects, aborting the whole transaction.
	outsideDir := t.TempDir()
	env.mgr = config.NewManager(filepath.Join(outsideDir, ".tt.json"))
	env.cfg.Planning().SetNext([]config.ProjectTaskLocator{{TaskID: moved.ID, ProjectID: source.ID}})

	app := env.appModel()
	handled, cmd := app.handleMoveTaskToProject(MoveTaskToProjectMsg{
		Task:          moved,
		SourceProject: source,
		DestProjectID: dest.ID,
	})
	if !handled {
		t.Fatal("move message was not handled")
	}
	if _, ok := drainSingleMsg(t, cmd).(ErrorMsg); !ok {
		t.Fatalf("expected ErrorMsg from failed move, got %T", drainSingleMsg(t, cmd))
	}

	if env.reloadProject(t, source).GetTaskByID(moved.ID) == nil {
		t.Fatal("source should still contain task after failed move")
	}
	if env.reloadProject(t, dest).GetTaskByID(moved.ID) != nil {
		t.Fatal("dest should be empty after failed move")
	}
}

// moveHandlerEnv seeds a source (with one task) and destination project on disk
// over a shared commit service + workspace ledger, wiring the app.Service the
// migrated cross-project move handler drives.
type moveHandlerEnv struct {
	dataDir string
	ledger  *storage.WorkspaceLedger
	svc     *app.Service
	mgr     *config.Manager
	cfg     *config.RuntimeState
	source  *models.Project
	dest    *models.Project
}

func newMoveHandlerEnv(t *testing.T, sourceTaskID, sourceTaskTitle string) *moveHandlerEnv {
	t.Helper()
	dataDir := t.TempDir()
	ledger := storage.NewWorkspaceLedger(filepath.Join(dataDir, ".tt", "undo"), []string{dataDir})
	commitSvc := storage.NewCommitServiceWithLedger(filepath.Join(dataDir, ".tt", "transactions"), []string{dataDir}, ledger)

	newProject := func(slug, name string, tasks []*models.Task) *models.Project {
		p, err := models.NewProject(uuid.New().String(), slug, "")
		if err != nil {
			t.Fatalf("NewProject: %v", err)
		}
		p.Name = name
		p.Slug = slug
		p.Path = filepath.Join(dataDir, "projects", slug)
		p.Tasks = tasks
		next := 1
		for _, tk := range tasks {
			if tk.ShortID >= next {
				next = tk.ShortID + 1
			}
		}
		p.NextShortID = next
		p.BuildTaskTree()
		return p
	}

	task, err := models.NewGenericTask(sourceTaskID, 1, sourceTaskTitle)
	if err != nil {
		t.Fatalf("NewTask: %v", err)
	}
	source := newProject("source", "Source", []*models.Task{task})
	dest := newProject("dest", "Dest", nil)

	repo := newScenarioRepo(commitSvc, source, dest)
	repo.seed(t)
	svc := app.NewServiceWithArtifacts(repo, nil, nil, nil)

	mgr := config.NewManager(filepath.Join(dataDir, ".tt.json"))
	cfg := newTestConfigAt(dataDir)
	setTestProjects(cfg, []*models.Project{source, dest})

	return &moveHandlerEnv{dataDir: dataDir, ledger: ledger, svc: svc, mgr: mgr, cfg: cfg, source: source, dest: dest}
}

func (e *moveHandlerEnv) appModel() *AppModel {
	return &AppModel{
		configManager: e.mgr,
		runtime:       testRuntimeAt(e.cfg.DataDir(), e.cfg),
		projects:      map[string]*models.Project{e.source.ID: e.source, e.dest.ID: e.dest},
		persister:     NewProjectService(),
		appService:    e.svc,
		views:         NewViewRegistry(),
		width:         80,
		height:        24,
	}
}

func (e *moveHandlerEnv) reloadProject(t *testing.T, p *models.Project) *models.Project {
	t.Helper()
	fresh := &models.Project{ID: p.ID, Name: p.Name, Slug: p.Slug, Path: p.Path}
	if err := storage.LoadProject(fresh); err != nil {
		t.Fatalf("reload project %s: %v", p.Slug, err)
	}
	return fresh
}

// assertMoveMsg drains the handler command, fails on a SaveErrorMsg, and returns
// the TaskMovedToProjectMsg it must emit.
func assertMoveMsg(t *testing.T, cmd tea.Cmd) TaskMovedToProjectMsg {
	t.Helper()
	msgs := drainTUITestCmd(t, cmd)
	for _, msg := range msgs {
		switch msg := msg.(type) {
		case SaveErrorMsg:
			t.Fatalf("move save failed: %v", msg.Err)
		case ErrorMsg:
			t.Fatalf("move failed: %v", msg.Err)
		case TaskMovedToProjectMsg:
			return msg
		}
	}
	t.Fatalf("expected TaskMovedToProjectMsg, got %#v", msgs)
	return TaskMovedToProjectMsg{}
}

func drainSingleMsg(t *testing.T, cmd tea.Cmd) tea.Msg {
	t.Helper()
	if cmd == nil {
		t.Fatal("expected a command")
	}
	return cmd()
}

// assertDiskConfigRef reloads the config from disk and asserts the durable
// planning refs for taskID point at projectID.
func assertDiskConfigRef(t *testing.T, mgr *config.Manager, taskID, projectID string) {
	t.Helper()
	cfg, err := config.NewManager(mgr.GetConfigPath()).LoadRuntimeState()
	if err != nil {
		t.Fatalf("load config from disk: %v", err)
	}
	if len(cfg.Planning().Next) != 1 || cfg.Planning().Next[0].TaskID != taskID || cfg.Planning().Next[0].ProjectID != projectID {
		t.Fatalf("NextOrder = %+v, want task %s in project %s", cfg.Planning().Next, taskID, projectID)
	}
	if len(cfg.Planning().Today) > 0 && (cfg.Planning().Today[0].TaskID != taskID || cfg.Planning().Today[0].ProjectID != projectID) {
		t.Fatalf("Today = %+v, want task %s in project %s", cfg.Planning().Today, taskID, projectID)
	}
}

func drainTUITestCmd(t *testing.T, cmd tea.Cmd) []tea.Msg {
	t.Helper()
	if cmd == nil {
		return nil
	}
	msg := cmd()
	if batch, ok := msg.(tea.BatchMsg); ok {
		msgs := make([]tea.Msg, 0, len(batch))
		for _, batchCmd := range batch {
			if batchCmd == nil {
				continue
			}
			msgs = append(msgs, batchCmd())
		}
		return msgs
	}
	return []tea.Msg{msg}
}
