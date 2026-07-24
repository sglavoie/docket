package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

type noteLedgerReconcileEnv struct {
	dataDir        string
	ledger         *storage.WorkspaceLedger
	project        *models.Project
	taskID         string
	relatedTaskID  string
	attachmentPath string
	transactionID  string
	view           *NotesViewModel
	linkedNoteView *NotesViewModel
	selector       *LinkedTaskSelectorModel
	attachments    *TaskAttachmentsModel
	notesSearch    *NotesSearchModel
	shell          *AppModel
}

func newNoteLedgerReconcileEnv(t *testing.T, global bool) *noteLedgerReconcileEnv {
	t.Helper()
	dataDir := t.TempDir()
	ledger := storage.NewWorkspaceLedger(filepath.Join(dataDir, ".tt", "undo"), testWorkspaceLedgerDepth, []string{dataDir})
	commitService := storage.NewCommitServiceWithLedger(filepath.Join(dataDir, ".tt", "transactions"), []string{dataDir}, ledger)

	project, err := models.NewProject(uuid.New().String(), "ledger-reconcile", "")
	if err != nil {
		t.Fatalf("NewProject: %v", err)
	}
	project.Path = filepath.Join(dataDir, "projects", project.Slug)
	task, err := models.NewTask(uuid.New().String(), 1, models.TaskTypeTask, "Before restore")
	if err != nil {
		t.Fatalf("NewTask: %v", err)
	}
	related, err := models.NewTask(uuid.New().String(), 2, models.TaskTypeTask, "Related task")
	if err != nil {
		t.Fatalf("NewTask: %v", err)
	}
	project.Tasks = []*models.Task{task, related}
	project.NextShortID = 3
	project.EnsureDefaultSections()
	project.BuildTaskTree()

	repo := newScenarioRepo(commitService, project)
	repo.seed(t)
	seedNoteMigrationCatalog(t, dataDir, project)
	manager := config.NewManager(filepath.Join(dataDir, ".tt.json"))
	runtime, err := manager.LoadRuntimeState()
	if err != nil {
		t.Fatalf("load runtime: %v", err)
	}
	planning := clonePlanningState(runtime.Planning())
	planning.FocusQueue = []config.FocusQueueItem{{TaskID: task.ID, ProjectID: project.ID}}
	planning.DailyPlan = []config.DailyPlanItem{{TaskID: task.ID, ProjectID: project.ID, PlannedDate: "2026-07-14"}}
	if commitErr := runtime.CommitPlanning(planning); commitErr != nil {
		t.Fatalf("seed planning: %v", commitErr)
	}
	machine := cloneMachineState(runtime.Machine())
	machine.RecentTaskVisits = []config.RecentTaskVisit{{TaskID: task.ID, ProjectID: project.ID}}
	if commitErr := runtime.CommitMachine(machine); commitErr != nil {
		t.Fatalf("seed machine: %v", commitErr)
	}

	service := app.NewServiceWithArtifacts(repo, nil, nil, app.NewStorageWorkspaceNotePort(dataDir), nil)
	note := models.NewNote("Ledger reconciliation")
	note.AddLink(task.ID)
	var activeView *NotesViewModel
	if global {
		if saveErr := storage.SaveNoteTo(storage.GlobalNotesDir(dataDir), note); saveErr != nil {
			t.Fatalf("save global note: %v", saveErr)
		}
		activeView = NewGlobalNotesViewModelWithService(storage.GlobalNotesDir(dataDir), service, ledger)
	} else {
		if saveErr := storage.SaveNote(project, note); saveErr != nil {
			t.Fatalf("save project note: %v", saveErr)
		}
		activeView = NewNotesViewModelWithService(project, service, ledger)
	}
	activeView.SetSize(80, 40)
	linkedNoteView := activeView
	if global {
		linkedNoteView = NewNotesViewModelWithService(project, service, ledger)
		linkedNoteView.SetSize(80, 40)
	}
	// Notes-view construction reloads its project to populate linked-task titles.
	// Re-resolve the canonical test pointers before building the transaction.
	task = project.GetTaskByID(task.ID)
	related = project.GetTaskByID(related.ID)

	views := NewViewRegistry()
	activeType := ViewNotes
	if global {
		activeType = ViewGlobalNotes
		views.Register(ViewNotes, linkedNoteView)
	}
	views.Register(activeType, activeView)
	notesSearch := NewNotesSearchModel(project)
	views.Register(ViewNotesSearch, notesSearch)
	shell := &AppModel{
		configManager:        manager,
		runtime:              runtime,
		projects:             map[string]*models.Project{project.ID: project},
		persister:            NewProjectService(),
		appService:           service,
		views:                views,
		viewStack:            []PresentationID{activeType},
		width:                80,
		height:               40,
		currentProject:       project,
		focusQueue:           []FocusTask{{Task: task, Project: project}},
		focusQueueIDs:        map[TaskIdentity]bool{taskIdentityFor(task, project): true},
		dailyPlan:            []FocusTask{{Task: task, Project: project}},
		dailyPlanIDs:         map[TaskIdentity]bool{taskIdentityFor(task, project): true},
		recentTaskVisits:     []RecentTaskVisitEntry{{Task: task, Project: project}},
		outlinerToggleStates: make(map[string]*OutlinerToggleState),
	}

	// NotesSearchModel also loads the project while building its linked-task
	// preview cache, so resolve the canonical pointers one final time immediately
	// before mutating and committing the fixture transaction.
	task = project.GetTaskByID(task.ID)
	related = project.GetTaskByID(related.ID)
	base, err := storage.LoadProjectDocument(project)
	if err != nil {
		t.Fatalf("load project document: %v", err)
	}
	attachmentSource := filepath.Join(t.TempDir(), "evidence.txt")
	if writeErr := os.WriteFile(attachmentSource, []byte("ledger evidence"), storage.FilePermissions); writeErr != nil {
		t.Fatalf("write attachment source: %v", writeErr)
	}
	task.Title = "After restore"
	task.UpdatedAt = time.Now().Add(time.Second)
	if relationErr := task.AddRelatedTo(related.ID); relationErr != nil {
		t.Fatalf("add relation: %v", relationErr)
	}
	if attachmentErr := task.AddAttachment(filepath.Base(attachmentSource), false, attachmentSource); attachmentErr != nil {
		t.Fatalf("add attachment metadata: %v", attachmentErr)
	}
	afterPlanning := clonePlanningState(runtime.Planning())
	afterPlanning.DailyPlan[0].PlannedDate = "2026-07-15"
	planningArtifacts, err := planningDomainArtifact(manager, afterPlanning)
	if err != nil {
		t.Fatalf("planning artifact: %v", err)
	}
	results, err := commitService.CommitMulti([]storage.ChangeSet{{
		Project:          project,
		ExpectedRevision: base.Revision,
		Base:             base,
		Artifacts:        planningArtifacts,
		AttachmentOps: []storage.AttachmentOp{{
			Kind:        storage.AttachmentCopy,
			Task:        task,
			DestProject: project,
			SourcePath:  attachmentSource,
		}},
		UndoAction:  "Reconcile Notes Ledger",
		UndoContext: task.Title,
	}})
	if err != nil {
		t.Fatalf("commit reconciliation fixture: %v", err)
	}
	if refreshErr := runtime.RefreshAfterCommit(); refreshErr != nil {
		t.Fatalf("refresh runtime after fixture commit: %v", refreshErr)
	}
	shell.syncProjectCache()
	if loadErr := storage.LoadProject(project); loadErr != nil {
		t.Fatalf("reload committed project: %v", loadErr)
	}
	committedTask := project.GetTaskByID(task.ID)
	if committedTask == nil || committedTask.Title != "After restore" || !committedTask.HasRelatedTo(related.ID) || len(committedTask.Attachments) != 1 {
		t.Fatalf("committed fixture task = %#v", committedTask)
	}
	shell.focusQueue[0] = FocusTask{Task: committedTask, Project: project}
	shell.dailyPlan[0] = FocusTask{Task: committedTask, Project: project}
	shell.recentTaskVisits[0] = RecentTaskVisitEntry{Task: committedTask, Project: project}
	activeView.tasksLoaded = false
	activeView.loadNotes()
	if linkedNoteView != activeView {
		linkedNoteView.tasksLoaded = false
		linkedNoteView.loadNotes()
	}
	selector := NewLinkedTaskSelectorModelWithConfig(committedTask, project, runtime, LinkCoordinator{})
	attachments := NewTaskAttachmentsModel(committedTask, project, runtime, AttachmentCoordinator{})
	views.Register(ViewLinkedTaskSelector, selector)
	views.Register(ViewTaskAttachments, attachments)

	return &noteLedgerReconcileEnv{
		dataDir:        dataDir,
		ledger:         ledger,
		project:        project,
		taskID:         task.ID,
		relatedTaskID:  related.ID,
		attachmentPath: storage.GetAttachmentPath(project, task, filepath.Base(attachmentSource)),
		transactionID:  results[0].TransactionID,
		view:           activeView,
		linkedNoteView: linkedNoteView,
		selector:       selector,
		attachments:    attachments,
		notesSearch:    notesSearch,
		shell:          shell,
	}
}

