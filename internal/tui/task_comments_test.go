package tui

import (
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
)

func TestTaskCommentsSaveCommentMutatesSynchronously(t *testing.T) {
	project, err := models.NewProject("project-1", "Comments Project", t.TempDir())
	if err != nil {
		t.Fatalf("NewProject: %v", err)
	}

	task, err := models.NewGenericTask("task-1", 1, "Test task")
	if err != nil {
		t.Fatalf("NewTask: %v", err)
	}
	project.Tasks = []*models.Task{task}

	model := NewTaskCommentsModel(task, project)
	model.coordinator = newTestSecondaryCoordinators(project).Subview
	model.addMode = true
	model.textInput.SetValue("New comment")

	cmd := model.saveComment()
	if cmd == nil {
		t.Fatal("expected non-nil command from saveComment")
	}

	// Verify state was mutated synchronously before cmd() runs
	if model.addMode {
		t.Fatal("expected addMode to be false after saveComment()")
	}
	if len(model.task.Comments) != 1 {
		t.Fatalf("expected 1 comment, got %d", len(model.task.Comments))
	}
	if model.task.Comments[0].Text != "New comment" {
		t.Fatalf("expected comment text 'New comment', got %q", model.task.Comments[0].Text)
	}
	if model.cursor != 0 {
		t.Fatalf("expected cursor at 0, got %d", model.cursor)
	}
	if model.err != nil {
		t.Fatalf("expected no error, got %v", model.err)
	}
}

func TestTaskCommentsSaveCommentEmptyText(t *testing.T) {
	project, err := models.NewProject("project-1", "Comments Project", t.TempDir())
	if err != nil {
		t.Fatalf("NewProject: %v", err)
	}

	task, err := models.NewGenericTask("task-1", 1, "Test task")
	if err != nil {
		t.Fatalf("NewTask: %v", err)
	}
	project.Tasks = []*models.Task{task}

	model := NewTaskCommentsModel(task, project)
	model.coordinator = newTestSecondaryCoordinators(project).Subview
	model.addMode = true
	model.textInput.SetValue("   ")

	cmd := model.saveComment()
	if cmd != nil {
		t.Fatal("expected nil command for empty comment")
	}
	if model.err == nil {
		t.Fatal("expected error for empty comment text")
	}
}

func TestTaskCommentsDeleteMutatesSynchronously(t *testing.T) {
	project, err := models.NewProject("project-1", "Comments Project", t.TempDir())
	if err != nil {
		t.Fatalf("NewProject: %v", err)
	}

	task, err := models.NewGenericTask("task-1", 1, "Test task")
	if err != nil {
		t.Fatalf("NewTask: %v", err)
	}
	task.Comments = []*models.Comment{
		{Text: "comment 1", CreatedAt: time.Now()},
		{Text: "comment 2", CreatedAt: time.Now()},
	}
	project.Tasks = []*models.Task{task}

	model := NewTaskCommentsModel(task, project)
	model.coordinator = newTestSecondaryCoordinators(project).Subview
	model.cursor = 1

	cmd := model.executeDelete()
	if cmd == nil {
		t.Fatal("expected non-nil command from executeDelete")
	}

	// Verify state was mutated synchronously before cmd() runs
	if len(model.task.Comments) != 1 {
		t.Fatalf("expected 1 comment after delete, got %d", len(model.task.Comments))
	}
	if model.task.Comments[0].Text != "comment 1" {
		t.Fatalf("expected remaining comment to be 'comment 1', got %q", model.task.Comments[0].Text)
	}
	if model.cursor != 0 {
		t.Fatalf("expected cursor at 0 after deleting last comment, got %d", model.cursor)
	}
}

