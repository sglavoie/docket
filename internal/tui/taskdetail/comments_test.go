package taskdetail

import (
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
)

// commentsFixture builds a task carrying the given comments inside a real
// project directory, which is what the seeded application service commits to.
func commentsFixture(t *testing.T, texts ...string) (*models.Project, *models.Task) {
	t.Helper()
	project, err := models.NewProject("project-1", "Comments Project", t.TempDir())
	if err != nil {
		t.Fatalf("NewProject: %v", err)
	}
	task, err := models.NewGenericTask("task-1", 1, "Test task")
	if err != nil {
		t.Fatalf("NewTask: %v", err)
	}
	for _, text := range texts {
		task.AddComment(text)
	}
	project.Tasks = []*models.Task{task}
	return project, task
}

// settle runs one panel command and feeds whatever it resolved to back through
// Update, the way the shell's delegation does. Every commit-aware transition
// happens there, so a test that only inspected the command would be asserting
// half the behavior.
func settle(t *testing.T, model *CommentsModel, cmd tea.Cmd) tea.Msg {
	t.Helper()
	if cmd == nil {
		t.Fatal("expected a commit command")
	}
	msg := cmd()
	model.Update(msg)
	return msg
}

func TestCommentsAddHoldsTheDraftUntilTheCommitConfirms(t *testing.T) {
	project, task := commentsFixture(t)
	model := newTestCommentsPanel(task, project)
	model.AddMode = true
	model.TextInput.SetValue("New comment")

	cmd := model.saveComment()
	if cmd == nil {
		t.Fatal("expected non-nil command from saveComment")
	}
	// Nothing about the comment has happened yet: the composer still holds the
	// user's text and the stored task is untouched.
	if !model.AddMode {
		t.Fatal("the add composer closed before the transaction confirmed")
	}
	if got := model.TextInput.Value(); got != "New comment" {
		t.Fatalf("pending add draft = %q, want it retained", got)
	}
	if len(model.Entries()) != 0 || len(task.Comments) != 0 {
		t.Fatalf("saveComment applied the comment optimistically: %d entrie(s), %d stored", len(model.Entries()), len(task.Comments))
	}

	msg, ok := settle(t, model, cmd).(CommentAddedMsg)
	if !ok {
		t.Fatal("the add commit did not resolve to CommentAddedMsg")
	}
	if msg.Owner == "" || msg.Token == 0 {
		t.Fatalf("CommentAddedMsg carries no correlation: %+v", msg)
	}
	if model.AddMode {
		t.Fatal("expected the composer to close once the transaction confirmed")
	}
	if len(model.Entries()) != 1 || model.Entries()[0].Text != "New comment" {
		t.Fatalf("confirmed add left entries %+v", model.Entries())
	}
	if model.Cursor != 0 {
		t.Fatalf("expected cursor at 0, got %d", model.Cursor)
	}
	if model.Err != nil {
		t.Fatalf("expected no error, got %v", model.Err)
	}
}

func TestCommentsSaveCommentEmptyText(t *testing.T) {
	project, task := commentsFixture(t)
	model := newTestCommentsPanel(task, project)
	model.AddMode = true
	model.TextInput.SetValue("   ")

	cmd := model.saveComment()
	if cmd != nil {
		t.Fatal("expected nil command for empty comment")
	}
	if model.Err == nil {
		t.Fatal("expected error for empty comment text")
	}
}

