package tui

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/tui/coordinator"
)

// Helper processes for scripts/cross_machine_gate.sh. Each test below is one
// role in a race the in-process suite cannot express: two real processes, each
// with its own machine identity, holding their own locks over one workspace on
// disk. They are compiled once with `go test -c ./internal/tui` and invoked by
// name, so the gate spends one build on a binary it then forks repeatedly.
//
// Every role skips unless TT_HELPER_DATA_DIR names the workspace it is to open,
// which is what keeps them inert under `just ci`: an ordinary `go test ./...`
// runs each one, finds no workspace, and skips.
//
// Machine identity is $HOME and TestMain gives each process a fresh temporary
// one, so two helpers are always two machines without the gate arranging it.

const (
	helperDataDirEnv = "TT_HELPER_DATA_DIR"
	helperReadyEnv   = "TT_HELPER_READY_FILE"
	helperGoEnv      = "TT_HELPER_GO_FILE"
	helperResultEnv  = "TT_HELPER_RESULT_FILE"
	helperTitleEnv   = "TT_HELPER_TASK_TITLE"
	helperNoteEnv    = "TT_HELPER_RESUME_NOTE"

	// helperRenameTag is the one vocabulary the seeded workspace carries, so the
	// gate never has to pass a name the seed and the rename both have to agree on.
	helperRenameTag = "work"
	// helperRenameDestination is what a rename that reached the commit would have
	// written. The refusal cases assert its absence, so it has to be a spelling
	// nothing else could produce.
	helperRenameDestination = "effort"

	helperBarrierTimeout = 60 * time.Second
	helperPollInterval   = 10 * time.Millisecond
)

// helperEnvironment is one helper process's whole view of the workspace: the
// same manager, runtime, ledger and service the running TUI builds, over a data
// directory another process minted.
type helperEnvironment struct {
	dataDir   string
	workspace *uitestWorkspace
	service   *app.Service
}

// openHelperWorkspace attaches to the workspace named by the environment. It
// skips rather than fails when there is none, which is the whole gate between a
// helper role and the ordinary suite it is compiled into.
func openHelperWorkspace(t *testing.T) helperEnvironment {
	t.Helper()
	dataDir := strings.TrimSpace(os.Getenv(helperDataDirEnv))
	if dataDir == "" {
		t.Skipf("%s is unset; this test is a cross-machine helper role", helperDataDirEnv)
	}
	manager := config.NewManager(config.ConfigPathForDataDir(dataDir))
	if _, err := manager.LoadDomains(); err != nil {
		t.Fatalf("LoadDomains(%q): %v", dataDir, err)
	}
	runtime, err := manager.LoadRuntimeState()
	if err != nil {
		t.Fatalf("LoadRuntimeState(%q): %v", dataDir, err)
	}
	ledger := workspaceLedgerFromConfig(runtime)
	return helperEnvironment{
		dataDir: dataDir,
		workspace: &uitestWorkspace{
			// Root deliberately equals DataDir and Cleanup is never called: the
			// directory belongs to the gate script, which owns removing it.
			Root:       dataDir,
			DataDir:    dataDir,
			ConfigPath: config.ConfigPathForDataDir(dataDir),
			Runtime:    runtime,
			Manager:    manager,
		},
		service: newTUIAppService(dataDir, manager.RuntimeDependencies(), ledger),
	}
}

// helperEnv reads one required environment value, so a role that was invoked
// without its inputs fails by name instead of racing on an empty path.
func helperEnv(t *testing.T, name string) string {
	t.Helper()
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		t.Fatalf("%s is required by this helper role", name)
	}
	return value
}

func helperSignal(t *testing.T, path string) {
	t.Helper()
	if err := os.WriteFile(path, []byte("ready\n"), storage.FilePermissions); err != nil {
		t.Fatalf("signal %q: %v", path, err)
	}
}

// helperAwait blocks until another process signals through the filesystem. A
// file is the only barrier two unrelated processes share here, and a bounded
// wait is what turns a partner that died into a named failure rather than a
// gate that hangs.
func helperAwait(t *testing.T, path string) {
	t.Helper()
	deadline := time.Now().Add(helperBarrierTimeout)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(path); err == nil {
			return
		}
		time.Sleep(helperPollInterval)
	}
	t.Fatalf("timed out after %s waiting for %q", helperBarrierTimeout, path)
}

