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

	"github.com/google/uuid"
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
	helperProjectEnv        = "TT_HELPER_PROJECT"
	helperProjectCommentEnv = "TT_HELPER_PROJECT_COMMENT"
	helperTargetEnv         = "TT_HELPER_TARGET_TEXT"

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

		// Two project comments so an edit and a removal each have a target that
		// was already on disk before either machine started. They are inert for
		// every other role: nothing reads a project comment unless it was asked to.
		p.Comments = []*models.ProjectComment{
			{ID: uuid.New().String(), Text: helperSeedCommentA, CreatedAt: helperSeedCommentInstant},
			{ID: uuid.New().String(), Text: helperSeedCommentB, CreatedAt: helperSeedCommentInstant.Add(time.Hour)},
		}
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
// app.CommitBarrier, both processes are demonstrably holding the same
// document version with nothing yet written, which is what the revision each one
// publishes says.
func TestCrossMachineHelperSetsResumeNote(t *testing.T) {
	env := openHelperWorkspace(t)
	note := helperEnv(t, helperNoteEnv)
	locator := helperTaskLocator(t, env, helperEnv(t, helperTitleEnv))
	defer helperHoldAtCommit(t, "Set Resume Note", locator.ProjectID, locator.TaskID)()

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
		defer helperHoldAtCommit(t, "Add Comment", locator.ProjectID, locator.TaskID)()
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
		defer helperHoldAtCommit(t, "Mark Reviewed", locator.ProjectID, locator.TaskID)()
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
		// No task axis: the identity this commit will carry is minted by the
		// commit the barrier is holding, so the project is all a caller can name.
		release := helperHoldAtCommit(t, "Add Task", project, "")
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

// TestCrossMachineHelperAppliesProjectCommentMutation is the project-scoped
// sibling of the task-mutation role: two machines adding, editing, or deleting
// comments on one project, each held at the same commit seam its partner is.
//
// It is a role of its own rather than a fourth TT_HELPER_MUTATION value for two
// reasons. A project comment names no task, so it resolves its target through a
// comment identity rather than a locator and its barrier target carries no task
// id at all. And the two roles have to be able to be the two sides of one race —
// a project comment and a task edit rewriting one tasks.md is a hazard neither
// role can express alone — which they could not be if selecting one meant not
// selecting the other.
//
// The three calls differ in what a refusal has to prove. An addition and an edit
// each introduce text, so the claim is that a refused one left that text nowhere;
// a removal introduces none, so the claim is that the comment it addressed is
// still there, checked by identity rather than by text because the machine that
// beat it may have been an edit.
func TestCrossMachineHelperAppliesProjectCommentMutation(t *testing.T) {
	env := openHelperWorkspace(t)
	kind := helperEnv(t, helperProjectCommentEnv)
	projectID := helperProjectID(t, env, helperEnv(t, helperProjectEnv))
	text := strings.TrimSpace(os.Getenv(helperTextEnv))

	switch kind {
	case helperProjectCommentAdd:
		defer helperHoldAtCommit(t, "Add Project Comment", projectID, "")()
		if _, err := env.service.AddProjectComment(projectID, text); err != nil {
			helperRefusedWithoutWriting(t, env, projectID, "AddProjectComment", text, err)
			return
		}
		helperRequireStored(t, kind, func() bool {
			return helperProjectCommentCount(t, env, projectID, text) == 1
		})
	case helperProjectCommentEdit:
		target := helperProjectCommentTarget(t, env, projectID, helperEnv(t, helperTargetEnv))
		defer helperHoldAtCommit(t, "Edit Project Comment", projectID, "")()
		if _, err := env.service.EditProjectComment(target, text); err != nil {
			helperRefusedWithoutWriting(t, env, projectID, "EditProjectComment", text, err)
			return
		}
		helperRequireStored(t, kind, func() bool {
			stored := helperProjectCommentByID(t, env, projectID, target.CommentID)
			return stored != nil && stored.Text == text
		})
	case helperProjectCommentRemove:
		target := helperProjectCommentTarget(t, env, projectID, helperEnv(t, helperTargetEnv))
		defer helperHoldAtCommit(t, "Remove Project Comment", projectID, "")()
		if _, err := env.service.RemoveProjectComment(target); err != nil {
			helperRequireStored(t, kind, func() bool {
				return helperProjectCommentByID(t, env, projectID, target.CommentID) != nil
			})
			helperRecordRefusal(t, "RemoveProjectComment", err)
			return
		}
		helperRequireStored(t, kind, func() bool {
			return helperProjectCommentByID(t, env, projectID, target.CommentID) == nil
		})
	default:
		t.Fatalf("%s=%q names no project-comment mutation", helperProjectCommentEnv, kind)
	}
	helperRecord(t, "committed")
}

// helperProjectID finds one project by name. A name is what the gate script and
// the seed can both spell; the identity is minted by the seed.
func helperProjectID(t *testing.T, env helperEnvironment, name string) string {
	t.Helper()
	for _, project := range env.workspace.Runtime.AvailableProjects() {
		if project.Name == name {
			return project.ID
		}
	}
	t.Fatalf("no project named %q in %s", name, env.dataDir)
	return ""
}

// helperStoredProjectComments reads one project's comments back off disk, for
// the same reason every other stored claim reloads: the question is what the
// bytes say after the other machine's write merged into them.
func helperStoredProjectComments(t *testing.T, env helperEnvironment, projectID string) []*models.ProjectComment {
	t.Helper()
	loaded := helperReloadProject(t, env, projectID)
	if loaded == nil {
		return nil
	}
	return loaded.Comments
}

func helperProjectCommentByID(t *testing.T, env helperEnvironment, projectID, commentID string) *models.ProjectComment {
	t.Helper()
	for _, comment := range helperStoredProjectComments(t, env, projectID) {
		if comment != nil && comment.ID == commentID {
			return comment
		}
	}
	return nil
}

// helperProjectCommentCount is how "exactly once" is asserted. A merge that kept
// two machines' additions is correct; a merge that kept one of them twice is the
// duplication an identity-keyed reconciliation exists to prevent, and counting is
// the only way to tell the two apart from the outside.
func helperProjectCommentCount(t *testing.T, env helperEnvironment, projectID, text string) int {
	t.Helper()
	count := 0
	for _, comment := range helperStoredProjectComments(t, env, projectID) {
		if comment != nil && comment.Text == text {
			count++
		}
	}
	return count
}

// helperProjectCommentTarget addresses one stored comment by its current text,
// resolving the identity and the precondition the application layer re-checks.
// It is read before the barrier is installed, so the target is the document as it
// stood before either machine moved.
func helperProjectCommentTarget(t *testing.T, env helperEnvironment, projectID, text string) app.ProjectCommentTarget {
	t.Helper()
	for _, comment := range helperStoredProjectComments(t, env, projectID) {
		if comment != nil && comment.Text == text {
			return app.ProjectCommentTarget{ProjectID: projectID, CommentID: comment.ID, Original: comment.Text}
		}
	}
	t.Fatalf("no project comment %q on project %s", text, projectID)
	return app.ProjectCommentTarget{}
}

// helperRefusedWithoutWriting classifies a refusal only after proving it left
// nothing behind. A conflict that recorded a bundle and still wrote half its
// document would satisfy every outcome check the gate makes, so the absence is
// asserted here — inside the process that was refused — rather than inferred.
func helperRefusedWithoutWriting(t *testing.T, env helperEnvironment, projectID, call, unwritten string, err error) {
	t.Helper()
	if count := helperProjectCommentCount(t, env, projectID, unwritten); count != 0 {
		helperRecord(t, "lost: a refused project comment wrote its text anyway")
		t.Fatalf("%s was refused with %v but left %q on disk %d time(s)", call, err, unwritten, count)
	}
	helperRecordRefusal(t, call, err)
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

// TestCrossMachineHelperUsesMigratedWorkspace is the new-schema half of the
// migration fixture: it opens a workspace an older binary minted and a current
// one migrated, and then uses the two blocks that migration admitted. Writing a
// resume note and a project comment and reading both back out of the bytes is
// what proves the migrated workspace is not merely readable but current.
//
// Both blocks are written here rather than in two roles because they are the
// same claim about the same document at two levels: RESUME is a task-level block
// the v18 marker bump admitted and PROJECT-COMMENTS is the document-level block
// v19 admitted, and a migration that produced a document able to carry one and
// not the other would pass a check that named only one of them.
func TestCrossMachineHelperUsesMigratedWorkspace(t *testing.T) {
	env := openHelperWorkspace(t)
	note := helperEnv(t, helperNoteEnv)
	comment := helperEnv(t, helperTextEnv)
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

	// The comment is reread through the loader rather than taken from the result,
	// because the claim is that the migrated document round-trips the block: a
	// write whose bytes no reader could parse back would report success here.
	written, err := env.service.AddProjectComment(locator.ProjectID, comment)
	if err != nil {
		t.Fatalf("AddProjectComment on a migrated workspace: %v", err)
	}
	stored := helperProjectCommentByID(t, env, locator.ProjectID, written.CommentID)
	if stored == nil {
		t.Fatalf("the project comment written to a migrated workspace did not read back")
	}
	if stored.Text != comment {
		t.Fatalf("stored project comment = %q, want %q", stored.Text, comment)
	}

	project := env.workspace.Runtime.GetProject(locator.ProjectID)
	if project == nil {
		t.Fatalf("the migrated catalog has no project %s", locator.ProjectID)
	}
	raw, err := os.ReadFile(storage.TasksFilePath(project.Path))
	if err != nil {
		t.Fatalf("read %s: %v", filepath.Base(project.Path), err)
	}
	for _, block := range []string{"<!-- RESUME -->", "<!-- PROJECT-COMMENTS -->"} {
		if !strings.Contains(string(raw), block) {
			t.Fatalf("migrated tasks.md carries no %s block:\n%s", block, raw)
		}
	}
	helperRecord(t, fmt.Sprintf("schema %d", config.CurrentConfigVersion))
}
