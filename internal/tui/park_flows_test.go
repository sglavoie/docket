package tui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/keymap"
)

func TestParkInputExplicitParkPersistsNote(t *testing.T) {
	app := newTestAppModel()
	undoDepth := 10
	app.config.UndoHistoryDepth = &undoDepth
	task := newBulkPickerTestTask(t, 1, "resume later")
	project := newBulkPickerTestProject(t, "Park Flow", task)

	handled, _ := app.handleTaskMessages(ShowParkInputMsg{Task: task, Project: project})
	if !handled {
		t.Fatal("ShowParkInputMsg should be handled")
	}
	if app.currentView() != ViewParkInput {
		t.Fatalf("currentView = %v, want ViewParkInput", app.currentView())
	}

	parkInput := app.views.Get(ViewParkInput).(*ParkInputModel)
	parkInput.input.SetValue("blocked on API review")
	_, doneCmd := parkInput.Update(syntheticSpecialKey(tea.KeyEnter))
	doneMsg, ok := doneCmd().(ParkNoteDoneMsg)
	if !ok {
		t.Fatalf("done command emitted %T, want ParkNoteDoneMsg", doneCmd())
	}

	handled, saveCmd := app.handleTaskMessages(doneMsg)
	if !handled {
		t.Fatal("ParkNoteDoneMsg should be handled")
	}
	requireBulkPickerSaveComplete(t, runBulkPickerBatchCmd(t, saveCmd))

	reloaded := loadBulkPickerTestProject(t, project)
	reloadedTask := reloaded.GetTaskByID(task.ID)
	if reloadedTask == nil {
		t.Fatal("parked task was not reloaded")
	}
	if !reloadedTask.IsParked() {
		t.Fatal("task should be parked")
	}
	if reloadedTask.ParkedNote != "blocked on API review" {
		t.Fatalf("ParkedNote = %q, want %q", reloadedTask.ParkedNote, "blocked on API review")
	}
	assertBulkPickerUndoTaskIDs(t, project, []string{task.ID})
	assertLastParkFlowUndoAction(t, project, "Park Task")
}

func TestParkInputEmptyEnterOnParkedTaskUnparks(t *testing.T) {
	app := newTestAppModel()
	undoDepth := 10
	app.config.UndoHistoryDepth = &undoDepth
	task := newBulkPickerTestTask(t, 1, "resume now")
	task.Park("next step")
	project := newBulkPickerTestProject(t, "Unpark Flow", task)

	handled, _ := app.handleTaskMessages(ShowParkInputMsg{Task: task, Project: project})
	if !handled {
		t.Fatal("ShowParkInputMsg should be handled")
	}

	parkInput := app.views.Get(ViewParkInput).(*ParkInputModel)
	if got := parkInput.input.Value(); got != "next step" {
		t.Fatalf("initial input value = %q, want existing parked note", got)
	}
	parkInput.input.SetValue("")
	_, doneCmd := parkInput.Update(syntheticSpecialKey(tea.KeyEnter))
	doneMsg, ok := doneCmd().(ParkNoteDoneMsg)
	if !ok {
		t.Fatalf("done command emitted %T, want ParkNoteDoneMsg", doneCmd())
	}
	if !doneMsg.Unparked {
		t.Fatal("empty Enter on a parked task should mark ParkNoteDoneMsg.Unparked")
	}

	handled, saveCmd := app.handleTaskMessages(doneMsg)
	if !handled {
		t.Fatal("ParkNoteDoneMsg should be handled")
	}
	requireBulkPickerSaveComplete(t, runBulkPickerBatchCmd(t, saveCmd))

	reloaded := loadBulkPickerTestProject(t, project)
	reloadedTask := reloaded.GetTaskByID(task.ID)
	if reloadedTask == nil {
		t.Fatal("unparked task was not reloaded")
	}
	if reloadedTask.IsParked() {
		t.Fatal("task should be unparked")
	}
	if reloadedTask.ParkedNote != "" || reloadedTask.ParkedAt != nil {
		t.Fatalf("parked fields should be cleared, got note=%q at=%v", reloadedTask.ParkedNote, reloadedTask.ParkedAt)
	}
	assertBulkPickerUndoTaskIDs(t, project, []string{task.ID})
	assertLastParkFlowUndoAction(t, project, "Unpark Task")
}

