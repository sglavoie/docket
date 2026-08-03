package tui

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
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

	model.appService = app.NewServiceWithArtifacts(repo, nil, nil, app.NewStorageWorkspaceNotePort(dataDir))
	model.runtime = testRuntimeAt(dataDir, testConfigWithProjects(project))
	setTestAppProjects(model, project)
	return model, project
}

func executeEntityOperation(app *AppModel, id OperationID, entity EntityIdentity) OperationExecutionResult {
	ctx := PresentationCommandContext(ViewCrossProjectOutliner)
	ctx.CurrentEntity = entity
	return app.operationExecutor.Execute(OperationExecutionRequest{
		ID:      id,
		Context: ctx,
	})
}

func onlyNoteFilename(t *testing.T, model *AppModel, location app.NoteLocation) string {
	t.Helper()
	notes, err := model.appService.NotesAt(location)
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

	result := executeEntityOperation(model, operationNoteOpen, ProjectNoteEntity(project.ID, filename))
	open, ok := result.Message.(OperationCommandResultMsg)
	if !ok || open.Cmd == nil {
		t.Fatalf("open note emitted %#v, want an external-editor command", result.Message)
	}

	move := executeEntityOperation(model, operationNoteMove, ProjectNoteEntity(project.ID, filename))
	moveManager, ok := move.Message.(ShowNotesManagerMsg)
	if !ok || moveManager.Scope != AllNotesScope() ||
		moveManager.Focus != (NoteRowIdentity{
			Location: app.ProjectNoteLocation(project.ID),
			Filename: filename,
		}) ||
		!moveManager.MoveSource.valid() ||
		moveManager.MoveSource.Location != app.ProjectNoteLocation(project.ID) {
		t.Fatalf("move note emitted %#v, want the searchable manager on its frozen source", move.Message)
	}

	// The note disappears after the row was built: the next execution must fail
	// rather than act on the stale identity.
	if err := os.Remove(filepath.Join(storage.NotesDir(project), filename)); err != nil {
		t.Fatalf("remove note: %v", err)
	}
	stale := executeEntityOperation(model, operationNoteOpen, ProjectNoteEntity(project.ID, filename))
	if stale.Err == nil {
		t.Fatalf("open on a removed note emitted %#v, want an unavailable error", stale.Message)
	}
}

func TestNoteOperationsRejectTargetsOutsideTheirScope(t *testing.T) {
	model, project := entityOperationApp(t, "Design log")
	filename := onlyNoteFilename(t, model, app.ProjectNoteLocation(project.ID))

	// currentEntityIdentity degrades to the focused project, so an unguarded note
	// descriptor would silently borrow it.
	ctx := PresentationCommandContext(ViewCrossProjectOutliner)
	if noteOperationApplicable(ctx) {
		t.Fatal("a note operation was offered with only a project in focus")
	}

	missing := executeEntityOperation(model, operationNoteOpen, ProjectNoteEntity("ghost", filename))
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
	entity := ProjectNoteEntity(project.ID, "broken.md")

	if result := executeEntityOperation(model, operationNoteMove, entity); result.Err == nil {
		t.Fatalf("move ran on a malformed note and emitted %#v", result.Message)
	}

	// Repair and removal stay reachable, otherwise a malformed note is stranded.
	rename := executeEntityOperation(model, operationNoteRename, entity)
	if rename.ValueRequired == nil || rename.ValueRequired.Provider != ValueProviderNoteTitle {
		t.Fatalf("rename on a malformed note emitted %#v, want a title prompt", rename)
	}
	if delete := executeEntityOperation(model, operationNoteDelete, entity); delete.ConfirmationRequired == nil {
		t.Fatalf("delete on a malformed note emitted %#v, want a confirmation", delete)
	}
}

func TestNoteDeleteRunsOnlyAfterConfirmation(t *testing.T) {
	model, project := entityOperationApp(t, "Throwaway")
	filename := onlyNoteFilename(t, model, app.ProjectNoteLocation(project.ID))
	entity := ProjectNoteEntity(project.ID, filename)

	result := executeEntityOperation(model, operationNoteDelete, entity)
	if result.ConfirmationRequired == nil || result.Message != nil {
		t.Fatalf("note delete ran without confirmation: %#v", result)
	}

	confirmed := model.operationExecutor.Execute(result.ConfirmationRequired.Request)
	command, ok := confirmed.Message.(OperationCommandResultMsg)
	if !ok || command.Cmd == nil {
		t.Fatalf("confirmed note delete emitted %#v, want a mutation command", confirmed.Message)
	}
	mutation, ok := command.Cmd().(ProjectNoteMutationResultMsg)
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
