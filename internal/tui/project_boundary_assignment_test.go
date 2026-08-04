package tui

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

const parkedBoundaryName = "On Hold"

// createParkedBoundary drives the shell's own create path so no test hand-builds
// a catalog the product could not have produced. Every project must already be
// registered before this runs, because the project fixtures rebuild the catalog
// from scratch.
func createParkedBoundary(t *testing.T, shell *AppModel) config.ProjectBoundary {
	t.Helper()
	handled, _ := shell.handleBoundaryMessages(CreateBoundaryMsg{Name: parkedBoundaryName})
	if !handled {
		t.Fatal("CreateBoundaryMsg was not handled")
	}
	for _, boundary := range shell.runtime.Catalog().Boundaries {
		if boundary.Name == parkedBoundaryName {
			return boundary
		}
	}
	t.Fatalf("boundary %q was not published to the catalog", parkedBoundaryName)
	return config.ProjectBoundary{}
}

func boundaryAssignmentApp(t *testing.T) (*AppModel, config.ProjectBoundary) {
	t.Helper()
	shell := contextHubApp(t)
	return shell, createParkedBoundary(t, shell)
}

// openBoundaryPicker drives the Project Hub action the way a user does: focus a
// row, open the sheet, run Set boundary, and let the shell route the resulting
// request to its overlay.
func openBoundaryPicker(t *testing.T, shell *AppModel, hub *ProjectHubModel, itemID ListItemID) *OperationValuePickerModel {
	t.Helper()
	hub.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	focusProjectHubItem(t, hub, itemID)
	hub.Update(syntheticKey(' '))
	if got := projectHubActionIDs(hub); !containsOperationID(got, operationProjectSetBoundary) {
		t.Fatalf("%s actions omitted Set boundary: %v", itemID, got)
	}
	if !hub.actions.surface.SetCursorToID(ListItemID("project-hub-action:" + string(operationProjectSetBoundary))) {
		t.Fatal("Set boundary was not selectable")
	}

	_, cmd := hub.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("Set boundary emitted no request")
	}
	request, ok := cmd().(OperationValueRequestMsg)
	if !ok {
		t.Fatalf("Set boundary emitted %#v, want a boundary value request", cmd())
	}
	if request.Provider != ValueProviderProjectBoundary {
		t.Fatalf("Set boundary asked for provider %q, want %q", request.Provider, ValueProviderProjectBoundary)
	}
	handled, overlayCmd := shell.handleOperationOverlayMessage(request)
	if !handled || overlayCmd != nil {
		t.Fatalf("boundary request = handled %v cmd %T, want the picker opened synchronously", handled, overlayCmd)
	}
	picker := shell.operationValuePicker
	if picker == nil || picker.provider != ValueProviderProjectBoundary {
		t.Fatalf("boundary request opened %#v, want the boundary picker", picker)
	}
	return picker
}

// answerBoundaryPicker selects one boundary and returns the command the shell
// produced. A refusal answers with a status timeout instead of an assignment, so
// callers decide whether running the command is meaningful.
func answerBoundaryPicker(
	t *testing.T,
	shell *AppModel,
	picker *OperationValuePickerModel,
	boundaryID string,
) tea.Cmd {
	t.Helper()
	if !picker.surface.SetCursorToID(ListItemID(projectBoundaryItemPrefix + boundaryID)) {
		t.Fatalf("boundary picker cannot select %q", boundaryID)
	}
	_, dismissed, selected := picker.handleKey(tea.KeyPressMsg{Code: tea.KeyEnter})
	if !dismissed {
		t.Fatal("Enter did not finish the boundary picker")
	}
	handled, cmd := shell.handleOperationOverlayMessage(selected)
	if !handled {
		t.Fatalf("boundary selection %#v was not handled", selected)
	}
	return cmd
}

func boundaryAssignmentMsg(t *testing.T, cmd tea.Cmd) AssignProjectBoundaryMsg {
	t.Helper()
	if cmd == nil {
		t.Fatal("the boundary selection produced no assignment")
	}
	msg := cmd()
	assignment, ok := msg.(AssignProjectBoundaryMsg)
	if !ok {
		t.Fatalf("boundary selection emitted %#v, want an assignment", msg)
	}
	return assignment
}