func TestParkInputRejectsCompletedTaskNewPark(t *testing.T) {
	task := newBulkPickerTestTask(t, 1, "done should stay done")
	task.Status = models.StatusDone
	project := newBulkPickerTestProject(t, "Completed Park Guard", task)
	parkInput := NewParkInputModel(task, project)

	parkInput.input.SetValue("resume after completion")
	_, doneCmd := parkInput.Update(syntheticSpecialKey(tea.KeyEnter))

	if doneCmd != nil {
		t.Fatal("completed task park should stay in the input view without emitting ParkNoteDoneMsg")
	}
	if task.IsParked() {
		t.Fatal("completed task should not receive new parked metadata")
	}
	if parkInput.err == nil || parkInput.err.Error() != "cannot park a done task" {
		t.Fatalf("err = %v, want completed-task rejection", parkInput.err)
	}
	reloaded := loadBulkPickerTestProject(t, project)
	if reloadedTask := reloaded.GetTaskByID(task.ID); reloadedTask == nil || reloadedTask.IsParked() {
		t.Fatalf("persisted completed task should remain unparked, got %#v", reloadedTask)
	}
}

func TestParkInputEmptyEnterClearsLegacyParkedCompletedTask(t *testing.T) {
	app := newTestAppModel()
	undoDepth := 10
	app.config.UndoHistoryDepth = &undoDepth
	task := newBulkPickerTestTask(t, 1, "completed legacy parked")
	task.Park("legacy metadata")
	task.Status = models.StatusDone
	project := newBulkPickerTestProject(t, "Completed Legacy Unpark", task)

	handled, _ := app.handleTaskMessages(ShowParkInputMsg{Task: task, Project: project})
	if !handled {
		t.Fatal("ShowParkInputMsg should be handled")
	}

	parkInput := app.views.Get(ViewParkInput).(*ParkInputModel)
	parkInput.input.SetValue("")
	_, doneCmd := parkInput.Update(syntheticSpecialKey(tea.KeyEnter))
	doneMsg, ok := doneCmd().(ParkNoteDoneMsg)
	if !ok {
		t.Fatalf("done command emitted %T, want ParkNoteDoneMsg", doneCmd())
	}
	if !doneMsg.Unparked {
		t.Fatal("empty Enter on a parked completed task should mark ParkNoteDoneMsg.Unparked")
	}

	handled, saveCmd := app.handleTaskMessages(doneMsg)
	if !handled {
		t.Fatal("ParkNoteDoneMsg should be handled")
	}
	requireBulkPickerSaveComplete(t, runBulkPickerBatchCmd(t, saveCmd))

	reloaded := loadBulkPickerTestProject(t, project)
	reloadedTask := reloaded.GetTaskByID(task.ID)
	if reloadedTask == nil {
		t.Fatal("unparked completed task was not reloaded")
	}
	if reloadedTask.IsParked() {
		t.Fatal("legacy parked metadata should be cleared from completed task")
	}
	if reloadedTask.Status != models.StatusDone {
		t.Fatalf("Status = %s, want done", reloadedTask.Status)
	}
	assertBulkPickerUndoTaskIDs(t, project, []string{task.ID})
	assertLastParkFlowUndoAction(t, project, "Unpark Task")
}

