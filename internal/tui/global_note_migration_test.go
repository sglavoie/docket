package tui

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

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
	ledger := storage.NewWorkspaceLedger(filepath.Join(dataDir, ".tt", "undo"), []string{dataDir})
	commitSvc := storage.NewCommitServiceWithLedger(filepath.Join(dataDir, ".tt", "transactions"), []string{dataDir}, ledger)
	project, err := models.NewProject(uuid.New().String(), "destination", "")
	if err != nil {
		t.Fatalf("NewProject: %v", err)
	}
	project.Path = filepath.Join(dataDir, "projects", "destination")
	repo := newScenarioRepo(commitSvc, project)
	repo.seed(t)
	seedNoteMigrationCatalog(t, dataDir, project)
	dir := storage.GlobalNotesDir(dataDir)
	for _, title := range titles {
		note := models.NewNote(title)
		note.Body = title + " body"
		if err := storage.SaveNoteTo(dir, note); err != nil {
			t.Fatalf("seed global note %q: %v", title, err)
		}
	}

	svc := app.NewServiceWithArtifacts(repo, nil, nil, app.NewStorageWorkspaceNotePort(dataDir))
	view := NewCombinedNotesViewModelWithService(
		dir,
		project,
		app.GlobalNoteLocation(),
		svc,
		ledger,
	)
	view.SetSize(80, 40)
	views := NewViewRegistry()
	views.Register(ViewGlobalNotes, view)
	shell := &AppModel{
		appService: svc,
		views:      views,
	}
	setTestPresentationPath(shell, ViewGlobalNotes)
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
	env := newGlobalNoteTUIEnv(t, "Edit me")
	before := env.view.notes[0]
	updated := cloneNoteForCommand(before)
	updated.Body = "edited body"
	updated.ModifiedAt = time.Now()
	cmd := noteMutationCmd(env.svc, app.GlobalNoteLocation(), app.NoteBatchRequest{
		Operations: []app.NoteMutationRequest{{Kind: app.NoteUpdate, Location: app.GlobalNoteLocation(), Note: updated}},
		UndoAction: "Edit Note",
	}, "Edited", false)
	result := runProjectNoteMutation(t, func() any { return cmd() })
	if result.Location != app.GlobalNoteLocation() {
		t.Fatalf("result location = %#v", result.Location)
	}
	if env.view.notes[0] != before || env.view.notes[0].Body != "Edit me body" {
		t.Fatal("command mutated the global view cache before Update")
	}
	if disk := env.note(t, before.Filename); disk.Body != "edited body" {
		t.Fatal("typed global transaction did not persist body edit")
	}
	entry, ok, err := env.ledger.SelectNewestUndo(storage.GlobalScope())
	if err != nil || !ok {
		t.Fatalf("global ledger entry ok=%v err=%v", ok, err)
	}
	if len(entry.ProjectIDs) != 0 {
		t.Fatalf("global-only transaction carried project IDs: %v", entry.ProjectIDs)
	}
	env.shell.handleNotesMessages(result)
	if env.view.notes[0].Body != "edited body" || env.view.notes[0] == before {
		t.Fatal("Update did not reconcile the committed global note")
	}
}

func TestCombinedNotesUndoRedoUsesWorkspaceLedgerOrder(t *testing.T) {
	env := newGlobalNoteTUIEnv(t, "Scoped note")
	note := env.view.notes[0]
	updated := cloneNoteForCommand(note)
	updated.Body = "updated scoped body"
	updated.ModifiedAt = time.Now()
	cmd := noteMutationCmd(env.svc, app.GlobalNoteLocation(), app.NoteBatchRequest{
		Operations: []app.NoteMutationRequest{{Kind: app.NoteUpdate, Location: app.GlobalNoteLocation(), Note: updated}},
		UndoAction: "Edit Note",
	}, "Edited", false)
	noteMutation := runProjectNoteMutation(t, func() any { return cmd() })
	env.shell.handleNotesMessages(noteMutation)
	if env.note(t, note.Filename).Body != "updated scoped body" {
		t.Fatal("global note mutation did not persist")
	}

	taskResult, err := env.svc.AddTask(app.AddTaskRequest{ProjectID: env.project.ID, Title: "Newer project task"})
	if err != nil {
		t.Fatalf("AddTask: %v", err)
	}
	taskEntry, ok, err := env.ledger.SelectNewestUndo(storage.GlobalScope())
	if err != nil || !ok {
		t.Fatalf("newest workspace transaction ok=%v err=%v", ok, err)
	}

	undo, ok := env.view.undoDelete()().(ProjectNoteLedgerResultMsg)
	if !ok || undo.Entry.TransactionID != taskEntry.TransactionID {
		t.Fatalf("combined Notes undo selected %#v, want newest transaction %q", undo, taskEntry.TransactionID)
	}
	env.shell.handleNotesMessages(undo)
	if env.note(t, note.Filename).Body != "updated scoped body" {
		t.Fatal("undoing the newer task mutation changed the older note mutation")
	}
	if err := storage.LoadProject(env.project); err != nil {
		t.Fatalf("reload project after note undo: %v", err)
	}
	if task := env.project.GetTaskByID(taskResult.TaskID); task != nil {
		t.Fatalf("combined Notes undo retained newer project task: %#v", task)
	}

	redo, ok := env.view.redoNoteMutation()().(ProjectNoteLedgerResultMsg)
	if !ok || redo.Entry.TransactionID != taskEntry.TransactionID {
		t.Fatalf("combined Notes redo selected %#v, want newest transaction %q", redo, taskEntry.TransactionID)
	}
	env.shell.handleNotesMessages(redo)
	if env.note(t, note.Filename).Body != "updated scoped body" {
		t.Fatal("redoing the newer task mutation changed the older note mutation")
	}
	if err := storage.LoadProject(env.project); err != nil {
		t.Fatalf("reload project after note redo: %v", err)
	}
	if task := env.project.GetTaskByID(taskResult.TaskID); task == nil || task.Title != "Newer project task" {
		t.Fatalf("combined Notes redo did not restore newer project task: %#v", task)
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
	t.Skip("retired: combined Notes has no tags or bulk mutations")
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
