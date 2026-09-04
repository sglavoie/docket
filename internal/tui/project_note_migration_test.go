package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/tui/notes"
	"github.com/sglavoie/tt/internal/tui/presentation"
	"github.com/sglavoie/tt/internal/tui/route"
)

// CommitArtifactTransaction makes the scenario repository exercise the same
// note-only commit and ledger boundary as the production TUI repository.
func (r *scenarioRepo) CommitArtifactTransaction(request storage.ArtifactCommitRequest) (string, error) {
	return r.svc.CommitArtifactTransaction(request)
}

type projectNoteTUIEnv struct {
	dataDir string
	ledger  *storage.WorkspaceLedger
	svc     *app.Service
	project *models.Project
	view    *notes.NotesViewModel
	shell   *AppModel
}

func newProjectNoteTUIEnv(t *testing.T, titles ...string) *projectNoteTUIEnv {
	t.Helper()
	dataDir := t.TempDir()
	ledger := storage.NewWorkspaceLedger(filepath.Join(dataDir, ".tt", "undo"), []string{dataDir})
	commitSvc := storage.NewCommitServiceWithLedger(filepath.Join(dataDir, ".tt", "transactions"), []string{dataDir}, ledger)

	project, err := models.NewProject(uuid.New().String(), "notes", "")
	if err != nil {
		t.Fatalf("NewProject: %v", err)
	}
	project.Path = filepath.Join(dataDir, "projects", "notes")
	repo := newScenarioRepo(commitSvc, project)
	repo.seed(t)
	seedNoteMigrationCatalog(t, dataDir, project)
	for _, title := range titles {
		note := models.NewNote(title)
		note.Body = title + " body"
		if err := saveTestNote(storage.NotesDir(project), note); err != nil {
			t.Fatalf("seed note %q: %v", title, err)
		}
	}

	svc := app.NewServiceWithArtifacts(repo, nil, nil, app.NewStorageWorkspaceNotePort(dataDir))
	view := notes.NewNotesViewModelWithService(
		storage.GlobalNotesDir(dataDir),
		route.AllNotesScope(),
		svc,
		ledger,
		project,
	)
	mountNotesManager(t, view, route.AllNotesScope())
	view.SetSize(80, 40)
	views := route.NewViewRegistry()
	views.Register(presentation.ViewNotes, view)
	shell := &AppModel{
		appService: svc,
		viewNav:    viewNavigationController{views: views},
	}
	setTestPresentationPath(shell, presentation.ViewNotes)
	return &projectNoteTUIEnv{dataDir: dataDir, ledger: ledger, svc: svc, project: project, view: view, shell: shell}
}

func seedNoteMigrationCatalog(t *testing.T, dataDir string, projects ...*models.Project) {
	t.Helper()
	store := config.NewDomainFileStore(dataDir)
	catalog := config.NewWorkspaceCatalog()
	for _, project := range projects {
		if err := catalog.UpsertProject(project); err != nil {
			t.Fatalf("seed workspace catalog project: %v", err)
		}
	}
	if err := store.SaveCatalog(catalog); err != nil {
		t.Fatalf("seed workspace catalog: %v", err)
	}
}

func (e *projectNoteTUIEnv) note(t *testing.T, filename string) *models.Note {
	t.Helper()
	note, err := storage.LoadNoteFrom(storage.NotesDir(e.project), filename)
	if err != nil {
		t.Fatalf("LoadNote(%s): %v", filename, err)
	}
	return note
}

func runProjectNoteMutation(t *testing.T, cmd func() any) notes.ProjectNoteMutationResultMsg {
	t.Helper()
	msg := cmd()
	result, ok := msg.(notes.ProjectNoteMutationResultMsg)
	if !ok {
		if failure, failed := msg.(notes.ProjectNoteMutationErrorMsg); failed {
			t.Fatalf("project note mutation failed: %v", failure.Err)
		}
		t.Fatalf("project note mutation message = %T", msg)
	}
	return result
}

func writeMalformedProjectNote(t *testing.T, env *projectNoteTUIEnv, filename string, raw []byte) string {
	t.Helper()
	dir := storage.NotesDir(env.project)
	if err := os.MkdirAll(dir, storage.DirPermissions); err != nil {
		t.Fatalf("create notes directory: %v", err)
	}
	path := filepath.Join(dir, filename)
	if err := os.WriteFile(path, raw, storage.FilePermissions); err != nil {
		t.Fatalf("write malformed note: %v", err)
	}
	env.view.LoadNotes()
	return path
}

