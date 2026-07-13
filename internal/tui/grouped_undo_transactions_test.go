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

	env.cfg.FocusQueue = []config.FocusQueueItem{{TaskID: moved.ID, ProjectID: source.ID}}
	env.cfg.DailyPlan = []config.DailyPlanItem{{TaskID: moved.ID, ProjectID: source.ID, PlannedDate: "2026-06-18"}}
	env.cfg.RecentTaskVisits = []config.RecentTaskVisit{{TaskID: moved.ID, ProjectID: source.ID}}
	if err := env.mgr.Save(env.cfg); err != nil {
		t.Fatalf("save config: %v", err)
	}

	app := env.appModel()
	app.focusQueue = []FocusTask{{Task: moved, Project: source}}
	app.focusQueueIDs = map[string]bool{moved.ID: true}
	app.dailyPlan = []FocusTask{{Task: moved, Project: source}}
	app.dailyPlanIDs = map[string]bool{moved.ID: true}
	app.recentTaskVisits = []RecentTaskVisitEntry{{Task: moved, Project: source}}

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
	// Durable + recent config refs point to dest (recent updated post-commit).
	assertDiskConfigRef(t, env.mgr, moved.ID, dest.ID, true)
	if app.focusQueue[0].Project.ID != dest.ID || app.dailyPlan[0].Project.ID != dest.ID || app.recentTaskVisits[0].Project.ID != dest.ID {
		t.Fatal("in-memory refs should point to dest after move")
	}

	// Ledger undo restores the move and the folded durable config refs (Focus
	// Queue, Daily Plan). Recent visits live in the machine-local machine.json,
	// which is excluded from the workspace transaction, so undo does NOT move them
	// back to source — assert on the durable refs only (checkRecent=false).
	if _, err := env.ledger.Undo(storage.GlobalScope()); err != nil {
		t.Fatalf("ledger undo: %v", err)
	}
	if env.reloadProject(t, source).GetTaskByID(moved.ID) == nil {
		t.Fatal("source should contain task again after undo")
	}
	if env.reloadProject(t, dest).GetTaskByID(moved.ID) != nil {
		t.Fatal("dest should not contain task after undo")
	}
	assertDiskConfigRef(t, env.mgr, moved.ID, source.ID, false)

	// Ledger redo re-applies the move and durable refs (recent visits, being
	// non-transactional, are not restored by redo — asserted on durable refs only).
	if _, err := env.ledger.Redo(storage.GlobalScope()); err != nil {
		t.Fatalf("ledger redo: %v", err)
	}
	if env.reloadProject(t, dest).GetTaskByID(moved.ID) == nil {
		t.Fatal("dest should contain task after redo")
	}
	assertDiskConfigRef(t, env.mgr, moved.ID, dest.ID, false)
}