func TestParkInputEscDiscardLeavesTaskUnchanged(t *testing.T) {
	task := newBulkPickerTestTask(t, 1, "leave unchanged")
	project := newBulkPickerTestProject(t, "Park Esc Flow", task)
	parkInput := NewParkInputModel(task, project)

	parkInput.input.SetValue("do not save")
	_, cmd := parkInput.Update(syntheticSpecialKey(tea.KeyEsc))
	if cmd != nil {
		t.Fatal("first Esc with changed input should ask for discard confirmation, not close immediately")
	}
	if !parkInput.confirmDiscard {
		t.Fatal("park input should show discard confirmation after changed Esc")
	}

	_, cmd = parkInput.Update(syntheticKey('y'))
	if cmd == nil {
		t.Fatal("confirming discard should emit BackMsg")
	}
	if _, ok := cmd().(BackMsg); !ok {
		t.Fatalf("confirming discard emitted %T, want BackMsg", cmd())
	}
	if task.IsParked() {
		t.Fatal("Esc discard should not park the task")
	}

	reloaded := loadBulkPickerTestProject(t, project)
	if reloaded.GetTaskByID(task.ID).IsParked() {
		t.Fatal("Esc discard should not persist a parked task")
	}
}

func TestParkKeybindingsEmitShowParkInput(t *testing.T) {
	task := newBulkPickerTestTask(t, 1, "park from key")
	project := newBulkPickerTestProject(t, "Park Keybindings", task)

	detail := NewTaskDetailModel(task, project, nil, nil)
	_, cmd := detail.handleKeyMsg(syntheticKey('P'))
	assertShowParkInputCmd(t, cmd, task, project)

	board := newTestFocusBoard()
	board.filtered = []boardRow{{Task: task, Project: project}}
	board.Dispatcher = keymap.NewDispatcher(board.buildKeyMap())
	_, cmd, handled := board.Dispatcher.Dispatch(syntheticKey('P'))
	if !handled {
		t.Fatal("focus board P should be handled")
	}
	assertShowParkInputCmd(t, cmd, task, project)
}

func assertShowParkInputCmd(t *testing.T, cmd tea.Cmd, task *models.Task, project *models.Project) {
	t.Helper()
	if cmd == nil {
		t.Fatal("expected non-nil command")
	}
	msg, ok := cmd().(ShowParkInputMsg)
	if !ok {
		t.Fatalf("command emitted %T, want ShowParkInputMsg", cmd())
	}
	if msg.Task != task || msg.Project != project {
		t.Fatalf("ShowParkInputMsg refs = (%p, %p), want (%p, %p)", msg.Task, msg.Project, task, project)
	}
}

type crossProjectParkFlowFixture struct {
	app      *AppModel
	taskA    *models.Task
	taskB    *models.Task
	projectA *models.Project
	projectB *models.Project
	outliner *OutlinerModel
}

func newCrossProjectParkFlowFixture(t *testing.T, mutateTaskB func(*models.Task)) crossProjectParkFlowFixture {
	t.Helper()

	app := newTestAppModel()
	taskA := newBulkPickerTestTask(t, 1, "alpha task")
	taskB := newBulkPickerTestTask(t, 1, "beta task")
	if mutateTaskB != nil {
		mutateTaskB(taskB)
	}
	projectA := newBulkPickerTestProject(t, "Cross Park Alpha", taskA)
	projectB := newBulkPickerTestProject(t, "Cross Park Beta", taskB)
	app.config.Projects = []*models.Project{projectA, projectB}
	app.config.CurrentProject = projectA.ID
	app.currentProject = projectA

	crossOutliner := NewOutlinerModel(nil, app.config, app.configManager)
	crossOutliner.SetSize(120, 20)
	crossOutliner.loadedProjects = []*models.Project{projectA, projectB}
	crossOutliner.rows = []outlinerRow{
		{IsSectionHeader: true, SectionName: models.SectionInbox, ProjectID: projectA.ID},
		{task: taskA, ProjectID: projectA.ID},
		{IsSeparator: true},
		{IsSectionHeader: true, SectionName: models.SectionInbox, ProjectID: projectB.ID},
		{task: taskB, ProjectID: projectB.ID},
	}
	crossOutliner.cursor = 4
	crossOutliner.Dispatcher = keymap.NewDispatcher(crossOutliner.buildKeyMap())
	app.crossOutliner = crossOutliner
	app.views.Register(ViewCrossProjectOutliner, crossOutliner)
	app.viewStack = append(app.viewStack, ViewCrossProjectOutliner)

	return crossProjectParkFlowFixture{
		app:      app,
		taskA:    taskA,
		taskB:    taskB,
		projectA: projectA,
		projectB: projectB,
		outliner: crossOutliner,
	}
}