func projectNoteIndex(t *testing.T, notes []*models.Note, filename string) int {
	t.Helper()
	for i, note := range notes {
		if note.Filename == filename {
			return i
		}
	}
	t.Fatalf("note %q not found", filename)
	return -1
}

func noteModels(rows []notes.LocatedNoteRow) []*models.Note {
	notes := make([]*models.Note, len(rows))
	for i := range rows {
		notes[i] = rows[i].Note
	}
	return notes
}

func TestProjectNoteMutationRefreshesOnlyAfterCommittedResult(t *testing.T) {
	env := newProjectNoteTUIEnv(t, "Edit me")
	before := env.view.VisibleRows[0].Note
	updated := notes.CloneNoteForCommand(before)
	updated.Body = "edited body"
	updated.ModifiedAt = time.Now()
	location := app.ProjectNoteLocation(env.project.ID)
	cmd := notes.NoteMutationCmd(env.svc, location, app.NoteBatchRequest{
		Operations: []app.NoteMutationRequest{{Kind: app.NoteUpdate, Location: location, Note: updated}},
		UndoAction: "Edit Note",
	}, "Edited")
	msg := runProjectNoteMutation(t, func() any { return cmd() })
	if env.view.VisibleRows[0].Note != before || env.view.VisibleRows[0].Note.Body != "Edit me body" {
		t.Fatal("command goroutine mutated the live notes view cache")
	}
	if disk := env.note(t, before.Filename); disk.Body != "edited body" {
		t.Fatal("typed transaction did not persist body edit")
	}

	env.shell.handleNotesMessages(msg)
	if env.view.VisibleRows[0].Note.Body != "edited body" || env.view.VisibleRows[0].Note == before {
		t.Fatal("Update handling did not replace the cache from committed state")
	}
}

func TestProjectMalformedNoteDelete(t *testing.T) {
	t.Run("single", func(t *testing.T) {
		env := newProjectNoteTUIEnv(t)
		path := writeMalformedProjectNote(t, env, "malformed-single.md", []byte("missing frontmatter\n"))
		notes := noteModels(env.view.VisibleRows)
		index := projectNoteIndex(t, notes, "malformed-single.md")
		if notes[index].ParseError == "" || notes[index].RawRevision == "" {
			t.Fatalf("view did not retain malformed note evidence: %+v", notes[index])
		}
		env.view.PendingDelete = env.view.VisibleRows[index].Identity()

		result := runProjectNoteMutation(t, func() any { return env.view.PerformDelete()() })
		if len(result.Result.Results) != 1 || result.Result.Results[0].Filename != "malformed-single.md" {
			t.Fatalf("single malformed delete result = %+v", result.Result)
		}
		env.shell.handleNotesMessages(result)
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("single malformed note still exists: %v", err)
		}
		if len(env.view.VisibleRows) != 0 {
			t.Fatalf("view retained deleted malformed note: %+v", env.view.VisibleRows)
		}
	})
}

func TestProjectNoteEditorResultCommitsTempContentAndRejectsMalformedOutput(t *testing.T) {
	env := newProjectNoteTUIEnv(t, "Editable")
	original := env.view.VisibleRows[0].Note
	originalContent, err := storage.RenderNoteContent(original)
	if err != nil {
		t.Fatal(err)
	}
	modified := strings.Replace(originalContent, "title: Editable", "title: Edited", 1)
	modified = strings.Replace(modified, "Editable body", "edited body", 1)
	handled, cmd := env.shell.handleNotesMessages(notes.ProjectNoteEditorDoneMsg{
		ProjectID:       env.project.ID,
		Original:        notes.CloneNoteForCommand(original),
		OriginalContent: originalContent,
		Modified:        modified,
	})
	if !handled || cmd == nil {
		t.Fatal("valid editor result did not schedule typed commit")
	}
	if disk := env.note(t, original.Filename); disk.Title != "Editable" {
		t.Fatal("editor result changed canonical note before commit command")
	}
	result := runProjectNoteMutation(t, func() any { return cmd() })
	env.shell.handleNotesMessages(result)
	committed := env.note(t, result.Result.Results[0].Filename)
	if committed.Title != "Edited" || committed.Body != "edited body" {
		t.Fatalf("committed editor note = %#v", committed)
	}
	if _, statErr := os.Stat(filepath.Join(storage.NotesDir(env.project), original.Filename)); !os.IsNotExist(statErr) {
		t.Fatalf("rename edit left original canonical file: %v", statErr)
	}

	before, err := os.ReadFile(filepath.Join(storage.NotesDir(env.project), committed.Filename))
	if err != nil {
		t.Fatal(err)
	}
	_, malformedCmd := env.shell.handleNotesMessages(notes.ProjectNoteEditorDoneMsg{
		ProjectID:       env.project.ID,
		Original:        notes.CloneNoteForCommand(committed),
		OriginalContent: modified,
		Modified:        "malformed output without frontmatter",
	})
	malformed, ok := malformedCmd().(notes.ProjectNoteMutationErrorMsg)
	if !ok || malformed.Err == nil {
		t.Fatalf("malformed editor result = %#v", malformedCmd())
	}
	after, err := os.ReadFile(filepath.Join(storage.NotesDir(env.project), committed.Filename))
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(before) {
		t.Fatal("malformed temporary editor output changed canonical note")
	}
}