func TestTaskCommentsSaveEditMutatesSynchronously(t *testing.T) {
	project, err := models.NewProject("project-1", "Comments Project", t.TempDir())
	if err != nil {
		t.Fatalf("NewProject: %v", err)
	}

	task, err := models.NewGenericTask("task-1", 1, "Test task")
	if err != nil {
		t.Fatalf("NewTask: %v", err)
	}
	created := time.Date(2025, 1, 15, 10, 30, 0, 0, time.Local)
	task.Comments = []*models.Comment{
		{Text: "original text", CreatedAt: created},
	}
	project.Tasks = []*models.Task{task}

	model := NewTaskCommentsModel(task, project)
	model.coordinator = newTestSecondaryCoordinators(project).Subview
	model.editMode = true
	model.editIndex = 0
	model.editTextInput.SetValue("edited text")

	cmd := model.saveEditedComment()
	if cmd == nil {
		t.Fatal("expected non-nil command from saveEditedComment")
	}

	// Verify state was mutated synchronously before cmd() runs
	if model.editMode {
		t.Fatal("expected editMode to be false after saveEditedComment()")
	}
	if model.task.Comments[0].Text != "edited text" {
		t.Fatalf("expected comment text 'edited text', got %q", model.task.Comments[0].Text)
	}
	// Timestamp should be preserved
	if !model.task.Comments[0].CreatedAt.Equal(created) {
		t.Fatalf("expected timestamp to be preserved, got %v", model.task.Comments[0].CreatedAt)
	}
	if model.err != nil {
		t.Fatalf("expected no error, got %v", model.err)
	}
}

func TestConvertedDecisionCommentIsFullyEditableThroughComments(t *testing.T) {
	project, err := models.NewProject("project-1", "Comments Project", t.TempDir())
	if err != nil {
		t.Fatalf("NewProject: %v", err)
	}
	task, err := models.NewGenericTask("task-1", 1, "Migrated narrative")
	if err != nil {
		t.Fatalf("NewTask: %v", err)
	}
	created := time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC)
	task.Comments = []*models.Comment{{
		Text:      "Decision: Use SQLite\nRationale: Local-first durability",
		CreatedAt: created,
	}}
	project.Tasks = []*models.Task{task}

	model := NewTaskCommentsModel(task, project)
	model.coordinator = newTestSecondaryCoordinators(project).Subview
	if cmd := model.startEditComment(); cmd == nil {
		t.Fatal("converted comment did not enter edit mode")
	}
	if got := model.editTextInput.Value(); got != task.Comments[0].Text {
		t.Fatalf("edit input = %q, want complete converted text %q", got, task.Comments[0].Text)
	}

	want := "Decision: Use SQLite\nRationale: Embedded deployment"
	model.editTextInput.SetValue(want)
	cmd := model.saveEditedComment()
	if cmd == nil {
		t.Fatal("editing converted comment returned no commit command")
	}
	if _, ok := cmd().(CommentEditedMsg); !ok {
		t.Fatalf("editing converted comment returned an unexpected result")
	}
	comment := project.Tasks[0].Comments[0]
	if comment.Text != want {
		t.Fatalf("edited comment text = %q, want %q", comment.Text, want)
	}
	if !comment.CreatedAt.Equal(created) {
		t.Fatalf("edited comment timestamp = %v, want %v", comment.CreatedAt, created)
	}
}

func TestTaskCommentsRenderEditModeOOB(t *testing.T) {
	project, err := models.NewProject("project-1", "Comments Project", t.TempDir())
	if err != nil {
		t.Fatalf("NewProject: %v", err)
	}

	task, err := models.NewGenericTask("task-1", 1, "Test task")
	if err != nil {
		t.Fatalf("NewTask: %v", err)
	}
	task.Comments = []*models.Comment{
		{Text: "only comment", CreatedAt: time.Now()},
	}
	project.Tasks = []*models.Task{task}

	model := NewTaskCommentsModel(task, project)
	model.coordinator = newTestSecondaryCoordinators(project).Subview
	model.editMode = true
	model.editIndex = 0

	// Simulate external deletion of all comments
	task.Comments = nil

	// View() must not panic when editIndex is out of bounds
	view := model.View()
	if view.Content == "" {
		t.Fatal("expected non-empty view output")
	}
}

