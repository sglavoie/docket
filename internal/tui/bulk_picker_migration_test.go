package tui

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

// This file covers the shared bulk picker/palette chokepoints migrated onto the
// BulkCoordinator/finalizeBatchResult path (session 33.7.3.3.2): the multiselect
// snooze and due-date pickers (handler_task_dates.go), the priority palette
// picker (handler_section.go PriorityPickerDoneMsg bulk branch), and the shared
// tag palette (app_handlers.go applyTagsToTaskRefs). Each migrated path commits
// ONE asynchronous cross-project transaction; the shell owns reload and
// selection-clear via the batch result. These tests drive the real transactional
// storage stack and the workspace undo ledger — the mutation→reload→undo cycle
// the migrated list/Kanban views already honor — instead of the retired
// synchronous raw-save / per-project .undo/meta.json behavior.

// appBulkPickerEnv seeds one or more projects on disk over a shared commit
// service + workspace ledger and wires the secondary coordinators an AppModel
// hands to the shared picker/palette handlers.
type appBulkPickerEnv struct {
	dataDir  string
	ledger   *storage.WorkspaceLedger
	svc      *app.Service
	features SecondaryCoordinators
	mgr      *config.Manager
	cfg      *config.RuntimeState
	projects []*models.Project
}

func newAppBulkPickerEnv(t *testing.T, taskGroups ...[]*models.Task) *appBulkPickerEnv {
	t.Helper()
	dataDir := t.TempDir()
	// Machine-local domain (machine.json) resolves under HOME; isolate it so a
	// checked machine commit + facade reload stays inside the temp workspace.
	t.Setenv("HOME", dataDir)
	ledger := storage.NewWorkspaceLedger(filepath.Join(dataDir, ".tt", "undo"), testWorkspaceLedgerDepth, []string{dataDir})
	commitSvc := storage.NewCommitServiceWithLedger(filepath.Join(dataDir, ".tt", "transactions"), []string{dataDir}, ledger)

	projects := make([]*models.Project, len(taskGroups))
	for i, tasks := range taskGroups {
		slug := fmt.Sprintf("bp%d", i)
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
		project.EnsureDefaultSections()
		project.BuildTaskTree()
		projects[i] = project
	}

	repo := newScenarioRepo(commitSvc, projects...)
	repo.seed(t)
	svc := app.NewService(repo, nil, nil)

	mgr := config.NewManager(filepath.Join(dataDir, ".tt.json"))
	cfg := newTestConfigAt(dataDir)
	setTestProjects(cfg, projects)
	// Persist the config so the domain files (catalog included) exist on disk:
	// a machine-domain commit in a migrated handler refreshes the facade via
	// Manager.Load, which must recompose the seeded projects rather than an
	// empty catalog.
	if err := saveTestRuntime(mgr, cfg); err != nil {
		t.Fatalf("seed config save: %v", err)
	}
	return &appBulkPickerEnv{
		dataDir:  dataDir,
		ledger:   ledger,
		svc:      svc,
		features: newSecondaryCoordinators(svc, mgr),
		mgr:      mgr,
		cfg:      cfg,
		projects: projects,
	}
}

// appModel builds an AppModel wired to the seeded projects' coordinators, ready
// to receive a shared picker/palette message.
func (e *appBulkPickerEnv) appModel() *AppModel {
	projects := make(map[string]*models.Project, len(e.projects))
	for _, project := range e.projects {
		projects[project.ID] = project
	}
	return &AppModel{
		configManager:        e.mgr,
		runtime:              testRuntimeAt(e.cfg.DataDir(), e.cfg),
		projects:             projects,
		persister:            NewProjectService(),
		appService:           e.svc,
		secondaryFeatures:    e.features,
		views:                NewViewRegistry(),
		viewStack:            []PresentationID{ViewDashboard},
		width:                80,
		height:               24,
		outlinerToggleStates: make(map[string]*OutlinerToggleState),
		projectLastView:      make(map[string]PresentationID),
	}
}

// refs builds the picker task refs for the tasks of project idx.
func (e *appBulkPickerEnv) refs(idx int, tasks ...*models.Task) []TaskProjectRef {
	refs := make([]TaskProjectRef, 0, len(tasks))
	for _, task := range tasks {
		refs = append(refs, TaskProjectRef{Task: task, Project: e.projects[idx]})
	}
	return refs
}

// reload reads project idx fresh from disk so an assertion sees committed bytes.
func (e *appBulkPickerEnv) reload(t *testing.T, idx int) *models.Project {
	t.Helper()
	p := e.projects[idx]
	fresh := &models.Project{ID: p.ID, Name: p.Name, Slug: p.Slug, Path: p.Path}
	if err := storage.LoadProject(fresh); err != nil {
		t.Fatalf("reload project %d: %v", idx, err)
	}
	return fresh
}

func (e *appBulkPickerEnv) undo(t *testing.T) {
	t.Helper()
	if _, err := e.ledger.Undo(storage.GlobalScope()); err != nil {
		t.Fatalf("ledger undo: %v", err)
	}
}

