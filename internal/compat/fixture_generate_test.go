package compat

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sglavoie/docket/internal/config"
	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/storage"
)

// regenerateFixturesEnv opts into rewriting the committed representative
// workspace. The fixture is built through the live writers rather than by hand,
// so its bytes are what this build would publish; regenerating it is a reviewed
// diff, never a side effect of an ordinary test run.
const regenerateFixturesEnv = "UPDATE_COMPAT_FIXTURES"

// Stable identities for the representative workspace. Fixed UUIDs keep a
// regenerated fixture diffable and let the census name what it expects.
const (
	fixtureAtlasID    = "6f1d3c2a-1b4e-4c8a-9a0e-0d5b7c1e2a01"
	fixtureHomeID     = "6f1d3c2a-1b4e-4c8a-9a0e-0d5b7c1e2a02"
	fixtureReadingID  = "6f1d3c2a-1b4e-4c8a-9a0e-0d5b7c1e2a03"
	fixtureArchivedID = "6f1d3c2a-1b4e-4c8a-9a0e-0d5b7c1e2a04"

	fixtureShipID      = "0a7e5d10-2c3b-4f6a-8e9d-000000000101"
	fixtureNotesID     = "0a7e5d10-2c3b-4f6a-8e9d-000000000102"
	fixtureProofID     = "0a7e5d10-2c3b-4f6a-8e9d-000000000103"
	fixturePromotedID  = "0a7e5d10-2c3b-4f6a-8e9d-000000000104"
	fixtureCertID      = "0a7e5d10-2c3b-4f6a-8e9d-000000000105"
	fixturePausedID    = "0a7e5d10-2c3b-4f6a-8e9d-000000000106"
	fixtureSomedayID   = "0a7e5d10-2c3b-4f6a-8e9d-000000000107"
	fixtureDoneID      = "0a7e5d10-2c3b-4f6a-8e9d-000000000108"
	fixtureCanceledID  = "0a7e5d10-2c3b-4f6a-8e9d-000000000109"
	fixturePaintID     = "0a7e5d10-2c3b-4f6a-8e9d-000000000201"
	fixtureTilesID     = "0a7e5d10-2c3b-4f6a-8e9d-000000000202"
	fixtureNovelID     = "0a7e5d10-2c3b-4f6a-8e9d-000000000301"
	fixtureLegacyID    = "0a7e5d10-2c3b-4f6a-8e9d-000000000401"
	fixtureHandoverID  = "0a7e5d10-2c3b-4f6a-8e9d-000000000402"
	fixtureUnfiledTask = "0a7e5d10-2c3b-4f6a-8e9d-000000000501"

	fixtureProjectCommentA = "3c9b2f4e-5d6a-4b7c-8d9e-0f1a2b3c4d01"
	fixtureProjectCommentB = "3c9b2f4e-5d6a-4b7c-8d9e-0f1a2b3c4d02"

	fixtureScopedLensID = "9e8d7c6b-5a49-4382-a1b0-c9d8e7f6a501"
	fixtureTodayLensID  = "9e8d7c6b-5a49-4382-a1b0-c9d8e7f6a502"

	fixtureAttachmentName = "release-checklist.txt"
)