func TestCommentsDeleteAppliesOnlyAfterTheCommitConfirms(t *testing.T) {
	project, task := commentsFixture(t, "comment 1", "comment 2")
	model := newTestCommentsPanel(task, project)
	model.Cursor = 1

	model.ConfirmDeleteSelectedComment()
	cmd := model.executeDelete()
	if cmd == nil {
		t.Fatal("expected non-nil command from executeDelete")
	}
	if len(model.Entries()) != 2 {
		t.Fatalf("executeDelete removed the row before the transaction confirmed: %+v", model.Entries())
	}
	if model.Cursor != 1 {
		t.Fatalf("pending delete moved the cursor to %d", model.Cursor)
	}

	if _, ok := settle(t, model, cmd).(CommentDeletedMsg); !ok {
		t.Fatal("the delete commit did not resolve to CommentDeletedMsg")
	}
	if len(model.Entries()) != 1 || model.Entries()[0].Text != "comment 1" {
		t.Fatalf("confirmed delete left entries %+v", model.Entries())
	}
	if model.Cursor != 0 {
		t.Fatalf("expected cursor at 0 after deleting the last comment, got %d", model.Cursor)
	}
}

func TestCommentsEditAppliesOnlyAfterTheCommitConfirmsAndKeepsTheTimestamp(t *testing.T) {
	project, task := commentsFixture(t, "original text")
	created := task.Comments[0].CreatedAt
	model := newTestCommentsPanel(task, project)
	model.EditMode = true
	model.editRef, _ = model.commentRefAt(0)
	model.EditTextInput.SetValue("edited text")

	cmd := model.saveEditedComment()
	if cmd == nil {
		t.Fatal("expected non-nil command from saveEditedComment")
	}
	if !model.EditMode {
		t.Fatal("the edit composer closed before the transaction confirmed")
	}
	if model.Entries()[0].Text != "original text" {
		t.Fatalf("saveEditedComment rewrote the row optimistically: %q", model.Entries()[0].Text)
	}

	if _, ok := settle(t, model, cmd).(CommentEditedMsg); !ok {
		t.Fatal("the edit commit did not resolve to CommentEditedMsg")
	}
	if model.EditMode {
		t.Fatal("expected the composer to close once the transaction confirmed")
	}
	if got := model.Entries()[0].Text; got != "edited text" {
		t.Fatalf("confirmed edit left text %q", got)
	}
	if !model.Entries()[0].CreatedAt.Equal(created) {
		t.Fatalf("expected timestamp to be preserved, got %v", model.Entries()[0].CreatedAt)
	}
	if model.Err != nil {
		t.Fatalf("expected no error, got %v", model.Err)
	}
}

func TestConvertedDecisionCommentIsFullyEditableThroughComments(t *testing.T) {
	project, task := commentsFixture(t, "Decision: Use SQLite\nRationale: Local-first durability")
	created := task.Comments[0].CreatedAt
	model := newTestCommentsPanel(task, project)
	if cmd := model.startEditComment(); cmd == nil {
		t.Fatal("converted comment did not enter edit mode")
	}
	if got := model.EditTextInput.Value(); got != task.Comments[0].Text {
		t.Fatalf("edit input = %q, want complete converted text %q", got, task.Comments[0].Text)
	}

	want := "Decision: Use SQLite\nRationale: Embedded deployment"
	model.EditTextInput.SetValue(want)
	cmd := model.saveEditedComment()
	if cmd == nil {
		t.Fatal("editing converted comment returned no commit command")
	}
	if _, ok := settle(t, model, cmd).(CommentEditedMsg); !ok {
		t.Fatal("editing converted comment returned an unexpected result")
	}
	comment := project.Tasks[0].Comments[0]
	if comment.Text != want {
		t.Fatalf("edited comment text = %q, want %q", comment.Text, want)
	}
	if !comment.CreatedAt.Equal(created) {
		t.Fatalf("edited comment timestamp = %v, want %v", comment.CreatedAt, created)
	}
}

