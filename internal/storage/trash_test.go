package storage

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/models"
)

func Test_getTrashDir(t *testing.T) {
	dataDir := "/home/user/.tt"
	trashDir := getTrashDir(dataDir)
	expected := "/home/user/.tt/.tt/trash"

	if trashDir != expected {
		t.Errorf("expected trash dir '%s', got '%s'", expected, trashDir)
	}
}

// writeProjectDir creates a project directory with a tasks.md file.
func writeProjectDir(t *testing.T, dir, content string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("failed to create project dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "tasks.md"), []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write tasks.md: %v", err)
	}
}

func TestMoveProjectToTrash_CreatesVersionedEntryAndTombstone(t *testing.T) {
	dataDir := filepath.Join(t.TempDir(), "tt-data")
	projectDir := filepath.Join(dataDir, "projects", "my-project")
	tasksContent := "# Project: My Project\n"
	writeProjectDir(t, projectDir, tasksContent)

	project := &models.Project{ID: "proj-1", Slug: "my-project", Name: "My Project", Path: projectDir}

	entryID, err := MoveProjectToTrash(project, dataDir, "test deletion", nil)
	if err != nil {
		t.Fatalf("MoveProjectToTrash failed: %v", err)
	}
	if entryID == "" {
		t.Fatal("expected a non-empty entry ID")
	}

	// Original directory is gone.
	if _, statErr := os.Stat(projectDir); !os.IsNotExist(statErr) {
		t.Error("original project directory should be removed")
	}

	// Entry is keyed by project ID (not raw slug).
	entryPath := filepath.Join(dataDir, ".tt", "trash", entryID)
	if _, statErr := os.Stat(entryPath); statErr != nil {
		t.Fatalf("versioned trash entry should exist: %v", statErr)
	}
	if _, statErr := os.Stat(filepath.Join(dataDir, ".tt", "trash", "my-project")); statErr == nil {
		t.Error("trash must not be keyed by raw slug")
	}

	// Task content is preserved under data/.
	content, err := os.ReadFile(filepath.Join(entryPath, "data", "tasks.md"))
	if err != nil {
		t.Fatalf("failed to read trashed tasks.md: %v", err)
	}
	if string(content) != tasksContent {
		t.Error("tasks.md content should be preserved in trash")
	}

	// Tombstone records identity, path, and reason.
	tomb, err := ReadTrashTombstone(dataDir, entryID)
	if err != nil {
		t.Fatalf("ReadTrashTombstone failed: %v", err)
	}
	if tomb.Project == nil || tomb.Project.ID != "proj-1" || tomb.Project.Slug != "my-project" {
		t.Errorf("tombstone project identity not preserved: %+v", tomb.Project)
	}
	if tomb.OriginalPath != projectDir {
		t.Errorf("tombstone OriginalPath = %q, want %q", tomb.OriginalPath, projectDir)
	}
	if tomb.DeletionReason != "test deletion" {
		t.Errorf("tombstone DeletionReason = %q, want %q", tomb.DeletionReason, "test deletion")
	}
	if tomb.SchemaVersion != trashTombstoneSchemaVersion {
		t.Errorf("tombstone SchemaVersion = %d, want %d", tomb.SchemaVersion, trashTombstoneSchemaVersion)
	}
}

func TestMoveProjectToTrash_MultipleGenerationsCoexistNoSilentPurge(t *testing.T) {
	dataDir := filepath.Join(t.TempDir(), "tt-data")

	// First generation.
	dir1 := filepath.Join(dataDir, "projects", "my-project")
	writeProjectDir(t, dir1, "# Gen One\n")
	entry1, err := MoveProjectToTrash(&models.Project{ID: "id-1", Slug: "my-project", Name: "Gen One", Path: dir1}, dataDir, "first", nil)
	if err != nil {
		t.Fatalf("first move failed: %v", err)
	}

	// A brand-new project reuses the same slug and is deleted too.
	dir2 := filepath.Join(dataDir, "projects", "my-project")
	writeProjectDir(t, dir2, "# Gen Two\n")
	entry2, err := MoveProjectToTrash(&models.Project{ID: "id-2", Slug: "my-project", Name: "Gen Two", Path: dir2}, dataDir, "second", nil)
	if err != nil {
		t.Fatalf("second move failed: %v", err)
	}

	if entry1 == entry2 {
		t.Fatal("expected distinct entry IDs for two generations")
	}

	entries, err := ListTrashEntries(dataDir)
	if err != nil {
		t.Fatalf("ListTrashEntries failed: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 coexisting trash generations, got %d", len(entries))
	}

	// Both generations remain intact — no silent purge of the older one.
	c1, _ := os.ReadFile(filepath.Join(dataDir, ".tt", "trash", entry1, "data", "tasks.md"))
	c2, _ := os.ReadFile(filepath.Join(dataDir, ".tt", "trash", entry2, "data", "tasks.md"))
	if string(c1) != "# Gen One\n" {
		t.Errorf("first generation content lost: %q", string(c1))
	}
	if string(c2) != "# Gen Two\n" {
		t.Errorf("second generation content lost: %q", string(c2))
	}
}