// TestRegenerateRepresentativeWorkspaceFixture rebuilds
// testdata/workspaces/representative through the live config and storage
// writers. Run it with:
//
//	UPDATE_COMPAT_FIXTURES=1 go test ./internal/compat/ -run TestRegenerateRepresentativeWorkspaceFixture
func TestRegenerateRepresentativeWorkspaceFixture(t *testing.T) {
	if os.Getenv(regenerateFixturesEnv) != "1" {
		t.Skipf("set %s=1 to regenerate the committed compatibility fixture", regenerateFixturesEnv)
	}
	home := t.TempDir()
	t.Setenv("HOME", home)
	dataDir := filepath.Join(t.TempDir(), "workspace")
	manager := config.NewManagerOnMachine(config.ConfigPathForDataDir(dataDir), home)
	if _, err := manager.LoadOrCreateDomains(); err != nil {
		t.Fatalf("bootstrap workspace: %v", err)
	}
	runtime, err := config.LoadRuntimeState(dataDir)
	if err != nil {
		t.Fatalf("load runtime: %v", err)
	}

	projects := buildRepresentativeProjects(t, runtime)
	catalog := runtime.Catalog()
	for _, project := range projects {
		if runtime.IsSystemUnfiledProject(project.ID) {
			continue
		}
		if err := catalog.UpsertProject(project); err != nil {
			t.Fatalf("catalog %s: %v", project.Slug, err)
		}
	}
	if err := manager.CommitCatalog(catalog); err != nil {
		t.Fatalf("commit catalog: %v", err)
	}

	planning := runtime.Planning()
	planning.SetToday([]config.ProjectTaskLocator{
		{ProjectID: fixtureAtlasID, TaskID: fixtureShipID},
		{ProjectID: fixtureHomeID, TaskID: fixtureTilesID},
	})
	planning.SetNext([]config.ProjectTaskLocator{
		{ProjectID: fixtureHomeID, TaskID: fixturePaintID},
		{ProjectID: fixtureReadingID, TaskID: fixtureNovelID},
	})
	if err := manager.CommitPlanning(planning); err != nil {
		t.Fatalf("commit planning: %v", err)
	}

	if err := manager.DomainCoordinator().CommitLensDefinitions(&config.WorkspaceLensDefinitions{
		Lenses: config.NamedLensList{
			{ID: fixtureScopedLensID, Name: "Atlas release", Mode: config.LensModeAll, Scope: fixtureAtlasID, Query: "#release", Sort: config.LensSort{Field: config.LensSortDue, Ascending: true}},
			{ID: fixtureTodayLensID, Name: "Today focus", Mode: config.LensModeToday},
		},
	}); err != nil {
		t.Fatalf("commit lenses: %v", err)
	}

	for _, project := range projects {
		writeFixtureProject(t, project)
	}
	writeFixtureNotes(t, dataDir, projects[0])
	writeFixtureAttachment(t, projects[0])

	target := filepath.Join("testdata", "workspaces", "representative")
	if err := os.RemoveAll(target); err != nil {
		t.Fatal(err)
	}
	copyPortableWorkspace(t, dataDir, target)
	t.Logf("regenerated %s", target)
}

func fixtureInstant(day, hour int) time.Time {
	return time.Date(2026, time.September, day, hour, 0, 0, 0, time.UTC)
}

func fixtureDate(day int) *time.Time {
	value := time.Date(2026, time.October, day, 0, 0, 0, 0, time.UTC)
	return &value
}

func fixturePointer(value time.Time) *time.Time { return &value }

func fixtureTask(id string, shortID int, title string, state models.WorkflowState, day int) *models.Task {
	at := fixtureInstant(day, 9)
	task := &models.Task{
		ID:             id,
		ShortID:        shortID,
		Title:          title,
		WorkflowState:  state,
		CreatedAt:      at,
		UpdatedAt:      at,
		StateChangedAt: at,
	}
	if state.IsTerminal() {
		task.CompletedAt = fixturePointer(at)
	}
	return task
}

func fixtureProject(t *testing.T, runtime *config.RuntimeState, id, name string) *models.Project {
	t.Helper()
	project, err := models.NewProject(id, name, "")
	if err != nil {
		t.Fatal(err)
	}
	project.Path = runtime.ProjectPath(project.Slug)
	project.CreatedAt = fixtureInstant(1, 8)
	project.UpdatedAt = fixtureInstant(1, 8)
	return project
}

func fixtureParent(task *models.Task, parent *models.Task) {
	id := parent.ID
	task.ParentID = &id
}

