package tui

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/testutil"
)

// renameAcceptEnv is a real workspace on disk driven through the production
// application wiring: the same repository, census, commit service, and undo
// ledger the running TUI uses. Nothing here is a test double, which is the point
// — a whole-workspace rename is only meaningful against the census the
// production repository actually takes.
type renameAcceptEnv struct {
	dataDir  string
	service  *app.Service
	ledger   *storage.WorkspaceLedger
	projects []*models.Project
}

func newRenameAcceptEnv(t *testing.T, projects ...*models.Project) *renameAcceptEnv {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	dataDir := t.TempDir()
	for _, project := range projects {
		project.Path = filepath.Join(dataDir, "projects", project.Slug)
		if err := os.MkdirAll(project.Path, storage.DirPermissions); err != nil {
			t.Fatalf("MkdirAll(%q): %v", project.Path, err)
		}
		for _, task := range project.Tasks {
			task.Project = project
		}
		project.BuildTaskTree()
		if err := storage.SaveProject(project); err != nil {
			t.Fatalf("SaveProject(%q): %v", project.ID, err)
		}
	}
	seed := newTestConfigAt(dataDir)
	setTestProjects(seed, projects)
	manager := config.NewManager(config.ConfigPathForDataDir(dataDir))
	if err := saveTestRuntime(manager, seed); err != nil {
		t.Fatalf("save rename runtime: %v", err)
	}
	runtime, err := config.LoadRuntimeState(dataDir)
	if err != nil {
		t.Fatalf("LoadRuntimeState: %v", err)
	}
	ledger := workspaceLedgerFromConfig(runtime)
	return &renameAcceptEnv{
		dataDir:  dataDir,
		service:  newTUIAppService(dataDir, manager.RuntimeDependencies(), ledger),
		ledger:   ledger,
		projects: projects,
	}
}

// diskTags reloads a project from disk and returns one task's stored tag list,
// so every assertion is about durable bytes rather than an in-memory pointer the
// mutation may have shared.
func (e *renameAcceptEnv) diskTags(t *testing.T, projectID, taskID string) []string {
	t.Helper()
	return e.diskTask(t, projectID, taskID).Tags
}

func (e *renameAcceptEnv) diskTask(t *testing.T, projectID, taskID string) *models.Task {
	t.Helper()
	for _, seeded := range e.projects {
		if seeded.ID != projectID {
			continue
		}
		reloaded := &models.Project{ID: seeded.ID, Name: seeded.Name, Slug: seeded.Slug, Path: seeded.Path}
		if err := storage.LoadProject(reloaded); err != nil {
			t.Fatalf("LoadProject(%q): %v", projectID, err)
		}
		for _, task := range reloaded.Tasks {
			if task.ID == taskID {
				return task
			}
		}
		t.Fatalf("project %s has no task %s on disk", projectID, taskID)
	}
	t.Fatalf("no seeded project %s", projectID)
	return nil
}

func renameAcceptProject(t *testing.T, id, name string, tasks ...*models.Task) *models.Project {
	t.Helper()
	project, err := models.NewProject(id, name, "")
	if err != nil {
		t.Fatalf("NewProject(%q): %v", name, err)
	}
	next := 1
	for _, task := range tasks {
		if task.ShortID >= next {
			next = task.ShortID + 1
		}
		project.Tasks = append(project.Tasks, task)
	}
	project.NextShortID = next
	return project
}

func renameAcceptTask(t *testing.T, id string, shortID int, title string, tags ...string) *models.Task {
	t.Helper()
	task, err := models.NewGenericTask(id, shortID, title)
	if err != nil {
		t.Fatalf("NewGenericTask(%q): %v", id, err)
	}
	task.Tags = tags
	return task
}

