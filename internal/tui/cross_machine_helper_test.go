package tui

import (
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/sglavoie/docket/internal/app"
	"github.com/sglavoie/docket/internal/config"
	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/storage"
	"github.com/sglavoie/docket/internal/tui/coordinator"
)

// Helper processes for scripts/cross_machine_gate.sh. Each test below is one
// role in a race the in-process suite cannot express: two real processes, each
// with its own machine identity, holding their own locks over one workspace on
// disk. They are compiled once with `go test -c ./internal/tui` and invoked by
// name, so the gate spends one build on a binary it then forks repeatedly.
//
// Every role skips unless DOCKET_HELPER_DATA_DIR names the workspace it is to open,
// which is what keeps them inert under `just ci`: an ordinary `go test ./...`
// runs each one, finds no workspace, and skips.
//
// Machine identity is $HOME and TestMain gives each process a fresh temporary
// one, so two helpers are always two machines without the gate arranging it.

const (
	helperDataDirEnv = "DOCKET_HELPER_DATA_DIR"
	helperReadyEnv   = "DOCKET_HELPER_READY_FILE"
	helperGoEnv      = "DOCKET_HELPER_GO_FILE"
	helperResultEnv  = "DOCKET_HELPER_RESULT_FILE"
	helperTitleEnv   = "DOCKET_HELPER_TASK_TITLE"
	helperNoteEnv    = "DOCKET_HELPER_RESUME_NOTE"
	// helperMutationEnv names which mutation the general role runs and
	// helperTextEnv carries the one value that mutation needs — a comment's text,
	// a new task's title, or nothing the review case reads. They are one pair
	// rather than one variable per role because the three mutations differ only
	// in which call they make, not in how they race.
	helperMutationEnv = "DOCKET_HELPER_MUTATION"
	helperTextEnv     = "DOCKET_HELPER_TEXT"

	helperMutationComment = "comment"
	helperMutationReview  = "review"
	helperMutationCreate  = "create"

	// helperProjectEnv names the project a project-scoped role mutates, and
	// helperProjectCommentEnv which of the three project-comment calls it makes.
	// The selector is a second variable rather than another helperMutationEnv
	// value because the two roles have to be able to race each other: a mixed
	// pair sets both, and each role reads only its own.
	//
	// helperTargetEnv addresses the existing comment an edit or a removal
	// rewrites. It carries the comment's current text and not its identity: a
	// comment's UUID is minted by the seed, so naming one would mean passing a
	// value through a file, while the text is something the gate script, the seed,
	// and the in-suite race can all spell.
	helperProjectEnv        = "DOCKET_HELPER_PROJECT"
	helperProjectCommentEnv = "DOCKET_HELPER_PROJECT_COMMENT"
	helperTargetEnv         = "DOCKET_HELPER_TARGET_TEXT"

	helperProjectCommentAdd    = "add"
	helperProjectCommentEdit   = "edit"
	helperProjectCommentRemove = "remove"

	// helperSeedCommentA and helperSeedCommentB are the two project comments the
	// seed writes onto the race project, so an edit or a removal has something to
	// address that neither machine had to create first. A helper that minted its
	// own target would put an extra write into the document its partner is about
	// to read, and the pair would be racing over that write instead of the one
	// under test — the same reason the seed carries the review flags.
	helperSeedCommentA = "Kickoff notes live in the shared drive"
	helperSeedCommentB = "The vendor contract review is booked for the 12th"

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

// helperSeedCommentInstant is the moment the two seeded project comments were
// created at. It is a fixed date rather than the clock so the canonical order
// the merge sorts by is the order written here, and so a document seeded twice
// in one gate run cannot differ in a way no assertion names.
var helperSeedCommentInstant = time.Date(2026, time.January, 5, 9, 0, 0, 0, time.UTC)

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
		// The planning builder is wired the way the composition root wires it. A
		// mutation that reconciles Today — a deadline step landing on today is one —
		// refuses outright without it, so a helper service missing it would report a
		// refusal that had nothing to do with the race it was forked for.
		service: newTUIAppService(dataDir, manager.RuntimeDependencies(), ledger).
			WithPlanningArtifacts(coordinator.ManagerPlanningArtifacts(manager)),
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

// helperHoldAtCommit installs the shared commit barrier for one undo action on
// one target and returns the deferred removal. Filtering is what lets a role
// whose mutation opens more than one transaction — or a process that reconciles
// something on the way past — be held at exactly the write under test: the
// action names which mutation, the project names which document that mutation is
// about to rewrite, and the task names which row inside it. Each axis is skipped
// when the caller leaves it empty, which is how a creation — whose task has no
// identity until the commit that mints it — and a project comment, which names
// no task at all, are held on the axes they can actually spell.
func helperHoldAtCommit(t *testing.T, undoAction, projectID, taskID string) func() {
	t.Helper()
	ready := strings.TrimSpace(os.Getenv(helperReadyEnv))
	proceed := helperEnv(t, helperGoEnv)
	app.CommitBarrier = func(action, heldProject, heldTask, expectedRevision string) {
		if action != undoAction {
			return
		}
		if projectID != "" && heldProject != projectID {
			return
		}
		if taskID != "" && heldTask != taskID {
			return
		}
		if ready != "" {
			helperSignalValue(t, ready, expectedRevision)
		}
		helperAwait(t, proceed)
	}
	return func() { app.CommitBarrier = nil }
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
