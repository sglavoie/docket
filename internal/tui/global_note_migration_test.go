package tui

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

type globalNoteTUIEnv struct {
	dataDir string
	dir     string
	ledger  *storage.WorkspaceLedger
	svc     *app.Service
	project *models.Project
	view    *NotesViewModel
	shell   *AppModel
}

func newGlobalNoteTUIEnv(t *testing.T, titles ...string) *globalNoteTUIEnv {
	t.Helper()
	dataDir := t.TempDir()
	ledger := storage.NewWorkspaceLedger(filepath.Join(dataDir, ".tt", "undo"), 0, []string{dataDir})
	commitSvc := storage.NewCommitServiceWithLedger(filepath.Join(dataDir, ".tt", "transactions"), []string{dataDir}, ledger)
	project, err := models.NewProject(uuid.New().String(), "destination", "")
	if err != nil {
		t.Fatalf("NewProject: %v", err)
	}
	project.Path = filepath.Join(dataDir, "projects", "destination")
	repo := newScenarioRepo(commitSvc, project)
	repo.seed(t)
	dir := storage.GlobalNotesDir(dataDir)
	for _, title := range titles {
		note := models.NewNote(title)
		note.Body = title + " body"
		if err := storage.SaveNoteTo(dir, note); err != nil {
			t.Fatalf("seed global note %q: %v", title, err)
		}
	}

	svc := app.NewServiceWithArtifacts(repo, nil, nil, app.NewStorageWorkspaceNotePort(dataDir), nil)
	view := NewGlobalNotesViewModelWithService(dir, svc, ledger)
	view.SetSize(80, 40)
	views := NewViewRegistry()
	views.Register(ViewGlobalNotes, view)
	shell := &AppModel{
		appService: svc,
		views:      views,
		viewStack:  []ViewType{ViewGlobalNotes},
		navHistory: NewNavHistory(),
	}
	return &globalNoteTUIEnv{dataDir: dataDir, dir: dir, ledger: ledger, svc: svc, project: project, view: view, shell: shell}
}

func (e *globalNoteTUIEnv) note(t *testing.T, filename string) *models.Note {
	t.Helper()
	note, err := storage.LoadNoteFrom(e.dir, filename)
	if err != nil {
		t.Fatalf("LoadNoteFrom(%s): %v", filename, err)
	}
	return note
}

func TestGlobalNoteMutationCommitsCopiedRequestBeforeReconciliation(t *testing.T) {
	env := newGlobalNoteTUIEnv(t, "Pin me")
	before := env.view.notes[0]
	cmd := env.view.togglePin()
	result := runProjectNoteMutation(t, func() any { return cmd() })
	if result.Location != app.GlobalNoteLocation() {
		t.Fatalf("result location = %#v", result.Location)
	}
	if env.view.notes[0] != before || env.view.notes[0].Pinned {
		t.Fatal("command mutated the global view cache before Update")
	}
	if disk := env.note(t, before.Filename); !disk.Pinned {
		t.Fatal("typed global transaction did not persist pin")
	}
	entry, ok, err := env.ledger.SelectNewestUndo(storage.GlobalScope())
	if err != nil || !ok {
		t.Fatalf("global ledger entry ok=%v err=%v", ok, err)
	}
	if len(entry.ProjectIDs) != 0 {
		t.Fatalf("global-only transaction carried project IDs: %v", entry.ProjectIDs)
	}
	env.shell.handleNotesMessages(result)
	if !env.view.notes[0].Pinned || env.view.notes[0] == before {
		t.Fatal("Update did not reconcile the committed global note")
	}
}

func TestGlobalBulkNoteDeleteIsAtomicOnStaleRevision(t *testing.T) {
	env := newGlobalNoteTUIEnv(t, "Alpha", "Beta")
	for _, note := range env.view.notes {
		env.view.selectedNotes[note.Filename] = true
	}
	env.view.multiSelectMode = true
	stale := env.view.notes[1]
	concurrent := env.note(t, stale.Filename)
	concurrent.Body = "newer beta"
	if err := storage.SaveNoteTo(env.dir, concurrent); err != nil {
		t.Fatalf("concurrent save: %v", err)
	}

	cmd := env.view.performBulkDelete()
	msg, ok := cmd().(ProjectNoteMutationErrorMsg)
	if !ok {
		t.Fatalf("global bulk delete = %T, want ProjectNoteMutationErrorMsg", cmd())
	}
	var conflict *storage.ArtifactConflictError
	if !errors.As(msg.Err, &conflict) {
		t.Fatalf("global stale error = %T %v", msg.Err, msg.Err)
	}
	env.shell.handleNotesMessages(msg)
	if !env.view.multiSelectMode || len(env.view.selectedNotes) != 2 {
		t.Fatal("rejected global batch cleared the retryable selection")
	}
	for _, note := range env.view.notes {
		if _, err := storage.LoadNoteFrom(env.dir, note.Filename); err != nil {
			t.Fatalf("atomic global batch partially deleted %s: %v", note.Filename, err)
		}
	}
}

func TestGlobalBulkNoteTagsCommitAsOneTransaction(t *testing.T) {
	env := newGlobalNoteTUIEnv(t, "One", "Two")
	cmd := env.shell.applyNoteTags(app.GlobalNoteLocation(), env.view.notes, []string{"batch"}, true)
	result := runProjectNoteMutation(t, func() any { return cmd() })
	if len(result.Result.Results) != 2 || result.Result.Results[0].TransactionID != result.Result.Results[1].TransactionID {
		t.Fatal("global bulk tags did not commit in one transaction")
	}
	env.shell.handleNotesMessages(result)
	for _, note := range env.view.notes {
		if got := env.note(t, note.Filename); len(got.Tags) != 1 || got.Tags[0] != "batch" {
			t.Fatalf("global bulk tag missing from %s: %v", note.Filename, got.Tags)
		}
	}
}

