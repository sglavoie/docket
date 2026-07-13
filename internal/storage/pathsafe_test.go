package storage

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/util"
)

// TestSafeChildNameRejectsTraversal confirms the storage wrapper forwards the
// util sentinel errors so callers can classify rejections uniformly.
func TestSafeChildNameRejectsTraversal(t *testing.T) {
	tests := []struct {
		input   string
		wantErr error
	}{
		{"notes.md", nil},
		{"", util.ErrPathEmpty},
		{"..", util.ErrPathTraversal},
		{"../etc/passwd", util.ErrPathSeparator},
		{"a/b", util.ErrPathSeparator},
		{"a\\b", util.ErrPathSeparator},
		{"/abs", util.ErrPathAbsolute},
	}
	for _, tc := range tests {
		err := SafeChildName(tc.input)
		if tc.wantErr == nil {
			if err != nil {
				t.Fatalf("SafeChildName(%q) = %v, want nil", tc.input, err)
			}
			continue
		}
		if !errors.Is(err, tc.wantErr) {
			t.Fatalf("SafeChildName(%q) = %v, want %v", tc.input, err, tc.wantErr)
		}
	}
}

// TestMoveProjectToTrashRejectsTraversalSlug guards the trash boundary: a
// project whose slug contains traversal must be rejected before any rename, and
// no file may be created outside the trash directory.
func TestMoveProjectToTrashRejectsTraversalSlug(t *testing.T) {
	tmpDir := t.TempDir()
	dataDir := filepath.Join(tmpDir, "tt-data")
	projectDir := filepath.Join(dataDir, "victim")
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatalf("mkdir project: %v", err)
	}

	project := &models.Project{
		ID:   "id",
		Slug: "../../escape",
		Name: "Escape",
		Path: projectDir,
	}
	if _, err := MoveProjectToTrash(project, dataDir, "", nil); err == nil {
		t.Fatal("expected MoveProjectToTrash to reject a traversal slug")
	}
	// Nothing should have been created above the data directory.
	if _, err := os.Stat(filepath.Join(tmpDir, "escape")); !os.IsNotExist(err) {
		t.Fatalf("traversal slug wrote outside data dir: stat err = %v", err)
	}
}

// TestRenameProjectDirRejectsTraversalSlug confirms rename refuses a traversal
// slug on either side of the operation.
func TestRenameProjectDirRejectsTraversalSlug(t *testing.T) {
	dataDir := t.TempDir()
	project := &models.Project{Slug: "old", Path: filepath.Join(dataDir, "projects", "old")}
	if err := RenameProjectDir(project, "old", "../escape", dataDir); err == nil {
		t.Fatal("expected RenameProjectDir to reject a traversal new slug")
	}
	if err := RenameProjectDir(project, "..\\win", "new", dataDir); err == nil {
		t.Fatal("expected RenameProjectDir to reject a windows-traversal old slug")
	}
}

// TestReadTrashTombstoneRejectsTraversal confirms tombstone reads treat an
// unsafe entry ID as invalid rather than following it out of the trash dir.
func TestReadTrashTombstoneRejectsTraversal(t *testing.T) {
	dataDir := t.TempDir()
	if _, err := ReadTrashTombstone(dataDir, "../../etc"); err == nil {
		t.Fatal("ReadTrashTombstone must reject a traversal entry ID")
	}
	if err := PurgeTrashEntry(dataDir, "../../etc"); err == nil {
		t.Fatal("PurgeTrashEntry must reject a traversal entry ID")
	}
}
