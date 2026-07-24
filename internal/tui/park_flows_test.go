package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/tui/keymap"
)

func TestParkInputExplicitParkPersistsNote(t *testing.T) {
	task := newBulkPickerTestTask(t, 1, "resume later")
	env := newAppBulkPickerEnv(t, []*models.Task{task})
	app := env.appModel()
	project := env.projects[0]

	handled, _ := app.handleTaskMessages(ShowParkInputMsg{Task: project.Tasks[0], Project: project})
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
	assertParkFlowSaved(t, app, saveCmd, "Parked")

	reloadedTask := env.reload(t, 0).GetTaskByID(task.ID)
	if reloadedTask == nil {
		t.Fatal("parked task was not reloaded")
	}
	if !reloadedTask.IsParked() {
		t.Fatal("task should be parked")
	}
	if reloadedTask.ParkedNote != "blocked on API review" {
		t.Fatalf("ParkedNote = %q, want %q", reloadedTask.ParkedNote, "blocked on API review")
	}

	// Undo through the workspace ledger reverts the park.
	action, err := env.ledger.Undo(storage.GlobalScope())
	if err != nil {
		t.Fatalf("ledger undo: %v", err)
	}
	if action != "Park Task" {
		t.Fatalf("undo action = %q, want %q", action, "Park Task")
	}
	if env.reload(t, 0).GetTaskByID(task.ID).IsParked() {
		t.Fatal("undo should have unparked the task")
	}
}

func TestParkInputEmptyEnterOnParkedTaskUnparks(t *testing.T) {
	task := newBulkPickerTestTask(t, 1, "resume now")
	task.Park("next step")
	env := newAppBulkPickerEnv(t, []*models.Task{task})
	app := env.appModel()
	project := env.projects[0]

	handled, _ := app.handleTaskMessages(ShowParkInputMsg{Task: project.Tasks[0], Project: project})
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
	assertParkFlowSaved(t, app, saveCmd, "Unparked")

	reloadedTask := env.reload(t, 0).GetTaskByID(task.ID)
	if reloadedTask == nil {
		t.Fatal("unparked task was not reloaded")
	}
	if reloadedTask.IsParked() {
		t.Fatal("task should be unparked")
	}
	if reloadedTask.ParkedNote != "" || reloadedTask.ParkedAt != nil {
		t.Fatalf("parked fields should be cleared, got note=%q at=%v", reloadedTask.ParkedNote, reloadedTask.ParkedAt)
	}

	action, err := env.ledger.Undo(storage.GlobalScope())
	if err != nil {
		t.Fatalf("ledger undo: %v", err)
	}
	if action != "Unpark Task" {
		t.Fatalf("undo action = %q, want %q", action, "Unpark Task")
	}
	if !env.reload(t, 0).GetTaskByID(task.ID).IsParked() {
		t.Fatal("undo should have restored the parked state")
	}
}

