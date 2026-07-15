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
	view    *NotesViewModel
	shell   *AppModel
}

func newProjectNoteTUIEnv(t *testing.T, titles ...string) *projectNoteTUIEnv {
	t.Helper()
	dataDir := t.TempDir()
	ledger := storage.NewWorkspaceLedger(filepath.Join(dataDir, ".tt", "undo"), 0, []string{dataDir})
	commitSvc := storage.NewCommitServiceWithLedger(filepath.Join(dataDir, ".tt", "transactions"), []string{dataDir}, ledger)

	project, err := models.NewProject(uuid.New().String(), "notes", "")
	if err != nil {
		t.Fatalf("NewProject: %v", err)
	}
	project.Path = filepath.Join(dataDir, "projects", "notes")
	repo := newScenarioRepo(commitSvc, project)
	repo.seed(t)
	for _, title := range titles {
		note := models.NewNote(title)
		note.Body = title + " body"
		if err := storage.SaveNote(project, note); err != nil {
			t.Fatalf("seed note %q: %v", title, err)
		}
	}

	svc := app.NewServiceWithArtifacts(repo, nil, nil, app.NewStorageWorkspaceNotePort(dataDir), nil)
	view := NewNotesViewModelWithService(project, svc, ledger)
	view.SetSize(80, 40)
	views := NewViewRegistry()
	views.Register(ViewNotes, view)
	shell := &AppModel{
		appService: svc,
		views:      views,
		viewStack:  []ViewType{ViewNotes},
		navHistory: NewNavHistory(),
	}
	return &projectNoteTUIEnv{dataDir: dataDir, ledger: ledger, svc: svc, project: project, view: view, shell: shell}
}

func (e *projectNoteTUIEnv) note(t *testing.T, filename string) *models.Note {
	t.Helper()
	note, err := storage.LoadNote(e.project, filename)
	if err != nil {
		t.Fatalf("LoadNote(%s): %v", filename, err)
	}
	return note
}

func runProjectNoteMutation(t *testing.T, cmd func() any) ProjectNoteMutationResultMsg {
	t.Helper()
	msg := cmd()
	result, ok := msg.(ProjectNoteMutationResultMsg)
	if !ok {
		if failure, failed := msg.(ProjectNoteMutationErrorMsg); failed {
			t.Fatalf("project note mutation failed: %v", failure.Err)
		}
		t.Fatalf("project note mutation message = %T", msg)
	}
	return result
}

func TestProjectNoteMutationRefreshesOnlyAfterCommittedResult(t *testing.T) {
	env := newProjectNoteTUIEnv(t, "Pin me")
	before := env.view.notes[0]
	if before.Pinned {
		t.Fatal("seed note unexpectedly pinned")
	}

	cmd := env.view.togglePin()
	msg := runProjectNoteMutation(t, func() any { return cmd() })
	if env.view.notes[0] != before || env.view.notes[0].Pinned {
		t.Fatal("command goroutine mutated the live notes view cache")
	}
	if disk := env.note(t, before.Filename); !disk.Pinned {
		t.Fatal("typed transaction did not persist pin")
	}

	env.shell.handleNotesMessages(msg)
	if !env.view.notes[0].Pinned || env.view.notes[0] == before {
		t.Fatal("Update handling did not replace the cache from committed state")
	}
}

func TestProjectNoteStaleCommitPreservesDiskAndViewCache(t *testing.T) {
	env := newProjectNoteTUIEnv(t, "Race")
	cached := env.view.notes[0]
	concurrent := env.note(t, cached.Filename)
	concurrent.Body = "concurrent body"
	if err := storage.SaveNote(env.project, concurrent); err != nil {
		t.Fatalf("concurrent save: %v", err)
	}

	cmd := env.view.togglePin()
	msg, ok := cmd().(ProjectNoteMutationErrorMsg)
	if !ok {
		t.Fatalf("stale mutation = %T, want ProjectNoteMutationErrorMsg", cmd())
	}
	var conflict *storage.ArtifactConflictError
	if !errors.As(msg.Err, &conflict) {
		t.Fatalf("stale error = %T %v, want ArtifactConflictError", msg.Err, msg.Err)
	}
	env.shell.handleNotesMessages(msg)
	if env.view.notes[0] != cached || env.view.notes[0].Pinned || env.view.notes[0].Body != "Race body" {
		t.Fatal("failed commit changed the live view cache")
	}
	if disk := env.note(t, cached.Filename); disk.Pinned || disk.Body != "concurrent body" {
		t.Fatalf("failed commit changed canonical note: pinned=%v body=%q", disk.Pinned, disk.Body)
	}
}