// helperRecord writes this process's outcome where the gate script can read it.
// Both sides of a race exit 0: which one committed is a result, not an error,
// and the script is what judges the pair.
func helperRecord(t *testing.T, outcome string) {
	t.Helper()
	path := strings.TrimSpace(os.Getenv(helperResultEnv))
	if path == "" {
		return
	}
	if err := os.WriteFile(path, []byte(outcome+"\n"), storage.FilePermissions); err != nil {
		t.Fatalf("record %q: %v", path, err)
	}
}

// helperTaskLocator finds one task by title across the whole workspace, reading
// each project from disk. Titles are what the gate script can name; IDs are
// minted by the seed and would have to be passed through a file nobody reads.
func helperTaskLocator(t *testing.T, env helperEnvironment, title string) app.TaskLocator {
	t.Helper()
	for _, project := range env.workspace.Runtime.AvailableProjects() {
		loaded := &models.Project{ID: project.ID, Name: project.Name, Slug: project.Slug, Path: project.Path}
		if err := storage.LoadProject(loaded); err != nil {
			t.Fatalf("LoadProject(%q): %v", project.Name, err)
		}
		for _, task := range loaded.Tasks {
			if task.Title == title {
				return app.TaskLocator{ProjectID: project.ID, TaskID: task.ID}
			}
		}
	}
	t.Fatalf("no task titled %q in %s", title, env.dataDir)
	return app.TaskLocator{}
}

// TestCrossMachineHelperSeedsWorkspace fills an otherwise empty workspace with
// the two projects and the shared tag every other role reads. It is a role and
// not a fixture because the workspace it fills was minted by a different
// process — in the migration case, by an older binary.
func TestCrossMachineHelperSeedsWorkspace(t *testing.T) {
	env := openHelperWorkspace(t)
	if _, err := env.workspace.addProject("Delivery", func(p *models.Project) {
		plan := newFixtureTask(p, "Draft the migration plan")
		plan.ApplyWorkflowState(models.StateBacklog)
		plan.Tags = []string{helperRenameTag}

		vendor := newFixtureTask(p, "Review the vendor contract")
		vendor.ApplyWorkflowState(models.StateBacklog)
		vendor.Tags = []string{helperRenameTag}
	}); err != nil {
		t.Fatalf("seed Delivery: %v", err)
	}
	if _, err := env.workspace.addProject("Support", func(p *models.Project) {
		followUp := newFixtureTask(p, "Follow up with the support queue")
		followUp.ApplyWorkflowState(models.StateBacklog)
		followUp.Tags = []string{helperRenameTag}
	}); err != nil {
		t.Fatalf("seed Support: %v", err)
	}
	if err := env.workspace.persistSyncedDomains(); err != nil {
		t.Fatalf("persist seeded domains: %v", err)
	}
	helperRecord(t, "seeded")
}

// TestCrossMachineHelperPendingRenameExpiresOnCatalogChange is the held half of
// the rename race. A preview pins a whole-workspace census including the catalog
// revision, so this process signals that it is holding one, waits for another
// process to change the catalog, and then applies. Expiring is the contract; a
// commit here would mean a rename was written against a workspace nobody
// previewed.
func TestCrossMachineHelperPendingRenameExpiresOnCatalogChange(t *testing.T) {
	env := openHelperWorkspace(t)
	ready := helperEnv(t, helperReadyEnv)
	proceed := helperEnv(t, helperGoEnv)

	preview, err := env.service.PreviewTaskNameRename(app.RenameTaskNameRequest{
		Namespace: app.TaskTagNamespace,
		From:      helperRenameTag,
		To:        helperRenameDestination,
	})
	if err != nil {
		t.Fatalf("PreviewTaskNameRename: %v", err)
	}
	if len(preview.Tasks) == 0 {
		t.Fatalf("the seeded workspace carries no #%s to rename", helperRenameTag)
	}

	helperSignal(t, ready)
	helperAwait(t, proceed)

	_, err = env.service.ApplyTaskNameRename(preview)
	if err == nil {
		helperRecord(t, "committed")
		t.Fatal("a rename previewed before a catalog change committed anyway")
	}
	if !errors.Is(err, storage.ErrStaleArtifact) && !errors.Is(err, storage.ErrStaleRevision) {
		helperRecord(t, "failed: "+err.Error())
		t.Fatalf("ApplyTaskNameRename error = %v, want a staleness refusal", err)
	}
	helperAssertTagUnwritten(t, env)
	helperRecord(t, "expired")
}