func updateParkFlowApp(t *testing.T, app *AppModel, msg tea.Msg) tea.Cmd {
	t.Helper()

	model, cmd := app.Update(msg)
	if model != app {
		updated, ok := model.(*AppModel)
		if !ok {
			t.Fatalf("Update returned %T, want *AppModel", model)
		}
		*app = *updated
	}
	return cmd
}

func immediateParkFlowMessages(t *testing.T, cmd tea.Cmd) []tea.Msg {
	t.Helper()
	if cmd == nil {
		return nil
	}
	return immediateParkFlowMessagesFromMsg(t, cmd())
}

func immediateParkFlowMessagesFromMsg(t *testing.T, msg tea.Msg) []tea.Msg {
	t.Helper()
	if batch, ok := msg.(tea.BatchMsg); ok {
		msgs := make([]tea.Msg, 0, len(batch))
		for _, batchCmd := range batch {
			if batchCmd == nil {
				continue
			}
			msgs = append(msgs, immediateParkFlowMessagesFromMsg(t, batchCmd())...)
		}
		return msgs
	}
	return []tea.Msg{msg}
}

func openCrossProjectParkInputThroughAppUpdate(t *testing.T, app *AppModel) {
	t.Helper()

	for _, msg := range immediateParkFlowMessages(t, updateParkFlowApp(t, app, syntheticKey('t'))) {
		if msg != nil {
			updateParkFlowApp(t, app, msg)
		}
	}

	msgs := immediateParkFlowMessages(t, updateParkFlowApp(t, app, syntheticKey('P')))
	foundShowPark := false
	for _, msg := range msgs {
		if _, ok := msg.(ShowParkInputMsg); ok {
			foundShowPark = true
		}
		updateParkFlowApp(t, app, msg)
	}
	if !foundShowPark {
		t.Fatalf("direct tP through AppModel.Update emitted %#v, want ShowParkInputMsg", msgs)
	}
	if app.currentView() != ViewParkInput {
		t.Fatalf("currentView = %v, want ViewParkInput", app.currentView())
	}
}

func typeParkFlowNoteThroughAppUpdate(t *testing.T, app *AppModel, note string) {
	t.Helper()
	for _, r := range note {
		updateParkFlowApp(t, app, syntheticKey(r))
	}
}

func clearParkFlowNoteThroughAppUpdate(t *testing.T, app *AppModel) {
	t.Helper()
	for i := 0; i < 300; i++ {
		parkInput := app.views.Get(ViewParkInput).(*ParkInputModel)
		if parkInput.input.Value() == "" {
			return
		}
		updateParkFlowApp(t, app, syntheticSpecialKey(tea.KeyBackspace))
	}
	t.Fatal("park input did not clear after repeated backspace keys")
}

func submitCrossProjectParkFlowThroughAppUpdate(t *testing.T, app *AppModel) []tea.Msg {
	t.Helper()

	doneMsgs := immediateParkFlowMessages(t, updateParkFlowApp(t, app, syntheticSpecialKey(tea.KeyEnter)))
	saveMsgs := make([]tea.Msg, 0)
	for _, msg := range doneMsgs {
		doneMsg, ok := msg.(ParkNoteDoneMsg)
		if !ok {
			t.Fatalf("Enter emitted %T, want ParkNoteDoneMsg", msg)
		}
		saveMsgs = append(saveMsgs, immediateParkFlowMessages(t, updateParkFlowApp(t, app, doneMsg))...)
	}
	return saveMsgs
}

