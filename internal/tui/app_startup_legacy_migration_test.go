package tui

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

type startupMigrationRepository struct {
	app.Repository
	failErr     error
	attempts    int
	lastAttempt []storage.ChangeSet
}

func (r *startupMigrationRepository) Commit(changesets ...storage.ChangeSet) ([]*storage.CommitResult, error) {
	r.attempts++
	r.lastAttempt = append([]storage.ChangeSet(nil), changesets...)
	if r.failErr != nil {
		return nil, r.failErr
	}
	return r.Repository.Commit(changesets...)
}

type startupMigrationLoadFailure struct {
	call int
	err  error
}

type startupMigrationPersister struct {
	Persister
	failures map[string]startupMigrationLoadFailure
	calls    map[string]int
}

func (p *startupMigrationPersister) Load(project *models.Project) error {
	p.calls[project.ID]++
	if failure, ok := p.failures[project.ID]; ok && p.calls[project.ID] == failure.call {
		return failure.err
	}
	return p.Persister.Load(project)
}

type startupMigrationFixture struct {
	dataDir       string
	manager       *config.Manager
	runtime       *config.RuntimeState
	legacyA       *models.Project
	legacyATasks  []*models.Task
	legacyB       *models.Project
	legacyBTask   *models.Task
	confirmed     *models.Project
	confirmedTask *models.Task
}

func newStartupMigrationFixture(t *testing.T) *startupMigrationFixture {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	dataDir := t.TempDir()
	manager := config.NewManager(config.ConfigPathForDataDir(dataDir))

	newTask := func(id string, shortID int, title string, state models.WorkflowState) *models.Task {
		t.Helper()
		task, err := models.NewTask(id, shortID, models.TaskTypeTask, title)
		if err != nil {
			t.Fatalf("NewTask(%s): %v", id, err)
		}
		task.WorkflowState = state
		return task
	}
	newProject := func(id, name string, tasks ...*models.Task) *models.Project {
		t.Helper()
		project, err := models.NewProject(id, name, filepath.Join(dataDir, "projects", id))
		if err != nil {
			t.Fatalf("NewProject(%s): %v", id, err)
		}
		project.Slug = id
		project.Tasks = tasks
		project.NextShortID = len(tasks) + 1
		if err := storage.SaveProject(project); err != nil {
			t.Fatalf("SaveProject(%s): %v", id, err)
		}
		return project
	}

	legacyA1 := newTask("legacy-a-1", 1, "Legacy A1", models.StateReady)
	legacyA2 := newTask("legacy-a-2", 2, "Legacy A2", models.StateReady)
	legacyBTask := newTask("legacy-b-1", 1, "Legacy B", models.StateReady)
	confirmedTask := newTask("confirmed-next", 1, "Confirmed Next", models.StateNext)
	legacyA := newProject("legacy-a", "Legacy A", legacyA1, legacyA2)
	legacyB := newProject("legacy-b", "Legacy B", legacyBTask)
	confirmed := newProject("confirmed", "Confirmed", confirmedTask)

	domains := config.NewDomains()
	domains.Settings.FzfEnabled = false
	domains.Settings.GlowEnabled = false
	for _, project := range []*models.Project{legacyA, legacyB, confirmed} {
		domains.Catalog.UpsertProject(project)
	}
	domains.Machine.CurrentProject = legacyA.ID
	domains.Planning.SetFocusQueue([]config.FocusQueueItem{
		{ProjectID: legacyA.ID, TaskID: legacyA1.ID},
		{ProjectID: legacyA.ID, TaskID: legacyA2.ID},
		{ProjectID: legacyB.ID, TaskID: legacyBTask.ID},
		{ProjectID: confirmed.ID, TaskID: confirmedTask.ID},
		{ProjectID: "retired-project", TaskID: "stale-task"},
	})
	if err := saveTestRuntime(manager, config.NewRuntimeState(dataDir, domains)); err != nil {
		t.Fatalf("seed runtime: %v", err)
	}
	runtime, err := config.LoadRuntimeState(dataDir)
	if err != nil {
		t.Fatalf("LoadRuntimeState: %v", err)
	}
	return &startupMigrationFixture{
		dataDir:       dataDir,
		manager:       manager,
		runtime:       runtime,
		legacyA:       legacyA,
		legacyATasks:  []*models.Task{legacyA1, legacyA2},
		legacyB:       legacyB,
		legacyBTask:   legacyBTask,
		confirmed:     confirmed,
		confirmedTask: confirmedTask,
	}
}