// The Hub action and the picker are one operation: the action names the project,
// the picker names the boundary, and the executor re-resolves both before the
// assignment message is built.
func TestProjectHubSetBoundaryRoundTripsThroughThePicker(t *testing.T) {
	shell, parked := boundaryAssignmentApp(t)
	hub := openProjectHub(t, shell)
	picker := openBoundaryPicker(t, shell, hub, "project-hub:project:beta")

	if hub.actions != nil {
		t.Fatal("the boundary picker opened beneath a still-open action sheet")
	}
	wantItems := []ListItemID{
		ListItemID(projectBoundaryItemPrefix + models.ActiveBoundaryID),
		ListItemID(projectBoundaryItemPrefix + parked.ID),
	}
	for i, item := range picker.surface.Items() {
		if i >= len(wantItems) || item.ID != wantItems[i] {
			t.Fatalf("boundary picker items = %#v, want %v", picker.surface.Items(), wantItems)
		}
	}
	current, ok := picker.surface.CurrentItem()
	if !ok || current.ID != wantItems[0] {
		t.Fatalf("boundary picker preselected %#v, want the project's current Active boundary", current)
	}

	assignment := boundaryAssignmentMsg(t, answerBoundaryPicker(t, shell, picker, parked.ID))
	if assignment.ProjectID != "beta" || assignment.BoundaryID != parked.ID {
		t.Fatalf("boundary selection emitted %#v, want a beta assignment to %q", assignment, parked.ID)
	}
	if shell.operationValuePicker != nil {
		t.Fatal("the boundary picker stayed open after the selection")
	}

	shell.Update(assignment)
	if got := shell.runtime.Catalog().ProjectBoundaryID("beta"); got != parked.ID {
		t.Fatalf("assigned boundary = %q, want %q", got, parked.ID)
	}
}

// An archived project keeps the boundary it will return to on restore, so the
// action is offered on both sides of the lifecycle and reassignment succeeds.
func TestProjectHubSetBoundaryServesArchivedProjects(t *testing.T) {
	shell := contextHubApp(t)
	archived := scopeTestProject("archived", "Archived", "1")
	archived.Archived = true
	archived.Path = t.TempDir()
	addTestAppProject(shell, archived)
	parked := createParkedBoundary(t, shell)

	hub := openProjectHub(t, shell)
	picker := openBoundaryPicker(t, shell, hub, "project-hub:archived:archived")
	assignment := boundaryAssignmentMsg(t, answerBoundaryPicker(t, shell, picker, parked.ID))
	if assignment.ProjectID != "archived" {
		t.Fatalf("archived selection emitted %#v, want an archived assignment", assignment)
	}

	shell.Update(assignment)
	if got := shell.runtime.Catalog().ProjectBoundaryID("archived"); got != parked.ID {
		t.Fatalf("archived project boundary = %q, want %q", got, parked.ID)
	}
}

// Choosing the boundary the project is already in is a decision, not a mutation:
// it reports informationally and opens no transaction at all.
func TestProjectHubSetBoundaryNoOpReportsWithoutMutating(t *testing.T) {
	shell, _ := boundaryAssignmentApp(t)
	hub := openProjectHub(t, shell)
	picker := openBoundaryPicker(t, shell, hub, "project-hub:project:beta")

	shell.Update(boundaryAssignmentMsg(t, answerBoundaryPicker(t, shell, picker, models.ActiveBoundaryID)))
	if shell.statusMessage == "" || !shell.statusIsInfo {
		t.Fatalf("a no-op assignment reported %q, want an informational status", shell.statusMessage)
	}
	if got := shell.runtime.Catalog().ProjectBoundaryID("beta"); got != models.ActiveBoundaryID {
		t.Fatalf("a no-op assignment moved the project to %q", got)
	}
}

