package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/external"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/tui/notes"
	"github.com/sglavoie/tt/internal/tui/operation"
	"github.com/sglavoie/tt/internal/tui/presentation"
	"github.com/sglavoie/tt/internal/tui/route"
	"github.com/sglavoie/tt/internal/tui/status"
)

// entityOperationApp wires the operation registry against a real note-capable
// service so note identities resolve through the same seam production uses.
func entityOperationApp(t *testing.T, titles ...string) (*AppModel, *models.Project) {
	t.Helper()
	model := typedTestApp(t)

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
	for _, title := range titles {
		note := models.NewNote(title)
		note.Body = title + " body"
		if err := saveTestNote(storage.NotesDir(project), note); err != nil {
			t.Fatalf("seed note %q: %v", title, err)
		}
	}

	model.domain.service = app.NewServiceWithArtifacts(repo, nil, nil, app.NewStorageWorkspaceNotePort(dataDir))
	model.domain.runtime = testRuntimeAt(dataDir, testConfigWithProjects(project))
	setTestAppProjects(model, project)
	return model, project
}

func executeEntityOperation(app *AppModel, id operation.OperationID, entity operation.EntityIdentity) operation.OperationExecutionResult {
	ctx := route.PresentationCommandContext(presentation.ViewCrossProjectOutliner)
	ctx.CurrentEntity = entity
	return app.operations.Executor.Execute(operation.OperationExecutionRequest{
		ID:      id,
		Context: ctx,
	})
}

func onlyNoteFilename(t *testing.T, model *AppModel, location app.NoteLocation) string {
	t.Helper()
	notes, err := model.domain.service.NotesAt(location)
	if err != nil {
		t.Fatalf("NotesAt: %v", err)
	}
	if len(notes) != 1 {
		t.Fatalf("seeded %d notes, want exactly 1", len(notes))
	}
	return notes[0].Filename
}

func TestNoteOperationsResolveTheNoteAtExecution(t *testing.T) {
	t.Setenv("TT_EDITOR", "true")
	t.Setenv("VISUAL", "")
	t.Setenv("EDITOR", "")
	model, project := entityOperationApp(t, "Design log")
	filename := onlyNoteFilename(t, model, app.ProjectNoteLocation(project.ID))

	result := executeEntityOperation(model, notes.OperationNoteOpen, operation.ProjectNoteEntity(project.ID, filename))
	open, ok := result.Message.(operation.OperationCommandResultMsg)
	if !ok || open.Cmd == nil {
		t.Fatalf("open note emitted %#v, want an external-editor command", result.Message)
	}

	move := executeEntityOperation(model, notes.OperationNoteMove, operation.ProjectNoteEntity(project.ID, filename))
	moveManager, ok := move.Message.(notes.ShowNotesManagerMsg)
	if !ok || moveManager.Scope != route.AllNotesScope() ||
		moveManager.Focus != (notes.NoteRowIdentity{
			Location: app.ProjectNoteLocation(project.ID),
			Filename: filename,
		}) ||
		!moveManager.MoveSource.Valid() ||
		moveManager.MoveSource.Location != app.ProjectNoteLocation(project.ID) {
		t.Fatalf("move note emitted %#v, want the searchable manager on its frozen source", move.Message)
	}

	// The note disappears after the row was built: the next execution must fail
	// rather than act on the stale identity.
	if err := os.Remove(filepath.Join(storage.NotesDir(project), filename)); err != nil {
		t.Fatalf("remove note: %v", err)
	}
	stale := executeEntityOperation(model, notes.OperationNoteOpen, operation.ProjectNoteEntity(project.ID, filename))
	if stale.Err == nil {
		t.Fatalf("open on a removed note emitted %#v, want an unavailable error", stale.Message)
	}
}

func TestNoteOperationsRejectTargetsOutsideTheirScope(t *testing.T) {
	model, project := entityOperationApp(t, "Design log")
	filename := onlyNoteFilename(t, model, app.ProjectNoteLocation(project.ID))

	// CurrentEntityIdentity degrades to the focused project, so an unguarded note
	// descriptor would silently borrow it.
	ctx := route.PresentationCommandContext(presentation.ViewCrossProjectOutliner)
	for _, descriptor := range notes.NoteOperationDescriptors() {
		if descriptor.Applicability == nil {
			t.Fatalf("%s carries no applicability guard", descriptor.ID)
		}
		if descriptor.Applicability(ctx) {
			t.Fatalf("%s was offered with only a project in focus", descriptor.ID)
		}
	}

	missing := executeEntityOperation(model, notes.OperationNoteOpen, operation.ProjectNoteEntity("ghost", filename))
	if missing.Err == nil {
		t.Fatalf("a note in a removed project emitted %#v, want an error", missing.Message)
	}
}

