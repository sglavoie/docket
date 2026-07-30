package storage

import (
	"errors"
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