func TestProjectNoteBulkDeleteIsAtomicAndKeepsSelectionOnFailure(t *testing.T) {
	env := newProjectNoteTUIEnv(t, "Alpha", "Beta")
	for _, note := range env.view.notes {
		env.view.selectedNotes[note.Filename] = true
	}
	env.view.multiSelectMode = true
	stale := env.view.notes[1]
	concurrent := env.note(t, stale.Filename)
	concurrent.Body = "newer beta"
	if err := storage.SaveNote(env.project, concurrent); err != nil {
		t.Fatalf("concurrent save: %v", err)
	}

	cmd := env.view.performBulkDelete()
	msg, ok := cmd().(ProjectNoteMutationErrorMsg)
	if !ok {
		t.Fatalf("bulk delete = %T, want ProjectNoteMutationErrorMsg", cmd())
	}
	env.shell.handleNotesMessages(msg)
	if !env.view.multiSelectMode || len(env.view.selectedNotes) != 2 {
		t.Fatal("rejected batch cleared the retryable selection")
	}
	for _, note := range env.view.notes {
		if _, err := storage.LoadNote(env.project, note.Filename); err != nil {
			t.Fatalf("atomic batch partially deleted %s: %v", note.Filename, err)
		}
	}
}

func TestProjectNoteBulkTagsAndLinksCommitAsSingleBatches(t *testing.T) {
	env := newProjectNoteTUIEnv(t, "One", "Two")
	notes := append([]*models.Note(nil), env.view.notes...)
	env.shell.pendingBulkNotes = newNoteMutationRefs(app.ProjectNoteLocation(env.project.ID), notes)
	env.view.multiSelectMode = true
	for _, note := range notes {
		env.view.selectedNotes[note.Filename] = true
	}

	tagCmd := env.shell.applyProjectNoteTags(env.project.ID, notes, []string{"batch"}, true)
	tagResult := runProjectNoteMutation(t, func() any { return tagCmd() })
	if len(tagResult.Result.Results) != 2 || tagResult.Result.Results[0].TransactionID != tagResult.Result.Results[1].TransactionID {
		t.Fatal("bulk tag did not report one shared transaction")
	}
	env.shell.handleNotesMessages(tagResult)
	for _, note := range env.view.notes {
		if got := env.note(t, note.Filename); len(got.Tags) != 1 || got.Tags[0] != "batch" {
			t.Fatalf("bulk tag missing from %s: %v", note.Filename, got.Tags)
		}
	}

	env.shell.pendingBulkNotes = newNoteMutationRefs(app.ProjectNoteLocation(env.project.ID), env.view.notes)
	env.shell.viewStack = []ViewType{ViewNotes, ViewNoteTaskLinker}
	handled, linkCmd := env.shell.handleNotesMessages(BulkNoteLinkDoneMsg{
		ProjectID:   env.project.ID,
		LinkChanges: map[string]bool{"task-1": true},
	})
	if !handled {
		t.Fatal("bulk link result was not handled")
	}
	linkResult := runProjectNoteMutation(t, func() any { return linkCmd() })
	if len(linkResult.Result.Results) != 2 || linkResult.Result.Results[0].TransactionID != linkResult.Result.Results[1].TransactionID {
		t.Fatal("bulk link did not report one shared transaction")
	}
	env.shell.handleNotesMessages(linkResult)
	for _, note := range env.view.notes {
		if got := env.note(t, note.Filename); !got.HasLink("task-1") {
			t.Fatalf("bulk link missing from %s", note.Filename)
		}
	}
}