func (e *noteLedgerReconcileEnv) restoreThroughNotes(t *testing.T, redo bool) ProjectNoteLedgerResultMsg {
	t.Helper()
	cmd := e.view.undoDelete()
	if redo {
		cmd = e.view.redoNoteMutation()
	}
	raw := cmd()
	msg, ok := raw.(ProjectNoteLedgerResultMsg)
	if !ok {
		t.Fatalf("notes ledger restore = %T, want ProjectNoteLedgerResultMsg", raw)
	}
	if msg.Entry.TransactionID != e.transactionID {
		t.Fatalf("restored transaction = %q, want %q", msg.Entry.TransactionID, e.transactionID)
	}
	if len(msg.Entry.ProjectIDs) != 1 || msg.Entry.ProjectIDs[0] != e.project.ID {
		t.Fatalf("restored project ids = %v, want [%s]", msg.Entry.ProjectIDs, e.project.ID)
	}
	if handled, _ := e.shell.handleNotesMessages(msg); !handled {
		t.Fatal("notes ledger result was not handled")
	}
	return msg
}

func (e *noteLedgerReconcileEnv) assertDiskReconciled(t *testing.T, after bool, wantTitle string) {
	t.Helper()
	diskProject := &models.Project{ID: e.project.ID, Slug: e.project.Slug, Name: e.project.Name, Path: e.project.Path}
	if err := storage.LoadProject(diskProject); err != nil {
		t.Fatalf("reload disk project: %v", err)
	}
	diskTask := diskProject.GetTaskByID(e.taskID)
	if diskTask == nil || diskTask.Title != wantTitle {
		t.Fatalf("disk task = %#v, want title %q", diskTask, wantTitle)
	}
	if got := diskTask.HasRelatedTo(e.relatedTaskID); got != after {
		t.Fatalf("disk linked-task relation = %v, want %v", got, after)
	}
	if got := len(diskTask.Attachments); got != boolInt(after) {
		t.Fatalf("disk attachment metadata count = %d, want %d", got, boolInt(after))
	}
	_, statErr := os.Stat(e.attachmentPath)
	if after && statErr != nil {
		t.Fatalf("attachment bytes missing after redo/commit: %v", statErr)
	}
	if !after && !os.IsNotExist(statErr) {
		t.Fatalf("attachment bytes remain after undo: %v", statErr)
	}
}