// helperAssertTagUnwritten proves the refusal wrote nothing: the destination
// name appears nowhere, which is a stronger claim than the source name still
// being present would be.
func helperAssertTagUnwritten(t *testing.T, env helperEnvironment) {
	t.Helper()
	for _, project := range env.workspace.Runtime.AvailableProjects() {
		loaded := &models.Project{ID: project.ID, Name: project.Name, Slug: project.Slug, Path: project.Path}
		if err := storage.LoadProject(loaded); err != nil {
			t.Fatalf("LoadProject(%q): %v", project.Name, err)
		}
		for _, task := range loaded.Tasks {
			if slices.Contains(task.Tags, helperRenameDestination) {
				t.Fatalf("task %q carries #%s after an expired rename", task.Title, helperRenameDestination)
			}
		}
	}
}

// TestCrossMachineHelperChangesCatalogDuringPendingRename is the other half: it
// waits until a rename is genuinely held, then commits a catalog change through
// the production transaction path. Creating a project is the smallest honest
// catalog write — it touches no task the rename named, so an expiration can only
// come from the catalog guard itself.
func TestCrossMachineHelperChangesCatalogDuringPendingRename(t *testing.T) {
	env := openHelperWorkspace(t)
	ready := helperEnv(t, helperReadyEnv)
	proceed := helperEnv(t, helperGoEnv)

	helperAwait(t, ready)
	helperCommitNewProject(t, env, "Interrupting Machine")
	helperSignal(t, proceed)
	helperRecord(t, "catalog changed")
}

// helperCommitNewProject creates one project the way the project creator does:
// the cloned catalog and the empty task document ride one transaction, so the
// workspace this leaves behind is one a later doctor still calls clean.
func helperCommitNewProject(t *testing.T, env helperEnvironment, name string) {
	t.Helper()
	runtime := env.workspace.Runtime
	project, err := models.NewProject(name+"-id", name, runtime.ProjectPath(models.Slugify(name)))
	if err != nil {
		t.Fatalf("NewProject(%q): %v", name, err)
	}
	catalog := coordinator.CloneWorkspaceCatalog(runtime.Catalog())
	if err = catalog.UpsertProject(project); err != nil {
		t.Fatalf("UpsertProject(%q): %v", name, err)
	}
	artifacts, err := coordinator.CatalogDomainArtifact(runtime, catalog)
	if err != nil {
		t.Fatalf("CatalogDomainArtifact: %v", err)
	}
	document, err := storage.CanonicalProjectDocument(project, nil)
	if err != nil {
		t.Fatalf("CanonicalProjectDocument(%q): %v", name, err)
	}
	content, err := document.WriteCurrent()
	if err != nil {
		t.Fatalf("WriteCurrent(%q): %v", name, err)
	}
	requireAbsent := ""
	artifacts = append(artifacts, storage.ArtifactChange{
		Path:             storage.TasksFilePath(project.Path),
		Data:             []byte(content),
		Mode:             storage.FilePermissions,
		ExpectedRevision: &requireAbsent,
	})
	if _, err = env.service.CommitProjectMutationResult(storage.OpsCommitRequest{
		Artifacts:   artifacts,
		ProjectIDs:  []string{project.ID},
		UndoAction:  "Create Project",
		UndoContext: project.Name,
	}); err != nil {
		t.Fatalf("commit new project %q: %v", name, err)
	}
}