// Both the Hub action and the picker answer re-read the live catalog, so a
// project or boundary that disappeared while the overlay was open is refused
// instead of being assigned from frozen row metadata.
func TestSetBoundaryRefusalsShareOneValidatedPath(t *testing.T) {
	t.Run("system Unfiled is never offered", func(t *testing.T) {
		shell, _ := boundaryAssignmentApp(t)
		catalog := cloneWorkspaceCatalog(shell.runtime.Catalog())
		catalog.SystemUnfiledProjectID = "alpha"
		shell.runtime.SetCatalogInMemory(catalog)

		result := executeProjectOperation(shell, operationProjectSetBoundary, projectOperationContext("alpha"))
		if result.Err == nil {
			t.Fatalf("Set boundary emitted %#v for the system Unfiled project", result.Message)
		}

		hub := openProjectHub(t, shell)
		hub.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
		focusProjectHubItem(t, hub, "project-hub:project:alpha")
		hub.Update(syntheticKey(' '))
		if got := projectHubActionIDs(hub); containsOperationID(got, operationProjectSetBoundary) {
			t.Fatalf("the system Unfiled project offered Set boundary: %v", got)
		}
	})

	t.Run("boundary deleted while the picker was open", func(t *testing.T) {
		shell, parked := boundaryAssignmentApp(t)
		hub := openProjectHub(t, shell)
		picker := openBoundaryPicker(t, shell, hub, "project-hub:project:beta")

		shell.handleBoundaryMessages(DeleteBoundaryMsg{
			BoundaryID: parked.ID, DestinationID: models.ActiveBoundaryID,
		})
		if _, ok := shell.runtime.Catalog().Boundary(parked.ID); ok {
			t.Fatal("the boundary was not deleted while the picker was open")
		}

		assignment := boundaryAssignmentMsg(t, answerBoundaryPicker(t, shell, picker, parked.ID))
		_, cmd := shell.handleBoundaryMessages(assignment)
		assertBoundaryErrorMentions(t, cmd, config.ErrBoundaryNotFound)
		if got := shell.runtime.Catalog().ProjectBoundaryID("beta"); got != models.ActiveBoundaryID {
			t.Fatalf("a refused assignment moved the project to %q", got)
		}
	})

	t.Run("project removed while the picker was open", func(t *testing.T) {
		shell, parked := boundaryAssignmentApp(t)
		hub := openProjectHub(t, shell)
		picker := openBoundaryPicker(t, shell, hub, "project-hub:project:beta")

		setTestAppProjects(shell, shell.projectByID("alpha"))

		answerBoundaryPicker(t, shell, picker, parked.ID)
		if shell.err == nil || !strings.Contains(shell.err.Error(), "beta") {
			t.Fatalf("removed-project refusal = %v, want it to name the missing project", shell.err)
		}
	})
}

// The editor is the second host of the same semantics. A rename that also parks
// the project must land as one transaction: the directory move, the tasks.md
// rewrite, the catalog record, and the planning cleanup share a single undo
// entry, so undo can never restore a name without its boundary.
func TestProjectEditorCommitsRenameAndParkInOneTransaction(t *testing.T) {
	project := newSession4Project(t, "Original")
	fixture := newSession4LifecycleFixture(t, project)
	live := fixture.app.projectByID(project.ID)
	oldPath := live.Path
	seedLifecyclePlanning(t, fixture, live)
	parked := createParkedBoundary(t, fixture.app)

	editor := newLifecycleProjectEditor(t, fixture, live, false)
	editor.nameInput.SetValue("Renamed Project")
	focusEditorBoundary(t, editor, parked.ID)

	if raw := editor.saveProject()(); !isProjectUpdated(raw) {
		t.Fatalf("rename-plus-park = %#v, want ProjectUpdatedMsg", raw)
	}

	newPath := filepath.Join(fixture.dataDir, "projects", "renamed-project")
	if live.Path != newPath {
		t.Fatalf("published path = %q, want %q", live.Path, newPath)
	}
	committed := reloadLifecycleRuntime(t, fixture)
	if got := committed.Catalog().ProjectBoundaryID(project.ID); got != parked.ID {
		t.Fatalf("committed boundary = %q, want %q", got, parked.ID)
	}
	if planning := committed.Planning(); len(planning.Today) != 0 || len(planning.GetNext()) != 0 {
		t.Fatalf("parking left planning behind: today=%v next=%v", planning.Today, planning.GetNext())
	}

	ledger := fixture.app.deps.historyLedger()
	entry, ok, err := ledger.SelectNewestUndo(storage.LedgerScope{ProjectID: project.ID})
	if err != nil || !ok || entry.Action != "Edit Project" {
		t.Fatalf("rename-plus-park undo entry = %#v/%v/%v", entry, ok, err)
	}
	if _, err := ledger.Undo(storage.LedgerScope{ProjectID: project.ID}); err != nil {
		t.Fatalf("undo rename-plus-park: %v", err)
	}
	restored := reloadLifecycleRuntime(t, fixture)
	if got := restored.Catalog().ProjectBoundaryID(project.ID); got != models.ActiveBoundaryID {
		t.Fatalf("undo left the project in %q, want %q", got, models.ActiveBoundaryID)
	}
	if planning := restored.Planning(); len(planning.Today) != 1 || len(planning.GetNext()) != 1 {
		t.Fatalf("undo did not restore planning: today=%v next=%v", planning.Today, planning.GetNext())
	}
	if reverted := restored.GetProject(project.ID); reverted == nil || reverted.Name != "Original" {
		t.Fatalf("undo restored %#v, want the original name", reverted)
	}
	if _, err := os.Stat(storage.TasksFilePath(oldPath)); err != nil {
		t.Fatalf("undo did not restore the old tasks path: %v", err)
	}
}