// TestCommentsRefuseASecondSubmissionWhileOneIsPending is the whole duplicate
// guard: one panel holds one outstanding mutation, so a second Ctrl+S, a second
// confirmed delete, and a second editor request all bounce rather than queueing
// a write nobody asked for twice.
func TestCommentsRefuseASecondSubmissionWhileOneIsPending(t *testing.T) {
	project, task := commentsFixture(t, "stored")
	model := newTestCommentsPanel(task, project)
	// A delete confirmation frozen before the add was submitted is the only way a
	// removal can still be answered while a mutation is pending, since opening one
	// afterwards is refused by the same gate.
	model.ConfirmDeleteSelectedComment()
	model.AddMode = true
	model.TextInput.SetValue("first press")

	cmd := model.saveComment()
	if cmd == nil {
		t.Fatal("expected the first submission to commit")
	}
	if model.saveComment() != nil {
		t.Fatal("a second Ctrl+S submitted the comment twice")
	}
	if model.executeDelete() != nil {
		t.Fatal("a delete slipped past the pending add")
	}
	if model.addCommentInExternalEditor() != nil {
		t.Fatal("an external editor request slipped past the pending add")
	}
	if model.BeginAddMode() != nil {
		t.Fatal("the add composer reopened over a pending submission")
	}
	if model.StatusMessage == "" {
		t.Fatal("a refused duplicate said nothing")
	}

	if _, ok := settle(t, model, cmd).(CommentAddedMsg); !ok {
		t.Fatal("the pending add did not resolve")
	}
	if len(model.Entries()) != 2 {
		t.Fatalf("the pending add committed %d comment(s), want exactly one more", len(model.Entries())-1)
	}
	if model.saveComment() != nil {
		t.Fatal("the composer is closed, so there is nothing to submit")
	}
}

// TestCommentsRefusedAddKeepsTheDraftAndTheStoredComments holds the failure
// half of the contract: the panel never applied the change, so a refusal has
// nothing to roll back and everything to put back in front of the user.
func TestCommentsRefusedAddKeepsTheDraftAndTheStoredComments(t *testing.T) {
	project, task := commentsFixture(t, "stored")
	model := newRefusingCommentsPanel(task, project)
	model.AddMode = true
	model.TextInput.SetValue("refused draft")

	settled := settle(t, model, model.saveComment())
	msg, ok := settled.(CommentMutationFailedMsg)
	if !ok {
		t.Fatalf("a refused add resolved to %T, want CommentMutationFailedMsg", settled)
	}
	if msg.Err == nil {
		t.Fatal("the refusal named no error")
	}
	if !model.AddMode {
		t.Fatal("a refused add closed the composer")
	}
	if got := model.TextInput.Value(); got != "refused draft" {
		t.Fatalf("refused add draft = %q, want it preserved", got)
	}
	if model.Err == nil {
		t.Fatal("a refused add left no error on the panel")
	}
	if len(task.Comments) != 1 || task.Comments[0].Text != "stored" {
		t.Fatalf("a refused add touched the stored comments: %d", len(task.Comments))
	}
}

func TestCommentsRefusedEditKeepsTheDraftAndTheStoredText(t *testing.T) {
	project, task := commentsFixture(t, "stored")
	model := newRefusingCommentsPanel(task, project)
	model.EditMode = true
	model.editRef, _ = model.commentRefAt(0)
	model.EditTextInput.SetValue("refused replacement")

	if _, ok := settle(t, model, model.saveEditedComment()).(CommentMutationFailedMsg); !ok {
		t.Fatal("a refused edit did not resolve to CommentMutationFailedMsg")
	}
	if !model.EditMode {
		t.Fatal("a refused edit closed the composer")
	}
	if got := model.EditTextInput.Value(); got != "refused replacement" {
		t.Fatalf("refused edit draft = %q, want it preserved", got)
	}
	if task.Comments[0].Text != "stored" {
		t.Fatalf("a refused edit rewrote the stored comment to %q", task.Comments[0].Text)
	}
}

func TestCommentsRefusedDeleteKeepsTheStoredComment(t *testing.T) {
	project, task := commentsFixture(t, "stored")
	model := newRefusingCommentsPanel(task, project)

	model.ConfirmDeleteSelectedComment()
	if _, ok := settle(t, model, model.executeDelete()).(CommentMutationFailedMsg); !ok {
		t.Fatal("a refused delete did not resolve to CommentMutationFailedMsg")
	}
	if len(model.Entries()) != 1 || len(task.Comments) != 1 {
		t.Fatalf("a refused delete removed the comment anyway: %d entrie(s)", len(model.Entries()))
	}
	if model.AddMode || model.EditMode {
		t.Fatal("a refused delete opened a composer")
	}
}