func TestProjectNoteTaskLinkerDoesNotMutateSourceBeforeCommit(t *testing.T) {
	env := newProjectNoteTUIEnv(t, "Linked")
	task, err := models.NewTask("task-1", 1, models.TaskTypeTask, "Task")
	if err != nil {
		t.Fatal(err)
	}
	env.project.Tasks = []*models.Task{task}
	source := env.view.notes[0]
	linker := NewNoteTaskLinkerModelWithService(env.project, source, env.svc, false)
	cmd := linker.toggleLink(task)
	msg, ok := cmd().(noteTaskLinkCommitResultMsg)
	if !ok {
		t.Fatalf("link commit = %T", cmd())
	}
	if source.HasLink(task.ID) || linker.note.HasLink(task.ID) {
		t.Fatal("link command mutated a live or overlay note before Update")
	}
	linker.Update(msg)
	if !linker.note.HasLink(task.ID) || source.HasLink(task.ID) {
		t.Fatal("linker failed to reconcile committed copy without mutating source")
	}
}

func TestProjectNoteEditorResultCommitsTempContentAndRejectsMalformedOutput(t *testing.T) {
	env := newProjectNoteTUIEnv(t, "Editable")
	original := env.view.notes[0]
	originalContent, err := storage.RenderNoteContent(original)
	if err != nil {
		t.Fatal(err)
	}
	modified := strings.Replace(originalContent, "title: Editable", "title: Edited", 1)
	modified = strings.Replace(modified, "Editable body", "edited body", 1)
	handled, cmd := env.shell.handleNotesMessages(ProjectNoteEditorDoneMsg{
		ProjectID:       env.project.ID,
		Original:        cloneNoteForCommand(original),
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
	committed := result.Result.Results[0].Note
	if committed == nil || committed.Title != "Edited" || committed.Body != "edited body" {
		t.Fatalf("committed editor note = %#v", committed)
	}
	if _, statErr := os.Stat(filepath.Join(storage.NotesDir(env.project), original.Filename)); !os.IsNotExist(statErr) {
		t.Fatalf("rename edit left original canonical file: %v", statErr)
	}

	before, err := os.ReadFile(filepath.Join(storage.NotesDir(env.project), committed.Filename))
	if err != nil {
		t.Fatal(err)
	}
	_, malformedCmd := env.shell.handleNotesMessages(ProjectNoteEditorDoneMsg{
		ProjectID:       env.project.ID,
		Original:        cloneNoteForCommand(committed),
		OriginalContent: modified,
		Modified:        "malformed output without frontmatter",
	})
	malformed, ok := malformedCmd().(ProjectNoteMutationErrorMsg)
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

	_, unchangedCmd := env.shell.handleNotesMessages(ProjectNoteEditorDoneMsg{
		ProjectID:       env.project.ID,
		Original:        cloneNoteForCommand(original),
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

	_, createCmd := env.shell.handleNotesMessages(ProjectNoteEditorDoneMsg{
		ProjectID:       env.project.ID,
		Original:        cloneNoteForCommand(original),
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
	filename := env.view.notes[0].Filename
	mutation := runProjectNoteMutation(t, func() any { return env.view.togglePin()() })
	env.shell.handleNotesMessages(mutation)
	if !env.note(t, filename).Pinned {
		t.Fatal("pin mutation did not commit")
	}

	undo, ok := env.view.undoDelete()().(ProjectNoteLedgerResultMsg)
	if !ok || undo.Redo {
		t.Fatalf("undo result = %#v", undo)
	}
	env.shell.handleNotesMessages(undo)
	if env.note(t, filename).Pinned || env.view.notes[0].Pinned {
		t.Fatal("project-scoped workspace undo did not refresh restored note")
	}

	redo, ok := env.view.redoNoteMutation()().(ProjectNoteLedgerResultMsg)
	if !ok || !redo.Redo {
		t.Fatalf("redo result = %#v", redo)
	}
	env.shell.handleNotesMessages(redo)
	if !env.note(t, filename).Pinned || !env.view.notes[0].Pinned {
		t.Fatal("project-scoped workspace redo did not refresh replayed note")
	}
}