func (f *startupMigrationFixture) activeProjects(t *testing.T, runtime *config.RuntimeState) []*models.Project {
	t.Helper()
	projects := runtime.ActiveProjects()
	if len(projects) != 3 {
		t.Fatalf("active projects = %d, want 3", len(projects))
	}
	return projects
}

func (f *startupMigrationFixture) service() (*app.Service, *startupMigrationRepository) {
	depth := effectiveUndoHistoryDepth(f.runtime.Settings())
	ledger := storage.NewWorkspaceLedger(config.UndoLedgerDir(f.dataDir), depth, []string{f.dataDir})
	repo := &startupMigrationRepository{
		Repository: newTUIAppRepository(f.dataDir, f.manager.RuntimeDependencies(), ledger),
	}
	return app.NewService(repo, nil, nil), repo
}

func newStartupMigrationModel(runtime *config.RuntimeState, persister Persister) *AppModel {
	return &AppModel{
		runtime:       runtime,
		persister:     persister,
		focusQueue:    []FocusTask{},
		focusQueueIDs: make(map[string]bool),
	}
}

func startupMigrationFocusIDs(model *AppModel) []string {
	ids := make([]string, 0, len(model.focusQueue))
	for _, item := range model.focusQueue {
		ids = append(ids, item.Task.ID)
	}
	return ids
}

func assertStartupMigrationFileUnchanged(t *testing.T, path, before string) {
	t.Helper()
	if after := mustReadFile(t, path); after != before {
		t.Fatalf("%s changed on migration failure\nbefore:\n%s\nafter:\n%s", path, before, after)
	}
}

func assertStartupMigrationFocusIDs(t *testing.T, model *AppModel, want ...string) {
	t.Helper()
	got := startupMigrationFocusIDs(model)
	if len(got) != len(want) {
		t.Fatalf("hydrated focus IDs = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("hydrated focus IDs = %v, want %v", got, want)
		}
	}
}

func assertStartupMigrationHasNoPlanningArtifact(t *testing.T, changesets []storage.ChangeSet) {
	t.Helper()
	for _, changeset := range changesets {
		if len(changeset.Artifacts) != 0 {
			t.Fatalf("legacy migration attempted planning artifacts: %v", artifactPaths(changeset))
		}
	}
}

