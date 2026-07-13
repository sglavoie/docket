package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/testutil"
)

// seedTrashEntry creates a project directory, moves it into versioned trash, and
// returns the resulting entry ID.
func seedTrashEntry(t *testing.T, dataDir, id, slug string) string {
	t.Helper()
	dir := filepath.Join(dataDir, "projects", slug)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("failed to create project dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "tasks.md"), []byte("# Project: "+slug+"\n"), 0o644); err != nil {
		t.Fatalf("failed to write tasks.md: %v", err)
	}
	entryID, err := storage.MoveProjectToTrash(&models.Project{ID: id, Slug: slug, Name: slug, Path: dir}, dataDir, "test", nil)
	if err != nil {
		t.Fatalf("MoveProjectToTrash failed: %v", err)
	}
	return entryID
}

func TestResolveTrashEntry_NotFound(t *testing.T) {
	tempDir, cleanup := testutil.TempProjectDir(t)
	defer cleanup()

	if _, err := resolveTrashEntry(tempDir, "nope"); err == nil {
		t.Error("expected error resolving an unknown reference")
	}
}

func TestResolveTrashEntry_Single(t *testing.T) {
	tempDir, cleanup := testutil.TempProjectDir(t)
	defer cleanup()

	entryID := seedTrashEntry(t, tempDir, "id-1", "solo")

	entry, err := resolveTrashEntry(tempDir, "solo")
	testutil.AssertNoError(t, err)
	if entry.EntryID != entryID {
		t.Errorf("resolved entry %q, want %q", entry.EntryID, entryID)
	}
}

func TestResolveTrashEntry_MultipleRequiresEntryID(t *testing.T) {
	tempDir, cleanup := testutil.TempProjectDir(t)
	defer cleanup()

	e1 := seedTrashEntry(t, tempDir, "id-1", "dup")
	e2 := seedTrashEntry(t, tempDir, "id-2", "dup")

	_, err := resolveTrashEntry(tempDir, "dup")
	if err == nil {
		t.Fatal("expected an error requiring explicit selection for a shared slug")
	}
	if !strings.Contains(err.Error(), "multiple") {
		t.Errorf("error should mention multiple generations: %v", err)
	}
	// Both entry IDs should be listed as candidates.
	if !strings.Contains(err.Error(), e1) || !strings.Contains(err.Error(), e2) {
		t.Errorf("error should list both candidate entry IDs: %v", err)
	}
}

func TestResolveTrashEntry_ByEntryID(t *testing.T) {
	tempDir, cleanup := testutil.TempProjectDir(t)
	defer cleanup()

	e1 := seedTrashEntry(t, tempDir, "id-1", "dup")
	_ = seedTrashEntry(t, tempDir, "id-2", "dup")

	entry, err := resolveTrashEntry(tempDir, e1)
	testutil.AssertNoError(t, err)
	if entry.EntryID != e1 {
		t.Errorf("resolved entry %q, want %q", entry.EntryID, e1)
	}
}

func TestProjectRestore_ActiveProjectConflict(t *testing.T) {
	cfg := testutil.NewTestConfig()

	existingProject := &models.Project{
		ID:   uuid.New().String(),
		Slug: "conflicting-slug",
		Name: "Existing Project",
	}
	cfg.Projects = []*models.Project{existingProject}

	found := cfg.GetProject("conflicting-slug")
	if found == nil {
		t.Error("expected to find existing project by slug")
	}
}

func TestConfigAddProjectOnRestore(t *testing.T) {
	cfg := &config.Config{}

	project := testutil.NewTestProject("Test Project")
	cfg.AddProject(project)

	testutil.AssertEqual(t, len(cfg.Projects), 1)
	testutil.AssertEqual(t, cfg.Projects[0].Name, "Test Project")
}

// TestProjectRestoreRejectsEmptySlug guards against an empty-string reference
// reaching the trash layer. The command must reject it up front, leaving any
// existing trash untouched.
func TestProjectRestoreRejectsEmptySlug(t *testing.T) {
	deps, _ := setupProjectTest(t)

	// Seed a legacy trash directory that must survive an empty-ref restore.
	trashDir := filepath.Join(deps.Config.DataDir, ".tt", "trash")
	siblingDir := filepath.Join(trashDir, "sibling-project")
	if err := os.MkdirAll(siblingDir, 0755); err != nil {
		t.Fatalf("failed to seed trash dir: %v", err)
	}

	err := runProjectRestore(projectRestoreCmd, []string{""})
	if err == nil {
		t.Fatal("expected error for empty slug")
	}

	if _, statErr := os.Stat(siblingDir); os.IsNotExist(statErr) {
		t.Fatal("sibling trash entry was destroyed by empty-slug restore")
	}
}