// TestCrossMachineHelperSetsResumeNote is run twice at once, each side naming
// its own task and its own note. The gate pairs it with two tasks in one
// project, so both processes rewrite one tasks.md having each read it before the
// other committed: the hazard is a lost update, not a lost race. Both sides may
// legitimately commit — a three-way merge resolves edits to distinct tasks — and
// a side that does not has three admissible outcomes: it may expire, it may
// record a conflict bundle, or it may find the note already equal. What it may
// never do is report a commit whose note is not the one on disk, which is what a
// lost update looks like from inside the process that lost it.
func TestCrossMachineHelperSetsResumeNote(t *testing.T) {
	env := openHelperWorkspace(t)
	note := helperEnv(t, helperNoteEnv)
	locator := helperTaskLocator(t, env, helperEnv(t, helperTitleEnv))
	ready := strings.TrimSpace(os.Getenv(helperReadyEnv))
	proceed := helperEnv(t, helperGoEnv)

	if ready != "" {
		helperSignal(t, ready)
	}
	helperAwait(t, proceed)

	result, err := env.service.SetResumeNote(locator, note)
	if err != nil {
		switch {
		case errors.Is(err, storage.ErrMergeConflict):
			// Two processes rewriting one line is the case the three-way merge
			// cannot resolve, so the loser's edit is durably recorded rather than
			// dropped. The bundle id is what the gate checks the workspace for.
			helperRecord(t, "conflicted: "+helperConflictIDs(err))
		case errors.Is(err, storage.ErrStaleRevision), errors.Is(err, storage.ErrStaleArtifact):
			helperRecord(t, "expired")
		default:
			helperRecord(t, "failed: "+err.Error())
			t.Fatalf("SetResumeNote error = %v, want a commit, a staleness refusal, or a recorded conflict", err)
		}
		return
	}
	if !result.Changed {
		helperRecord(t, "unchanged")
		return
	}
	// A commit that reported success has to be readable as this process's own
	// note, or "committed" would be a claim about nothing.
	stored := helperStoredResumeNote(t, env, locator)
	if stored != note {
		helperRecord(t, "lost: "+stored)
		t.Fatalf("stored resume note = %q, want the note this process committed", stored)
	}
	helperRecord(t, "committed")
}

// helperConflictIDs names the bundles a refused commit recorded, so the gate can
// look for them on disk instead of trusting this process's word that a conflict
// happened.
func helperConflictIDs(err error) string {
	var conflict *storage.CommitConflictError
	if !errors.As(err, &conflict) {
		return "unnamed"
	}
	ids := make([]string, 0, len(conflict.Conflicts))
	for _, recorded := range conflict.Conflicts {
		ids = append(ids, recorded.ConflictID)
	}
	return strings.Join(ids, ",")
}

func helperStoredResumeNote(t *testing.T, env helperEnvironment, locator app.TaskLocator) string {
	t.Helper()
	for _, project := range env.workspace.Runtime.AvailableProjects() {
		if project.ID != locator.ProjectID {
			continue
		}
		loaded := &models.Project{ID: project.ID, Name: project.Name, Slug: project.Slug, Path: project.Path}
		if err := storage.LoadProject(loaded); err != nil {
			t.Fatalf("LoadProject(%q): %v", project.Name, err)
		}
		for _, task := range loaded.Tasks {
			if task.ID == locator.TaskID {
				return task.ResumeNote
			}
		}
	}
	t.Fatalf("no task %s/%s on disk", locator.ProjectID, locator.TaskID)
	return ""
}

// TestCrossMachineHelperUsesMigratedWorkspace is the new-schema half of the
// migration fixture: it opens a workspace an older binary minted and a current
// one migrated, and then uses the field that migration admitted. Writing a
// resume note and reading the RESUME block back out of the bytes is what proves
// the migrated workspace is not merely readable but current.
func TestCrossMachineHelperUsesMigratedWorkspace(t *testing.T) {
	env := openHelperWorkspace(t)
	note := helperEnv(t, helperNoteEnv)
	if schema := env.workspace.Runtime.Catalog().SchemaVersion; schema != config.CurrentConfigVersion {
		t.Fatalf("migrated catalog schema = %d, want %d", schema, config.CurrentConfigVersion)
	}

	locator := helperTaskLocator(t, env, helperEnv(t, helperTitleEnv))
	if _, err := env.service.SetResumeNote(locator, note); err != nil {
		t.Fatalf("SetResumeNote on a migrated workspace: %v", err)
	}
	if stored := helperStoredResumeNote(t, env, locator); stored != note {
		t.Fatalf("stored resume note = %q, want %q", stored, note)
	}

	project := env.workspace.Runtime.GetProject(locator.ProjectID)
	if project == nil {
		t.Fatalf("the migrated catalog has no project %s", locator.ProjectID)
	}
	raw, err := os.ReadFile(storage.TasksFilePath(project.Path))
	if err != nil {
		t.Fatalf("read %s: %v", filepath.Base(project.Path), err)
	}
	if !strings.Contains(string(raw), "<!-- RESUME -->") {
		t.Fatalf("migrated tasks.md carries no RESUME block:\n%s", raw)
	}
	helperRecord(t, fmt.Sprintf("schema %d", config.CurrentConfigVersion))
}
