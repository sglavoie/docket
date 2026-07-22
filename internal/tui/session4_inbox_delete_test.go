package tui

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

// inboxDeleteEnv wires a local InboxProcessorModel over the real transactional
// stack so a session-4 test reads as: press x/X -> shell confirm -> commit ->
// processor advances. The processor's project is the SAME pointer the AppModel
// owns, so the shell's post-commit reload is observed by the processor.
type inboxDeleteEnv struct {
	m         *AppModel
	processor *InboxProcessorModel
	project   *models.Project
	tasks     []*models.Task
}

func newInboxDeleteEnv(t *testing.T) *inboxDeleteEnv {
	t.Helper()
	dataDir := t.TempDir()
	ledger := storage.NewWorkspaceLedger(filepath.Join(dataDir, ".tt", "undo"), testWorkspaceLedgerDepth, []string{dataDir})
	commitSvc := storage.NewCommitServiceWithLedger(filepath.Join(dataDir, ".tt", "transactions"), []string{dataDir}, ledger)

	a := seedTask(t, 1, "Triage A", models.StateInbox)
	b := seedTask(t, 2, "Triage B", models.StateInbox)
	c := seedTask(t, 3, "Triage C", models.StateInbox)

	proj, err := models.NewProject(uuid.New().String(), "alpha", "")
	if err != nil {
		t.Fatalf("NewProject: %v", err)
	}
	proj.Slug = "alpha"
	proj.Name = "Alpha"
	proj.Path = filepath.Join(dataDir, "projects", "alpha")
	proj.Tasks = []*models.Task{a, b, c}
	proj.NextShortID = 4
	proj.EnsureDefaultSections()
	proj.BuildTaskTree()

	repo := newScenarioRepo(commitSvc, proj)
	repo.seed(t)
	svc := app.NewService(repo, nil, nil)

	mgr := config.NewManager(filepath.Join(dataDir, ".tt.json"))
	cfg := newTestConfigAt(dataDir)
	setTestProjects(cfg, []*models.Project{proj})
	cfg.Machine().CurrentProject = proj.ID
	if err := saveTestRuntime(mgr, cfg); err != nil {
		t.Fatalf("seed config: %v", err)
	}
	runtime := testRuntimeAt(cfg.DataDir(), cfg)

	m := &AppModel{
		configManager:        mgr,
		runtime:              runtime,
		projects:             map[string]*models.Project{proj.ID: proj},
		persister:            NewProjectService(),
		appService:           svc,
		secondaryFeatures:    newSecondaryCoordinators(svc, nil),
		views:                NewViewRegistry(),
		viewStack:            []PresentationID{ViewDashboard},
		width:                80,
		height:               24,
		outlinerToggleStates: make(map[string]*OutlinerToggleState),
		projectLastView:      make(map[string]PresentationID),
	}

	processor := NewInboxProcessorModel(proj, mgr, runtime)
	m.inboxProcessor = processor
	m.views.Register(ViewInboxProcessor, processor)
	m.pushView(ViewInboxProcessor)

	return &inboxDeleteEnv{m: m, processor: processor, project: proj, tasks: []*models.Task{a, b, c}}
}

func (e *inboxDeleteEnv) reload(t *testing.T) *models.Project {
	t.Helper()
	fresh := &models.Project{ID: e.project.ID, Name: e.project.Name, Slug: e.project.Slug, Path: e.project.Path}
	if err := storage.LoadProject(fresh); err != nil {
		t.Fatalf("reload: %v", err)
	}
	return fresh
}

// TestInboxDeleteEndToEndAdvancesAfterCommit is the headline session-4
// regression: pressing x/X on an inbox task routes through the canonical
// shell-owned transaction, and only after the commit does the processor mark the
// task processed and advance to the natural next item.
func TestInboxDeleteEndToEndAdvancesAfterCommit(t *testing.T) {
	e := newInboxDeleteEnv(t)
	m, proc := e.m, e.processor
	proc.currentIndex = 0
	deletedID := e.tasks[0].ID
	nextID := e.tasks[1].ID
	deletedIdentity := TaskIdentity{ProjectID: e.project.ID, TaskID: deletedID}

	// Press x: the processor emits the canonical request; nothing is mutated yet.
	reqCmd := proc.requestDelete()
	if reqCmd == nil {
		t.Fatal("requestDelete returned nil")
	}
	req, ok := reqCmd().(TaskDeleteRequestedMsg)
	if !ok {
		t.Fatalf("requestDelete emitted %T, want TaskDeleteRequestedMsg", reqCmd())
	}
	if req.Origin != ViewInboxProcessor {
		t.Errorf("origin = %v, want ViewInboxProcessor", req.Origin)
	}
	m.handleTaskDeleteRequested(req)
	if !m.taskDeleteConfirm.IsActive() {
		t.Fatal("shell delete confirmation should be armed")
	}
	if proc.processedIDs[deletedIdentity] {
		t.Fatal("request must not mark the task processed before commit")
	}

	// Confirm: the shell commits the atomic transaction.
	_, cmd := m.handleGlobalKeyPress(keyPress("y"))
	if pending, ok := singleArmedInboxDelete(m); !ok || pending.identity != deletedIdentity || pending.origin != ViewInboxProcessor {
		t.Fatalf("pendingInboxDelete not armed for the deleted task, got %+v (armed=%v)", pending, ok)
	}
	batch := runBatchResult(t, cmd)
	m.handleBatchTransactionResult(batch)

	// The task is gone from disk and the processor advanced to the next task.
	if e.reload(t).GetTaskByID(deletedID) != nil {
		t.Error("deleted task should be removed from disk")
	}
	if !proc.processedIDs[deletedIdentity] {
		t.Error("deleted task should be marked processed after commit")
	}
	if len(m.pendingInboxDeletes) != 0 {
		t.Errorf("pendingInboxDelete should be cleared after apply, got %+v", m.pendingInboxDeletes)
	}
	if got, _ := proc.currentTaskRef(); got == nil || got.ID != nextID {
		t.Errorf("cursor should advance to the next task, got %v", got)
	}
}