func completeCrossProjectParkSaveThroughAppUpdate(t *testing.T, app *AppModel, saveMsgs []tea.Msg) {
	t.Helper()

	foundSaveComplete := false
	for _, msg := range saveMsgs {
		saveMsg, ok := msg.(SaveCompleteMsg)
		if !ok {
			continue
		}
		foundSaveComplete = true
		updateParkFlowApp(t, app, saveMsg)
		break
	}
	if !foundSaveComplete {
		t.Fatalf("expected SaveCompleteMsg, got %#v", saveMsgs)
	}
	if app.crossOutliner.loadedProjects != nil || !app.crossOutliner.crossLoading {
		t.Fatalf("save completion should start cross-project reload, loaded=%v loading=%v",
			app.crossOutliner.loadedProjects, app.crossOutliner.crossLoading)
	}

	loadedMsg, ok := app.crossOutliner.loadCrossProjectsCmd()().(crossProjectsLoadedMsg)
	if !ok {
		t.Fatal("loadCrossProjectsCmd should emit crossProjectsLoadedMsg")
	}
	updateParkFlowApp(t, app, loadedMsg)
}

func runCrossProjectParkFlowThroughAppUpdate(t *testing.T, app *AppModel, note string, clearExisting bool) {
	t.Helper()

	openCrossProjectParkInputThroughAppUpdate(t, app)
	if clearExisting {
		clearParkFlowNoteThroughAppUpdate(t, app)
	}
	typeParkFlowNoteThroughAppUpdate(t, app, note)
	completeCrossProjectParkSaveThroughAppUpdate(t, app, submitCrossProjectParkFlowThroughAppUpdate(t, app))
}

func TestCrossProjectOutlinerTParkWithNoteThroughAppUpdatePersistsAndRenders(t *testing.T) {
	fixture := newCrossProjectParkFlowFixture(t, nil)

	runCrossProjectParkFlowThroughAppUpdate(t, fixture.app, "resume beta", false)

	plain := ansi.Strip(fixture.outliner.renderOutlinerTreeContent(120))
	if !strings.Contains(plain, "resume beta") {
		t.Fatalf("cross-project rows should show parked note after reload, got:\n%s", plain)
	}

	reloadedA := loadBulkPickerTestProject(t, fixture.projectA)
	if reloadedA.GetTaskByID(fixture.taskA.ID).IsParked() {
		t.Fatal("project A task should not be parked")
	}
	reloadedB := loadBulkPickerTestProject(t, fixture.projectB)
	reloadedTaskB := reloadedB.GetTaskByID(fixture.taskB.ID)
	if reloadedTaskB == nil || !reloadedTaskB.IsParked() || reloadedTaskB.ParkedNote != "resume beta" {
		t.Fatalf("project B task parked state = %#v, want note %q", reloadedTaskB, "resume beta")
	}
}

func TestCrossProjectOutlinerTParkBlankNoteThroughAppUpdatePersistsAndRendersFallback(t *testing.T) {
	fixture := newCrossProjectParkFlowFixture(t, nil)

	runCrossProjectParkFlowThroughAppUpdate(t, fixture.app, "", false)

	reloadedA := loadBulkPickerTestProject(t, fixture.projectA)
	if reloadedA.GetTaskByID(fixture.taskA.ID).IsParked() {
		t.Fatal("project A task should not be parked")
	}
	reloadedB := loadBulkPickerTestProject(t, fixture.projectB)
	reloadedTaskB := reloadedB.GetTaskByID(fixture.taskB.ID)
	if reloadedTaskB == nil || !reloadedTaskB.IsParked() || reloadedTaskB.ParkedNote != "" {
		t.Fatalf("project B task parked state = %#v, want blank parked note", reloadedTaskB)
	}

	plain := ansi.Strip(fixture.outliner.renderOutlinerTreeContent(120))
	if !strings.Contains(plain, "parked") {
		t.Fatalf("cross-project rows should show fallback parked marker after reload, got:\n%s", plain)
	}
}