func TestTaskCommentsEditExternalEditorFromEditModeOOB(t *testing.T) {
	project, err := models.NewProject("project-1", "Comments Project", t.TempDir())
	if err != nil {
		t.Fatalf("NewProject: %v", err)
	}

	task, err := models.NewGenericTask("task-1", 1, "Test task")
	if err != nil {
		t.Fatalf("NewTask: %v", err)
	}
	task.Comments = []*models.Comment{
		{Text: "only comment", CreatedAt: time.Now()},
	}
	project.Tasks = []*models.Task{task}

	model := NewTaskCommentsModel(task, project)
	model.coordinator = newTestSecondaryCoordinators(project).Subview
	model.editMode = true
	model.editIndex = 0

	// Simulate external deletion of all comments
	task.Comments = nil

	// Must not panic; should cancel edit mode and return an error cmd
	cmd := model.editCommentInExternalEditorFromEditMode()
	if cmd == nil {
		t.Fatal("expected non-nil error command when editIndex is out of bounds")
	}
	if model.editMode {
		t.Fatal("expected editMode to be canceled when comment was deleted externally")
	}
}

func TestTaskCommentsSaveEditOOBAfterExternalDeletion(t *testing.T) {
	project, err := models.NewProject("project-1", "Comments Project", t.TempDir())
	if err != nil {
		t.Fatalf("NewProject: %v", err)
	}

	task, err := models.NewGenericTask("task-1", 1, "Test task")
	if err != nil {
		t.Fatalf("NewTask: %v", err)
	}
	task.Comments = []*models.Comment{
		{Text: "original", CreatedAt: time.Now()},
	}
	project.Tasks = []*models.Task{task}

	model := NewTaskCommentsModel(task, project)
	model.coordinator = newTestSecondaryCoordinators(project).Subview
	model.editMode = true
	model.editIndex = 0
	model.editTextInput.SetValue("edited text")

	// Simulate external deletion of all comments
	task.Comments = nil

	// saveEditedComment should detect OOB and return error cmd
	cmd := model.saveEditedComment()
	if cmd == nil {
		t.Fatal("expected non-nil error command when editIndex is out of bounds")
	}
	if model.editMode {
		t.Fatal("expected editMode to be canceled when comment was deleted externally")
	}
}

func TestTaskCommentsHasUnsavedEditChangesIgnoresSurroundingWhitespace(t *testing.T) {
	project, err := models.NewProject("project-1", "Comments Project", t.TempDir())
	if err != nil {
		t.Fatalf("NewProject: %v", err)
	}

	task, err := models.NewGenericTask("task-1", 1, "Test task")
	if err != nil {
		t.Fatalf("NewTask: %v", err)
	}
	// Stored comment text has trailing whitespace (e.g., from prior editor save).
	task.Comments = []*models.Comment{
		{Text: "hello world\n", CreatedAt: time.Now()},
	}
	project.Tasks = []*models.Task{task}

	model := NewTaskCommentsModel(task, project)
	model.coordinator = newTestSecondaryCoordinators(project).Subview
	model.editMode = true
	model.editIndex = 0
	// User opens edit; textarea is populated with the stored value but trims on input.
	model.editTextInput.SetValue("hello world")

	if model.hasUnsavedEditChanges() {
		t.Fatal("expected no unsaved changes when input differs from stored text only by surrounding whitespace")
	}
}

func TestTaskCommentsLeanPanelHasNoResolvedControl(t *testing.T) {
	project, err := models.NewProject("project-1", "Comments Project", t.TempDir())
	if err != nil {
		t.Fatalf("NewProject: %v", err)
	}

	task, err := models.NewGenericTask("task-1", 1, "Test task")
	if err != nil {
		t.Fatalf("NewTask: %v", err)
	}
	task.Comments = []*models.Comment{
		{Text: "comment", CreatedAt: time.Now()},
	}
	project.Tasks = []*models.Task{task}

	model := NewTaskCommentsModel(task, project)
	model.coordinator = newTestSecondaryCoordinators(project).Subview
	if cmd := model.handleNormalKey("r"); cmd != nil {
		t.Fatalf("retired resolved key emitted command %#v", cmd())
	}
	for _, key := range model.HandledKeys() {
		if key == "r" {
			t.Fatal("retired resolved key remains in comments key map")
		}
	}
}