// TestTaskNameRenameCommitsOneUndoRecordAndReversesInOneStep is the durable
// acceptance path: a rename spanning two projects commits as one transaction
// with one ledger entry, and a single undo — then a single redo — moves both
// projects together.
func TestTaskNameRenameCommitsOneUndoRecordAndReversesInOneStep(t *testing.T) {
	alpha := renameAcceptProject(t, "alpha-id", "Alpha",
		renameAcceptTask(t, "a1", 1, "first", "work", "urgent"),
		renameAcceptTask(t, "a2", 2, "second", "work"),
	)
	beta := renameAcceptProject(t, "beta-id", "Beta", renameAcceptTask(t, "b1", 1, "third", "work"))
	gamma := renameAcceptProject(t, "gamma-id", "Gamma", renameAcceptTask(t, "g1", 1, "unrelated", "ops"))
	env := newRenameAcceptEnv(t, alpha, beta, gamma)

	result, err := env.service.RenameTaskName(app.RenameTaskNameRequest{
		Namespace: app.TaskTagNamespace,
		From:      "work",
		To:        "effort",
	})
	if err != nil {
		t.Fatalf("RenameTaskName: %v", err)
	}
	if len(result.Tasks) != 3 || result.TransactionID == "" {
		t.Fatalf("result = %+v, want three renamed tasks under one transaction", result)
	}
	if !slices.Equal(result.TouchedProjectIDs, []string{"alpha-id", "beta-id"}) {
		t.Fatalf("touched projects = %v, want only the two carrying the source name", result.TouchedProjectIDs)
	}
	if got := env.diskTags(t, "alpha-id", "a1"); !slices.Equal(got, []string{"effort", "urgent"}) {
		t.Fatalf("alpha a1 tags on disk = %v, want the rename with the unrelated tag kept", got)
	}
	if got := env.diskTags(t, "gamma-id", "g1"); !slices.Equal(got, []string{"ops"}) {
		t.Fatalf("gamma tags on disk = %v, want the untouched project", got)
	}

	entry, ok, err := env.ledger.SelectNewestUndo(storage.GlobalScope())
	if err != nil || !ok {
		t.Fatalf("SelectNewestUndo ok=%v err=%v", ok, err)
	}
	if entry.Action != "Rename Tag" || !strings.Contains(entry.Context, "work → effort") {
		t.Fatalf("ledger entry = %q/%q, want the rename named once", entry.Action, entry.Context)
	}
	if !slices.Equal(entry.ProjectIDs, []string{"alpha-id", "beta-id"}) {
		t.Fatalf("ledger project IDs = %v, want only the written projects", entry.ProjectIDs)
	}

	if _, err := env.ledger.Undo(storage.GlobalScope()); err != nil {
		t.Fatalf("Undo: %v", err)
	}
	for _, locator := range []app.TaskLocator{{ProjectID: "alpha-id", TaskID: "a2"}, {ProjectID: "beta-id", TaskID: "b1"}} {
		if got := env.diskTags(t, locator.ProjectID, locator.TaskID); !slices.Equal(got, []string{"work"}) {
			t.Fatalf("%s tags after one undo = %v, want the source name back", locator.TaskID, got)
		}
	}
	if _, err := env.ledger.Redo(storage.GlobalScope()); err != nil {
		t.Fatalf("Redo: %v", err)
	}
	for _, locator := range []app.TaskLocator{{ProjectID: "alpha-id", TaskID: "a2"}, {ProjectID: "beta-id", TaskID: "b1"}} {
		if got := env.diskTags(t, locator.ProjectID, locator.TaskID); !slices.Equal(got, []string{"effort"}) {
			t.Fatalf("%s tags after one redo = %v, want the rename back", locator.TaskID, got)
		}
	}
}

// TestTaskNameRenameCensusIncludesParkedAndExcludesArchived pins the production
// census population: parking is a descriptive project tag a vocabulary rename
// has no business honoring, while an archived project is out of the workspace
// the rename describes.
func TestTaskNameRenameCensusIncludesParkedAndExcludesArchived(t *testing.T) {
	active := renameAcceptProject(t, "active-id", "Active", renameAcceptTask(t, "act1", 1, "active", "work"))
	parked := renameAcceptProject(t, "parked-id", "Parked", renameAcceptTask(t, "prk1", 1, "shelved", "work"))
	parked.Tags = []string{models.ParkedProjectTag}
	archived := renameAcceptProject(t, "archived-id", "Archived", renameAcceptTask(t, "arc1", 1, "filed away", "work"))
	archived.Archived = true
	env := newRenameAcceptEnv(t, active, parked, archived)

	result, err := env.service.RenameTaskName(app.RenameTaskNameRequest{
		Namespace: app.TaskTagNamespace,
		From:      "work",
		To:        "effort",
	})
	if err != nil {
		t.Fatalf("RenameTaskName: %v", err)
	}
	if !slices.Equal(result.TouchedProjectIDs, []string{"active-id", "parked-id"}) {
		t.Fatalf("touched projects = %v, want the active and parked projects only", result.TouchedProjectIDs)
	}
	if got := env.diskTags(t, "parked-id", "prk1"); !slices.Equal(got, []string{"effort"}) {
		t.Fatalf("parked project tags on disk = %v, want the rename applied", got)
	}
	if got := env.diskTags(t, "archived-id", "arc1"); !slices.Equal(got, []string{"work"}) {
		t.Fatalf("archived project tags on disk = %v, want them untouched", got)
	}
}