func TestProjectNoteEditorCreateDoesNotPersistPlaceholder(t *testing.T) {
	env := newProjectNoteTUIEnv(t)
	original := models.NewNote("Untitled Note")
	originalContent, err := storage.RenderNoteContent(original)
	if err != nil {
		t.Fatal(err)
	}
	modified := strings.Replace(originalContent, "title: Untitled Note", "title: Created in Temp", 1)
	modified += "\ncreated body\n"

	_, unchangedCmd := env.shell.handleNotesMessages(notes.ProjectNoteEditorDoneMsg{
		ProjectID:       env.project.ID,
		Original:        notes.CloneNoteForCommand(original),
		OriginalContent: originalContent,
		Modified:        originalContent,
		Create:          true,
	})
	if unchangedCmd != nil {
		t.Fatal("unchanged create editor should not schedule a transaction")
	}
	if notes, notesErr := env.svc.NotesAt(app.ProjectNoteLocation(env.project.ID)); notesErr != nil || len(notes) != 0 {
		t.Fatalf("unchanged editor persisted placeholder: notes=%d err=%v", len(notes), notesErr)
	}

	_, createCmd := env.shell.handleNotesMessages(notes.ProjectNoteEditorDoneMsg{
		ProjectID:       env.project.ID,
		Original:        notes.CloneNoteForCommand(original),
		OriginalContent: originalContent,
		Modified:        modified,
		Create:          true,
	})
	result := runProjectNoteMutation(t, func() any { return createCmd() })
	env.shell.handleNotesMessages(result)
	notes, err := env.svc.NotesAt(app.ProjectNoteLocation(env.project.ID))
	if err != nil || len(notes) != 1 {
		t.Fatalf("created note lookup: notes=%d err=%v", len(notes), err)
	}
	if notes[0].Title != "Created in Temp" || notes[0].Filename == original.Filename {
		t.Fatalf("created note = %#v", notes[0])
	}
}

func TestProjectNoteWorkspaceLedgerUndoRedoRefreshesCommittedState(t *testing.T) {
	env := newProjectNoteTUIEnv(t, "Undoable")
	filename := env.view.VisibleRows[0].Note.Filename
	updated := notes.CloneNoteForCommand(env.view.VisibleRows[0].Note)
	updated.Body = "undoable edit"
	updated.ModifiedAt = time.Now()
	location := app.ProjectNoteLocation(env.project.ID)
	cmd := notes.NoteMutationCmd(env.svc, location, app.NoteBatchRequest{
		Operations: []app.NoteMutationRequest{{Kind: app.NoteUpdate, Location: location, Note: updated}},
		UndoAction: "Edit Note",
	}, "Edited")
	mutation := runProjectNoteMutation(t, func() any { return cmd() })
	env.shell.handleNotesMessages(mutation)
	if env.note(t, filename).Body != "undoable edit" {
		t.Fatal("edit mutation did not commit")
	}

	undo, ok := env.view.UndoDelete()().(notes.ProjectNoteLedgerResultMsg)
	if !ok || undo.Redo {
		t.Fatalf("undo result = %#v", undo)
	}
	env.shell.handleNotesMessages(undo)
	if env.note(t, filename).Body != "Undoable body" || env.view.VisibleRows[0].Note.Body != "Undoable body" {
		t.Fatal("project-scoped workspace undo did not refresh restored note")
	}

	redo, ok := env.view.RedoNoteMutation()().(notes.ProjectNoteLedgerResultMsg)
	if !ok || !redo.Redo {
		t.Fatalf("redo result = %#v", redo)
	}
	env.shell.handleNotesMessages(redo)
	if env.note(t, filename).Body != "undoable edit" || env.view.VisibleRows[0].Note.Body != "undoable edit" {
		t.Fatal("project-scoped workspace redo did not refresh replayed note")
	}
}