func TestApplyPendingInboxDeleteGuards(t *testing.T) {
	e := newInboxDeleteEnv(t)
	m, proc := e.m, e.processor
	proc.currentIndex = 0
	identity := TaskIdentity{ProjectID: e.project.ID, TaskID: e.tasks[0].ID}
	proc.pendingDeletes = map[TaskIdentity]inboxPendingDelete{
		identity: {focus: TaskIdentity{ProjectID: e.project.ID, TaskID: e.tasks[1].ID}},
	}
	armInboxToken := func(token uint64) {
		m.pendingInboxDeletes = map[uint64]pendingInboxDelete{
			token: {token: token, origin: ViewInboxProcessor, identity: identity},
		}
	}

	// No armed pending: no-op regardless of token.
	m.pendingInboxDeletes = nil
	if cmd := m.applyPendingInboxDelete(1, nil, []string{e.tasks[0].ID}); cmd != nil {
		t.Errorf("unarmed applyPendingInboxDelete should be a no-op, got %v", cmd)
	}

	// Armed with token 7. A batch carrying no token (0) or a DIFFERENT token is an
	// unrelated result: it must not consume the pending state and must LEAVE it
	// in-flight so THIS delete's matching result can still apply.
	armInboxToken(7)
	if cmd := m.applyPendingInboxDelete(0, nil, []string{e.tasks[0].ID}); cmd != nil {
		t.Errorf("token 0 (non-delete batch) should be a no-op, got %v", cmd)
	}
	if cmd := m.applyPendingInboxDelete(8, nil, []string{e.tasks[0].ID}); cmd != nil {
		t.Errorf("mismatched token should be a no-op, got %v", cmd)
	}
	if _, ok := singleArmedInboxDelete(m); !ok {
		t.Fatal("an unrelated result must leave the in-flight Inbox delete armed")
	}
	if proc.processedIDs[identity] {
		t.Error("an unrelated batch must not mark the task processed")
	}

	// The matching token but a batch that did not actually remove the pending task:
	// clears the pending state (defensive) and applies nothing.
	armInboxToken(7)
	if cmd := m.applyPendingInboxDelete(7, nil, []string{"someone-else"}); cmd != nil {
		t.Errorf("matching token but mismatched removed IDs should be a no-op, got %v", cmd)
	}
	if proc.processedIDs[identity] {
		t.Error("mismatched removed IDs must not mark the task processed")
	}
	if len(m.pendingInboxDeletes) != 0 {
		t.Error("a token-matched result should clear its own pending state")
	}

	// A modern payload with locators must never fall back to its legacy ID list.
	// The same UUID from a different project is not the pending task.
	armInboxToken(9)
	if cmd := m.applyPendingInboxDelete(9, []app.TaskLocator{{ProjectID: "other-project", TaskID: e.tasks[0].ID}}, []string{e.tasks[0].ID}); cmd != nil {
		t.Errorf("mismatched locator should not apply through the legacy ID fallback, got %v", cmd)
	}
	if proc.processedIDs[identity] {
		t.Error("mismatched project locator must not mark the pending task processed")
	}
}

func TestIsInboxOrigin(t *testing.T) {
	for _, id := range []PresentationID{ViewInboxProcessor, ViewGlobalInboxProcessor} {
		if !isInboxOrigin(id) {
			t.Errorf("isInboxOrigin(%v) = false, want true", id)
		}
	}
	for _, id := range []PresentationID{ViewOutliner, ViewTaskDetail, ViewFocusBoard, ViewDashboard} {
		if isInboxOrigin(id) {
			t.Errorf("isInboxOrigin(%v) = true, want false", id)
		}
	}
}

// TestNoUserFacingTDBindingRemains is the repository assertion required by
// session 4: the retired delete chord (lowercase "t" followed by uppercase "D")
// must be absent from every user-facing surface — keybinding help, palette
// command shortcuts, and tutor documentation. The chord is assembled from parts
// so the literal token does not reappear in the source.
func TestNoUserFacingTDBindingRemains(t *testing.T) {
	retiredChord := "t" + "D"

	for _, kb := range GetKeybindingRegistry().bindings {
		if kb.Key == retiredChord {
			t.Errorf("keybinding registry still advertises retired delete chord %q: %+v", retiredChord, kb)
		}
	}

	app, _ := newTestAppModelWithProject()
	for _, group := range app.commandGroups() {
		for _, cmd := range group {
			if cmd.Shortcut == retiredChord {
				t.Errorf("palette command %q still advertises retired shortcut %q", cmd.Name, retiredChord)
			}
		}
	}

	retiredTutorMarker := "(" + retiredChord + ")"
	for _, topic := range tutorTopics() {
		if strings.Contains(topic.Content, retiredTutorMarker) {
			t.Errorf("tutor topic %q still documents the retired %s chord", topic.Title, retiredTutorMarker)
		}
	}
}