func (e *appBulkPickerEnv) redo(t *testing.T) {
	t.Helper()
	if _, err := e.ledger.Redo(storage.GlobalScope()); err != nil {
		t.Fatalf("ledger redo: %v", err)
	}
}

// TestHandlePriorityPickerDoneBulkMigratedCommitsAsync verifies the priority
// palette picker (PaletteBulkPriorityMsg → ViewPriorityPicker → PriorityPickerDoneMsg
// with pending bulk tasks) now commits one asynchronous cross-project transaction
// through the BulkCoordinator: the batch result carries the selection and per-view
// summary, both projects' priorities are committed on disk, the shell clears the
// cached Outliner multi-select on commit, and the workspace ledger restores the
// prior priorities. A non-zero baseline avoids the omitted-empty-field undo gap.
func TestHandlePriorityPickerDoneBulkMigratedCommitsAsync(t *testing.T) {
	taskA := newBulkPickerTestTask(t, 1, "alpha priority")
	taskB := newBulkPickerTestTask(t, 1, "beta priority")
	taskA.Priority = 5
	taskB.Priority = 5
	env := newAppBulkPickerEnv(t, []*models.Task{taskA}, []*models.Task{taskB})
	app := env.appModel()

	app.crossOutliner = NewOutlinerModel(nil, app.configManager, app.runtime)
	app.crossOutliner.loadedProjects = env.projects
	app.crossOutliner.multiSelectMode = true
	app.crossOutliner.SelectTaskIdentity(taskIdentityFor(taskA, env.projects[0]))
	app.crossOutliner.SelectTaskIdentity(taskIdentityFor(taskB, env.projects[1]))
	app.views.Register(ViewCrossProjectOutliner, app.crossOutliner)

	app.pendingBulkTasks = append(env.refs(0, taskA), env.refs(1, taskB)...)
	app.pushView(ViewPriorityPicker)

	handled, cmd := app.handleSectionMessages(PriorityPickerDoneMsg{Priority: 3})
	if !handled {
		t.Fatal("PriorityPickerDoneMsg should be handled")
	}
	if app.pendingBulkTasks != nil {
		t.Fatal("pending bulk tasks should be cleared once the batch is issued")
	}

	msg := runBatchResult(t, cmd)
	if msg.Summary != "Set priority 3 for 2 tasks" {
		t.Errorf("summary = %q, want %q", msg.Summary, "Set priority 3 for 2 tasks")
	}
	if len(msg.ClearSelectionTaskIDs) != 2 {
		t.Errorf("clear selection ids = %v, want 2", msg.ClearSelectionTaskIDs)
	}

	if got := env.reload(t, 0).GetTaskByID(taskA.ID); got == nil || got.Priority != 3 {
		t.Fatalf("projectA task priority = %v, want 3", got)
	}
	if got := env.reload(t, 1).GetTaskByID(taskB.ID); got == nil || got.Priority != 3 {
		t.Fatalf("projectB task priority = %v, want 3", got)
	}

	app.handleBatchTransactionResult(msg)
	if app.crossOutliner.multiSelectMode || app.crossOutliner.SelectedCount() != 0 {
		t.Fatal("committed batch should clear cached Outliner multi-select")
	}

	env.undo(t)
	if got := env.reload(t, 0).GetTaskByID(taskA.ID); got == nil || got.Priority != 5 {
		t.Fatalf("undo projectA priority = %v, want 5", got)
	}
	if got := env.reload(t, 1).GetTaskByID(taskB.ID); got == nil || got.Priority != 5 {
		t.Fatalf("undo projectB priority = %v, want 5", got)
	}
}

// TestHandleDueDateChangedSingleTaskMigratedCommitsAsync verifies the single-task
// due-date overlay (SetDueDateMsg with a single Task → ViewDateInput →
// DueDateChangedMsg with an explicit value and no pending bulk selection) now
// commits one asynchronous transaction through the one-task BulkCoordinator
// wrapper: the due date (with the timed flag) is committed on disk, and the
// workspace ledger restores the prior state.
func TestHandleDueDateChangedSingleTaskMigratedCommitsAsync(t *testing.T) {
	task := newBulkPickerTestTask(t, 1, "single due")
	initial := time.Date(2026, 1, 1, 0, 0, 0, 0, time.Local)
	task.DueDate = &initial
	env := newAppBulkPickerEnv(t, []*models.Task{task})
	app := env.appModel()

	app.pushView(ViewDateInput)

	dueDate := time.Date(2026, 8, 3, 9, 15, 0, 0, time.Local)
	handled, cmd := app.handleDueDateChanged(DueDateChangedMsg{
		Task:             task,
		Project:          env.projects[0],
		DueDate:          &dueDate,
		HasTime:          true,
		HasExplicitValue: true,
	})
	if !handled {
		t.Fatal("DueDateChangedMsg should be handled")
	}

	msg := runBatchResult(t, cmd)
	if msg.Summary != "Set due date for #1" {
		t.Errorf("summary = %q, want %q", msg.Summary, "Set due date for #1")
	}

	assertTaskDueDate(t, env.reload(t, 0).GetTaskByID(task.ID), dueDate, true)

	env.undo(t)
	assertTaskDueDate(t, env.reload(t, 0).GetTaskByID(task.ID), initial, false)
}