// mountCommentsPanel opens the panel over a live shell on a task carrying two
// comments, and returns the driver, the panel, and a key press helper.
func mountCommentsPanel(t *testing.T) (*UITestDriver, *TaskCommentsModel, func(string)) {
	t.Helper()
	driver, workspace, err := NewFixtureDriver(FixtureFresh, 100, 30)
	if err != nil {
		t.Fatalf("NewFixtureDriver: %v", err)
	}
	t.Cleanup(func() { _ = workspace.Cleanup() })

	project, err := models.NewProject("project-1", "Comments Project", t.TempDir())
	if err != nil {
		t.Fatalf("NewProject: %v", err)
	}
	task, err := models.NewGenericTask("task-1", 1, "Test task")
	if err != nil {
		t.Fatalf("NewTask: %v", err)
	}
	task.Comments = []*models.Comment{
		{Text: "first comment", CreatedAt: time.Now()},
		{Text: "second comment", CreatedAt: time.Now()},
	}
	project.Tasks = []*models.Task{task}

	driver.dispatch(ShowTaskCommentsMsg{Task: task, Project: project})
	panel := driver.App().activeCommentsPanel()
	if panel == nil {
		t.Fatal("ShowTaskCommentsMsg did not mount the Comments panel")
	}
	if panel.dispatch == nil {
		t.Fatal("the mounted panel holds no dispatch seam, so every routed key resolves nothing")
	}

	press := func(key string) {
		t.Helper()
		if err := driver.SendKey(key); err != nil {
			t.Fatalf("SendKey(%q): %v", key, err)
		}
	}
	return driver, panel, press
}

// TestCommentsPanelKeysReachTheirOperations presses routed keys through a live
// shell. The Comments scenarios only add and save a comment, so a dispatch seam
// the shell forgot to hand down would leave the acting keys silently doing
// nothing with every other check still green.
func TestCommentsPanelKeysReachTheirOperations(t *testing.T) {
	driver, panel, press := mountCommentsPanel(t)

	press("enter")
	if !panel.expandedView {
		t.Fatal("Enter did not reach tui.comment.expand")
	}
	press("esc")

	press("space")
	if driver.App().contextActionSheet == nil {
		t.Fatal("Space did not reach tui.comment.actions")
	}
	press("esc")
	if driver.App().contextActionSheet != nil {
		t.Fatal("Esc left the comment action sheet open")
	}

	press("e")
	if !panel.editMode {
		t.Fatal("e did not reach tui.comment.edit")
	}
	press("esc")

	var captured string
	withFakeClipboard(t, nil, &captured)
	press("y")
	if captured != "first comment" {
		t.Fatalf("y copied %q, want the comment under the cursor", captured)
	}
	press("Y")
	if captured != "1. first comment\n2. second comment" {
		t.Fatalf("Y copied %q, want every comment as a numbered list", captured)
	}

	press("a")
	if !panel.addMode {
		t.Fatal("a did not reach tui.comment.add")
	}
}

// TestCommentsRetiredKeysAreSilent holds the keys the panel gave up. Each one
// means something else app-wide, so the panel must answer none of them rather
// than quietly keeping its old meaning.
func TestCommentsRetiredKeysAreSilent(t *testing.T) {
	driver, panel, press := mountCommentsPanel(t)

	for _, key := range []string{"n", "d", "x"} {
		press(key)
		if panel.confirm.IsActive() {
			t.Fatalf("retired key %q raised confirm %q", key, panel.confirm.Action())
		}
		if panel.addMode {
			t.Fatalf("retired key %q entered add mode", key)
		}
		if driver.App().contextActionSheet != nil {
			t.Fatalf("retired key %q opened the action sheet", key)
		}
	}
}

// TestCommentsUndoKeysAskTheShellForTheWorkspaceLedger pins the two keys the
// panel took back from silence. Adding, editing, and removing a comment are
// ledger transactions, so the panel that made them answers the app-wide undo;
// the core undo descriptors answer only to the Outliner, so the press asks the
// shell rather than dispatching through the executor.
func TestCommentsUndoKeysAskTheShellForTheWorkspaceLedger(t *testing.T) {
	driver, _, press := mountCommentsPanel(t)

	press("u")
	if driver.pendingCmd == nil {
		t.Fatal("u emitted no workspace undo request")
	}
	if msg, ok := driver.pendingCmd().(WorkspaceUndoMsg); !ok || msg.Redo {
		t.Fatalf("u emitted %#v, want an undo request", driver.pendingCmd())
	}

	press("ctrl+r")
	if driver.pendingCmd == nil {
		t.Fatal("Ctrl+R emitted no workspace redo request")
	}
	if msg, ok := driver.pendingCmd().(WorkspaceUndoMsg); !ok || !msg.Redo {
		t.Fatalf("Ctrl+R emitted %#v, want a redo request", driver.pendingCmd())
	}
}

