package tui

import (
	"os"
	"reflect"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/operation"
	"github.com/sglavoie/tt/internal/tui/outliner"
)

func setupTestKey(t *testing.T, key string) tea.KeyPressMsg {
	t.Helper()
	msg, err := parseUITestKey(key)
	if err != nil {
		t.Fatal(err)
	}
	return msg
}

func setupTestModel(t *testing.T, groups ...[]*models.Task) (*AppModel, *appBulkPickerEnv) {
	t.Helper()
	env := newAppBulkPickerEnv(t, groups...)
	model := env.appModel()
	model.domain.service = newTUIAppService(env.dataDir, env.mgr.RuntimeDependencies(), env.ledger)
	return model, env
}

func setupTestTask(t *testing.T, short int, title string) *models.Task {
	t.Helper()
	task := newBulkPickerTestTask(t, short, title)
	task.ApplyWorkflowState(models.StateDoing)
	return task
}

func submitSetupDraft(t *testing.T, model *AppModel) checkInSetupSavedMsg {
	t.Helper()
	model.operations.CheckInEditor.HandlePaste(tea.PasteMsg{Content: "+7d"})
	cmd := model.handleCheckInSetupKey(setupTestKey(t, "ctrl+s"))
	if cmd == nil {
		t.Fatal("save produced no command")
	}
	if duplicate := model.handleCheckInSetupKey(setupTestKey(t, "ctrl+s")); duplicate != nil {
		t.Fatal("duplicate save launched")
	}
	return cmd().(checkInSetupSavedMsg)
}

func TestCheckInSetupSaveSkipRevisitAndPerTaskUndo(t *testing.T) {
	a, b, c := setupTestTask(t, 1, "first"), setupTestTask(t, 2, "skip"), setupTestTask(t, 1, "last")
	model, env := setupTestModel(t, []*models.Task{a, b}, []*models.Task{c})
	model.startCheckInSetup()
	if model.checkInSetup == nil || len(model.checkInSetup.candidates) != 3 {
		t.Fatalf("pass missing: %v", model.feedback.err)
	}
	first := submitSetupDraft(t, model)
	model.handleCheckInSetupSaved(first)
	model.handleCheckInSetupSaved(first) // duplicate delivery must not advance.
	if model.checkInSetup.index != 1 || model.checkInSetup.saved != 1 {
		t.Fatal("wrong progress after duplicate")
	}
	model.handleCheckInSetupKey(setupTestKey(t, "ctrl+n"))
	if model.checkInSetup.index != 2 {
		t.Fatal("skip did not advance")
	}
	last := submitSetupDraft(t, model)
	model.handleCheckInSetupSaved(last)
	if model.checkInSetup != nil || model.operations.CheckInEditor != nil {
		t.Fatal("finished pass still mounted")
	}
	if !strings.Contains(model.feedback.message, "2 saved, 1 skipped") {
		t.Fatal(model.feedback.message)
	}
	if !env.reload(t, 0).GetTaskByID(b.ID).CheckIn().IsEmpty() {
		t.Fatal("skip wrote schedule")
	}
	env.undo(t)
	if !env.reload(t, 1).GetTaskByID(c.ID).CheckIn().IsEmpty() || env.reload(t, 0).GetTaskByID(a.ID).CheckIn().IsEmpty() {
		t.Fatal("one undo did not restore only the last save")
	}
	env.redo(t)
	model.startCheckInSetup()
	if model.checkInSetup == nil || len(model.checkInSetup.candidates) != 1 || model.checkInSetup.candidates[0].Locator.TaskID != b.ID {
		t.Fatal("revisit included already scheduled tasks")
	}
	model.handleCheckInSetupKey(setupTestKey(t, "esc"))
	if model.checkInSetup != nil || !strings.Contains(model.feedback.message, "1 remaining") {
		t.Fatal("exit lost pending result")
	}
}