// A rejected commit must leave every effect behind: a rename that also parks the
// project cannot publish half of itself, or the workspace would describe a
// renamed project that is still planned for today.
func TestProjectEditorRenameAndParkRollBackTogether(t *testing.T) {
	project := newSession4Project(t, "Original")
	fixture := newSession4LifecycleFixture(t, project)
	live := fixture.app.projectByID(project.ID)
	oldPath := live.Path
	seedLifecyclePlanning(t, fixture, live)
	parked := createParkedBoundary(t, fixture.app)

	commitErr := errors.New("workspace is read-only")
	fixture.app.appService = app.NewServiceWithArtifacts(&failingOpsRepo{
		tuiAppRepository: newTUIAppRepository(
			fixture.dataDir,
			fixture.manager.RuntimeDependencies(),
			fixture.app.deps.historyLedger(),
		),
		err: commitErr,
	}, nil, nil, nil)

	editor := newLifecycleProjectEditor(t, fixture, live, false)
	editor.nameInput.SetValue("Renamed Project")
	focusEditorBoundary(t, editor, parked.ID)

	raw := editor.saveProject()()
	failure, ok := raw.(ErrorMsg)
	if !ok || !strings.Contains(failure.Err.Error(), commitErr.Error()) {
		t.Fatalf("failed rename-plus-park = %#v, want the commit failure", raw)
	}

	if live.Name != "Original" || live.Path != oldPath {
		t.Fatalf("a failed save published %q at %q", live.Name, live.Path)
	}
	committed := reloadLifecycleRuntime(t, fixture)
	if got := committed.Catalog().ProjectBoundaryID(project.ID); got != models.ActiveBoundaryID {
		t.Fatalf("a failed save published boundary %q", got)
	}
	if planning := committed.Planning(); len(planning.Today) != 1 || len(planning.GetNext()) != 1 {
		t.Fatalf("a failed save published the planning cleanup: today=%v next=%v", planning.Today, planning.GetNext())
	}
	if _, err := os.Stat(storage.TasksFilePath(oldPath)); err != nil {
		t.Fatalf("a failed save moved the project directory: %v", err)
	}
	if _, err := os.Stat(filepath.Join(fixture.dataDir, "projects", "renamed-project")); !os.IsNotExist(err) {
		t.Fatalf("a failed save left a renamed directory: %v", err)
	}
}

// The boundary row is a real form field: it takes its turn in the focus order
// and moves only while focused, so ←/→ never edits two fields at once.
func TestProjectEditorBoundaryRowJoinsTheFocusOrder(t *testing.T) {
	project := newSession4Project(t, "Original")
	fixture := newSession4LifecycleFixture(t, project)
	createParkedBoundary(t, fixture.app)
	editor := newLifecycleProjectEditor(t, fixture, fixture.app.projectByID(project.ID), false)

	want := []projectEditorField{
		projectEditorFieldContext,
		projectEditorFieldBoundary,
		projectEditorFieldDescription,
		projectEditorFieldName,
	}
	for _, field := range want {
		editor.Update(tea.KeyPressMsg{Code: tea.KeyTab})
		if editor.focusIndex != field {
			t.Fatalf("Tab landed on field %d, want %d", editor.focusIndex, field)
		}
	}

	editor.Update(syntheticKeyStr("right"))
	if editor.selectedBoundaryID() != editor.originalBoundaryID {
		t.Fatal("→ moved the boundary selection while another field was focused")
	}
	for editor.focusIndex != projectEditorFieldBoundary {
		editor.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	}
	editor.Update(syntheticKeyStr("right"))
	if editor.selectedBoundaryID() == editor.originalBoundaryID {
		t.Fatal("→ did not move the focused boundary selection")
	}
	editor.Update(syntheticKeyStr("left"))
	if editor.selectedBoundaryID() != editor.originalBoundaryID {
		t.Fatal("← did not return to the original boundary")
	}
	editor.Update(syntheticKeyStr("right"))
	if !editor.hasChanges {
		t.Fatal("a boundary change did not mark the form dirty")
	}
}