func TestParkInputRejectsCompletedTaskNewPark(t *testing.T) {
	task := newBulkPickerTestTask(t, 1, "done should stay done")
	task.WorkflowState = models.StateDone
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
	task := newBulkPickerTestTask(t, 1, "completed legacy parked")
	task.Park("legacy metadata")
	task.WorkflowState = models.StateDone
	env := newAppBulkPickerEnv(t, []*models.Task{task})
	app := env.appModel()
	project := env.projects[0]

	handled, _ := app.handleTaskMessages(ShowParkInputMsg{Task: project.Tasks[0], Project: project})
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
	assertParkFlowSaved(t, app, saveCmd, "Unparked")

	reloadedTask := env.reload(t, 0).GetTaskByID(task.ID)
	if reloadedTask == nil {
		t.Fatal("unparked completed task was not reloaded")
	}
	if reloadedTask.IsParked() {
		t.Fatal("legacy parked metadata should be cleared from completed task")
	}
	if reloadedTask.State() != models.StateDone {
		t.Fatalf("State = %s, want done", reloadedTask.State())
	}

	action, err := env.ledger.Undo(storage.GlobalScope())
	if err != nil {
		t.Fatalf("ledger undo: %v", err)
	}
	if action != "Unpark Task" {
		t.Fatalf("undo action = %q, want %q", action, "Unpark Task")
	}
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

	detail := NewTaskDetailModel(task, project, nil)
	_, cmd := detail.handleKeyMsg(syntheticKey('P'))
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

	taskA := newBulkPickerTestTask(t, 1, "alpha task")
	taskB := newBulkPickerTestTask(t, 1, "beta task")
	if mutateTaskB != nil {
		mutateTaskB(taskB)
	}
	env := newAppBulkPickerEnv(t, []*models.Task{taskA}, []*models.Task{taskB})
	app := env.appModel()
	projectA := env.projects[0]
	projectB := env.projects[1]
	taskA = projectA.Tasks[0]
	taskB = projectB.Tasks[0]
	setTestCurrentProject(app, projectA.ID)
	app.currentProject = projectA

	crossOutliner := NewOutlinerModel(nil, app.configManager, app.runtime)
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

	// Park now saves synchronously inside handleParkNoteDone, so there is no
	// async SaveCompleteMsg. The returned batch drives the cross-project reload
	// (ViewDidAppearMsg + crossProjectsLoadedMsg). Feed every emitted message
	// back through Update so the outliner rebuilds its rows from reloaded disk
	// state.
	if len(saveMsgs) == 0 {
		t.Fatal("park save should emit reload messages")
	}
	sawReload := false
	queue := append([]tea.Msg(nil), saveMsgs...)
	for i := 0; len(queue) > 0; i++ {
		if i > 200 {
			t.Fatal("park save reload did not settle")
		}
		msg := queue[0]
		queue = queue[1:]
		if _, ok := msg.(crossProjectsLoadedMsg); ok {
			sawReload = true
		}
		queue = append(queue, immediateParkFlowMessages(t, updateParkFlowApp(t, app, msg))...)
	}
	if !sawReload {
		t.Fatalf("park save should trigger cross-project reload, got %#v", saveMsgs)
	}
	if app.crossOutliner.crossLoading {
		t.Fatal("cross-project reload should have completed")
	}
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
	taskA := newBulkPickerTestTask(t, 1, "alpha task")
	taskB := newBulkPickerTestTask(t, 1, "beta task")
	env := newAppBulkPickerEnv(t, []*models.Task{taskA}, []*models.Task{taskB})
	app := env.appModel()
	projectA := env.projects[0]
	projectB := env.projects[1]
	taskA = projectA.Tasks[0]
	taskB = projectB.Tasks[0]
	setTestCurrentProject(app, projectA.ID)
	app.currentProject = projectA

	crossOutliner := NewOutlinerModel(nil, app.configManager, app.runtime)
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
	// Park saves synchronously, so the reload is already in flight and the
	// success status is set before the returned batch runs.
	if crossOutliner.loadedProjects != nil || !crossOutliner.crossLoading {
		t.Fatalf("park save should start cross-project reload, loaded=%v loading=%v", crossOutliner.loadedProjects, crossOutliner.crossLoading)
	}
	if !strings.HasPrefix(app.statusMessage, "Parked") {
		t.Fatalf("status message = %q, want a Parked message", app.statusMessage)
	}
	runBulkPickerBatchCmd(t, saveCmd)

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

// assertParkFlowSaved verifies the synchronous park/unpark save succeeded: the
// success status is set before the returned batch runs, and draining the batch
// (popView + refresh cmds) does not panic.
func assertParkFlowSaved(t *testing.T, app *AppModel, saveCmd tea.Cmd, wantVerb string) {
	t.Helper()
	if !strings.HasPrefix(app.statusMessage, wantVerb) {
		t.Fatalf("status message = %q, want prefix %q", app.statusMessage, wantVerb)
	}
	if saveCmd != nil {
		runBulkPickerBatchCmd(t, saveCmd)
	}
}