func TestNoteOperationsPreserveMalformedNoteRestrictions(t *testing.T) {
	model, project := entityOperationApp(t)
	dir := storage.NotesDir(project)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "broken.md"), []byte("no frontmatter here\n"), 0o644); err != nil {
		t.Fatalf("write malformed note: %v", err)
	}
	entity := operation.ProjectNoteEntity(project.ID, "broken.md")

	if result := executeEntityOperation(model, notes.OperationNoteMove, entity); result.Err == nil {
		t.Fatalf("move ran on a malformed note and emitted %#v", result.Message)
	}

	// Repair and removal stay reachable, otherwise a malformed note is stranded.
	rename := executeEntityOperation(model, notes.OperationNoteRename, entity)
	if rename.ValueRequired == nil || rename.ValueRequired.Provider != operation.ValueProviderNoteTitle {
		t.Fatalf("rename on a malformed note emitted %#v, want a title prompt", rename)
	}
	if delete := executeEntityOperation(model, notes.OperationNoteDelete, entity); delete.ConfirmationRequired == nil {
		t.Fatalf("delete on a malformed note emitted %#v, want a confirmation", delete)
	}
}

func TestNoteDeleteRunsOnlyAfterConfirmation(t *testing.T) {
	model, project := entityOperationApp(t, "Throwaway")
	filename := onlyNoteFilename(t, model, app.ProjectNoteLocation(project.ID))
	entity := operation.ProjectNoteEntity(project.ID, filename)

	result := executeEntityOperation(model, notes.OperationNoteDelete, entity)
	if result.ConfirmationRequired == nil || result.Message != nil {
		t.Fatalf("note delete ran without confirmation: %#v", result)
	}

	confirmed := model.operations.Executor.Execute(result.ConfirmationRequired.Request)
	command, ok := confirmed.Message.(operation.OperationCommandResultMsg)
	if !ok || command.Cmd == nil {
		t.Fatalf("confirmed note delete emitted %#v, want a mutation command", confirmed.Message)
	}
	mutation, ok := command.Cmd().(notes.ProjectNoteMutationResultMsg)
	if !ok {
		t.Fatalf("note delete produced %#v, want a note mutation result", command.Cmd())
	}
	if mutation.Location != app.ProjectNoteLocation(project.ID) {
		t.Fatalf("note delete committed to %#v, want the project collection", mutation.Location)
	}
	if _, err := os.Stat(filepath.Join(storage.NotesDir(project), filename)); !os.IsNotExist(err) {
		t.Fatalf("note file still present after delete: %v", err)
	}
}

// TestNoteCopyOperationsNameTheNoteInEitherCollection holds the two copies to the
// one thing that makes the pair worth having: y takes the row's primary text and
// Y the fuller qualified form. Two notes sharing a title — one global, one in a
// project — are the case that proves it, because the title alone cannot say
// which of them the user is holding and the full path always can.
func TestNoteCopyOperationsNameTheNoteInEitherCollection(t *testing.T) {
	model, project := entityOperationApp(t, "Design log")
	projectFile := onlyNoteFilename(t, model, app.ProjectNoteLocation(project.ID))

	globalDir := storage.GlobalNotesDir(model.domain.runtime.DataDir())
	globalNote := models.NewNote("Design log")
	globalNote.Body = "Design log body"
	if err := saveTestNote(globalDir, globalNote); err != nil {
		t.Fatalf("seed global note: %v", err)
	}

	var copied string
	withFakeClipboard(t, nil, &copied)

	for _, testCase := range []struct {
		name   string
		entity operation.EntityIdentity
		path   string
	}{
		{
			name:   "project",
			entity: operation.ProjectNoteEntity(project.ID, projectFile),
			path:   filepath.Join(storage.NotesDir(project), projectFile),
		},
		{
			name:   "global",
			entity: operation.GlobalNoteEntity(globalNote.Filename),
			path:   filepath.Join(globalDir, globalNote.Filename),
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			copied = ""
			title := executeEntityOperation(model, notes.OperationNoteCopyTitle, testCase.entity)
			if title.Err != nil {
				t.Fatalf("copy note title: %v", title.Err)
			}
			if copied != "Design log" {
				t.Fatalf("copied title = %q, want %q", copied, "Design log")
			}
			if title.Status != "Note title copied to clipboard" {
				t.Fatalf("copy title status = %q, want the clipboard confirmation", title.Status)
			}

			copied = ""
			path := executeEntityOperation(model, notes.OperationNoteCopyPath, testCase.entity)
			if path.Err != nil {
				t.Fatalf("copy note path: %v", path.Err)
			}
			if copied != testCase.path {
				t.Fatalf("copied path = %q, want %q", copied, testCase.path)
			}
			if path.Status != "Note path copied to clipboard" {
				t.Fatalf("copy path status = %q, want the clipboard confirmation", path.Status)
			}
			if _, err := os.Stat(copied); err != nil {
				t.Fatalf("copied path does not name a file on disk: %v", err)
			}
		})
	}
}

