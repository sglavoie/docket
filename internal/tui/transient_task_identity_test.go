package tui

import (
	"errors"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

func TestPaletteDuplicateTaskIDsSelectAndBulkIndependently(t *testing.T) {
	sharedID := "shared-task"
	alpha := &models.Project{ID: "alpha", Name: "Alpha"}
	beta := &models.Project{ID: "beta", Name: "Beta"}
	alphaTask := &models.Task{ID: sharedID, Title: "Alpha copy", WorkflowState: models.StateReady}
	betaTask := &models.Task{ID: sharedID, Title: "Beta copy", WorkflowState: models.StateReady}
	palette := newScoringPalette([]PaletteItem{
		{Name: alphaTask.Title, Category: ItemCategoryTasks, Task: alphaTask, Project: alpha},
		{Name: betaTask.Title, Category: ItemCategoryTasks, Task: betaTask, Project: beta},
	})
	palette.showAll = true
	palette.filterItems()

	indexByProject := make(map[string]int)
	for i, filtered := range palette.filteredItems {
		if filtered.Item.Project != nil {
			indexByProject[filtered.Item.Project.ID] = i
		}
	}
	alphaIdentity := taskIdentityFor(alphaTask, alpha)
	betaIdentity := taskIdentityFor(betaTask, beta)
	for _, identity := range []TaskIdentity{alphaIdentity, betaIdentity} {
		palette.cursor = indexByProject[identity.ProjectID]
		palette.updateSearchMode(tea.KeyPressMsg{Code: tea.KeySpace}) //nolint:errcheck
	}
	if len(palette.selectedTasks) != 2 || !palette.selectedTasks[alphaIdentity] || !palette.selectedTasks[betaIdentity] {
		t.Fatalf("duplicate palette selections = %+v, want both qualified identities", palette.selectedTasks)
	}

	actions := palette.getBulkActionsForSelectedTasks()
	var bulk CommandPaletteBulkActionMsg
	for _, action := range actions {
		if msg, ok := action.Handler().(CommandPaletteBulkActionMsg); ok {
			bulk = msg
			break
		}
	}
	if len(bulk.Tasks) != 2 {
		t.Fatalf("bulk targets = %+v, want both duplicate tasks", bulk.Tasks)
	}
	got := make(map[TaskIdentity]bool)
	for _, ref := range bulk.Tasks {
		got[ref.Identity()] = true
	}
	if !got[alphaIdentity] || !got[betaIdentity] {
		t.Fatalf("bulk targets lost project ownership: %+v", got)
	}

	palette.cursor = indexByProject[alpha.ID]
	palette.updateSearchMode(tea.KeyPressMsg{Code: tea.KeySpace}) //nolint:errcheck
	if len(palette.selectedTasks) != 1 || palette.selectedTasks[alphaIdentity] || !palette.selectedTasks[betaIdentity] {
		t.Fatalf("deselecting alpha changed the wrong duplicate: %+v", palette.selectedTasks)
	}
}

func TestRecentTaskVisitsKeepDuplicateIDsAndPinStateIndependent(t *testing.T) {
	app := newTestAppForQuickSwitch()
	sharedID := "shared-task"
	alpha := &models.Project{ID: "alpha", Name: "Alpha"}
	beta := &models.Project{ID: "beta", Name: "Beta"}
	alphaTask := &models.Task{ID: sharedID, Title: "Alpha copy"}
	betaTask := &models.Task{ID: sharedID, Title: "Beta copy"}

	app.recordTaskVisit(alphaTask, alpha)
	app.recentTaskVisits[0].Pinned = true
	app.recordTaskVisit(betaTask, beta)
	if len(app.recentTaskVisits) != 2 {
		t.Fatalf("duplicate UUID visits collapsed: %+v", app.recentTaskVisits)
	}
	if app.recentTaskVisits[0].Identity() != taskIdentityFor(betaTask, beta) || app.recentTaskVisits[0].Pinned {
		t.Fatalf("beta visit inherited alpha pin state: %+v", app.recentTaskVisits[0])
	}
	if app.recentTaskVisits[1].Identity() != taskIdentityFor(alphaTask, alpha) || !app.recentTaskVisits[1].Pinned {
		t.Fatalf("alpha visit or pin state was lost: %+v", app.recentTaskVisits[1])
	}

	visits := app.runtime.Machine().GetRecentTaskVisits()
	if len(visits) != 2 || visits[0].ProjectID != beta.ID || visits[1].ProjectID != alpha.ID {
		t.Fatalf("persisted duplicate visits = %+v, want beta/shared then alpha/shared", visits)
	}
}

func TestQuickSwitchPinCursorRestoresQualifiedDuplicate(t *testing.T) {
	app := newTestAppForQuickSwitch()
	sharedID := "shared-task"
	alpha := &models.Project{ID: "alpha", Name: "Alpha"}
	beta := &models.Project{ID: "beta", Name: "Beta"}
	app.recentTaskVisits = []RecentTaskVisitEntry{
		{Task: &models.Task{ID: sharedID, Title: "Alpha copy"}, Project: alpha, Pinned: true},
		{Task: &models.Task{ID: sharedID, Title: "Beta copy"}, Project: beta, Pinned: true},
	}
	app.rebuildQuickSwitchFilter()
	app.quickSwitchCursor = 1

	app.quickSwitchTogglePin()

	if app.quickSwitchCursor != 1 {
		t.Fatalf("cursor moved to the other same-UUID visit: got %d want 1", app.quickSwitchCursor)
	}
	entry := app.recentTaskVisits[app.quickSwitchFiltered[app.quickSwitchCursor]]
	if entry.Project.ID != beta.ID || entry.Pinned {
		t.Fatalf("pin toggle targeted wrong duplicate: %+v", entry)
	}
}

type duplicateGlobalInboxEnv struct {
	m         *AppModel
	processor *GlobalInboxProcessorModel
	alpha     *models.Project
	beta      *models.Project
	sharedID  string
}

func newDuplicateGlobalInboxEnv(t *testing.T) *duplicateGlobalInboxEnv {
	t.Helper()
	base := newTaskDeleteEnv(t)
	alphaTask := base.sibling
	betaTask := base.lone
	betaTask.ID = alphaTask.ID
	alphaTask.ApplyWorkflowState(models.StateInbox)
	betaTask.ApplyWorkflowState(models.StateInbox)
	base.alpha.Tasks = []*models.Task{alphaTask}
	base.beta.Tasks = []*models.Task{betaTask}
	base.alpha.BuildTaskTree()
	base.beta.BuildTaskTree()
	if err := storage.SaveProject(base.alpha); err != nil {
		t.Fatalf("save alpha duplicate inbox: %v", err)
	}
	if err := storage.SaveProject(base.beta); err != nil {
		t.Fatalf("save beta duplicate inbox: %v", err)
	}

	m := base.appModel()
	processor := NewGlobalInboxProcessorModel(m.configManager, m.runtime)
	if len(processor.inboxTasks) != 2 {
		t.Fatalf("global duplicate inbox tasks = %d, want 2", len(processor.inboxTasks))
	}
	m.globalInboxProcessor = processor
	m.views.Register(ViewGlobalInboxProcessor, processor)
	m.pushView(ViewGlobalInboxProcessor)
	return &duplicateGlobalInboxEnv{
		m: m, processor: processor, alpha: base.alpha, beta: base.beta, sharedID: alphaTask.ID,
	}
}

func (e *duplicateGlobalInboxEnv) indexForProject(t *testing.T, projectID string) int {
	t.Helper()
	for i, item := range e.processor.inboxTasks {
		if item.Project.ID == projectID && item.Task.ID == e.sharedID {
			return i
		}
	}
	t.Fatalf("duplicate inbox task not found for project %q", projectID)
	return -1
}

func TestGlobalInboxDuplicateIDSkipUndoAndFocusStayQualified(t *testing.T) {
	e := newDuplicateGlobalInboxEnv(t)
	alphaIdentity := TaskIdentity{ProjectID: e.alpha.ID, TaskID: e.sharedID}
	betaIdentity := TaskIdentity{ProjectID: e.beta.ID, TaskID: e.sharedID}
	e.processor.currentIndex = e.indexForProject(t, e.alpha.ID)

	if cmd := e.processor.skipCurrent(); cmd != nil {
		t.Fatal("skipping alpha should advance to the beta duplicate without closing")
	}
	if !e.processor.skippedIDs[alphaIdentity] || e.processor.skippedIDs[betaIdentity] {
		t.Fatalf("skip crossed duplicate identities: %+v", e.processor.skippedIDs)
	}
	if task, project := e.processor.currentTaskRef(); taskIdentityFor(task, project) != betaIdentity {
		t.Fatalf("skip focus = %v, want beta duplicate", taskIdentityFor(task, project))
	}

	if cmd := e.processor.undoLastAction(); cmd != nil {
		t.Fatal("undoing an in-memory skip should not save")
	}
	if e.processor.skippedIDs[alphaIdentity] || e.processor.skippedIDs[betaIdentity] {
		t.Fatalf("undo changed the wrong duplicate: %+v", e.processor.skippedIDs)
	}
	if task, project := e.processor.currentTaskRef(); taskIdentityFor(task, project) != alphaIdentity {
		t.Fatalf("undo focus = %v, want restored alpha duplicate", taskIdentityFor(task, project))
	}
}

func TestGlobalInboxDuplicateIDOverlappingDeletesApplyBothOrders(t *testing.T) {
	for _, tc := range []struct {
		name      string
		betaFirst bool
	}{
		{name: "alpha then beta"},
		{name: "beta then alpha", betaFirst: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := newDuplicateGlobalInboxEnv(t)
			alphaIdentity := TaskIdentity{ProjectID: e.alpha.ID, TaskID: e.sharedID}
			betaIdentity := TaskIdentity{ProjectID: e.beta.ID, TaskID: e.sharedID}
			alphaResult := confirmGlobalInboxDeleteAt(t, e.m, e.processor, e.indexForProject(t, e.alpha.ID))
			betaResult := confirmGlobalInboxDeleteAt(t, e.m, e.processor, e.indexForProject(t, e.beta.ID))
			if len(e.processor.pendingDeletes) != 2 {
				t.Fatalf("same UUIDs should retain two qualified pending slots, got %+v", e.processor.pendingDeletes)
			}

			first, second := alphaResult, betaResult
			if tc.betaFirst {
				first, second = betaResult, alphaResult
			}
			cmd1 := deliverInboxDeleteResult(e.m, first)
			cmd2 := deliverInboxDeleteResult(e.m, second)
			closes, processed, skipped := inboxCloseCounts(t, cmd1, cmd2)
			if closes != 1 || processed != 2 || skipped != 0 {
				t.Fatalf("close outcome = (%d closes %d processed %d skipped), want (1 2 0)", closes, processed, skipped)
			}
			if !e.processor.processedIDs[alphaIdentity] || !e.processor.processedIDs[betaIdentity] {
				t.Fatalf("qualified processed set = %+v, want both duplicates", e.processor.processedIDs)
			}
			if len(e.processor.pendingDeletes) != 0 || len(e.m.pendingInboxDeletes) != 0 {
				t.Fatalf("delete bookkeeping survived: processor=%+v shell=%+v", e.processor.pendingDeletes, e.m.pendingInboxDeletes)
			}
		})
	}
}

