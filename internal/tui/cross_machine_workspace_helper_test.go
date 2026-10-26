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

	"github.com/sglavoie/docket/internal/app"
	"github.com/sglavoie/docket/internal/config"
	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/storage"
	"github.com/sglavoie/docket/internal/tui/coordinator"
)

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

// TestCrossMachineHelperOpensWorkspace opens a workspace the way a launch does,
// migrating one owed an upgrade, and writes nothing else. It is the migration
// case's upgrade step: doctor refuses such a workspace, so opening is the one
// route across it.
func TestCrossMachineHelperOpensWorkspace(t *testing.T) {
	dataDir := strings.TrimSpace(os.Getenv(helperDataDirEnv))
	if dataDir == "" {
		t.Skipf("%s is unset; this test is a cross-machine helper role", helperDataDirEnv)
	}
	if _, err := config.NewManager(config.ConfigPathForDataDir(dataDir)).LoadOrCreateDomains(); err != nil {
		t.Fatalf("LoadOrCreateDomains(%q): %v", dataDir, err)
	}
	helperRecord(t, "opened")
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
		// A checklist on the same task, for the same reason the flags and the
		// comments are here: a promotion race has to be fought over items that were
		// on disk before either machine started. A helper that appended its own item
		// first would put an extra write into the document its partner is about to
		// read, and the frozen-checklist identity the promotion is refused by would
		// then be measuring that write rather than the race. Three items so a
		// reorder has somewhere to move one to, and all three unchecked so a
		// promotion lands in Backlog rather than Done.
		plan.Checklist = []models.ChecklistItem{
			{Text: helperSeedItemA},
			{Text: helperSeedItemB},
			{Text: helperSeedItemC},
		}

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