// TestTUITaskMoveAppliesDestSectionAndCanonicalTask verifies the migrated move
// resolves and moves the CANONICAL source task (not a stale view copy passed in
// the message) and applies the chosen destination section as part of the move
// transaction.
func TestTUITaskMoveAppliesDestSectionAndCanonicalTask(t *testing.T) {
	env := newMoveHandlerEnv(t, "move-task-1", "Move me")
	source, dest := env.source, env.dest
	moved := source.GetTaskByID("move-task-1")
	oldCanonicalShortID := moved.ShortID

	app := env.appModel()
	staleTask, err := models.NewTask(moved.ID, 99, models.TaskTypeTask, "Stale view copy")
	if err != nil {
		t.Fatalf("new stale task: %v", err)
	}
	destSection := models.SectionWaiting

	handled, cmd := app.handleMoveTaskToProject(MoveTaskToProjectMsg{
		Task:          staleTask,
		SourceProject: source,
		DestProjectID: dest.ID,
		DestSection:   &destSection,
	})
	if !handled {
		t.Fatal("move message was not handled")
	}
	movedMsg := assertMoveMsg(t, cmd)
	if movedMsg.OldShortID != oldCanonicalShortID {
		t.Fatalf("OldShortID = %d, want canonical short ID %d", movedMsg.OldShortID, oldCanonicalShortID)
	}
	if models.SectionForState(staleTask.State()) != "" {
		t.Fatalf("stale message task must not be mutated, got section %q", models.SectionForState(staleTask.State()))
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
	if models.SectionForState(destTask.State()) != destSection {
		t.Fatalf("destination task section = %q, want %q", models.SectionForState(destTask.State()), destSection)
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
	env.cfg.FocusQueue = []config.FocusQueueItem{{TaskID: moved.ID, ProjectID: source.ID}}

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
	cfg     *config.Config
	source  *models.Project
	dest    *models.Project
}

func newMoveHandlerEnv(t *testing.T, sourceTaskID, sourceTaskTitle string) *moveHandlerEnv {
	t.Helper()
	dataDir := t.TempDir()
	ledger := storage.NewWorkspaceLedger(filepath.Join(dataDir, ".tt", "undo"), 0, []string{dataDir})
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
		p.EnsureDefaultSections()
		p.BuildTaskTree()
		return p
	}

	task, err := models.NewTask(sourceTaskID, 1, models.TaskTypeTask, sourceTaskTitle)
	if err != nil {
		t.Fatalf("NewTask: %v", err)
	}
	source := newProject("source", "Source", []*models.Task{task})
	dest := newProject("dest", "Dest", nil)

	repo := newScenarioRepo(commitSvc, source, dest)
	repo.seed(t)
	svc := app.NewService(repo, nil, nil)

	mgr := config.NewManager(filepath.Join(dataDir, ".tt.json"))
	cfg := config.NewConfig()
	depth := 50
	cfg.UndoHistoryDepth = &depth
	cfg.Projects = []*models.Project{source, dest}
	cfg.CurrentProject = source.ID

	return &moveHandlerEnv{dataDir: dataDir, ledger: ledger, svc: svc, mgr: mgr, cfg: cfg, source: source, dest: dest}
}

func (e *moveHandlerEnv) appModel() *AppModel {
	return &AppModel{
		config:                   e.cfg,
		configManager:            e.mgr,
		persister:                NewProjectService(),
		appService:               e.svc,
		views:                    NewViewRegistry(),
		viewStack:                []ViewType{ViewDashboard},
		width:                    80,
		height:                   24,
		outlinerToggleStates:     make(map[string]*OutlinerToggleState),
		projectLastView:          make(map[string]ViewType),
		navHistory:               NewNavHistory(),
		projectTreeSpatialStates: make(map[string]*ProjectTreeSpatialState),
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
// planning refs (and optionally recent visits) for taskID point at projectID.
func assertDiskConfigRef(t *testing.T, mgr *config.Manager, taskID, projectID string, checkRecent bool) {
	t.Helper()
	cfg, err := config.NewManager(mgr.GetConfigPath()).Load()
	if err != nil {
		t.Fatalf("load config from disk: %v", err)
	}
	if len(cfg.FocusQueue) != 1 || cfg.FocusQueue[0].TaskID != taskID || cfg.FocusQueue[0].ProjectID != projectID {
		t.Fatalf("FocusQueue = %+v, want task %s in project %s", cfg.FocusQueue, taskID, projectID)
	}
	if len(cfg.DailyPlan) > 0 && (cfg.DailyPlan[0].TaskID != taskID || cfg.DailyPlan[0].ProjectID != projectID) {
		t.Fatalf("DailyPlan = %+v, want task %s in project %s", cfg.DailyPlan, taskID, projectID)
	}
	if checkRecent && len(cfg.RecentTaskVisits) > 0 && (cfg.RecentTaskVisits[0].TaskID != taskID || cfg.RecentTaskVisits[0].ProjectID != projectID) {
		t.Fatalf("RecentTaskVisits = %+v, want task %s in project %s", cfg.RecentTaskVisits, taskID, projectID)
	}
}

func TestGlobalMoveSectionSelectedCarriesDestSectionWithoutMutatingTask(t *testing.T) {
	source := &models.Project{ID: "source", Name: "Source"}
	dest := &models.Project{ID: "dest", Name: "Dest"}
	task, err := models.NewTask("move-task-1", 4, models.TaskTypeTask, "Move me")
	if err != nil {
		t.Fatalf("new task: %v", err)
	}

	app := &AppModel{}
	app.globalMoveState.sourceProject = source
	app.globalMoveState.sourceTask = task
	app.globalMoveState.destProject = dest

	handled, cmd := app.handleGlobalMoveSectionSelected(GlobalMoveSectionSelectedMsg{
		Section: models.SectionWaiting,
	})
	if !handled {
		t.Fatal("section selection was not handled")
	}
	if cmd == nil {
		t.Fatal("expected move command")
	}
	if models.SectionForState(task.State()) != "" {
		t.Fatalf("source task should not be mutated before move message, got section %q", models.SectionForState(task.State()))
	}

	rawMsg := cmd()
	msg, ok := rawMsg.(MoveTaskToProjectMsg)
	if !ok {
		t.Fatalf("expected MoveTaskToProjectMsg, got %T", rawMsg)
	}
	if msg.Task != task || msg.SourceProject != source || msg.DestProjectID != dest.ID || !msg.FromGlobal {
		t.Fatalf("unexpected move message: %#v", msg)
	}
	if msg.DestSection == nil || *msg.DestSection != models.SectionWaiting {
		t.Fatalf("DestSection = %v, want %q", msg.DestSection, models.SectionWaiting)
	}
	if app.globalMoveState.sourceProject != nil || app.globalMoveState.sourceTask != nil || app.globalMoveState.destProject != nil {
		t.Fatal("global move state should be cleared")
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