func TestCheckInSetupKeepsFixedPopulationAndRechecksBeforeSave(t *testing.T) {
	a, b := setupTestTask(t, 1, "first"), setupTestTask(t, 2, "second")
	model, env := setupTestModel(t, []*models.Task{a, b})
	model.startCheckInSetup()
	p := env.reload(t, 0)
	p.GetTaskByID(a.ID).ApplyWorkflowState(models.StateBacklog)
	p.GetTaskByID(b.ID).ApplyWorkflowState(models.StateDone)
	added := setupTestTask(t, 3, "new arrival")
	p.Tasks = append(p.Tasks, added)
	p.NextShortID = 4
	if err := storage.SaveProject(p); err != nil {
		t.Fatal(err)
	}
	result := submitSetupDraft(t, model)
	if !result.skipped || result.err != nil {
		t.Fatalf("stale save = %+v", result)
	}
	model.handleCheckInSetupSaved(result)
	if model.checkInSetup != nil || !strings.Contains(model.feedback.message, "2 no longer eligible") {
		t.Fatal(model.feedback.message)
	}
	if !env.reload(t, 0).GetTaskByID(added.ID).CheckIn().IsEmpty() {
		t.Fatal("fixed pass widened to arrival")
	}
	model.startCheckInSetup()
	if len(model.checkInSetup.candidates) != 1 {
		t.Fatal("new pass did not see arrival")
	}
	model.finishCheckInSetup()
}

func TestCheckInSetupUnreadableWorkspaceRetainsDraftAndHonestCoverage(t *testing.T) {
	a, b := setupTestTask(t, 1, "first"), setupTestTask(t, 1, "second")
	model, env := setupTestModel(t, []*models.Task{a}, []*models.Task{b})
	path := storage.TasksFilePath(env.projects[1].Path)
	original, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	// A directory in place of tasks.md fails even when tests run with elevated permissions.
	breakRead := func() {
		if err := os.Remove(path); err != nil {
			t.Fatal(err)
		}
		if err := os.Mkdir(path, 0700); err != nil {
			t.Fatal(err)
		}
	}
	repair := func() {
		if err := os.Remove(path); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, original, 0600); err != nil {
			t.Fatal(err)
		}
	}
	breakRead()
	model.startCheckInSetup()
	if model.checkInSetup != nil || model.feedback.err == nil || !strings.Contains(model.feedback.err.Error(), "could not cover") {
		t.Fatal("partial initial census reported success")
	}
	repair()
	model.startCheckInSetup()
	breakRead()
	failed := submitSetupDraft(t, model)
	model.handleCheckInSetupSaved(failed)
	if failed.err == nil || model.checkInSetup.index != 0 || model.operations.CheckInEditor.DateInput() != "+7d" || !strings.Contains(model.operations.CheckInEditor.Notice(), "Save failed") {
		t.Fatal("failed save discarded draft or advanced")
	}
	if !env.reload(t, 0).GetTaskByID(a.ID).CheckIn().IsEmpty() {
		t.Fatal("failed save wrote schedule")
	}
	repair()
	// Retry the same draft, without pasting or reconstructing it.
	cmd := model.handleCheckInSetupKey(setupTestKey(t, "ctrl+s"))
	model.handleCheckInSetupSaved(failed) // An old failure cannot consume the retry.
	if !model.checkInSetup.busy {
		t.Fatal("old failed result consumed retry")
	}
	result := cmd().(checkInSetupSavedMsg)
	model.handleCheckInSetupSaved(result)
	if result.err != nil || model.checkInSetup.index != 1 {
		t.Fatalf("retry = %+v", result)
	}
	breakRead()
	model.finishCheckInSetup()
	model.startCheckInSetup()
	if model.checkInSetup != nil {
		t.Fatal("revisit ignored unreadable project")
	}
}