// TestCommentsIgnoreOutcomesThatAreNotTheirs is the navigation guarantee: a
// commit that lands after the user moved on must not reopen the panel it was
// submitted from, and must never apply to whatever is mounted now.
func TestCommentsIgnoreOutcomesThatAreNotTheirs(t *testing.T) {
	project, task := commentsFixture(t)
	model := newTestCommentsPanel(task, project)
	model.AddMode = true
	model.TextInput.SetValue("in flight")

	cmd := model.saveComment()
	msg, ok := cmd().(CommentAddedMsg)
	if !ok {
		t.Fatal("the add commit did not resolve to CommentAddedMsg")
	}

	// Another owner's success for the same token.
	model.Update(CommentAddedMsg{Owner: "task:elsewhere/other", Token: msg.Token})
	if !model.AddMode {
		t.Fatal("a foreign owner's success closed this composer")
	}
	// This owner, but a submission this panel is not waiting on.
	model.Update(CommentAddedMsg{Owner: msg.Owner, Token: msg.Token + 1})
	if !model.AddMode {
		t.Fatal("a stale token's success closed this composer")
	}
	// A foreign refusal must not install this panel's error or draft either.
	model.Update(CommentMutationFailedMsg{Owner: "task:elsewhere/other", Token: msg.Token})
	if model.Err != nil {
		t.Fatalf("a foreign refusal installed the error %v", model.Err)
	}

	// The panel is dismissed; the real outcome arrives afterwards.
	model.OnExit()
	model.Update(msg)
	if model.AddMode || model.EditMode {
		t.Fatal("an abandoned outcome reopened a composer on a dismissed panel")
	}
}

// TestCommentsExternalEditorResultCommitsThroughUpdate keeps the editor's
// callback off the model: it returns a value, and the panel records the pending
// submission where every other state change is already made.
func TestCommentsExternalEditorResultCommitsThroughUpdate(t *testing.T) {
	project, task := commentsFixture(t)
	model := newTestCommentsPanel(task, project)
	model.AddMode = true

	finished := commentEditorFinishedMsg{owner: model.ownerKey(), kind: commentMutationAdd, text: "written in the editor"}
	_, cmd := model.Update(finished)
	if cmd == nil {
		t.Fatal("the editor result raised no commit")
	}
	if len(task.Comments) != 0 {
		t.Fatal("the editor result was applied before the transaction confirmed")
	}
	if _, ok := settle(t, model, cmd).(CommentAddedMsg); !ok {
		t.Fatal("the editor-driven add did not resolve to CommentAddedMsg")
	}
	if len(model.Entries()) != 1 || model.Entries()[0].Text != "written in the editor" {
		t.Fatalf("the editor-driven add left entries %+v", model.Entries())
	}

	// An editor result for another owner is dropped rather than applied here.
	if _, cmd := model.Update(commentEditorFinishedMsg{owner: "task:elsewhere/other", kind: commentMutationAdd, text: "not ours"}); cmd != nil {
		t.Fatal("a foreign editor result raised a commit on this panel")
	}
}

func TestCommentsRenderEditModeOOB(t *testing.T) {
	project, task := commentsFixture(t, "only comment")
	model := newTestCommentsPanel(task, project)
	model.EditMode = true
	model.editRef, _ = model.commentRefAt(0)

	// Simulate external deletion of all comments
	task.Comments = nil
	model.RefreshAfterReload()

	// View() must not panic when the edited comment is gone
	view := model.View()
	if view.Content == "" {
		t.Fatal("expected non-empty view output")
	}
}