func TestCrossProjectOutlinerTUnparkThroughAppUpdateClearsParkedState(t *testing.T) {
	const existingNote = "resume beta"
	fixture := newCrossProjectParkFlowFixture(t, func(task *models.Task) {
		task.Park(existingNote)
	})

	runCrossProjectParkFlowThroughAppUpdate(t, fixture.app, "", true)

	reloadedB := loadBulkPickerTestProject(t, fixture.projectB)
	reloadedTaskB := reloadedB.GetTaskByID(fixture.taskB.ID)
	if reloadedTaskB == nil {
		t.Fatal("project B task was not reloaded")
	}
	if reloadedTaskB.IsParked() {
		t.Fatalf("project B task should be unparked, got %#v", reloadedTaskB)
	}
	if reloadedTaskB.ParkedNote != "" || reloadedTaskB.ParkedAt != nil {
		t.Fatalf("parked fields should be cleared, got note=%q at=%v", reloadedTaskB.ParkedNote, reloadedTaskB.ParkedAt)
	}

	plain := ansi.Strip(fixture.outliner.renderOutlinerTreeContent(120))
	if strings.Contains(plain, existingNote) || strings.Contains(plain, "parked") {
		t.Fatalf("cross-project rows should not show parked metadata after unpark, got:\n%s", plain)
	}
}

