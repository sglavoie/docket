package tui

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

// entityOperationApp wires the operation registry against a real note-capable
// service so note identities resolve through the same seam production uses.
func entityOperationApp(t *testing.T, titles ...string) (*AppModel, *models.Project) {
	t.Helper()
	model := typedTestApp(t)

	dataDir := t.TempDir()
	ledger := storage.NewWorkspaceLedger(filepath.Join(dataDir, ".tt", "undo"), testWorkspaceLedgerDepth, []string{dataDir})
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

	model.appService = app.NewServiceWithArtifacts(repo, nil, nil, app.NewStorageWorkspaceNotePort(dataDir), nil)
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
		Source:  OperationSourceFinder,
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
	model, project := entityOperationApp(t, "Design log")
	filename := onlyNoteFilename(t, model, app.ProjectNoteLocation(project.ID))

	result := executeEntityOperation(model, operationNoteOpen, ProjectNoteEntity(project.ID, filename))
	open, ok := result.Message.(ShowNotesMsg)
	if !ok || open.Project == nil || open.Project.ID != project.ID || open.Focus != filename {
		t.Fatalf("open note emitted %#v, want the seeded project note", result.Message)
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

	// A project note names a project collection, so the global-only move must not
	// be offered for it.
	ctx := PresentationCommandContext(ViewCrossProjectOutliner)
	ctx.CurrentEntity = ProjectNoteEntity(project.ID, filename)
	if globalNoteOperationApplicable(ctx) {
		t.Fatal("move to project was offered for a project note")
	}
	ctx.CurrentEntity = GlobalNoteEntity(filename)
	if !globalNoteOperationApplicable(ctx) {
		t.Fatal("move to project was withheld from a global note")
	}

	// currentEntityIdentity degrades to the focused project, so an unguarded note
	// descriptor would silently borrow it.
	ctx = PresentationCommandContext(ViewCrossProjectOutliner)
	ctx.FocusedProjectID = project.ID
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

	for _, id := range []OperationID{
		operationNotePreview,
		operationNoteTags,
		operationNoteLink,
		operationNotePin,
		operationNoteCopyTitle,
		operationNoteCopyBody,
		operationNoteCopyTitleBody,
	} {
		if result := executeEntityOperation(model, id, entity); result.Err == nil {
			t.Fatalf("%s ran on a malformed note and emitted %#v", id, result.Message)
		}
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

func TestTagOperationsApplyToTheOriginatingView(t *testing.T) {
	model := contextHubApp(t)
	alpha := model.projectByID("alpha")
	alpha.Tasks[0].Tags = []string{"urgent"}
	model.crossOutliner.crossForceRebuild = true
	model.crossOutliner.rebuildRows()

	ctx := PresentationCommandContext(ViewCrossProjectOutliner)
	ctx.CurrentEntity = WorkspaceTagEntity("urgent")
	result := model.operationExecutor.Execute(OperationExecutionRequest{
		ID:      operationTagFilter,
		Context: ctx,
		Source:  OperationSourceFinder,
	})
	filter, ok := result.Message.(ApplyTagFilterMsg)
	if !ok || filter.Tag != "urgent" || filter.Source != ViewCrossProjectOutliner {
		t.Fatalf("tag filter emitted %#v, want the originating Outliner", result.Message)
	}

	handled, _ := model.handleTagMessages(filter)
	if !handled {
		t.Fatal("the tag filter message was not handled")
	}
	if !model.crossOutliner.filter.Tags["urgent"] {
		t.Fatalf("Outliner tag filter = %v, want urgent applied", model.crossOutliner.filter.Tags)
	}
}

func TestTagOperationsOpenTheManagerForTheirScope(t *testing.T) {
	model := contextHubApp(t)
	alpha := model.projectByID("alpha")
	alpha.Tasks[0].Tags = []string{"urgent"}

	global := executeEntityOperation(model, operationTagManage, WorkspaceTagEntity("urgent"))
	if _, ok := global.Message.(ShowGlobalTagManagerMsg); !ok {
		t.Fatalf("an aggregate tag opened %#v, want the global tag manager", global.Message)
	}

	alpha.Tags = []string{"urgent"}
	scoped := executeEntityOperation(model, operationTagManage, ProjectTagEntity("alpha", "urgent"))
	manager, ok := scoped.Message.(ShowTagManagerMsg)
	if !ok || manager.Project == nil || manager.Project.ID != "alpha" {
		t.Fatalf("a project tag opened %#v, want alpha's tag manager", scoped.Message)
	}
}

func TestTagOperationsNeverOfferAggregateDeletion(t *testing.T) {
	model := contextHubApp(t)

	workspace := PresentationCommandContext(ViewCrossProjectOutliner)
	workspace.CurrentEntity = WorkspaceTagEntity("urgent")
	scoped := PresentationCommandContext(ViewCrossProjectOutliner)
	scoped.CurrentEntity = ProjectTagEntity("alpha", "urgent")

	for _, id := range []OperationID{operationTagDeleteProject, operationTagRenameProject} {
		descriptor, ok := model.operationRegistry.Descriptor(id)
		if !ok {
			t.Fatalf("%s is not registered", id)
		}
		if descriptor.Applicability(workspace) {
			t.Fatalf("%s was offered for an aggregate workspace tag", id)
		}
		if !descriptor.Applicability(scoped) {
			t.Fatalf("%s was withheld from a project-qualified tag", id)
		}
	}

	rename, ok := model.operationRegistry.Descriptor(operationTagRenameWorkspace)
	if !ok {
		t.Fatal("the workspace tag rename is not registered")
	}
	if !rename.Applicability(workspace) || rename.Applicability(scoped) {
		t.Fatal("the workspace tag rename must apply only to aggregate tags")
	}

	result := executeEntityOperation(model, operationTagDeleteProject, WorkspaceTagEntity("urgent"))
	if result.Err == nil {
		t.Fatalf("deleting an aggregate tag emitted %#v, want a refusal", result.Message)
	}
}

func TestTagOperationsRejectTagsThatLeftTheirScope(t *testing.T) {
	model := contextHubApp(t)

	// No project uses the tag, so neither the aggregate nor the project identity
	// resolves.
	if result := executeEntityOperation(model, operationTagPin, WorkspaceTagEntity("ghost")); result.Err == nil {
		t.Fatalf("pinning an unused aggregate tag emitted %#v", result.Message)
	}
	if result := executeEntityOperation(model, operationTagPin, ProjectTagEntity("alpha", "ghost")); result.Err == nil {
		t.Fatalf("pinning a tag absent from alpha emitted %#v", result.Message)
	}

	alpha := model.projectByID("alpha")
	alpha.Tags = []string{"urgent"}
	result := executeEntityOperation(model, operationTagPin, ProjectTagEntity("alpha", "urgent"))
	if result.Err != nil {
		t.Fatalf("pinning a live project tag failed: %v", result.Err)
	}
	if _, ok := result.Message.(OperationCommandResultMsg); !ok {
		t.Fatalf("project tag pin emitted %#v, want a mutation command", result.Message)
	}
}

func TestSavedFilterOperationsResolveByStableID(t *testing.T) {
	model, _ := entityOperationApp(t)
	if err := model.commitPlanningMutation(func(p *config.PlanningState) {
		p.SaveFilter(config.SavedFilter{ID: "sf-1", Name: "Urgent", Mode: "snapshot"})
	}); err != nil {
		t.Fatalf("seed saved filter: %v", err)
	}

	applied := executeEntityOperation(model, operationSavedFilterApply, SavedFilterEntity("sf-1"))
	load, ok := applied.Message.(LoadSavedFilterMsg)
	if !ok || load.Filter.ID != "sf-1" {
		t.Fatalf("apply emitted %#v, want the sf-1 filter", applied.Message)
	}

	edited := executeEntityOperation(model, operationSavedFilterEdit, SavedFilterEntity("sf-1"))
	editor, ok := edited.Message.(ShowFilterEditorMsg)
	if !ok || editor.Filter == nil || editor.Filter.ID != "sf-1" {
		t.Fatalf("edit emitted %#v, want the builder on sf-1", edited.Message)
	}

	missing := executeEntityOperation(model, operationSavedFilterApply, SavedFilterEntity("sf-gone"))
	if missing.Err == nil {
		t.Fatalf("applying a deleted filter emitted %#v, want an error", missing.Message)
	}
}

func TestSavedFilterDeleteConfirmsAndClearsTheActiveIndicator(t *testing.T) {
	model, _ := entityOperationApp(t)
	if err := model.commitPlanningMutation(func(p *config.PlanningState) {
		p.SaveFilter(config.SavedFilter{ID: "sf-1", Name: "Urgent", Mode: "snapshot"})
	}); err != nil {
		t.Fatalf("seed saved filter: %v", err)
	}
	model.crossOutliner.activeSavedFilter = "Urgent"

	result := executeEntityOperation(model, operationSavedFilterDelete, SavedFilterEntity("sf-1"))
	if result.ConfirmationRequired == nil || result.Message != nil {
		t.Fatalf("saved filter delete ran without confirmation: %#v", result)
	}
	confirmed := model.operationExecutor.Execute(result.ConfirmationRequired.Request)
	request, ok := confirmed.Message.(DeleteSavedFilterMsg)
	if !ok || request.ID != "sf-1" {
		t.Fatalf("confirmed delete emitted %#v, want a sf-1 delete", confirmed.Message)
	}

	if handled, _ := model.handleSavedFilterMessages(request); !handled {
		t.Fatal("the delete request was not handled")
	}
	if model.runtime.Planning().GetSavedFilter("sf-1") != nil {
		t.Fatal("the saved filter survived its confirmed delete")
	}
	if model.crossOutliner.activeSavedFilter != "" {
		t.Fatalf("active saved filter = %q, want cleared", model.crossOutliner.activeSavedFilter)
	}
}

func TestEntityOperationRefreshKeepsFinderRecents(t *testing.T) {
	model := contextHubApp(t)
	model.recordFinderDestination(config.RecentDestination{
		Kind: config.RecentDestinationProject,
		ID:   "beta",
	})
	before := model.runtime.Machine().GetRecentDestinations()
	if len(before) != 1 {
		t.Fatalf("seeded %d recents, want 1", len(before))
	}

	model.handleTagMessages(TagRemovedMsg{Name: "urgent"})
	model.handleSavedFilterMessages(DeleteSavedFilterMsg{ID: "sf-gone"})
	model.reconcileNoteResult(ProjectNoteMutationResultMsg{
		Location:       app.GlobalNoteLocation(),
		SuccessMessage: "Deleted: scratch",
	})

	after := model.runtime.Machine().GetRecentDestinations()
	if len(after) != 1 || after[0] != before[0] {
		t.Fatalf("Finder refresh rewrote machine recents: %#v, want %#v", after, before)
	}
}
