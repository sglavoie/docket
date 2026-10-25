package taskdetail

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/storage"
	"github.com/sglavoie/docket/internal/tui/coordinator"
)

// TestProjectCommentOwnerPresentsNewestFirst holds the one presentation
// difference between the two adapters. Storage keeps comments oldest first
// because that order is canonical; the panel opens on the newest.
func TestProjectCommentOwnerPresentsNewestFirst(t *testing.T) {
	project := projectCommentsFixture(t)
	project.Comments = append(project.Comments, nil)
	owner := NewProjectCommentOwner(project, nil, coordinator.ProjectCommentCoordinator{})

	entries := owner.Entries()
	if len(entries) != 2 {
		t.Fatalf("entries = %+v, want the two storable comments", entries)
	}
	if entries[0].ID != newerCommentID || entries[1].ID != olderCommentID {
		t.Fatalf("entry order = %q then %q, want newest first", entries[0].ID, entries[1].ID)
	}
	if entries[0].Text != "Newer" || entries[0].CreatedAt.IsZero() {
		t.Fatalf("newest entry = %+v, want its stored text and instant", entries[0])
	}
}

// TestProjectCommentOwnerNamesTheProjectNotTheRow guards the labels: a row
// number is not this owner's identity, so nothing it writes may name one.
func TestProjectCommentOwnerNamesTheProjectNotTheRow(t *testing.T) {
	project := projectCommentsFixture(t)
	owner := NewProjectCommentOwner(project, nil, coordinator.ProjectCommentCoordinator{})

	if key := owner.Key(); key != "project:project-1" {
		t.Fatalf("owner key = %q, want the project-qualified key", key)
	}
	if title := owner.Title(); title.Prefix != "Comments · Comments Project" || title.Task != nil {
		t.Fatalf("title = %+v, want a project-named prefix and no task", title)
	}
	if task, owning := owner.TaskRef(); task != nil || owning != nil {
		t.Fatal("the project owner lent the shell a task reference")
	}
	header := owner.ExternalEditHeader(CommentRef{Index: 1, ID: olderCommentID, Original: "Older"})
	if !strings.Contains(header, "Comments Project") {
		t.Fatalf("edit header = %q, want it to name the project", header)
	}
	if strings.Contains(header, "#1") || strings.Contains(header, "comment 1") {
		t.Fatalf("edit header = %q, want no row number in it", header)
	}
	if add := owner.ExternalAddHeader(); !strings.Contains(add, "Comments Project") {
		t.Fatalf("add header = %q, want it to name the project", add)
	}
}

// TestProjectCommentPanelEditsTheCommentUnderTheCursorByIdentity is the
// acceptance path for editing: the cursor sits on the newest row, which is the
// last stored comment, so a mutation applied by list position would rewrite the
// wrong one.
func TestProjectCommentPanelEditsTheCommentUnderTheCursorByIdentity(t *testing.T) {
	model, project := newTestProjectCommentsPanel(t, projectCommentsFixture(t))
	model.Cursor = 0

	cmd := model.startEditComment()
	if cmd == nil {
		t.Fatal("expected the edit composer to open")
	}
	if model.EditTextInput.Value() != "Newer" {
		t.Fatalf("edit buffer = %q, want the newest comment", model.EditTextInput.Value())
	}
	model.EditTextInput.SetValue("Newer, revised")

	if _, ok := settle(t, model, model.saveEditedComment()).(CommentEditedMsg); !ok {
		t.Fatal("the edit commit did not resolve to CommentEditedMsg")
	}
	if model.EditMode {
		t.Fatal("the edit composer stayed open after the transaction confirmed")
	}
	if project.Comments[0].Text != "Older" {
		t.Fatalf("the older comment = %q, want it untouched", project.Comments[0].Text)
	}
	if project.Comments[1].Text != "Newer, revised" {
		t.Fatalf("the edited comment = %q", project.Comments[1].Text)
	}
	if project.Comments[1].ID != newerCommentID {
		t.Fatalf("editing minted a new identity: %q", project.Comments[1].ID)
	}
}

// TestProjectCommentPanelDeletesTheCommentUnderTheCursorByIdentity is the same
// acceptance for removal.
func TestProjectCommentPanelDeletesTheCommentUnderTheCursorByIdentity(t *testing.T) {
	model, project := newTestProjectCommentsPanel(t, projectCommentsFixture(t))
	model.Cursor = 0

	model.ConfirmDeleteSelectedComment()
	if _, ok := settle(t, model, model.executeDelete()).(CommentDeletedMsg); !ok {
		t.Fatal("the delete commit did not resolve to CommentDeletedMsg")
	}
	if len(project.Comments) != 1 || project.Comments[0].ID != olderCommentID {
		t.Fatalf("stored comments = %+v, want the older one alone", project.Comments)
	}
	if len(model.Entries()) != 1 || model.Entries()[0].Text != "Older" {
		t.Fatalf("panel entries = %+v", model.Entries())
	}
}