func TestGlobalNoteMoveUsesDestinationIDAndSharedLedgerScopes(t *testing.T) {
	env := newGlobalNoteTUIEnv(t, "Move me")
	source := env.view.notes[0]
	env.shell.pendingGlobalNoteMove = &pendingNoteMove{Source: newNoteMutationRef(app.GlobalNoteLocation(), source)}
	handled, cmd := env.shell.handleNotesMessages(GlobalNoteMoveDoneMsg{ProjectID: env.project.ID, ProjectName: env.project.Name})
	if !handled || cmd == nil || env.shell.pendingGlobalNoteMove != nil {
		t.Fatal("global move did not freeze and dispatch its pending request")
	}
	result := runProjectNoteMutation(t, func() any { return cmd() })
	if _, err := os.Stat(filepath.Join(env.dir, source.Filename)); !os.IsNotExist(err) {
		t.Fatalf("move left global source: %v", err)
	}
	destinationNotes, err := env.svc.NotesAt(app.ProjectNoteLocation(env.project.ID))
	if err != nil || len(destinationNotes) != 1 {
		t.Fatalf("destination notes=%d err=%v", len(destinationNotes), err)
	}
	entry, ok, err := env.ledger.SelectNewestUndo(storage.GlobalScope())
	if err != nil || !ok || len(entry.ProjectIDs) != 1 || entry.ProjectIDs[0] != env.project.ID {
		t.Fatalf("move global entry=%#v ok=%v err=%v", entry, ok, err)
	}
	projectEntry, ok, err := env.ledger.SelectNewestUndo(storage.ProjectScope(env.project.ID))
	if err != nil || !ok || projectEntry.TransactionID != entry.TransactionID {
		t.Fatalf("move project selection=%#v ok=%v err=%v", projectEntry, ok, err)
	}
	env.shell.handleNotesMessages(result)

	undo, ok := env.view.undoDelete()().(ProjectNoteLedgerResultMsg)
	if !ok || undo.Location != app.GlobalNoteLocation() || undo.Redo {
		t.Fatalf("global undo result = %#v", undo)
	}
	env.shell.handleNotesMessages(undo)
	if len(env.view.notes) != 1 || env.view.notes[0].Title != source.Title {
		t.Fatal("global undo did not restore the move source")
	}
	redo, ok := env.view.redoNoteMutation()().(ProjectNoteLedgerResultMsg)
	if !ok || !redo.Redo {
		t.Fatalf("global redo result = %#v", redo)
	}
	env.shell.handleNotesMessages(redo)
	if len(env.view.notes) != 0 {
		t.Fatal("global redo did not replay the move")
	}
}

func TestGlobalNoteEditorResultUsesTypedRenameTransaction(t *testing.T) {
	env := newGlobalNoteTUIEnv(t, "Editable")
	original := env.view.notes[0]
	content, err := storage.RenderNoteContent(original)
	if err != nil {
		t.Fatal(err)
	}
	modified := strings.Replace(content, "title: Editable", "title: Edited globally", 1)
	handled, cmd := env.shell.handleNotesMessages(ProjectNoteEditorDoneMsg{
		Location:        app.GlobalNoteLocation(),
		Original:        cloneNoteForCommand(original),
		OriginalContent: content,
		Modified:        modified,
	})
	if !handled || cmd == nil {
		t.Fatal("global editor result did not schedule a typed transaction")
	}
	result := runProjectNoteMutation(t, func() any { return cmd() })
	env.shell.handleNotesMessages(result)
	if len(env.view.notes) != 1 || env.view.notes[0].Title != "Edited globally" {
		t.Fatalf("global editor reconciliation = %#v", env.view.notes)
	}
	if _, err := os.Stat(filepath.Join(env.dir, original.Filename)); !os.IsNotExist(err) {
		t.Fatalf("global rename left original canonical file: %v", err)
	}
}

func TestGlobalNoteCreateEditorResultDoesNotPersistPlaceholder(t *testing.T) {
	env := newGlobalNoteTUIEnv(t)
	original := models.NewNote("Untitled Note")
	content, err := storage.RenderNoteContent(original)
	if err != nil {
		t.Fatal(err)
	}
	modified := strings.Replace(content, "title: Untitled Note", "title: Created globally", 1)
	modified += "\ncreated body\n"
	handled, cmd := env.shell.handleNotesMessages(ProjectNoteEditorDoneMsg{
		Location:        app.GlobalNoteLocation(),
		Original:        cloneNoteForCommand(original),
		OriginalContent: content,
		Modified:        modified,
		Create:          true,
	})
	if !handled || cmd == nil {
		t.Fatal("global create editor result did not schedule a typed transaction")
	}
	if notes, err := env.svc.NotesAt(app.GlobalNoteLocation()); err != nil || len(notes) != 0 {
		t.Fatalf("global placeholder persisted before commit: notes=%d err=%v", len(notes), err)
	}
	result := runProjectNoteMutation(t, func() any { return cmd() })
	env.shell.handleNotesMessages(result)
	if len(env.view.notes) != 1 || env.view.notes[0].Title != "Created globally" || env.view.notes[0].Body != "created body" {
		t.Fatalf("created global note = %#v", env.view.notes)
	}
}