func TestCrossProjectOutlinerTPParkFlowUsesCursorTaskProjectAndReloads(t *testing.T) {
	app := newTestAppModel()
	taskA := newBulkPickerTestTask(t, 1, "alpha task")
	taskB := newBulkPickerTestTask(t, 1, "beta task")
	projectA := newBulkPickerTestProject(t, "Cross Park Alpha", taskA)
	projectB := newBulkPickerTestProject(t, "Cross Park Beta", taskB)
	app.config.Projects = []*models.Project{projectA, projectB}
	app.config.CurrentProject = projectA.ID
	app.currentProject = projectA

	crossOutliner := NewOutlinerModel(nil, app.config, app.configManager)
	crossOutliner.SetSize(120, 20)
	crossOutliner.loadedProjects = []*models.Project{projectA, projectB}
	crossOutliner.rows = []outlinerRow{
		{IsSectionHeader: true, SectionName: models.SectionInbox, ProjectID: projectA.ID},
		{task: taskA, ProjectID: projectA.ID},
		{IsSeparator: true},
		{IsSectionHeader: true, SectionName: models.SectionInbox, ProjectID: projectB.ID},
		{task: taskB, ProjectID: projectB.ID},
	}
	crossOutliner.cursor = 4
	crossOutliner.Dispatcher = keymap.NewDispatcher(crossOutliner.buildKeyMap())
	app.crossOutliner = crossOutliner
	app.views.Register(ViewCrossProjectOutliner, crossOutliner)
	app.viewStack = append(app.viewStack, ViewCrossProjectOutliner)

	if _, cmd, consumed := crossOutliner.Dispatcher.Dispatch(syntheticKey('t')); !consumed || cmd != nil {
		t.Fatalf("dispatch t consumed=%v cmd=%v, want consumed prefix without command", consumed, cmd)
	}
	_, cmd, consumed := crossOutliner.Dispatcher.Dispatch(syntheticKey('P'))
	if !consumed {
		t.Fatal("dispatch P under task prefix should be consumed")
	}
	assertShowParkInputCmd(t, cmd, taskB, projectB)

	showMsg := cmd().(ShowParkInputMsg)
	handled, _ := app.handleTaskMessages(showMsg)
	if !handled {
		t.Fatal("ShowParkInputMsg should be handled")
	}
	if app.currentView() != ViewParkInput {
		t.Fatalf("currentView = %v, want ViewParkInput", app.currentView())
	}

	parkInput := app.views.Get(ViewParkInput).(*ParkInputModel)
	parkInput.input.SetValue("resume beta")
	_, doneCmd := parkInput.Update(syntheticSpecialKey(tea.KeyEnter))
	doneMsg, ok := doneCmd().(ParkNoteDoneMsg)
	if !ok {
		t.Fatalf("done command emitted %T, want ParkNoteDoneMsg", doneCmd())
	}
	if doneMsg.Task != taskB || doneMsg.Project != projectB {
		t.Fatalf("ParkNoteDoneMsg refs = (%p, %p), want (%p, %p)", doneMsg.Task, doneMsg.Project, taskB, projectB)
	}

	handled, saveCmd := app.handleTaskMessages(doneMsg)
	if !handled {
		t.Fatal("ParkNoteDoneMsg should be handled")
	}
	saveMsgs := runBulkPickerBatchCmd(t, saveCmd)
	requireBulkPickerSaveComplete(t, saveMsgs)

	var refreshCmd tea.Cmd
	for _, msg := range saveMsgs {
		if saveMsg, ok := msg.(SaveCompleteMsg); ok {
			refreshCmd = app.handleSaveComplete(saveMsg)
			break
		}
	}
	if refreshCmd == nil {
		t.Fatal("expected save completion to return a refresh command")
	}
	if crossOutliner.loadedProjects != nil || !crossOutliner.crossLoading {
		t.Fatalf("save completion should start cross-project reload, loaded=%v loading=%v", crossOutliner.loadedProjects, crossOutliner.crossLoading)
	}

	loadedMsg, ok := crossOutliner.loadCrossProjectsCmd()().(crossProjectsLoadedMsg)
	if !ok {
		t.Fatal("loadCrossProjectsCmd should emit crossProjectsLoadedMsg")
	}
	crossOutliner.handleCrossProjectsLoaded(loadedMsg)
	plain := ansi.Strip(crossOutliner.renderOutlinerTreeContent(120))
	if !strings.Contains(plain, "resume beta") {
		t.Fatalf("cross-project rows should show parked note after reload, got:\n%s", plain)
	}

	reloadedA := loadBulkPickerTestProject(t, projectA)
	if reloadedA.GetTaskByID(taskA.ID).IsParked() {
		t.Fatal("project A task should not be parked")
	}
	reloadedB := loadBulkPickerTestProject(t, projectB)
	reloadedTaskB := reloadedB.GetTaskByID(taskB.ID)
	if reloadedTaskB == nil || !reloadedTaskB.IsParked() || reloadedTaskB.ParkedNote != "resume beta" {
		t.Fatalf("project B task parked state = %#v, want note %q", reloadedTaskB, "resume beta")
	}
}

func assertLastParkFlowUndoAction(t *testing.T, project *models.Project, want string) {
	t.Helper()

	metaPath := filepath.Join(project.Path, ".undo", "meta.json")
	content, err := os.ReadFile(metaPath)
	if err != nil {
		t.Fatalf("ReadFile(%q): %v", metaPath, err)
	}

	var meta struct {
		Entries []struct {
			Action string `json:"action"`
		} `json:"entries"`
	}
	if err := json.Unmarshal(content, &meta); err != nil {
		t.Fatalf("Unmarshal undo meta: %v", err)
	}
	if len(meta.Entries) == 0 {
		t.Fatal("expected at least one undo entry")
	}
	if got := meta.Entries[len(meta.Entries)-1].Action; got != want {
		t.Fatalf("undo action = %q, want %q", got, want)
	}
}