func (e *noteLedgerReconcileEnv) assertReconciled(t *testing.T, after bool) {
	t.Helper()
	wantTitle := "Before restore"
	wantDate := "2026-07-14"
	if after {
		wantTitle = "After restore"
		wantDate = "2026-07-15"
	}
	e.assertDiskReconciled(t, after, wantTitle)

	if got := e.shell.runtime.Planning().DailyPlan[0].PlannedDate; got != wantDate {
		t.Fatalf("runtime daily-plan date = %q, want %q", got, wantDate)
	}
	cachedTask := e.shell.projectByID(e.project.ID).GetTaskByID(e.taskID)
	if cachedTask == nil || cachedTask.Title != wantTitle {
		t.Fatalf("cached task = %#v, want title %q", cachedTask, wantTitle)
	}
	for name, ref := range map[string]*models.Task{
		"focus queue":  e.shell.focusQueue[0].Task,
		"daily plan":   e.shell.dailyPlan[0].Task,
		"recent visit": e.shell.recentTaskVisits[0].Task,
	} {
		if ref != cachedTask {
			t.Fatalf("%s pointer = %p, want reloaded task %p", name, ref, cachedTask)
		}
	}
	if e.attachments.task != cachedTask || len(e.attachments.task.Attachments) != boolInt(after) {
		t.Fatalf("attachments view task = %#v, want reloaded task with after=%v", e.attachments.task, after)
	}
	if e.selector.sourceTask != cachedTask || len(e.selector.relations) != boolInt(after) {
		t.Fatalf("linked selector source/relations = %p/%d, want %p/%d", e.selector.sourceTask, len(e.selector.relations), cachedTask, boolInt(after))
	}

	e.linkedNoteView.tasksLoaded = true
	e.linkedNoteView.cachedTasks = e.project.Tasks
	e.linkedNoteView.loadNotes()
	var linkedTask *models.Task
	for _, task := range e.linkedNoteView.cachedTasks {
		if task.ID == e.taskID {
			linkedTask = task
			break
		}
	}
	if linkedTask == nil || linkedTask.Title != wantTitle {
		t.Fatalf("note linked-task cache = %#v, want title %q", linkedTask, wantTitle)
	}
	if !e.notesSearch.stale {
		t.Fatal("notes search index was not marked stale")
	}
	if e.view.statusIsError || !strings.Contains(e.view.statusMessage, "Reconcile Notes Ledger") {
		t.Fatalf("notes status = %q error=%v", e.view.statusMessage, e.view.statusIsError)
	}
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func TestProjectNoteLedgerRestoreReconcilesTasksPlanningAttachmentsAndViews(t *testing.T) {
	env := newNoteLedgerReconcileEnv(t, false)
	env.restoreThroughNotes(t, false)
	env.assertReconciled(t, false)
	env.restoreThroughNotes(t, true)
	env.assertReconciled(t, true)
}

func TestGlobalNoteLedgerRestoreReportsNoChangeForUnrelatedWorkspaceMutation(t *testing.T) {
	env := newNoteLedgerReconcileEnv(t, true)
	raw := env.view.undoDelete()()
	failure, ok := raw.(ProjectNoteLedgerErrorMsg)
	if !ok {
		t.Fatalf("global notes undo = %T, want ProjectNoteLedgerErrorMsg", raw)
	}
	if failure.Err == nil || !strings.Contains(failure.Err.Error(), "no undo history available for artifact root") {
		t.Fatalf("global notes undo error = %v", failure.Err)
	}
	if handled, _ := env.shell.handleNotesMessages(failure); !handled {
		t.Fatal("global notes ledger error was not handled")
	}
	if !env.view.statusIsError || !strings.Contains(env.view.statusMessage, "no undo history available for artifact root") {
		t.Fatalf("global notes status = %q error=%v", env.view.statusMessage, env.view.statusIsError)
	}
	env.assertDiskReconciled(t, true, "After restore")
	entry, available, err := env.ledger.SelectNewestUndo(storage.GlobalScope())
	if err != nil || !available || entry.TransactionID != env.transactionID {
		t.Fatalf("unrelated workspace mutation was consumed: entry=%#v available=%v err=%v", entry, available, err)
	}
}

func TestProjectNoteLedgerRestoreReportsRefreshFailureWithoutUndoingRestore(t *testing.T) {
	env := newNoteLedgerReconcileEnv(t, false)
	raw := env.view.undoDelete()()
	msg, ok := raw.(ProjectNoteLedgerResultMsg)
	if !ok {
		t.Fatalf("notes ledger undo message = %T", raw)
	}
	staleFocusTask := env.shell.focusQueue[0].Task
	if err := os.WriteFile(config.PlanningStatePath(env.dataDir), []byte("{not-json"), storage.FilePermissions); err != nil {
		t.Fatalf("corrupt planning fixture: %v", err)
	}
	if err := os.WriteFile(config.PlanningStatePath(env.dataDir)+".bak", []byte("{also-not-json"), storage.FilePermissions); err != nil {
		t.Fatalf("corrupt planning backup fixture: %v", err)
	}
	if handled, _ := env.shell.handleNotesMessages(msg); !handled {
		t.Fatal("notes ledger result was not handled")
	}

	diskProject := &models.Project{ID: env.project.ID, Slug: env.project.Slug, Name: env.project.Name, Path: env.project.Path}
	if err := storage.LoadProject(diskProject); err != nil {
		t.Fatalf("reload disk project: %v", err)
	}
	if got := diskProject.GetTaskByID(env.taskID).Title; got != "Before restore" {
		t.Fatalf("completed undo was rolled back after refresh failure: title=%q", got)
	}
	if fresh := env.project.GetTaskByID(env.taskID); fresh == staleFocusTask {
		t.Fatal("reference repair ran despite the failed runtime reload")
	}
	if env.shell.focusQueue[0].Task != staleFocusTask {
		t.Fatal("failed runtime reload unexpectedly rewired focus-queue pointers")
	}
	if !env.view.statusIsError || !strings.Contains(env.view.statusMessage, "undo completed, refresh failed: reload runtime configuration") {
		t.Fatalf("refresh failure status = %q error=%v", env.view.statusMessage, env.view.statusIsError)
	}
	entry, available, err := env.ledger.SelectNewestRedo(storage.ProjectScope(env.project.ID))
	if err != nil || !available || entry.TransactionID != env.transactionID {
		t.Fatalf("completed undo missing from redo stack: entry=%#v available=%v err=%v", entry, available, err)
	}
}