func TestCheckInSetupEscapeDuringSaveWaitsForOutcome(t *testing.T) {
	task := setupTestTask(t, 1, "one")
	model, env := setupTestModel(t, []*models.Task{task})
	model.startCheckInSetup()
	model.operations.CheckInEditor.HandlePaste(tea.PasteMsg{Content: "+7d"})
	cmd := model.handleCheckInSetupKey(setupTestKey(t, "ctrl+s"))
	model.handleCheckInSetupKey(setupTestKey(t, "esc"))
	if model.checkInSetup == nil || !model.checkInSetup.exiting {
		t.Fatal("exit abandoned in-flight save")
	}
	result := cmd().(checkInSetupSavedMsg)
	model.handleCheckInSetupSaved(result)
	if model.checkInSetup != nil || !strings.Contains(model.feedback.message, "1 saved") || env.reload(t, 0).Tasks[0].CheckIn().IsEmpty() {
		t.Fatal("exit misreported committed schedule")
	}
	model.handleCheckInSetupSaved(result)
	if model.checkInSetup != nil {
		t.Fatal("late delivery reopened pass")
	}
}

func TestCheckInSetupMenuHelpAndViewPreservation(t *testing.T) {
	driver, view := menuBarDriver(t)
	model := driver.App()
	candidates, err := model.domain.service.CheckInSetupCandidates()
	if err != nil || len(candidates) == 0 {
		t.Fatalf("fixture census: %v %v", candidates, err)
	}
	// Enter a query that hides all tasks and retain its exact presentation.
	for _, key := range []string{"/", "z", "z", "z", "enter"} {
		pressMenuBarKey(t, driver, key)
	}
	checkpoint := view.CaptureOutlinerPresentationCheckpoint()
	menu := menuWithID(t, resolveOutlinerMenus(t, model, chrome.ModeAll), outliner.MenuIDTask)
	if !menuHasLabel(menu, "Set up check-ins") {
		t.Fatal("setup missing from global menu")
	}
	found := false
	for _, item := range model.coreInputHelpItems() {
		if item.Description == "Set up check-ins" && strings.Contains(item.Key, "F10") {
			found = true
		}
	}
	if !found {
		t.Fatal("setup missing from help")
	}
	driver.dispatch(outliner.MenuBarActivateMsg{OperationID: operation.OperationCoreSetUpCheckIns})
	driver.drain()
	if model.checkInSetup == nil || len(model.checkInSetup.candidates) != len(candidates) {
		t.Fatalf("hidden tasks missing from setup: %v", model.feedback.err)
	}
	screen := driver.Observe(ObserveOptions{}).PlainScreen
	for _, text := range []string{"Set up check-ins", "remaining after this", "Ctrl+N", candidates[0].Title} {
		if !strings.Contains(screen, text) {
			t.Fatalf("missing %q:\n%s", text, screen)
		}
	}
	pressMenuBarKey(t, driver, "enter") // preset open: Escape still exits the whole setup.
	pressMenuBarKey(t, driver, "esc")
	if model.checkInSetup != nil || !reflect.DeepEqual(checkpoint, view.CaptureOutlinerPresentationCheckpoint()) {
		t.Fatal("setup changed underlying view")
	}
}

func TestCheckInSetupProductionCensusExcludesUnreadableInactiveOwners(t *testing.T) {
	model, env := setupTestModel(t,
		[]*models.Task{setupTestTask(t, 1, "active")},
		[]*models.Task{setupTestTask(t, 1, "parked")},
		[]*models.Task{setupTestTask(t, 1, "archived")})
	env.projects[1].Tags = []string{models.ParkedProjectTag}
	env.projects[2].Archived = true
	setTestProjects(env.cfg, env.projects)
	if err := saveTestRuntime(env.mgr, env.cfg); err != nil {
		t.Fatal(err)
	}
	for _, project := range env.projects[1:] {
		path := storage.TasksFilePath(project.Path)
		if err := os.Remove(path); err != nil {
			t.Fatal(err)
		}
		if err := os.Mkdir(path, 0700); err != nil {
			t.Fatal(err)
		}
	}
	model.startCheckInSetup()
	if model.checkInSetup == nil || len(model.checkInSetup.candidates) != 1 {
		t.Fatalf("inactive owners spoiled census: %v", model.feedback.err)
	}
	// The active owner is parked through a fresh catalog while its editor is open.
	env.projects[0].Tags = []string{models.ParkedProjectTag}
	setTestProjects(env.cfg, env.projects)
	if err := saveTestRuntime(env.mgr, env.cfg); err != nil {
		t.Fatal(err)
	}
	result := submitSetupDraft(t, model)
	model.handleCheckInSetupSaved(result)
	if result.err != nil || !result.skipped || model.checkInSetup != nil {
		t.Fatalf("fresh parking = %+v", result)
	}
	if !env.reload(t, 0).Tasks[0].CheckIn().IsEmpty() {
		t.Fatal("parked owner was scheduled")
	}
}