func TestCommentsEditExternalEditorFromEditModeOOB(t *testing.T) {
	project, task := commentsFixture(t, "only comment")
	model := newTestCommentsPanel(task, project)
	model.EditMode = true
	model.editRef, _ = model.commentRefAt(0)

	// Simulate external deletion of all comments
	task.Comments = nil

	// Must not panic; should cancel edit mode and return an error cmd
	cmd := model.editCommentInExternalEditorFromEditMode()
	if cmd == nil {
		t.Fatal("expected non-nil error command when the edited comment is gone")
	}
	if model.EditMode {
		t.Fatal("expected editMode to be canceled when comment was deleted externally")
	}
}

func TestCommentsSaveEditOOBAfterExternalDeletion(t *testing.T) {
	project, task := commentsFixture(t, "original")
	model := newTestCommentsPanel(task, project)
	model.EditMode = true
	model.editRef, _ = model.commentRefAt(0)
	model.EditTextInput.SetValue("edited text")

	// Simulate external deletion of all comments
	task.Comments = nil

	// saveEditedComment should detect OOB and return error cmd
	cmd := model.saveEditedComment()
	if cmd == nil {
		t.Fatal("expected non-nil error command when the edited comment is gone")
	}
	if model.EditMode {
		t.Fatal("expected editMode to be canceled when comment was deleted externally")
	}
}

func TestCommentsHasUnsavedEditChangesIgnoresSurroundingWhitespace(t *testing.T) {
	project, task := commentsFixture(t)
	// Stored comment text has trailing whitespace (e.g., from prior editor save).
	task.Comments = []*models.Comment{
		{Text: "hello world\n", CreatedAt: time.Now()},
	}
	model := newTestCommentsPanel(task, project)
	model.EditMode = true
	model.editRef, _ = model.commentRefAt(0)
	// User opens edit; textarea is populated with the stored value but trims on input.
	model.EditTextInput.SetValue("hello world")

	if model.hasUnsavedEditChanges() {
		t.Fatal("expected no unsaved changes when input differs from stored text only by surrounding whitespace")
	}
}

func TestCommentsLeanPanelHasNoResolvedControl(t *testing.T) {
	project, task := commentsFixture(t, "comment")
	model := newTestCommentsPanel(task, project)
	if cmd := model.HandleNormalKey("r"); cmd != nil {
		t.Fatalf("retired resolved key emitted command %#v", cmd())
	}
	for _, key := range model.HandledKeys() {
		if key == "r" {
			t.Fatal("retired resolved key remains in comments key map")
		}
	}
}

// TestCommentsPanelReadsItsOwnerRatherThanATaskPointer is the adapter seam
// itself: the panel projects entries, a title, and editor preambles from the
// owner, so an owner that is not a task needs no change here.
func TestCommentsPanelReadsItsOwnerRatherThanATaskPointer(t *testing.T) {
	project, task := commentsFixture(t, "one", "two")
	owner := NewTaskCommentOwner(task, project, newTestSecondaryCoordinators(project).Subview)
	model := NewCommentsPanel(owner, "")

	if got := len(model.Entries()); got != 2 {
		t.Fatalf("the panel projected %d entrie(s) from its owner, want 2", got)
	}
	if model.Entries()[0].Text != "one" || model.Entries()[1].Text != "two" {
		t.Fatalf("the task adapter reordered stored comments: %+v", model.Entries())
	}
	if got := owner.Title(); got.Prefix != "Comments · #1: " || got.Task != task {
		t.Fatalf("task owner title = %+v", got)
	}
	if got := owner.Key(); got != "task:project-1/task-1" {
		t.Fatalf("task owner key = %q", got)
	}
	refTask, refProject := model.GetTaskRef()
	if refTask != task || refProject != project {
		t.Fatal("the panel's task ref no longer comes from its owner")
	}
	if got := owner.ExternalAddHeader(); got == "" || got == owner.ExternalEditHeader(CommentRef{}) {
		t.Fatalf("the owner's editor preambles do not distinguish add from edit: %q", got)
	}
}