// TestCommentsSubStatesKeepUAsTypedText holds u inside every sub-state that owns
// the key stream. The panel answers it only in browse, so a comment carrying the
// letter can never reverse the transaction the user is in the middle of writing,
// and neither the expanded reader nor the delete confirmation turns a stray
// press into a ledger restore.
func TestCommentsSubStatesKeepUAsTypedText(t *testing.T) {
	driver, panel, press := mountCommentsPanel(t)
	refuseLedger := func(state string) {
		t.Helper()
		if driver.pendingCmd == nil {
			return
		}
		if _, ok := driver.pendingCmd().(WorkspaceUndoMsg); ok {
			t.Fatalf("u reached the workspace ledger while %s owned the key stream", state)
		}
	}

	press("a")
	if !panel.addMode {
		t.Fatal("a did not enter add mode")
	}
	press("u")
	refuseLedger("the add composer")
	if got := panel.textInput.Value(); got != "u" {
		t.Fatalf("add-mode input = %q, want %q", got, "u")
	}
	press("esc")
	press("y")

	press("e")
	if !panel.editMode {
		t.Fatal("e did not enter edit mode")
	}
	press("u")
	refuseLedger("the edit composer")
	if got := panel.editTextInput.Value(); !strings.HasSuffix(got, "u") {
		t.Fatalf("edit-mode input = %q, want the typed u appended", got)
	}
	press("esc")
	press("y")

	press("enter")
	if !panel.expandedView {
		t.Fatal("Enter did not open the expanded comment")
	}
	press("u")
	refuseLedger("the expanded comment")
	press("esc")

	press("space")
	press("enter")
	if !panel.confirm.IsActive() {
		t.Fatal("the sheet's Delete row raised no confirmation")
	}
	press("u")
	refuseLedger("the delete confirmation")
}

// TestCommentsPanelWorkspaceUndoRedoRefreshesAndNamesTheLedgerEntry walks the
// promise the panel used to break. A comment mutation commits through
// commitProject with an UndoAction and installs a live `u undo` toast, so the
// panel that made it has to answer both ledger keys itself: the open list
// rebuilds rather than keeping a reversed row on screen, and the status strip
// names the transaction that moved rather than restoring in silence.
func TestCommentsPanelWorkspaceUndoRedoRefreshesAndNamesTheLedgerEntry(t *testing.T) {
	driver, press, typeText := mountLiveCommentsPanel(t)
	comments := func() []string {
		t.Helper()
		panel := driver.App().activeCommentsPanel()
		if panel == nil {
			t.Fatal("the Comments panel left the route")
		}
		texts := make([]string, 0, len(panel.task.Comments))
		for _, comment := range panel.task.Comments {
			texts = append(texts, comment.Text)
		}
		return texts
	}

	const added = "checked the redirect handler"
	before := len(comments())
	press("a")
	typeText(added)
	press("ctrl+s")
	driver.Wait(defaultWaitTimeout)
	if got := comments(); len(got) != before+1 || got[len(got)-1] != added {
		t.Fatalf("after the add the panel lists %v, want %q appended", got, added)
	}
	if !strings.Contains(driver.App().statusMessage, "u undo") {
		t.Fatalf("the add left status %q, want the recovery toast that offers u", driver.App().statusMessage)
	}

	press("u")
	driver.Wait(defaultWaitTimeout)
	if driver.App().currentView() != ViewTaskComments {
		t.Fatalf("u left the panel for %s", driver.App().currentView())
	}
	if got := comments(); len(got) != before || slices.Contains(got, added) {
		t.Fatalf("after u the panel lists %v, want the added comment gone", got)
	}
	if got := driver.App().statusMessage; !strings.Contains(got, "Undid: Add Comment") {
		t.Fatalf("u left status %q, want it to name the restored transaction", got)
	}

	press("ctrl+r")
	driver.Wait(defaultWaitTimeout)
	if driver.App().currentView() != ViewTaskComments {
		t.Fatalf("Ctrl+R left the panel for %s", driver.App().currentView())
	}
	if got := comments(); len(got) != before+1 || !slices.Contains(got, added) {
		t.Fatalf("after Ctrl+R the panel lists %v, want the comment back", got)
	}
	if got := driver.App().statusMessage; !strings.Contains(got, "Redo: Add Comment") {
		t.Fatalf("Ctrl+R left status %q, want it to name the replayed transaction", got)
	}

	if !strings.Contains(driver.Render(), added) {
		t.Fatal("the replayed comment is absent from the rendered list")
	}
}