// buildRepresentativeProjects returns the workspace's projects, the rich one
// first. Each feature the fixture manifest claims is placed here on purpose; the
// census in compat_fixture_test.go is what proves it survived the round trip.
func buildRepresentativeProjects(t *testing.T, runtime *config.RuntimeState) []*models.Project {
	t.Helper()

	atlas := fixtureProject(t, runtime, fixtureAtlasID, "Atlas Launch")
	atlas.Description = "Ship the Atlas desktop build and its companion notes."
	atlas.Tags = []string{"client", "work"}
	atlas.Pinned = true
	atlas.PinnedAt = fixturePointer(fixtureInstant(1, 9))
	atlas.Comments = []*models.ProjectComment{
		{ID: fixtureProjectCommentA, Text: "Kickoff agreed: beta before October.", CreatedAt: fixtureInstant(2, 10)},
		{ID: fixtureProjectCommentB, Text: "Signing certificate is the long pole.\nTrack it weekly.", CreatedAt: fixtureInstant(3, 11)},
	}

	ship := fixtureTask(fixtureShipID, 1, "Ship beta build", models.StateDoing, 4)
	ship.Description = "Cut the release branch.\n\n- build\n- notarize\n- upload"
	ship.Priority = 1
	ship.Pinned = true
	ship.PinnedAt = fixturePointer(fixtureInstant(4, 10))
	ship.Tags = []string{"feature", "release"}
	ship.Context = "laptop"
	ship.DueDate = fixturePointer(time.Date(2026, time.October, 9, 17, 30, 0, 0, time.UTC))
	ship.NextCheckIn = fixtureDate(8)
	ship.CheckInRepeatDays = 7
	ship.LastReviewedAt = fixturePointer(fixtureInstant(30, 16))
	ship.ResumeNote = "Waiting on notarization log; rerun step 3."
	ship.Comments = []*models.Comment{
		{Text: "Branch cut from main.", CreatedAt: fixtureInstant(5, 12)},
		{Text: "Notarization failed once; retrying.", CreatedAt: fixtureInstant(6, 12)},
	}
	ship.Attachments = []models.Attachment{
		{Filename: fixtureAttachmentName, OriginalPath: "/tmp/" + fixtureAttachmentName, AddedAt: fixtureInstant(5, 13)},
		{Filename: "https://example.com/atlas/release", IsURL: true, OriginalPath: "https://example.com/atlas/release", AddedAt: fixtureInstant(5, 14)},
		{Filename: "cmd/release.go:42", IsCodeRef: true, LineNumber: 42, OriginalPath: "cmd/release.go", AddedAt: fixtureInstant(5, 15)},
	}
	ship.Dependencies = []models.TaskReference{{ProjectID: fixtureHomeID, TaskID: fixturePaintID}}

	notes := fixtureTask(fixtureNotesID, 2, "Write release notes", models.StateBacklog, 5)
	fixtureParent(notes, ship)
	notes.Priority = 2
	notes.Checklist = []models.ChecklistItem{{Text: "Gather changelog", Done: true}, {Text: "Draft summary"}}
	notes.DeferredUntil = fixturePointer(time.Date(2026, time.October, 7, 9, 0, 0, 0, time.UTC))
	notes.DeferCount = 2

	proof := fixtureTask(fixtureProofID, 3, "Proofread release notes", models.StateInbox, 6)
	fixtureParent(proof, notes)
	proof.Review = true

	promoted := fixtureTask(fixturePromotedID, 4, "Collect screenshots", models.StateBacklog, 6)
	fixtureParent(promoted, notes)
	promoted.ChecklistOrigin = &models.ChecklistOrigin{ProjectID: fixtureAtlasID, TaskID: fixtureNotesID, Index: 1}

	cert := fixtureTask(fixtureCertID, 5, "Await signing certificate", models.StateWaiting, 7)
	cert.DueDate = fixtureDate(12)
	cert.NextCheckIn = fixtureDate(10)
	cert.Tags = []string{"question"}

	paused := fixtureTask(fixturePausedID, 6, "Localize installer", models.StatePaused, 8)
	someday := fixtureTask(fixtureSomedayID, 7, "Dark mode icons", models.StateSomeday, 8)
	someday.Tags = []string{"idea"}
	done := fixtureTask(fixtureDoneID, 8, "Set up CI signing", models.StateDone, 9)
	done.LastReviewedAt = fixturePointer(fixtureInstant(8, 15))
	canceled := fixtureTask(fixtureCanceledID, 9, "Old updater shim", models.StateCanceled, 9)
	atlas.Tasks = []*models.Task{ship, notes, proof, promoted, cert, paused, someday, done, canceled}

	home := fixtureProject(t, runtime, fixtureHomeID, "Home Renovation")
	home.Tags = []string{"personal"}
	paint := fixtureTask(fixturePaintID, 1, "Pick paint colors", models.StateBacklog, 10)
	paint.Dependencies = []models.TaskReference{{ProjectID: fixtureAtlasID, TaskID: fixtureCertID}}
	tiles := fixtureTask(fixtureTilesID, 2, "Order bathroom tiles", models.StateDoing, 11)
	tiles.Priority = 3
	home.Tasks = []*models.Task{paint, tiles}

	reading := fixtureProject(t, runtime, fixtureReadingID, "Reading List")
	reading.Tags = []string{models.ParkedProjectTag}
	novel := fixtureTask(fixtureNovelID, 1, "Finish the novel", models.StateBacklog, 12)
	reading.Tasks = []*models.Task{novel}

	archived := fixtureProject(t, runtime, fixtureArchivedID, "Old Website")
	archived.Archived = true
	archived.ArchivedAt = fixturePointer(fixtureInstant(20, 9))
	archived.Tags = []string{"work"}
	legacy := fixtureTask(fixtureLegacyID, 1, "Migrate DNS", models.StateDone, 13)
	handover := fixtureTask(fixtureHandoverID, 2, "Hand over credentials", models.StateWaiting, 14)
	handover.NextCheckIn = fixtureDate(20)
	handover.CheckInRepeatDays = 14
	archived.Tasks = []*models.Task{legacy, handover}

	unfiled := runtime.SystemUnfiledProject()
	if unfiled == nil {
		t.Fatal("bootstrap produced no system Unfiled project")
	}
	unfiled.Tasks = []*models.Task{fixtureTask(fixtureUnfiledTask, 1, "Call the bank", models.StateInbox, 15)}

	return []*models.Project{atlas, home, reading, archived, unfiled}
}