func TestCheckInSetupMinimumLayoutAndSavedViewRestoration(t *testing.T) {
	driver, view := menuBarDriver(t)
	model := driver.App()
	candidates, err := model.domain.service.CheckInSetupCandidates()
	if err != nil {
		t.Fatal(err)
	}
	// Scope, folds, and visual selection stay owned by the view.
	view.PushProjectScope(candidates[len(candidates)-1].Locator.ProjectID)
	pressMenuBarKey(t, driver, "t")
	pressMenuBarKey(t, driver, "j")
	pressMenuBarKey(t, driver, "v")
	checkpoint := view.CaptureOutlinerPresentationCheckpoint()
	model.startCheckInSetup()
	for _, size := range [][2]int{{62, 24}, {80, 24}, {100, 35}} {
		driver.Resize(size[0], size[1])
		driver.drain()
		observation := driver.Observe(ObserveOptions{})
		if len(observation.LayoutIssues) > 0 {
			t.Fatalf("%v: %v\n%s", size, observation.LayoutIssues, observation.PlainScreen)
		}
		if !strings.Contains(observation.PlainScreen, "Ctrl+N") || !strings.Contains(observation.PlainScreen, "Esc exit") {
			t.Fatalf("setup guidance missing at %v:\n%s", size, observation.PlainScreen)
		}
	}
	model.operations.CheckInEditor.HandlePaste(tea.PasteMsg{Content: "today"})
	pressMenuBarKey(t, driver, "ctrl+s")
	if model.checkInSetup != nil {
		pressMenuBarKey(t, driver, "esc")
	}
	if !reflect.DeepEqual(checkpoint, view.CaptureOutlinerPresentationCheckpoint()) {
		t.Fatal("saved pass lost view context")
	}
	first := candidates[0].Locator
	if task := model.projectByID(first.ProjectID).GetTaskByID(first.TaskID); task == nil || task.CheckIn().IsEmpty() {
		t.Fatal("saved schedule absent from refreshed view data")
	}
	if err = driver.Restart(); err != nil {
		t.Fatal(err)
	}
	fresh, err := driver.App().domain.service.CheckInSetupCandidates()
	if err != nil {
		t.Fatal(err)
	}
	for _, candidate := range fresh {
		if candidate.Locator == first {
			t.Fatal("restart offered an already saved schedule")
		}
	}
}

func TestCheckInSetupAdvanceReadFailureDoesNotCountAsSkip(t *testing.T) {
	model, env := setupTestModel(t, []*models.Task{setupTestTask(t, 1, "first")}, []*models.Task{setupTestTask(t, 1, "second")})
	model.startCheckInSetup()
	result := submitSetupDraft(t, model)
	path := storage.TasksFilePath(env.projects[1].Path)
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(path, 0700); err != nil {
		t.Fatal(err)
	}
	model.handleCheckInSetupSaved(result)
	pass := model.checkInSetup
	if pass == nil || pass.saved != 1 || pass.index != 1 || pass.changed != 0 || pass.skipped != 0 {
		t.Fatalf("read failure advanced: %+v", pass)
	}
	if !strings.Contains(model.operations.CheckInEditor.Notice(), "coverage incomplete") {
		t.Fatal("read failure not visible")
	}
	model.handleCheckInSetupKey(setupTestKey(t, "esc"))
	if !strings.Contains(model.feedback.message, "1 remaining") || !strings.Contains(model.feedback.message, "coverage incomplete") {
		t.Fatal(model.feedback.message)
	}
}
