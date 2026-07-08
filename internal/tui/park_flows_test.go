package tui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	tea "charm.land/bubbletea/v2"

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