// TestHandleDueDateChangedSingleTaskClearMigratedCommitsAsync verifies the
// single-task overlay's clear ("none") path commits through the one-task
// BulkCoordinator wrapper and the ledger restores the prior due date.
func TestHandleDueDateChangedSingleTaskClearMigratedCommitsAsync(t *testing.T) {
	task := newBulkPickerTestTask(t, 1, "single due clear")
	initial := time.Date(2026, 3, 4, 0, 0, 0, 0, time.Local)
	task.DueDate = &initial
	env := newAppBulkPickerEnv(t, []*models.Task{task})
	app := env.appModel()

	app.pushView(ViewDateInput)

	handled, cmd := app.handleDueDateChanged(DueDateChangedMsg{
		Task:             task,
		Project:          env.projects[0],
		DueDate:          nil,
		HasExplicitValue: true,
	})
	if !handled {
		t.Fatal("DueDateChangedMsg should be handled")
	}

	msg := runBatchResult(t, cmd)
	if msg.Summary != "Cleared due date for #1" {
		t.Errorf("summary = %q, want %q", msg.Summary, "Cleared due date for #1")
	}

	if got := env.reload(t, 0).GetTaskByID(task.ID); got == nil || got.DueDate != nil {
		t.Fatalf("due date should be cleared on disk, got %v", got.DueDate)
	}

	env.undo(t)
	assertTaskDueDate(t, env.reload(t, 0).GetTaskByID(task.ID), initial, false)
}

// TestHandleSnoozeDateChangedSingleTaskMigratedCommitsAsync verifies the
// single-task snooze overlay now commits one asynchronous transaction through the
// one-task BulkCoordinator wrapper: the snooze is committed on disk, and the
// workspace ledger restores the prior snooze.
func TestHandleSnoozeDateChangedSingleTaskMigratedCommitsAsync(t *testing.T) {
	task := newBulkPickerTestTask(t, 1, "single snooze")
	initial := time.Date(2026, 2, 1, 0, 0, 0, 0, time.Local)
	task.SnoozedUntil = &initial
	env := newAppBulkPickerEnv(t, []*models.Task{task})
	app := env.appModel()

	app.pushView(ViewSnoozeInput)

	snooze := time.Date(2026, 9, 9, 0, 0, 0, 0, time.Local)
	handled, cmd := app.handleSnoozeDateChanged(SnoozeDateChangedMsg{
		Task:             task,
		Project:          env.projects[0],
		SnoozedUntil:     &snooze,
		HasExplicitValue: true,
	})
	if !handled {
		t.Fatal("SnoozeDateChangedMsg should be handled")
	}

	msg := runBatchResult(t, cmd)
	if msg.Summary != "Snoozed #1 until 2026-09-09" {
		t.Errorf("summary = %q, want %q", msg.Summary, "Snoozed #1 until 2026-09-09")
	}

	assertTaskSnoozedUntil(t, env.reload(t, 0).GetTaskByID(task.ID), snooze)

	env.undo(t)
	assertTaskSnoozedUntil(t, env.reload(t, 0).GetTaskByID(task.ID), initial)
}

// TestApplyTagsToTaskSingleTaskMigratedCommitsAsync verifies the single-task tag
// selector path (app_handlers.go applyTagsToTask) now commits one asynchronous
// transaction through the one-task BulkCoordinator wrapper, mirroring the
// multi-task applyTagsToTaskRefs sibling: the tags are committed on disk, and the
// workspace ledger restores the prior tag set.
func TestApplyTagsToTaskSingleTaskMigratedCommitsAsync(t *testing.T) {
	task := newBulkPickerTestTask(t, 1, "single tag", "old")
	env := newAppBulkPickerEnv(t, []*models.Task{task})
	app := env.appModel()

	cmd := app.applyTagsToTask(task, env.projects[0], []string{"alpha", "beta"})
	msg := runBatchResult(t, cmd)
	if msg.Summary != "Tagged #1" {
		t.Errorf("summary = %q, want %q", msg.Summary, "Tagged #1")
	}

	got := env.reload(t, 0).GetTaskByID(task.ID)
	if got == nil {
		t.Fatal("task not found after tag commit")
	}
	if len(got.Tags) != 2 || got.Tags[0] != "alpha" || got.Tags[1] != "beta" {
		t.Fatalf("tags = %v, want [alpha beta]", got.Tags)
	}

	env.undo(t)
	restored := env.reload(t, 0).GetTaskByID(task.ID)
	if restored == nil || len(restored.Tags) != 1 || restored.Tags[0] != "old" {
		t.Fatalf("undo tags = %v, want [old]", restored.Tags)
	}
}