// TestStartupMigratesLegacyFocusQueueBeforeHydration locks FR-009-003 for the TUI
// startup path: legacy Focus Queue members (listed in the ordering-only index but
// whose workflow state predates the state-driven queue) are backfilled to
// StateNext BEFORE hydrateFocusQueue prunes non-StateNext entries. First launch
// must therefore convert every eligible member — preserving its queue order and
// persisting the change — instead of silently discarding the owner's shortlist.
func TestStartupMigratesLegacyFocusQueueBeforeHydration(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	dataDir := t.TempDir()
	manager := config.NewManager(config.ConfigPathForDataDir(dataDir))

	projectPath := filepath.Join(dataDir, "projects", "project-1")
	project, err := models.NewProject("project-1", "Project 1", projectPath)
	if err != nil {
		t.Fatalf("NewProject: %v", err)
	}
	project.Slug = "project-1"

	// Two legacy tasks: both StateReady (pre-state-driven), listed in the queue.
	task1, err := models.NewTask("task-1", 1, models.TaskTypeTask, "First")
	if err != nil {
		t.Fatalf("NewTask task-1: %v", err)
	}
	task1.WorkflowState = models.StateReady
	task2, err := models.NewTask("task-2", 2, models.TaskTypeTask, "Second")
	if err != nil {
		t.Fatalf("NewTask task-2: %v", err)
	}
	task2.WorkflowState = models.StateReady
	project.Tasks = []*models.Task{task1, task2}
	project.NextShortID = 3
	if saveErr := storage.SaveProject(project); saveErr != nil {
		t.Fatalf("SaveProject: %v", saveErr)
	}

	domains := config.NewDomains()
	domains.Settings.FzfEnabled = false
	domains.Settings.GlowEnabled = false
	domains.Catalog.UpsertProject(project)
	domains.Machine.CurrentProject = project.ID
	// Legacy queue: two members recorded in order, both not yet StateNext.
	domains.Planning.SetFocusQueue([]config.FocusQueueItem{
		{ProjectID: project.ID, TaskID: task1.ID},
		{ProjectID: project.ID, TaskID: task2.ID},
	})
	if saveErr := saveTestRuntime(manager, config.NewRuntimeState(dataDir, domains)); saveErr != nil {
		t.Fatalf("seed runtime: %v", saveErr)
	}

	runtime, err := config.LoadRuntimeState(dataDir)
	if err != nil {
		t.Fatalf("LoadRuntimeState: %v", err)
	}

	app, err := NewAppModel(runtime, manager)
	if err != nil {
		t.Fatalf("NewAppModel: %v", err)
	}

	// Both legacy members were migrated to StateNext and survived hydration prune,
	// preserving their relative queue order.
	if got := len(app.focusQueue); got != 2 {
		t.Fatalf("focus queue members = %d, want 2 (legacy members migrated, not pruned)", got)
	}
	if app.focusQueue[0].Task.ID != task1.ID || app.focusQueue[1].Task.ID != task2.ID {
		t.Fatalf("focus queue order = [%s, %s], want [task-1, task-2]",
			app.focusQueue[0].Task.ID, app.focusQueue[1].Task.ID)
	}
	for _, ft := range app.focusQueue {
		if ft.Task.State() != models.StateNext {
			t.Fatalf("migrated member %s state = %s, want next", ft.Task.ID, ft.Task.State())
		}
	}

	// The ordering-only index was not pruned: the unmigrated queue is intact.
	if q := runtime.Planning().GetFocusQueue(); len(q) != 2 {
		t.Fatalf("focus queue index = %d entries, want 2 (unmigrated queue not deleted)", len(q))
	}

	// The migration persisted to disk, so a second launch reads StateNext directly.
	reloaded := &models.Project{ID: project.ID, Name: project.Name, Slug: project.Slug, Path: projectPath}
	if err := storage.LoadProject(reloaded); err != nil {
		t.Fatalf("reload migrated project: %v", err)
	}
	for _, id := range []string{task1.ID, task2.ID} {
		task := reloaded.GetTaskByID(id)
		if task == nil {
			t.Fatalf("migrated task %s missing from disk", id)
		}
		if task.State() != models.StateNext {
			t.Fatalf("disk state for %s = %s, want next (persisted migration)", id, task.State())
		}
	}
}