// TestProjectCommentPanelSelectsTheAddedCommentAtTheTop holds the selection
// half of newest-first presentation: an add arrives at row 0, and a cursor
// restored by index would sit on the comment that used to be there.
func TestProjectCommentPanelSelectsTheAddedCommentAtTheTop(t *testing.T) {
	model, _ := newTestProjectCommentsPanel(t, projectCommentsFixture(t))
	model.Cursor = 1
	model.AddMode = true
	model.TextInput.SetValue("Newest of all")

	if _, ok := settle(t, model, model.saveComment()).(CommentAddedMsg); !ok {
		t.Fatal("the add commit did not resolve to CommentAddedMsg")
	}
	if len(model.Entries()) != 3 {
		t.Fatalf("entries = %+v, want three", model.Entries())
	}
	if model.Cursor != 0 || model.Entries()[0].Text != "Newest of all" {
		t.Fatalf("cursor = %d over %+v, want the new comment at the top", model.Cursor, model.Entries())
	}
}

// TestProjectCommentPanelKeepsSelectionAcrossAReload holds the other selection
// case: a comment added by another surface lands above the selected row, and
// the cursor must follow its comment rather than its position.
func TestProjectCommentPanelKeepsSelectionAcrossAReload(t *testing.T) {
	model, project := newTestProjectCommentsPanel(t, projectCommentsFixture(t))
	model.Cursor = 1 // the older comment, at the bottom

	project.Comments = append(project.Comments, &models.ProjectComment{
		ID:        "cccccccc-dddd-eeee-ffff-000000000000",
		Text:      "Arrived elsewhere",
		CreatedAt: time.Date(2026, 3, 5, 9, 0, 0, 0, time.UTC),
	})
	if !model.RefreshAfterReload() {
		t.Fatal("the reload lost the owner")
	}
	if got := model.Entries()[model.Cursor].ID; got != olderCommentID {
		t.Fatalf("cursor landed on %q, want the comment it was on", got)
	}
}

// TestProjectCommentOwnerReResolvesTheProject covers the reason the adapter
// holds a resolver rather than a pointer: a commit, a rename, or a park replaces
// the in-memory object, and an adapter still drawing the discarded one would
// show stale text and comment on a project nobody is looking at.
func TestProjectCommentOwnerReResolvesTheProject(t *testing.T) {
	original := projectCommentsFixture(t)
	replacement, err := models.NewProject("project-1", "Renamed Project", original.Path)
	if err != nil {
		t.Fatalf("NewProject: %v", err)
	}
	replacement.Comments = []*models.ProjectComment{
		{ID: newerCommentID, Text: "Newer", CreatedAt: time.Date(2026, 3, 4, 10, 0, 0, 0, time.UTC)},
	}
	replacement.Tags = []string{"parked"}

	current := original
	owner := NewProjectCommentOwner(original, func(id string) *models.Project {
		if current != nil && current.ID == id {
			return current
		}
		return nil
	}, coordinator.ProjectCommentCoordinator{})

	current = replacement
	if !owner.Reload() {
		t.Fatal("a parked project stopped being readable")
	}
	if title := owner.Title(); title.Prefix != "Comments · Renamed Project" {
		t.Fatalf("title after the replacement = %q", title.Prefix)
	}
	if entries := owner.Entries(); len(entries) != 1 || entries[0].ID != newerCommentID {
		t.Fatalf("entries after the replacement = %+v", entries)
	}

	current = nil
	if owner.Reload() {
		t.Fatal("a removed project still resolved")
	}
}

// TestProjectCommentPanelRefusesAChangedTargetWithoutLosingTheDraft is the
// precondition acceptance: the edit composer is measured against the text it was
// opened on, so a comment rewritten elsewhere refuses the save and the user's
// replacement stays in front of them.
func TestProjectCommentPanelRefusesAChangedTargetWithoutLosingTheDraft(t *testing.T) {
	model, project := newTestProjectCommentsPanel(t, projectCommentsFixture(t))
	model.Cursor = 0
	model.startEditComment()
	model.EditTextInput.SetValue("mine")

	// The change has to reach the stored document: the application layer
	// re-resolves the target against a fresh snapshot, which is the whole point of
	// carrying the original text across the seam.
	project.Comments[1].Text = "someone else's"
	if err := storage.SaveProject(project); err != nil {
		t.Fatalf("SaveProject: %v", err)
	}
	msg, ok := settle(t, model, model.saveEditedComment()).(CommentMutationFailedMsg)
	if !ok {
		t.Fatal("an edit against changed text was not refused")
	}
	if msg.Err == nil || !strings.Contains(msg.Err.Error(), "changed since it was read") {
		t.Fatalf("refusal = %v, want it to name the changed target", msg.Err)
	}
	if !model.EditMode || model.EditTextInput.Value() != "mine" {
		t.Fatalf("refused edit lost the draft: editing=%v buffer=%q", model.EditMode, model.EditTextInput.Value())
	}
	if project.Comments[1].Text != "someone else's" || project.Comments[0].Text != "Older" {
		t.Fatalf("a refused edit mutated stored comments: %+v", project.Comments)
	}
}