// mountLiveCommentsPanel opens the panel the way a user does, over a workspace
// whose commits reach the durable ledger. mountCommentsPanel's task is built in
// memory beside the fixture, so a mutation there records nothing to undo.
func mountLiveCommentsPanel(t *testing.T) (*UITestDriver, func(string), func(string)) {
	t.Helper()
	driver, workspace, err := NewFixtureDriver(FixtureRich, 100, 30)
	if err != nil {
		t.Fatalf("NewFixtureDriver: %v", err)
	}
	t.Cleanup(func() { _ = workspace.Cleanup() })

	press := func(key string) {
		t.Helper()
		if err := driver.SendKey(key); err != nil {
			t.Fatalf("SendKey(%q): %v", key, err)
		}
		driver.drain()
	}
	typeText := func(text string) {
		t.Helper()
		for _, r := range text {
			driver.dispatch(tea.KeyPressMsg{Code: r, Text: string(r)})
			driver.drain()
		}
	}

	press("j")
	press("j")
	press("space")
	typeText("comments")
	press("enter")
	if driver.App().activeCommentsPanel() == nil {
		t.Fatalf("the task action sheet did not open the Comments panel; view = %s", driver.App().currentView())
	}
	return driver, press, typeText
}

// TestCommentsSpaceSheetReachesDelete walks the whole new path for Delete, which
// is the retirement this session is really about: neither d nor x removes a
// comment now, so the Space sheet has to, and the confirmation behind it has to
// still commit the removal.
func TestCommentsSpaceSheetReachesDelete(t *testing.T) {
	driver, panel, press := mountCommentsPanel(t)

	press("space")
	sheet := driver.App().contextActionSheet
	if sheet == nil {
		t.Fatal("Space did not open the comment action sheet")
	}
	want := []OperationID{operationCommentDelete}
	if got := sheet.operationIDs(); !reflect.DeepEqual(got, want) {
		t.Fatalf("comment sheet lists %v, want exactly %v", got, want)
	}

	press("enter")
	if driver.App().contextActionSheet != nil {
		t.Fatal("choosing Delete left the sheet open over the confirmation")
	}
	if !panel.confirm.IsActive() || panel.confirm.Action() != "delete" {
		t.Fatalf("the sheet's Delete row raised confirm %q, want the delete prompt", panel.confirm.Action())
	}

	press("y")
	if len(panel.task.Comments) != 1 || panel.task.Comments[0].Text != "second comment" {
		t.Fatalf("confirming the sheet's Delete left %d comment(s), want the first one removed", len(panel.task.Comments))
	}
}