// TestStartupLegacyFocusQueueMigrationUsesWorkspaceTransaction locks FR-013-003 for
// the TUI startup path: the automatic first-use legacy migration must journal its
// StateNext backfill through ONE revision-checked workspace transaction with a
// durable ledger entry (not raw per-project saves), commit every changed project
// atomically, survive a restart, and be restorable. Two projects each hold a legacy
// queue member, so the batch spans projects and a single ledger undo must revert
// both together.
func TestStartupLegacyFocusQueueMigrationUsesWorkspaceTransaction(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	dataDir := t.TempDir()
	depth := 10
	manager := config.NewManager(config.ConfigPathForDataDir(dataDir))

	newLegacyProject := func(id, name, slug string) (*models.Project, *models.Task) {
		projectPath := filepath.Join(dataDir, "projects", slug)
		project, err := models.NewProject(id, name, projectPath)
		if err != nil {
			t.Fatalf("NewProject(%s): %v", name, err)
		}
		project.Slug = slug
		task, err := models.NewTask(id+"-task", 1, models.TaskTypeTask, name+" task")
		if err != nil {
			t.Fatalf("NewTask(%s): %v", name, err)
		}
		task.WorkflowState = models.StateReady
		project.Tasks = []*models.Task{task}
		project.NextShortID = 2
		if err := storage.SaveProject(project); err != nil {
			t.Fatalf("SaveProject(%s): %v", name, err)
		}
		return project, task
	}

	projA, taskA := newLegacyProject("project-a", "Project A", "project-a")
	projB, taskB := newLegacyProject("project-b", "Project B", "project-b")

	domains := config.NewDomains()
	domains.Settings.FzfEnabled = false
	domains.Settings.GlowEnabled = false
	domains.Settings.UndoHistoryDepth = &depth
	domains.Catalog.UpsertProject(projA)
	domains.Catalog.UpsertProject(projB)
	domains.Machine.CurrentProject = projA.ID
	// Legacy queue: one member per project, both still StateReady (pre-state-driven).
	domains.Planning.SetFocusQueue([]config.FocusQueueItem{
		{ProjectID: projA.ID, TaskID: taskA.ID},
		{ProjectID: projB.ID, TaskID: taskB.ID},
	})
	if err := saveTestRuntime(manager, config.NewRuntimeState(dataDir, domains)); err != nil {
		t.Fatalf("seed runtime: %v", err)
	}

	runtime, err := config.LoadRuntimeState(dataDir)
	if err != nil {
		t.Fatalf("LoadRuntimeState: %v", err)
	}

	if _, err := NewAppModel(runtime, manager); err != nil {
		t.Fatalf("NewAppModel: %v", err)
	}

	// Both legacy members were backfilled to StateNext, and the change survives a
	// restart (a fresh disk read reports StateNext).
	assertDiskState := func(project *models.Project, taskID string, want models.WorkflowState) {
		t.Helper()
		reloaded := &models.Project{ID: project.ID, Name: project.Name, Slug: project.Slug, Path: project.Path}
		if err := storage.LoadProject(reloaded); err != nil {
			t.Fatalf("reload %s: %v", project.Name, err)
		}
		task := reloaded.GetTaskByID(taskID)
		if task == nil {
			t.Fatalf("task %s missing from %s after reload", taskID, project.Name)
		}
		if task.State() != want {
			t.Fatalf("disk state for %s in %s = %s, want %s", taskID, project.Name, task.State(), want)
		}
	}
	assertDiskState(projA, taskA.ID, models.StateNext)
	assertDiskState(projB, taskB.ID, models.StateNext)

	// The migration committed through the workspace undo ledger — not a raw save —
	// so exactly one restorable transaction is recorded.
	ledger := storage.NewWorkspaceLedger(config.UndoLedgerDir(dataDir), depth, []string{dataDir})
	if count, err := ledger.UndoCount(storage.GlobalScope()); err != nil || count != 1 {
		t.Fatalf("ledger UndoCount = %d, err=%v, want exactly 1 migration transaction", count, err)
	}

	// The transaction is restorable and atomic: one ledger undo reverts BOTH
	// projects to their pre-migration StateReady in a single step, proving every
	// changed project committed in one workspace transaction.
	if _, err := ledger.Undo(storage.GlobalScope()); err != nil {
		t.Fatalf("ledger.Undo: %v", err)
	}
	assertDiskState(projA, taskA.ID, models.StateReady)
	assertDiskState(projB, taskB.ID, models.StateReady)
}