// TestProjectCommentPanelRefusesAVanishedTarget holds the other precondition: a
// comment deleted underneath an open editor refuses the save by name instead of
// rewriting whichever comment now occupies the row.
//
// The composer stays open over that refusal. Nothing can put the vanished comment
// back, so the buffer is the only copy of the replacement in the process, and a
// refusal that reset it would take the user's work as the price of protecting the
// list. Discarding it stays an explicit act.
func TestProjectCommentPanelRefusesAVanishedTarget(t *testing.T) {
	model, project := newTestProjectCommentsPanel(t, projectCommentsFixture(t))
	model.Cursor = 0
	model.startEditComment()
	model.EditTextInput.SetValue("mine")

	project.Comments = project.Comments[:1]
	refusal := refusalText(t, model.saveEditedComment())
	if !strings.Contains(refusal, "deleted externally") {
		t.Fatalf("refusal = %q, want it to name the vanished target", refusal)
	}
	if !model.EditMode || model.EditTextInput.Value() != "mine" {
		t.Fatalf("a refused save lost the draft: editing=%v buffer=%q", model.EditMode, model.EditTextInput.Value())
	}
	if project.Comments[0].Text != "Older" {
		t.Fatalf("the surviving comment was rewritten: %+v", project.Comments)
	}

	// The retained draft is discarded the ordinary way and only the ordinary way:
	// Esc asks, and the answer is what spends it.
	model.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if !model.IsConfirmActive() {
		t.Fatal("Esc over a retained draft did not ask before discarding it")
	}
	model.Update(tea.KeyPressMsg{Code: 'y', Text: "y"})
	if model.EditMode || model.EditTextInput.Value() != "" {
		t.Fatalf("a confirmed discard kept the composer: editing=%v buffer=%q",
			model.EditMode, model.EditTextInput.Value())
	}
	if len(project.Comments) != 1 || project.Comments[0].Text != "Older" {
		t.Fatalf("the discard disturbed stored comments: %+v", project.Comments)
	}
}

// TestProjectCommentPanelRefusesAMutationOnARemovedOwner holds the visible
// refusal a removed project earns. The draft is the user's only copy, so it is
// not discarded with the owner.
func TestProjectCommentPanelRefusesAMutationOnARemovedOwner(t *testing.T) {
	project := projectCommentsFixture(t)
	coordinators := newTestSecondaryCoordinators(project)
	model := newProjectCommentsPanel(project, coordinators.ProjectComment, func(string) *models.Project { return nil })
	model.AddMode = true
	model.TextInput.SetValue("into the void")

	cmd := model.saveComment()
	if cmd == nil {
		t.Fatal("expected a refusal command from a removed owner")
	}
	if model.TextInput.Value() != "into the void" || !model.AddMode {
		t.Fatalf("a removed owner discarded the draft: mode=%v buffer=%q", model.AddMode, model.TextInput.Value())
	}
	if len(project.Comments) != 2 {
		t.Fatalf("a refused add wrote a comment: %+v", project.Comments)
	}
}

// TestProjectCommentPanelDropsAnAbandonedEditorResult holds the correlation
// rule for the project adapter: an external editor that finished after the panel
// moved on cannot commit against whatever is mounted now.
func TestProjectCommentPanelDropsAnAbandonedEditorResult(t *testing.T) {
	model, project := newTestProjectCommentsPanel(t, projectCommentsFixture(t))

	cmd := model.applyCommentEditorResult(commentEditorFinishedMsg{
		owner: "project:another-project",
		kind:  commentMutationEdit,
		ref:   CommentRef{Index: 0, ID: newerCommentID, Original: "Newer"},
		text:  "from an abandoned editor",
	})
	if cmd != nil {
		t.Fatal("an abandoned editor result produced a command")
	}
	if model.EditMode || model.AddMode {
		t.Fatal("an abandoned editor result reopened a composer")
	}
	if project.Comments[1].Text != "Newer" {
		t.Fatalf("an abandoned editor result committed: %+v", project.Comments)
	}
}

// TestProjectCommentPanelCommitsAnExternalEditorResult is the other half of the
// external-editor path: a buffer that came back for the owner that is still
// mounted commits against the identity it was opened on, not against the row
// that identity happened to occupy.
func TestProjectCommentPanelCommitsAnExternalEditorResult(t *testing.T) {
	model, project := newTestProjectCommentsPanel(t, projectCommentsFixture(t))
	model.Cursor = 0
	ref, ok := model.commentRefAt(0)
	if !ok {
		t.Fatal("the panel drew no comment row")
	}

	cmd := model.applyCommentEditorResult(commentEditorFinishedMsg{
		owner: model.ownerKey(),
		kind:  commentMutationEdit,
		ref:   ref,
		text:  "Newer, from the editor",
	})
	if _, ok := settle(t, model, cmd).(CommentEditedMsg); !ok {
		t.Fatal("an external editor result did not commit")
	}
	if project.Comments[1].Text != "Newer, from the editor" || project.Comments[1].ID != newerCommentID {
		t.Fatalf("stored comments after the external edit = %+v", project.Comments)
	}
	if project.Comments[0].Text != "Older" {
		t.Fatalf("the external edit touched the other comment: %+v", project.Comments)
	}
}
