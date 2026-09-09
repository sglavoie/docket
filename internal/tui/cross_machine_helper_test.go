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
	// helperMutationEnv names which mutation the general role runs and
	// helperTextEnv carries the one value that mutation needs — a comment's text,
	// a new task's title, or nothing the review case reads. They are one pair
	// rather than one variable per role because the three mutations differ only
	// in which call they make, not in how they race.
	helperMutationEnv = "TT_HELPER_MUTATION"
	helperTextEnv     = "TT_HELPER_TEXT"

	helperMutationComment = "comment"
	helperMutationReview  = "review"
	helperMutationCreate  = "create"

	// helperRenameTag is the one vocabulary the seeded workspace carries, so the
	// gate never has to pass a name the seed and the rename both have to agree on.
	helperRenameTag = "work"
	// helperRaceProjectName owns the one tasks.md the resume-note race is fought
	// over, and the two titles below are the two tasks inside it. They are named
	// here because the gate script, the seed, and the in-suite regression all have
	// to agree on them.
	helperRaceProjectName = "Delivery"
	helperRaceTitleA      = "Draft the migration plan"
	helperRaceTitleB      = "Review the vendor contract"
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
	helperSignalValue(t, path, "ready")
}

// helperSignalValue publishes one line through a barrier file. The resume-note
// role publishes the project revision its mutation snapshot was taken at, so the
// party that releases both processes can prove they are holding one version of
// the document rather than merely that both are waiting.
//
// The write lands through a rename because the content is now read and not only
// stat'ed: a poller that saw a file being written in place could read it empty
// and compare two revisions that were never observed.
func helperSignalValue(t *testing.T, path, value string) {
	t.Helper()
	staging := path + ".partial"
	if err := os.WriteFile(staging, []byte(value+"\n"), storage.FilePermissions); err != nil {
		t.Fatalf("signal %q: %v", path, err)
	}
	if err := os.Rename(staging, path); err != nil {
		t.Fatalf("publish signal %q: %v", path, err)
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
	if _, err := seedCrossMachineWorkspace(env.workspace); err != nil {
		t.Fatal(err)
	}
	helperRecord(t, "seeded")
}

// seedCrossMachineWorkspace writes the two projects and the one shared tag every
// helper role reads, and returns the project holding the two tasks the
// resume-note race rewrites through a single tasks.md. It is shared with the
// in-suite regression so the forked roles and the test that forks them cannot
// disagree about the fixture they are racing over.
func seedCrossMachineWorkspace(workspace *uitestWorkspace) (*models.Project, error) {
	race, err := workspace.addProject(helperRaceProjectName, func(p *models.Project) {
		plan := newFixtureTask(p, helperRaceTitleA)
		plan.ApplyWorkflowState(models.StateBacklog)
		plan.Tags = []string{helperRenameTag}
		// Both race tasks carry the review flag from the seed. Acknowledgement is
		// only admitted for a task that awaits review, and the two mutations that
		// race have to be the acknowledgements themselves: a helper that flagged
		// its own target first would put an extra write into the document its
		// partner is about to read, and the pair would be racing over that write
		// instead of over the one under test. A flag is inert for every other role.
		plan.Review = true

		vendor := newFixtureTask(p, helperRaceTitleB)
		vendor.ApplyWorkflowState(models.StateBacklog)
		vendor.Tags = []string{helperRenameTag}
		vendor.Review = true
	})
	if err != nil {
		return nil, fmt.Errorf("seed %s: %w", helperRaceProjectName, err)
	}
	if _, err = workspace.addProject("Support", func(p *models.Project) {
		followUp := newFixtureTask(p, "Follow up with the support queue")
		followUp.ApplyWorkflowState(models.StateBacklog)
		followUp.Tags = []string{helperRenameTag}
	}); err != nil {
		return nil, fmt.Errorf("seed Support: %w", err)
	}
	if err = workspace.persistSyncedDomains(); err != nil {
		return nil, fmt.Errorf("persist seeded domains: %w", err)
	}
	return race, nil
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
//
// The barrier is installed inside SetResumeNote rather than wrapped around it.
// The locator resolved out here reads a snapshot the mutation then discards, so
// a pair released before the call would prove only that both processes had found
// a task: one of them could still read, commit and exit before the other read
// anything, and the "race" would be two sequential writes. Held at
// app.TaskCommitBarrier, both processes are demonstrably holding the same
// document version with nothing yet written, which is what the revision each one
// publishes says.
func TestCrossMachineHelperSetsResumeNote(t *testing.T) {
	env := openHelperWorkspace(t)
	note := helperEnv(t, helperNoteEnv)
	locator := helperTaskLocator(t, env, helperEnv(t, helperTitleEnv))
	defer helperHoldAtCommit(t, "Set Resume Note")()

	result, err := env.service.SetResumeNote(locator, note)
	if err != nil {
		helperRecordRefusal(t, "SetResumeNote", err)
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

// TestCrossMachineHelperAppliesTaskMutation is the resume-note role generalized
// to the three mutations this release added a key for: appending a comment,
// acknowledging a review, and creating a fresh task. All three are single-task
// writes into one shared tasks.md, so all three carry the same lost-update
// hazard and all three are held at the same seam; only the call and the
// on-disk claim differ, which is why they are one role selected by
// TT_HELPER_MUTATION rather than three near-copies.
//
// Creation is the sharpest of the three. The other two rewrite a task each
// machine already resolved, while two creations mint two identities and two
// short IDs against one document that named neither, so a merge that dropped
// one would be invisible to the machine that lost it. It is also the only one of
// the three whose loser is guaranteed to lose: both machines mint the same short
// id, so the second commit's merge is textually clean and semantically invalid,
// and the contract it proves is the classified refusal plus the retry.
func TestCrossMachineHelperAppliesTaskMutation(t *testing.T) {
	env := openHelperWorkspace(t)
	kind := helperEnv(t, helperMutationEnv)
	text := strings.TrimSpace(os.Getenv(helperTextEnv))
	title := helperEnv(t, helperTitleEnv)

	switch kind {
	case helperMutationComment:
		locator := helperTaskLocator(t, env, title)
		defer helperHoldAtCommit(t, "Add Comment")()
		if _, err := env.service.AddComment(locator, text); err != nil {
			helperRecordRefusal(t, "AddComment", err)
			return
		}
		helperRequireStored(t, kind, func() bool {
			task := helperStoredTask(t, env, locator)
			return task != nil && slices.ContainsFunc(task.Comments, func(c *models.Comment) bool {
				return c != nil && c.Text == text
			})
		})
	case helperMutationReview:
		locator := helperTaskLocator(t, env, title)
		defer helperHoldAtCommit(t, "Mark Reviewed")()
		if _, err := env.service.MarkReviewed(locator); err != nil {
			helperRecordRefusal(t, "MarkReviewed", err)
			return
		}
		helperRequireStored(t, kind, func() bool {
			task := helperStoredTask(t, env, locator)
			return task != nil && !task.Review
		})
	case helperMutationCreate:
		// The title names the project's existing task the new one joins, so the
		// creation lands in the document the other machine is rewriting.
		project := helperTaskLocator(t, env, title).ProjectID
		create := func() error {
			_, err := env.service.AddTask(app.AddTaskRequest{
				ProjectID: project,
				Title:     text,
				State:     models.StateBacklog,
			})
			return err
		}
		release := helperHoldAtCommit(t, "Add Task")
		err := create()
		// Released before the retry rather than deferred: the retry is a second
		// commit of the same undo action, and a barrier still installed would hold
		// it at a rendezvous the partner has already left.
		release()
		if err != nil {
			if !errors.Is(err, storage.ErrMergeConflict) {
				helperRecordRefusal(t, "AddTask", err)
				return
			}
			// Two machines minting an identity against one document produce the one
			// merge that is textually clean and still an invalid project: both
			// additions are non-overlapping, so both survive, and both answer to the
			// short id that was next free in the document neither had seen the other
			// write. The loser's whole commit is refused and recorded as a retry-only
			// bundle, so its task is nowhere yet and the retry — which reads what the
			// winner left — is what actually completes this creation.
			recorded := helperConflictIDs(err)
			if helperStoredTaskTitled(t, env, project, text) != nil {
				helperRecord(t, "lost: a refused creation wrote its task anyway")
				t.Fatal("a creation refused as a conflict left partial task data on disk")
			}
			if retryErr := create(); retryErr != nil {
				helperRecord(t, "failed: "+retryErr.Error())
				t.Fatalf("retrying a creation refused as %s: %v", recorded, retryErr)
			}
			helperRequireStored(t, kind, func() bool {
				return helperStoredTaskTitled(t, env, project, text) != nil
			})
			helperRecord(t, "retried: "+recorded)
			return
		}
		helperRequireStored(t, kind, func() bool {
			return helperStoredTaskTitled(t, env, project, text) != nil
		})
	default:
		t.Fatalf("%s=%q names no helper mutation", helperMutationEnv, kind)
	}
	helperRecord(t, "committed")
}

// helperHoldAtCommit installs the shared commit barrier for one undo action and
// returns the deferred removal. Filtering on the action is what lets a role
// whose mutation opens more than one transaction — or a process that reconciles
// something on the way past — be held at exactly the write under test.
func helperHoldAtCommit(t *testing.T, undoAction string) func() {
	t.Helper()
	ready := strings.TrimSpace(os.Getenv(helperReadyEnv))
	proceed := helperEnv(t, helperGoEnv)
	app.TaskCommitBarrier = func(action string, _ app.TaskLocator, expectedRevision string) {
		if action != undoAction {
			return
		}
		if ready != "" {
			helperSignalValue(t, ready, expectedRevision)
		}
		helperAwait(t, proceed)
	}
	return func() { app.TaskCommitBarrier = nil }
}

// helperRecordRefusal classifies the three refusals a losing machine may
// legitimately report and fails on anything else. Losing a race is a result;
// failing for another reason is a defect, and the gate has to be able to tell
// them apart from the recorded line alone.
func helperRecordRefusal(t *testing.T, call string, err error) {
	t.Helper()
	switch {
	case errors.Is(err, storage.ErrMergeConflict):
		// Two processes rewriting one region is the case the three-way merge
		// cannot resolve, so the loser's edit is durably recorded rather than
		// dropped. The bundle id is what the gate checks the workspace for.
		helperRecord(t, "conflicted: "+helperConflictIDs(err))
	case errors.Is(err, storage.ErrStaleRevision), errors.Is(err, storage.ErrStaleArtifact):
		helperRecord(t, "expired")
	default:
		helperRecord(t, "failed: "+err.Error())
		t.Fatalf("%s error = %v, want a commit, a staleness refusal, or a recorded conflict", call, err)
	}
}

// helperRequireStored turns a reported commit into a claim about the bytes. A
// machine that says "committed" and cannot find its own write on disk is the
// lost update this whole gate exists to catch, so it records that and fails
// rather than reporting success.
func helperRequireStored(t *testing.T, kind string, stored func() bool) {
	t.Helper()
	if !stored() {
		helperRecord(t, "lost")
		t.Fatalf("the %s this process committed is not on disk", kind)
	}
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
	task := helperStoredTask(t, env, locator)
	if task == nil {
		t.Fatalf("no task %s/%s on disk", locator.ProjectID, locator.TaskID)
	}
	return task.ResumeNote
}

// helperReloadProject reads one project's tasks.md back off disk. Every stored
// claim goes through it rather than through the runtime this process already
// holds: the point of the assertion is what the bytes say after the other
// machine's write merged into them, and an in-memory task would answer with what
// this process wrote regardless.
func helperReloadProject(t *testing.T, env helperEnvironment, projectID string) *models.Project {
	t.Helper()
	for _, project := range env.workspace.Runtime.AvailableProjects() {
		if project.ID != projectID {
			continue
		}
		loaded := &models.Project{ID: project.ID, Name: project.Name, Slug: project.Slug, Path: project.Path}
		if err := storage.LoadProject(loaded); err != nil {
			t.Fatalf("LoadProject(%q): %v", project.Name, err)
		}
		return loaded
	}
	return nil
}

// helperStoredTask returns the identified task as the file now holds it, or nil
// when the merge did not keep it.
func helperStoredTask(t *testing.T, env helperEnvironment, locator app.TaskLocator) *models.Task {
	t.Helper()
	loaded := helperReloadProject(t, env, locator.ProjectID)
	if loaded == nil {
		return nil
	}
	for _, task := range loaded.Tasks {
		if task.ID == locator.TaskID {
			return task
		}
	}
	return nil
}

// helperStoredTaskTitled is the creation role's reading of the same file. A new
// task has no identity the gate could have been told in advance, so the title
// this process minted is what it looks for.
func helperStoredTaskTitled(t *testing.T, env helperEnvironment, projectID, title string) *models.Task {
	t.Helper()
	loaded := helperReloadProject(t, env, projectID)
	if loaded == nil {
		return nil
	}
	for _, task := range loaded.Tasks {
		if task.Title == title {
			return task
		}
	}
	return nil
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