// TestNoteCopyOperationsSurviveAMalformedNoteAndAFailedClipboard covers the two
// ways a copy can meet something broken. A note that will not parse still has a
// title and a path, so copying stays available where moving does not; a
// clipboard that refuses reports the failure rather than claiming a copy.
func TestNoteCopyOperationsSurviveAMalformedNoteAndAFailedClipboard(t *testing.T) {
	model, project := entityOperationApp(t)
	dir := storage.NotesDir(project)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "broken.md"), []byte("no frontmatter here\n"), 0o644); err != nil {
		t.Fatalf("write malformed note: %v", err)
	}
	entity := operation.ProjectNoteEntity(project.ID, "broken.md")

	var copied string
	withFakeClipboard(t, nil, &copied)
	if result := executeEntityOperation(model, notes.OperationNoteCopyPath, entity); result.Err != nil {
		t.Fatalf("copy path on a malformed note: %v", result.Err)
	}
	if want := filepath.Join(dir, "broken.md"); copied != want {
		t.Fatalf("copied path = %q, want %q", copied, want)
	}
	if result := executeEntityOperation(model, notes.OperationNoteCopyTitle, entity); result.Err != nil {
		t.Fatalf("copy title on a malformed note: %v", result.Err)
	}
	if copied == "" {
		t.Fatal("copy title on a malformed note copied nothing, want the filename fallback title")
	}

	withFakeClipboard(t, external.ErrClipboardUnsupported, nil)
	failed := executeEntityOperation(model, notes.OperationNoteCopyTitle, entity)
	if failed.Err == nil {
		t.Fatalf("a refusing clipboard emitted %#v, want an error", failed.Message)
	}
	if !strings.Contains(failed.Err.Error(), "copy failed") {
		t.Fatalf("clipboard failure = %v, want it named as a copy failure", failed.Err)
	}
}

// TestNotesBrowseKeysReachTheCopyOperations presses the two keys on the live
// manager rather than executing their IDs. The browse switch in the Notes update
// loop is what decides whether a key ever reaches the grammar, so a binding the
// strategy declares and that switch drops would pass every other Notes test.
func TestNotesBrowseKeysReachTheCopyOperations(t *testing.T) {
	model, project := entityOperationApp(t, "Design log")
	filename := onlyNoteFilename(t, model, app.ProjectNoteLocation(project.ID))

	view := notes.NewNotesViewModelWithService(
		storage.GlobalNotesDir(model.domain.runtime.DataDir()),
		route.AllNotesScope(),
		model.domain.service,
		nil,
		project,
	)
	model.manageHub.MountModule(route.ManageParams{Module: route.ManageNotes, NotesScope: view.Scope}, view)
	view.SetOperationSystem(model.GetOperationRegistry(), model.dispatchOperationKey)
	view.SetSize(80, 24)
	if len(view.VisibleRows) != 1 {
		t.Fatalf("notes rows = %d, want the one seeded note", len(view.VisibleRows))
	}

	var copied string
	withFakeClipboard(t, nil, &copied)

	_, cmd := view.Update(keyPress("y"))
	if cmd == nil {
		t.Fatal("y produced no command on the Notes manager")
	}
	if copied != "Design log" {
		t.Fatalf("y copied %q, want the note title", copied)
	}
	// The status strip is the only place a copy is visible, so the key has to
	// carry the confirmation out of the handler and not just the copied value.
	if status, ok := cmd().(status.StatusMsg); !ok || string(status) != "Note title copied to clipboard" {
		t.Fatalf("y emitted %#v, want the clipboard confirmation status", cmd())
	}

	copied = ""
	_, cmd = view.Update(keyPress("Y"))
	if cmd == nil {
		t.Fatal("Y produced no command on the Notes manager")
	}
	if want := filepath.Join(storage.NotesDir(project), filename); copied != want {
		t.Fatalf("Y copied %q, want %q", copied, want)
	}
	if status, ok := cmd().(status.StatusMsg); !ok || string(status) != "Note path copied to clipboard" {
		t.Fatalf("Y emitted %#v, want the clipboard confirmation status", cmd())
	}
}