func TestMoveProjectToTrash_RejectsEmptyIDAndSlug(t *testing.T) {
	dataDir := filepath.Join(t.TempDir(), "tt-data")
	dir := filepath.Join(dataDir, "projects", "p")
	writeProjectDir(t, dir, "# P\n")

	if _, err := MoveProjectToTrash(&models.Project{ID: "", Slug: "p", Path: dir}, dataDir, "", nil); err == nil {
		t.Error("expected error for empty project ID")
	}
	if _, err := MoveProjectToTrash(&models.Project{ID: "id", Slug: "", Path: dir}, dataDir, "", nil); err == nil {
		t.Error("expected error for empty slug")
	}
	// The project directory must survive a rejected move.
	if _, err := os.Stat(dir); err != nil {
		t.Errorf("project dir disappeared after rejected move: %v", err)
	}
}

func TestRestoreTrashEntry_FaithfulMetadata(t *testing.T) {
	dataDir := filepath.Join(t.TempDir(), "tt-data")
	projectDir := filepath.Join(dataDir, "projects", "faithful")
	writeProjectDir(t, projectDir, "# Faithful\n\n## Task: One\n<!-- id: t1, status: todo, priority: 1 -->\n")

	archivedAt := time.Now().Add(-48 * time.Hour)
	pinnedAt := time.Now().Add(-72 * time.Hour)
	createdAt := time.Now().Add(-200 * time.Hour)
	original := &models.Project{
		ID:         "stable-id-123",
		Slug:       "faithful",
		Name:       "Faithful Project",
		Context:    "work",
		Path:       projectDir,
		CreatedAt:  createdAt,
		Archived:   true,
		ArchivedAt: &archivedAt,
		Pinned:     true,
		PinnedAt:   &pinnedAt,
	}

	entryID, err := MoveProjectToTrash(original, dataDir, "faithful test", nil)
	if err != nil {
		t.Fatalf("MoveProjectToTrash failed: %v", err)
	}

	restored, tomb, err := RestoreTrashEntry(dataDir, entryID)
	if err != nil {
		t.Fatalf("RestoreTrashEntry failed: %v", err)
	}
	if tomb == nil {
		t.Fatal("expected a tombstone from RestoreTrashEntry")
	}

	if restored.ID != "stable-id-123" {
		t.Errorf("restored ID = %q, want the original stable ID", restored.ID)
	}
	if restored.Name != "Faithful Project" {
		t.Errorf("restored Name = %q, want %q", restored.Name, "Faithful Project")
	}
	if restored.Context != "work" {
		t.Errorf("restored Context = %q, want %q", restored.Context, "work")
	}
	if !restored.CreatedAt.Equal(createdAt) {
		t.Errorf("restored CreatedAt = %v, want %v", restored.CreatedAt, createdAt)
	}
	if !restored.Archived || restored.ArchivedAt == nil {
		t.Error("archived state should be preserved on restore")
	}
	if !restored.Pinned || restored.PinnedAt == nil {
		t.Error("pinned state should be preserved on restore")
	}
	if len(restored.Tasks) != 1 {
		t.Errorf("expected 1 task loaded from disk, got %d", len(restored.Tasks))
	}
	if restored.Path != filepath.Join(dataDir, "projects", "faithful") {
		t.Errorf("restored Path = %q", restored.Path)
	}

	// Project dir is back and the trash entry is gone.
	if _, err := os.Stat(restored.Path); err != nil {
		t.Errorf("restored project dir missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dataDir, ".tt", "trash", entryID)); !os.IsNotExist(err) {
		t.Error("trash entry should be removed after restore")
	}
}

func TestRestoreTrashEntry_ConflictWithActive(t *testing.T) {
	dataDir := filepath.Join(t.TempDir(), "tt-data")
	projectDir := filepath.Join(dataDir, "projects", "clash")
	writeProjectDir(t, projectDir, "# Clash\n")
	entryID, err := MoveProjectToTrash(&models.Project{ID: "id", Slug: "clash", Name: "Clash", Path: projectDir}, dataDir, "", nil)
	if err != nil {
		t.Fatalf("MoveProjectToTrash failed: %v", err)
	}

	// Re-create an active project at the destination slug.
	writeProjectDir(t, projectDir, "# New Clash\n")

	if _, _, err := RestoreTrashEntry(dataDir, entryID); err == nil {
		t.Error("expected restore to fail when an active project already occupies the slug")
	}
}

func TestRestoreTrashEntry_NotExists(t *testing.T) {
	dataDir := filepath.Join(t.TempDir(), "tt-data")
	if _, _, err := RestoreTrashEntry(dataDir, "does-not-exist__1"); err == nil {
		t.Error("expected error restoring a non-existent entry")
	}
	if _, _, err := RestoreTrashEntry(dataDir, ""); err == nil {
		t.Error("expected error restoring an empty entry ID")
	}
}

func TestFindTrashEntries_BySlugAndEntryID(t *testing.T) {
	dataDir := filepath.Join(t.TempDir(), "tt-data")
	dir := filepath.Join(dataDir, "projects", "dup")

	writeProjectDir(t, dir, "# A\n")
	e1, _ := MoveProjectToTrash(&models.Project{ID: "id-a", Slug: "dup", Name: "A", Path: dir}, dataDir, "", nil)
	writeProjectDir(t, dir, "# B\n")
	e2, _ := MoveProjectToTrash(&models.Project{ID: "id-b", Slug: "dup", Name: "B", Path: dir}, dataDir, "", nil)

	bySlug, err := FindTrashEntries(dataDir, "dup")
	if err != nil {
		t.Fatalf("FindTrashEntries(slug) failed: %v", err)
	}
	if len(bySlug) != 2 {
		t.Fatalf("expected 2 entries for shared slug, got %d", len(bySlug))
	}

	byID, err := FindTrashEntries(dataDir, e1)
	if err != nil {
		t.Fatalf("FindTrashEntries(entryID) failed: %v", err)
	}
	if len(byID) != 1 || byID[0].EntryID != e1 {
		t.Fatalf("expected exactly the e1 entry, got %+v", byID)
	}

	none, err := FindTrashEntries(dataDir, "missing")
	if err != nil {
		t.Fatalf("FindTrashEntries(missing) failed: %v", err)
	}
	if len(none) != 0 {
		t.Errorf("expected no matches for unknown ref, got %d", len(none))
	}
	_ = e2
}

func TestListTrashEntries_EmptyAndSorted(t *testing.T) {
	dataDir := filepath.Join(t.TempDir(), "tt-data")

	entries, err := ListTrashEntries(dataDir)
	if err != nil {
		t.Fatalf("ListTrashEntries on empty failed: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("expected 0 entries, got %d", len(entries))
	}

	dir := filepath.Join(dataDir, "projects", "p")
	writeProjectDir(t, dir, "# older\n")
	if _, moveErr := MoveProjectToTrash(&models.Project{ID: "old", Slug: "p", Name: "older", Path: dir}, dataDir, "", nil); moveErr != nil {
		t.Fatalf("move older: %v", moveErr)
	}
	time.Sleep(2 * time.Millisecond)
	writeProjectDir(t, dir, "# newer\n")
	if _, moveErr := MoveProjectToTrash(&models.Project{ID: "new", Slug: "p", Name: "newer", Path: dir}, dataDir, "", nil); moveErr != nil {
		t.Fatalf("move newer: %v", moveErr)
	}

	entries, err = ListTrashEntries(dataDir)
	if err != nil {
		t.Fatalf("ListTrashEntries failed: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	if !entries[0].DeletedAt.After(entries[1].DeletedAt) {
		t.Error("entries should be sorted newest deletion first")
	}
}

func TestPurgeTrashEntry(t *testing.T) {
	dataDir := filepath.Join(t.TempDir(), "tt-data")
	dir := filepath.Join(dataDir, "projects", "p")
	writeProjectDir(t, dir, "# P\n")
	entryID, _ := MoveProjectToTrash(&models.Project{ID: "id", Slug: "p", Name: "P", Path: dir}, dataDir, "", nil)

	if err := PurgeTrashEntry(dataDir, entryID); err != nil {
		t.Fatalf("PurgeTrashEntry failed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dataDir, ".tt", "trash", entryID)); !os.IsNotExist(err) {
		t.Error("entry should be gone after purge")
	}

	// Purging a missing entry is a no-op; an empty ID is rejected.
	if err := PurgeTrashEntry(dataDir, "missing__1"); err != nil {
		t.Errorf("purging a missing entry should be a no-op, got %v", err)
	}
	if err := PurgeTrashEntry(dataDir, ""); err == nil {
		t.Error("expected error purging an empty entry ID")
	}
}

func TestRestoreTrashEntry_ConfigRefsRoundTrip(t *testing.T) {
	dataDir := filepath.Join(t.TempDir(), "tt-data")
	dir := filepath.Join(dataDir, "projects", "refs")
	writeProjectDir(t, dir, "# Refs\n")

	refs := json.RawMessage(`{"focus_queue":[{"task_id":"t1","project_id":"id"}]}`)
	entryID, err := MoveProjectToTrash(&models.Project{ID: "id", Slug: "refs", Name: "Refs", Path: dir}, dataDir, "", refs)
	if err != nil {
		t.Fatalf("MoveProjectToTrash failed: %v", err)
	}

	_, tomb, err := RestoreTrashEntry(dataDir, entryID)
	if err != nil {
		t.Fatalf("RestoreTrashEntry failed: %v", err)
	}
	// The tombstone is re-marshaled with indentation, so compare semantically
	// (the opaque config-ref payload must survive the round trip unchanged).
	if !jsonEqual(t, tomb.ConfigRefs, refs) {
		t.Errorf("config refs not round-tripped: got %s, want %s", tomb.ConfigRefs, refs)
	}
}

func jsonEqual(t *testing.T, a, b json.RawMessage) bool {
	t.Helper()
	var av, bv any
	if err := json.Unmarshal(a, &av); err != nil {
		t.Fatalf("unmarshal a: %v", err)
	}
	if err := json.Unmarshal(b, &bv); err != nil {
		t.Fatalf("unmarshal b: %v", err)
	}
	ab, _ := json.Marshal(av)
	bb, _ := json.Marshal(bv)
	return string(ab) == string(bb)
}

func TestMoveAndRestoreRoundTrip(t *testing.T) {
	dataDir := filepath.Join(t.TempDir(), "tt-data")
	projectDir := filepath.Join(dataDir, "projects", "my-project")
	originalContent := "# Project: My Project\n\n## Task: Test Task\n<!-- id: task1, status: todo, priority: 1 -->\n"
	writeProjectDir(t, projectDir, originalContent)

	project := &models.Project{ID: "rt-id", Slug: "my-project", Name: "My Project", Path: projectDir}
	entryID, err := MoveProjectToTrash(project, dataDir, "", nil)
	if err != nil {
		t.Fatalf("MoveProjectToTrash failed: %v", err)
	}

	restored, _, err := RestoreTrashEntry(dataDir, entryID)
	if err != nil {
		t.Fatalf("RestoreTrashEntry failed: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(restored.Path, "tasks.md"))
	if err != nil {
		t.Fatalf("failed to read restored tasks.md: %v", err)
	}
	if string(content) != originalContent {
		t.Error("content should be preserved after round trip")
	}
}

// TestRestoreTrashEntry_LegacyEntry covers a pre-versioned trash directory (a
// bare <slug>/ dir with tasks.md at its root and no tombstone). It must still be
// listable and restorable via the best-effort legacy fallback.
func TestRestoreTrashEntry_LegacyEntry(t *testing.T) {
	dataDir := filepath.Join(t.TempDir(), "tt-data")
	legacyDir := filepath.Join(dataDir, ".tt", "trash", "legacy-project")
	if err := os.MkdirAll(legacyDir, 0o755); err != nil {
		t.Fatalf("failed to seed legacy trash: %v", err)
	}
	if err := os.WriteFile(filepath.Join(legacyDir, "tasks.md"), []byte("# Project: Legacy Name\n"), 0o644); err != nil {
		t.Fatalf("failed to write legacy tasks.md: %v", err)
	}

	entries, err := ListTrashEntries(dataDir)
	if err != nil {
		t.Fatalf("ListTrashEntries failed: %v", err)
	}
	if len(entries) != 1 || !entries[0].Legacy || entries[0].Slug != "legacy-project" {
		t.Fatalf("legacy entry not surfaced correctly: %+v", entries)
	}

	restored, _, err := RestoreTrashEntry(dataDir, "legacy-project")
	if err != nil {
		t.Fatalf("RestoreTrashEntry(legacy) failed: %v", err)
	}
	if restored.ID == "" {
		t.Error("legacy restore should synthesize a fresh ID")
	}
	if restored.Name != "Legacy Name" {
		t.Errorf("legacy restore should parse the project name, got %q", restored.Name)
	}
	if _, err := os.Stat(filepath.Join(dataDir, "projects", "legacy-project", "tasks.md")); err != nil {
		t.Errorf("legacy project data should be restored: %v", err)
	}
}