// TestTaskNameRenameFailsClosedWhileSnapshotStaysTolerant is the strictness
// contract stated as the difference it makes: one unreadable project fails the
// whole rename, and the ordinary single-task path over the same workspace keeps
// working by skipping that project.
func TestTaskNameRenameFailsClosedWhileSnapshotStaysTolerant(t *testing.T) {
	alpha := renameAcceptProject(t, "alpha-id", "Alpha", renameAcceptTask(t, "a1", 1, "readable", "work"))
	broken := renameAcceptProject(t, "broken-id", "Broken", renameAcceptTask(t, "b1", 1, "unreadable", "work"))
	env := newRenameAcceptEnv(t, alpha, broken)
	if err := os.WriteFile(storage.TasksFilePath(broken.Path), []byte("not a task document\n"), storage.FilePermissions); err != nil {
		t.Fatalf("corrupt broken tasks.md: %v", err)
	}

	_, err := env.service.RenameTaskName(app.RenameTaskNameRequest{
		Namespace: app.TaskTagNamespace,
		From:      "work",
		To:        "effort",
	})
	if err == nil || !strings.Contains(err.Error(), "Broken") {
		t.Fatalf("RenameTaskName error = %v, want a census failure naming the unreadable project", err)
	}
	if got := env.diskTags(t, "alpha-id", "a1"); !slices.Equal(got, []string{"work"}) {
		t.Fatalf("alpha tags after a failed census = %v, want them untouched", got)
	}

	if _, err := env.service.ExecuteTaskOperation(app.TaskOperationRequest{
		Operation: app.TaskOperationEditTitle,
		Targets:   []app.TaskLocator{{ProjectID: "alpha-id", TaskID: "a1"}},
		Text:      "still editable",
	}); err != nil {
		t.Fatalf("single-task edit over the same workspace: %v", err)
	}
	if got := env.diskTask(t, "alpha-id", "a1").Title; got != "still editable" {
		t.Fatalf("title after tolerant edit = %q, want the edit to have landed", got)
	}
}

// TestTaskNameRenameExpiresAgainstARealConcurrentWrite drives the guards through
// the real commit boundary: a project written between preview and apply expires
// the preview under the commit locks, and the rollback leaves every project's
// bytes exactly as they were.
func TestTaskNameRenameExpiresAgainstARealConcurrentWrite(t *testing.T) {
	alpha := renameAcceptProject(t, "alpha-id", "Alpha", renameAcceptTask(t, "a1", 1, "source", "work"))
	beta := renameAcceptProject(t, "beta-id", "Beta", renameAcceptTask(t, "b1", 1, "unrelated", "ops"))
	env := newRenameAcceptEnv(t, alpha, beta)

	preview, err := env.service.PreviewTaskNameRename(app.RenameTaskNameRequest{
		Namespace: app.TaskTagNamespace,
		From:      "work",
		To:        "effort",
	})
	if err != nil {
		t.Fatalf("PreviewTaskNameRename: %v", err)
	}
	beta.Tasks[0].Title = "written by someone else"
	if err := storage.SaveProject(beta); err != nil {
		t.Fatalf("concurrent SaveProject(beta): %v", err)
	}

	if _, err := env.service.ApplyTaskNameRename(preview); !errors.Is(err, storage.ErrStaleRevision) {
		t.Fatalf("ApplyTaskNameRename error = %v, want ErrStaleRevision", err)
	}
	if got := env.diskTags(t, "alpha-id", "a1"); !slices.Equal(got, []string{"work"}) {
		t.Fatalf("alpha tags after an expired apply = %v, want them untouched", got)
	}
	if _, ok, err := env.ledger.SelectNewestUndo(storage.GlobalScope()); err != nil || ok {
		t.Fatalf("expired apply recorded an undo entry: ok=%v err=%v", ok, err)
	}
}

// TestFallbackCommitRefusesReadOnlyGuards keeps the non-filesystem fallback
// honest: it cannot validate a guard, so it refuses the commit rather than
// reporting success for a mutation whose read set was never proven.
func TestFallbackCommitRefusesReadOnlyGuards(t *testing.T) {
	previous := storage.DefaultRepository
	storage.DefaultRepository = testutil.NewMockRepository()
	t.Cleanup(func() { storage.DefaultRepository = previous })

	dataDir := t.TempDir()
	repo := newTUIAppRepository(dataDir, config.NewRuntimeDependencies(config.NewDomainFileStore(dataDir)), nil)
	project, err := models.NewProject("alpha-id", "Alpha", filepath.Join(dataDir, "projects", "alpha"))
	if err != nil {
		t.Fatalf("NewProject: %v", err)
	}

	cases := []struct {
		name      string
		changeset storage.ChangeSet
	}{
		{"project guard", storage.ChangeSet{
			Project:              project,
			ProjectPreconditions: []storage.ProjectRevisionPrecondition{{ProjectID: "beta-id", Path: filepath.Join(dataDir, "projects", "beta")}},
		}},
		{"artifact guard", storage.ChangeSet{
			Project:               project,
			ArtifactPreconditions: []storage.ArtifactPrecondition{{Path: config.WorkspaceCatalogPath(dataDir), ExpectedRevision: "deadbeef"}},
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := repo.Commit(tc.changeset); err == nil || !strings.Contains(err.Error(), "read-only preconditions") {
				t.Fatalf("fallback Commit error = %v, want a refusal naming read-only preconditions", err)
			}
		})
	}
}