func writeFixtureProject(t *testing.T, project *models.Project) {
	t.Helper()
	if err := os.MkdirAll(project.Path, 0o755); err != nil {
		t.Fatal(err)
	}
	rendered, err := storage.WriteProjectMarkdown(project)
	if err != nil {
		t.Fatalf("render %s: %v", project.Slug, err)
	}
	if err := os.WriteFile(storage.TasksFilePath(project.Path), []byte(rendered), 0o644); err != nil {
		t.Fatal(err)
	}
}

func writeFixtureNotes(t *testing.T, dataDir string, project *models.Project) {
	t.Helper()
	write := func(dir string, note *models.Note) {
		content, err := storage.RenderNoteContent(note)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, note.Filename), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(storage.NotesDir(project), &models.Note{
		Title:      "Release runbook",
		CreatedAt:  fixtureInstant(3, 9),
		ModifiedAt: fixtureInstant(6, 9),
		Filename:   "2026-09-03-090000_0a1b2c3d-release-runbook.md",
		Body:       "# Release runbook\n\n1. Tag\n2. Notarize\n3. Upload\n",
	})
	write(storage.GlobalNotesDir(dataDir), &models.Note{
		Title:      "Weekly review",
		CreatedAt:  fixtureInstant(2, 18),
		ModifiedAt: fixtureInstant(2, 18),
		Filename:   "2026-09-02-180000_4e5f6a7b-weekly-review.md",
		Body:       "Look at Waiting and check-ins first.\n",
	})
}

func writeFixtureAttachment(t *testing.T, project *models.Project) {
	t.Helper()
	ship := project.Tasks[0]
	path, err := storage.ResolveAttachmentPath(project, ship, fixtureAttachmentName)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("1. build\n2. notarize\n3. upload\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

// copyPortableWorkspace copies the portable artifacts a desktop publishes and
// nothing machine-local or transient: no journals, undo images, migration
// backups, locks, or domain backup sidecars.
func copyPortableWorkspace(t *testing.T, source, target string) {
	t.Helper()
	excludedDirs := map[string]bool{
		filepath.Join(".tt", "transactions"): true,
		filepath.Join(".tt", "undo"):         true,
		filepath.Join(".tt", "backups"):      true,
		filepath.Join(".tt", "conflicts"):    true,
	}
	err := filepath.WalkDir(source, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if excludedDirs[relative] {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(relative, ".bak") || strings.HasSuffix(relative, ".lock") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		destination := filepath.Join(target, relative)
		if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
			return err
		}
		return os.WriteFile(destination, data, 0o644)
	})
	if err != nil {
		t.Fatal(err)
	}
}