func TestGlobalInboxDuplicateIDOverlappingSuccessFailureIsolated(t *testing.T) {
	for _, tc := range []struct {
		name         string
		failureFirst bool
	}{
		{name: "success then failure"},
		{name: "failure then success", failureFirst: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := newDuplicateGlobalInboxEnv(t)
			alphaIdentity := TaskIdentity{ProjectID: e.alpha.ID, TaskID: e.sharedID}
			betaIdentity := TaskIdentity{ProjectID: e.beta.ID, TaskID: e.sharedID}
			alphaResult := confirmGlobalInboxDeleteAt(t, e.m, e.processor, e.indexForProject(t, e.alpha.ID))
			betaToken := armInboxDeleteNoCommit(t, e.m, &e.processor.inboxCore, e.indexForProject(t, e.beta.ID))
			failure := TransactionErrorMsg{Err: errors.New("delete rejected"), DeleteRequestToken: betaToken}

			deliverFailure := func() {
				if handled, _ := e.m.handleGlobalSaveMessage(failure); !handled {
					t.Fatal("TransactionErrorMsg should be handled")
				}
			}
			var successCmd tea.Cmd
			if tc.failureFirst {
				deliverFailure()
				successCmd = deliverInboxDeleteResult(e.m, alphaResult)
			} else {
				successCmd = deliverInboxDeleteResult(e.m, alphaResult)
				deliverFailure()
			}
			if closes, _, _ := inboxCloseCounts(t, successCmd); closes != 0 {
				t.Fatalf("surviving beta duplicate should keep processor open, got %d closes", closes)
			}
			if !e.processor.processedIDs[alphaIdentity] || e.processor.processedIDs[betaIdentity] {
				t.Fatalf("success/failure crossed duplicate identities: %+v", e.processor.processedIDs)
			}
			if len(e.processor.pendingDeletes) != 0 || len(e.m.pendingInboxDeletes) != 0 {
				t.Fatalf("outcomes should clear only their own slots: processor=%+v shell=%+v", e.processor.pendingDeletes, e.m.pendingInboxDeletes)
			}
			processed, skipped, remaining, total := e.processor.getProgressStats()
			if processed != 1 || skipped != 0 || remaining != 1 || total != 2 {
				t.Fatalf("progress = (%d %d %d %d), want (1 0 1 2)", processed, skipped, remaining, total)
			}
			task, project := e.processor.currentTaskRef()
			if task == nil || project == nil || taskIdentityFor(task, project) != betaIdentity {
				t.Fatalf("focus moved off surviving beta duplicate: task=%v project=%v", task, project)
			}
		})
	}
}