// TestStartupLegacyFocusQueueMigrationFailurePreservesOrderIndex locks
// FR-014-003: destructive FocusQueue pruning is enabled only after every
// referenced project loaded, the atomic migration committed, and every touched
// project reloaded. Until then startup may hydrate tasks already confirmed Next,
// but it must not rewrite planning.json or discard legacy/stale-looking entries.
func TestStartupLegacyFocusQueueMigrationFailurePreservesOrderIndex(t *testing.T) {
	t.Run("referenced-project load failure prevents migration and pruning", func(t *testing.T) {
		fixture := newStartupMigrationFixture(t)
		projects := fixture.activeProjects(t, fixture.runtime)
		loadErr := errors.New("injected initial load failure")
		persister := &startupMigrationPersister{
			Persister: NewProjectService(),
			failures: map[string]startupMigrationLoadFailure{
				fixture.legacyA.ID: {call: 1, err: loadErr},
			},
			calls: make(map[string]int),
		}
		model := newStartupMigrationModel(fixture.runtime, persister)
		svc, repo := fixture.service()
		planningPath := config.PlanningStatePath(fixture.dataDir)
		planningBefore := mustReadFile(t, planningPath)
		projectBefore := map[string]string{
			fixture.legacyA.ID:   mustReadFile(t, storage.GetTasksFilePath(fixture.legacyA)),
			fixture.legacyB.ID:   mustReadFile(t, storage.GetTasksFilePath(fixture.legacyB)),
			fixture.confirmed.ID: mustReadFile(t, storage.GetTasksFilePath(fixture.confirmed)),
		}

		startupLoads := make(map[string]error)
		migrationErr := model.migrateLegacyFocusQueue(svc, projects, startupLoads)
		if !errors.Is(migrationErr, loadErr) {
			t.Fatalf("migration error = %v, want initial load failure", migrationErr)
		}
		model.hydrateFocusQueue(projects, startupLoads, false)

		if repo.attempts != 0 {
			t.Fatalf("migration commit attempts = %d, want 0 after a load failure", repo.attempts)
		}
		for _, id := range []string{fixture.legacyA.ID, fixture.legacyB.ID, fixture.confirmed.ID} {
			if persister.calls[id] != 1 {
				t.Fatalf("load calls for %s = %d, want 1 (all referenced projects loaded once)", id, persister.calls[id])
			}
		}
		assertStartupMigrationFocusIDs(t, model, fixture.confirmedTask.ID)
		assertStartupMigrationFileUnchanged(t, planningPath, planningBefore)
		for _, project := range []*models.Project{fixture.legacyA, fixture.legacyB, fixture.confirmed} {
			assertStartupMigrationFileUnchanged(t, storage.GetTasksFilePath(project), projectBefore[project.ID])
		}
	})

	t.Run("atomic transaction failure preserves bytes and succeeds on retry", func(t *testing.T) {
		fixture := newStartupMigrationFixture(t)
		projects := fixture.activeProjects(t, fixture.runtime)
		model := newStartupMigrationModel(fixture.runtime, NewProjectService())
		svc, repo := fixture.service()
		commitErr := errors.New("injected migration transaction failure")
		repo.failErr = commitErr
		planningPath := config.PlanningStatePath(fixture.dataDir)
		planningBefore := mustReadFile(t, planningPath)
		projectBefore := map[string]string{
			fixture.legacyA.ID: mustReadFile(t, storage.GetTasksFilePath(fixture.legacyA)),
			fixture.legacyB.ID: mustReadFile(t, storage.GetTasksFilePath(fixture.legacyB)),
		}
		startupLoads := make(map[string]error)

		migrationErr := model.migrateLegacyFocusQueue(svc, projects, startupLoads)
		if !errors.Is(migrationErr, commitErr) {
			t.Fatalf("migration error = %v, want transaction failure", migrationErr)
		}
		model.hydrateFocusQueue(projects, startupLoads, migrationErr == nil)

		if repo.attempts != 1 || len(repo.lastAttempt) != 2 {
			t.Fatalf("migration transaction attempts = %d with %d changesets, want one attempt with two project changesets",
				repo.attempts, len(repo.lastAttempt))
		}
		assertStartupMigrationHasNoPlanningArtifact(t, repo.lastAttempt)
		assertStartupMigrationFocusIDs(t, model, fixture.confirmedTask.ID)
		assertStartupMigrationFileUnchanged(t, planningPath, planningBefore)
		assertStartupMigrationFileUnchanged(t, storage.GetTasksFilePath(fixture.legacyA), projectBefore[fixture.legacyA.ID])
		assertStartupMigrationFileUnchanged(t, storage.GetTasksFilePath(fixture.legacyB), projectBefore[fixture.legacyB.ID])

		// A later startup gets the same untouched legacy evidence and can retry the
		// whole transaction successfully before normal stale-entry pruning resumes.
		repo.failErr = nil
		retryRuntime, err := config.LoadRuntimeState(fixture.dataDir)
		if err != nil {
			t.Fatalf("reload runtime for retry: %v", err)
		}
		retryProjects := fixture.activeProjects(t, retryRuntime)
		retryModel := newStartupMigrationModel(retryRuntime, NewProjectService())
		retryLoads := make(map[string]error)
		if err := retryModel.migrateLegacyFocusQueue(svc, retryProjects, retryLoads); err != nil {
			t.Fatalf("retry migration: %v", err)
		}
		retryModel.hydrateFocusQueue(retryProjects, retryLoads, true)
		assertStartupMigrationFocusIDs(t, retryModel,
			fixture.legacyATasks[0].ID, fixture.legacyATasks[1].ID, fixture.legacyBTask.ID, fixture.confirmedTask.ID)
		for _, task := range fixture.legacyATasks {
			if state := diskTaskState(t, fixture.legacyA, task.ID); state != models.StateNext {
				t.Fatalf("retried disk state for %s = %s, want next", task.ID, state)
			}
		}
		if state := diskTaskState(t, fixture.legacyB, fixture.legacyBTask.ID); state != models.StateNext {
			t.Fatalf("retried disk state for %s = %s, want next", fixture.legacyBTask.ID, state)
		}
		if got := diskFocusQueueLen(t, fixture.dataDir); got != 4 {
			t.Fatalf("focus queue after confirmed retry = %d entries, want 4 (stale entry pruned)", got)
		}
	})

	t.Run("post-commit reload failures preserve index and fresh launch hydrates disk state", func(t *testing.T) {
		fixture := newStartupMigrationFixture(t)
		projects := fixture.activeProjects(t, fixture.runtime)
		reloadAErr := errors.New("injected Legacy A reload failure")
		reloadBErr := errors.New("injected Legacy B reload failure")
		persister := &startupMigrationPersister{
			Persister: NewProjectService(),
			failures: map[string]startupMigrationLoadFailure{
				fixture.legacyA.ID: {call: 2, err: reloadAErr},
				fixture.legacyB.ID: {call: 2, err: reloadBErr},
			},
			calls: make(map[string]int),
		}
		model := newStartupMigrationModel(fixture.runtime, persister)
		svc, repo := fixture.service()
		planningPath := config.PlanningStatePath(fixture.dataDir)
		planningBefore := mustReadFile(t, planningPath)
		startupLoads := make(map[string]error)

		migrationErr := model.migrateLegacyFocusQueue(svc, projects, startupLoads)
		if !errors.Is(migrationErr, reloadAErr) || !errors.Is(migrationErr, reloadBErr) {
			t.Fatalf("migration error = %v, want both post-commit reload failures", migrationErr)
		}
		model.hydrateFocusQueue(projects, startupLoads, migrationErr == nil)

		if repo.attempts != 1 || len(repo.lastAttempt) != 2 {
			t.Fatalf("migration transaction attempts = %d with %d changesets, want one committed two-project transaction",
				repo.attempts, len(repo.lastAttempt))
		}
		assertStartupMigrationHasNoPlanningArtifact(t, repo.lastAttempt)
		for _, id := range []string{fixture.legacyA.ID, fixture.legacyB.ID} {
			if persister.calls[id] != 2 {
				t.Fatalf("load calls for touched project %s = %d, want initial load plus attempted reload", id, persister.calls[id])
			}
		}
		assertStartupMigrationFocusIDs(t, model, fixture.confirmedTask.ID)
		assertStartupMigrationFileUnchanged(t, planningPath, planningBefore)
		for _, task := range fixture.legacyATasks {
			if state := diskTaskState(t, fixture.legacyA, task.ID); state != models.StateNext {
				t.Fatalf("committed disk state for %s = %s, want next", task.ID, state)
			}
		}
		if state := diskTaskState(t, fixture.legacyB, fixture.legacyBTask.ID); state != models.StateNext {
			t.Fatalf("committed disk state for %s = %s, want next", fixture.legacyBTask.ID, state)
		}

		freshRuntime, err := config.LoadRuntimeState(fixture.dataDir)
		if err != nil {
			t.Fatalf("fresh runtime after reload failure: %v", err)
		}
		freshProjects := fixture.activeProjects(t, freshRuntime)
		freshModel := newStartupMigrationModel(freshRuntime, NewProjectService())
		freshService, freshRepo := fixture.service()
		freshLoads := make(map[string]error)
		if err := freshModel.migrateLegacyFocusQueue(freshService, freshProjects, freshLoads); err != nil {
			t.Fatalf("fresh launch migration confirmation: %v", err)
		}
		if freshRepo.attempts != 0 {
			t.Fatalf("fresh launch migration commits = %d, want 0 for already-migrated disk state", freshRepo.attempts)
		}
		freshModel.hydrateFocusQueue(freshProjects, freshLoads, true)
		assertStartupMigrationFocusIDs(t, freshModel,
			fixture.legacyATasks[0].ID, fixture.legacyATasks[1].ID, fixture.legacyBTask.ID, fixture.confirmedTask.ID)
		if got := diskFocusQueueLen(t, fixture.dataDir); got != 4 {
			t.Fatalf("fresh-launch focus queue = %d entries, want 4 after confirmed stale prune", got)
		}
	})
}