// TestCommentsHelpKeyRaisesTheCommentsGrammar holds `?` to the surface it was
// pressed on. The core help descriptor hardcodes the Outliner, so dispatching it
// through the executor would answer a Comments press with the Outliner's
// grammar; the panel raises its own instead, and the press stays a literal
// character in every sub-state that owns the key stream.
func TestCommentsHelpKeyRaisesTheCommentsGrammar(t *testing.T) {
	driver, panel, sendKey := mountCommentsPanel(t)
	// Help arrives as a command rather than a synchronous mutation, so every press
	// drains the way the scenario runner and the live program do.
	press := func(key string) {
		t.Helper()
		sendKey(key)
		driver.drain()
	}

	press("?")
	if got := driver.App().currentView(); got != ViewHelp {
		t.Fatalf("? left the panel on %v, want the Help view", got)
	}
	help, ok := driver.App().views.Get(ViewHelp).(*HelpViewModel)
	if !ok {
		t.Fatalf("the Help view is %T, want *HelpViewModel", driver.App().views.Get(ViewHelp))
	}
	if help.viewName != "Comments" || help.sourceView != ViewTaskComments {
		t.Fatalf("help header = %s/%q, want ViewTaskComments/\"Comments\"", help.sourceView, help.viewName)
	}

	documented := make(map[string]string, len(help.items))
	for _, item := range help.items {
		documented[item.Key] = item.Description
	}
	if documented["enter"] != "Expand comment" {
		t.Fatalf("Comments help describes Enter as %q, want the panel's own grammar: %+v", documented["enter"], help.items)
	}
	if documented["space"] != "Comment actions" {
		t.Fatalf("Comments help omits the Space sheet: %+v", help.items)
	}
	// `?` documents itself only because the keymap names it: the core descriptor
	// is inapplicable off the Outliner, so a registry projection would drop it.
	if documented["?"] != "Help" {
		t.Fatalf("Comments help omits ? itself: %+v", help.items)
	}
	for _, outlinerOnly := range []string{"tab", "T", "ctrl+p"} {
		if description, listed := documented[outlinerOnly]; listed {
			t.Fatalf("Comments help leaked the Outliner key %q (%q)", outlinerOnly, description)
		}
	}
	press("?")
	if got := driver.App().currentView(); got != ViewTaskComments {
		t.Fatalf("closing help landed on %v, want the panel it came from", got)
	}

	// Every sub-state owns its whole key stream, so help is a browse-only press.
	press("a")
	press("?")
	if got := driver.App().currentView(); got != ViewTaskComments {
		t.Fatalf("? in add mode left the panel for %v", got)
	}
	if got := panel.textInput.Value(); got != "?" {
		t.Fatalf("add-mode buffer after ? = %q, want a literal %q", got, "?")
	}
	press("esc")
	press("y")
	if panel.addMode {
		t.Fatal("discarding the add buffer left add mode active")
	}

	press("e")
	press("?")
	if got := driver.App().currentView(); got != ViewTaskComments {
		t.Fatalf("? in edit mode left the panel for %v", got)
	}
	if got := panel.editTextInput.Value(); !strings.Contains(got, "?") {
		t.Fatalf("edit-mode buffer after ? = %q, want a literal %q in it", got, "?")
	}
	press("esc")
	press("y")

	press("enter")
	if !panel.expandedView {
		t.Fatal("Enter did not expand the comment under the cursor")
	}
	press("?")
	if got := driver.App().currentView(); got != ViewTaskComments {
		t.Fatalf("? in the expanded comment left the panel for %v", got)
	}
	press("esc")

	panel.confirm.Show("delete", "Delete comment [0]?")
	press("?")
	if got := driver.App().currentView(); got != ViewTaskComments {
		t.Fatalf("? under the delete confirmation left the panel for %v", got)
	}
	if !panel.confirm.IsActive() {
		t.Fatal("? answered the delete confirmation instead of being ignored")
	}
	press("n")
}

// TestCommentsPanelUnboundKeysAreSilent holds the keys the Comments grammar
// never declared. They must stay quiet rather than falling through to some other
// surface's meaning.
func TestCommentsPanelUnboundKeysAreSilent(t *testing.T) {
	driver, panel, press := mountCommentsPanel(t)

	for _, key := range []string{"u", "o", "p", "%"} {
		press(key)
		if panel.confirm.IsActive() {
			t.Fatalf("unbound key %q raised confirm %q", key, panel.confirm.Action())
		}
		if panel.addMode || panel.editMode || panel.expandedView {
			t.Fatalf("unbound key %q changed the panel mode", key)
		}
		if driver.App().contextActionSheet != nil {
			t.Fatalf("unbound key %q opened an action sheet", key)
		}
		if got := driver.App().currentView(); got != ViewTaskComments {
			t.Fatalf("unbound key %q left the panel for %v", key, got)
		}
	}
}
