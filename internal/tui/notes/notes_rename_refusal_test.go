package notes

import (
	"errors"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/docket/internal/app"
	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/tui/route"
)

// TestNotesRenameRefusalKeepsTypedTitle pins that a refused rename hands its
// draft back: the refusal carries the typed title, and the view reopens the
// rename holding it instead of making the user retype it.
func TestNotesRenameRefusalKeepsTypedTitle(t *testing.T) {
	note := &models.Note{Title: "Old", Filename: "old.md", ModifiedAt: time.Unix(1, 0)}
	service := &combinedNotesService{
		byLocation: map[app.NoteLocation][]*models.Note{app.GlobalNoteLocation(): {note}},
		applyErr:   errors.New("a note named Taken already exists"),
	}
	view := NewNotesViewModelWithService(t.TempDir(), route.AllNotesScope(), service, nil)
	mountNotesManager(t, view)
	view.SetSize(80, 24)

	view.StartRename()
	if !view.renaming {
		t.Fatal("expected rename to open")
	}
	view.renameInput.SetValue("Taken")
	_, cmd := view.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected a rename command")
	}
	if view.renaming {
		t.Fatal("rename stays closed while the commit is in flight")
	}
	refusal, ok := cmd().(ProjectNoteMutationErrorMsg)
	if !ok {
		t.Fatal("expected the rename to be refused")
	}
	if refusal.RenameDraft == nil || refusal.RenameDraft.Title != "Taken" {
		t.Fatalf("refusal draft = %+v, want the typed title", refusal.RenameDraft)
	}

	if !view.ReopenRefusedRename(refusal.RenameDraft) {
		t.Fatal("expected the refused rename to reopen")
	}
	if !view.renaming || view.renameInput.Value() != "Taken" {
		t.Fatalf("reopened rename = %v %q, want the typed title", view.renaming, view.renameInput.Value())
	}
	// A refusal arriving while another interaction holds the keys displaces nothing.
	if view.ReopenRefusedRename(refusal.RenameDraft) {
		t.Fatal("a second refusal must not reopen over an open rename")
	}
}