// The system Unfiled project has exactly one legal boundary, so its row is fixed
// and no key can move it off Active.
func TestProjectEditorPinsSystemUnfiledToActive(t *testing.T) {
	inbox := newSession4Project(t, "Inbox")
	fixture := newSession4LifecycleFixture(t, inbox)
	catalog := cloneWorkspaceCatalog(fixture.app.runtime.Catalog())
	catalog.SystemUnfiledProjectID = inbox.ID
	if err := fixture.app.runtime.CommitCatalog(catalog); err != nil {
		t.Fatalf("protect Inbox: %v", err)
	}
	createParkedBoundary(t, fixture.app)

	live := fixture.app.projectByID(inbox.ID)
	editor := newLifecycleProjectEditor(t, fixture, live, true)
	for editor.focusIndex != projectEditorFieldBoundary {
		editor.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	}
	editor.Update(syntheticKeyStr("right"))
	if editor.selectedBoundaryID() != models.ActiveBoundaryID {
		t.Fatalf("system Unfiled boundary = %q, want %q", editor.selectedBoundaryID(), models.ActiveBoundaryID)
	}
	if !strings.Contains(editor.renderBoundarySelector(), config.ActiveBoundaryName) {
		t.Fatalf("system Unfiled boundary row = %q, want a fixed Active row", editor.renderBoundarySelector())
	}

	if raw := editor.saveProject()(); !isProjectUpdated(raw) {
		t.Fatalf("system Unfiled save = %#v, want ProjectUpdatedMsg", raw)
	}
	if got := reloadLifecycleRuntime(t, fixture).Catalog().ProjectBoundaryID(inbox.ID); got != models.ActiveBoundaryID {
		t.Fatalf("system Unfiled committed boundary = %q, want %q", got, models.ActiveBoundaryID)
	}
}

// Creation stays a one-answer decision: a new project is always born Active and
// is moved afterwards through the shared assignment path.
func TestProjectCreatorLeavesNewProjectsInActive(t *testing.T) {
	fixture := newSession4LifecycleFixture(t)
	createParkedBoundary(t, fixture.app)
	creator := NewProjectCreatorModel(fixture.app.runtime, fixture.app.configManager, fixture.app.appService)
	creator.nameInput.SetValue("Fresh Project")

	raw := creator.createProject()()
	created, ok := raw.(ProjectCreatedMsg)
	if !ok || created.Project == nil {
		t.Fatalf("create result = %#v, want ProjectCreatedMsg", raw)
	}
	if got := reloadLifecycleRuntime(t, fixture).Catalog().ProjectBoundaryID(created.Project.ID); got != models.ActiveBoundaryID {
		t.Fatalf("created project boundary = %q, want %q", got, models.ActiveBoundaryID)
	}
}

// failingOpsRepo rejects the one transaction the editor's atomic save depends
// on, which is how the rollback assertions distinguish "not published" from
// "never prepared".
type failingOpsRepo struct {
	*tuiAppRepository
	err error
}

func (r *failingOpsRepo) CommitOpsTransaction(storage.OpsCommitRequest) (string, error) {
	return "", r.err
}

func seedLifecyclePlanning(t *testing.T, fixture *session4LifecycleFixture, project *models.Project) {
	t.Helper()
	locator := config.ProjectTaskLocator{ProjectID: project.ID, TaskID: project.Tasks[0].ID}
	planning := clonePlanningState(fixture.app.runtime.Planning())
	planning.Today = []config.ProjectTaskLocator{locator}
	planning.SetNext([]config.ProjectTaskLocator{locator})
	if err := fixture.app.runtime.CommitPlanning(planning); err != nil {
		t.Fatalf("seed planning: %v", err)
	}
	fixture.app.rebuildPlanningFromRuntime()
}

func newLifecycleProjectEditor(
	t *testing.T,
	fixture *session4LifecycleFixture,
	project *models.Project,
	identityProtected bool,
) *ProjectEditorModel {
	t.Helper()
	return NewProjectEditorModel(
		project,
		fixture.app.configManager,
		fixture.app.runtime,
		fixture.app.appService,
		identityProtected,
	)
}

func focusEditorBoundary(t *testing.T, editor *ProjectEditorModel, boundaryID string) {
	t.Helper()
	for i, boundary := range editor.boundaries {
		if boundary.ID == boundaryID {
			editor.boundaryCursor = i
			return
		}
	}
	t.Fatalf("editor omitted boundary %q", boundaryID)
}

func reloadLifecycleRuntime(t *testing.T, fixture *session4LifecycleFixture) *config.RuntimeState {
	t.Helper()
	runtime, err := config.LoadRuntimeState(fixture.dataDir)
	if err != nil {
		t.Fatalf("reload workspace: %v", err)
	}
	return runtime
}

func isProjectUpdated(msg tea.Msg) bool {
	_, ok := msg.(ProjectUpdatedMsg)
	return ok
}
