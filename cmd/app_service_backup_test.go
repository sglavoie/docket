package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/sglavoie/tt/internal/storage"
)

func TestApplyArtifactChangeMaintainsBackupForFallbackRepository(t *testing.T) {
	root := t.TempDir()
	primary := filepath.Join(root, "planning.json")
	backup := primary + ".bak"

	if err := applyArtifactChange(storage.ArtifactChange{Path: primary, Data: []byte("v1"), BackupPath: backup}); err != nil {
		t.Fatalf("first applyArtifactChange: %v", err)
	}
	assertFallbackArtifactBytes(t, primary, []byte("v1"))
	if _, err := os.Stat(backup); !os.IsNotExist(err) {
		t.Fatalf("first save backup stat err = %v, want absent", err)
	}

	if err := applyArtifactChange(storage.ArtifactChange{Path: primary, Data: []byte("v2"), BackupPath: backup}); err != nil {
		t.Fatalf("second applyArtifactChange: %v", err)
	}
	assertFallbackArtifactBytes(t, primary, []byte("v2"))
	assertFallbackArtifactBytes(t, backup, []byte("v1"))

	if err := applyArtifactChange(storage.ArtifactChange{Path: primary, Delete: true, BackupPath: backup}); err != nil {
		t.Fatalf("delete applyArtifactChange: %v", err)
	}
	if _, err := os.Stat(primary); !os.IsNotExist(err) {
		t.Fatalf("deleted primary stat err = %v, want absent", err)
	}
	assertFallbackArtifactBytes(t, backup, []byte("v2"))
}

func TestApplyArtifactChangeBackupFailureLeavesPrimaryUnchanged(t *testing.T) {
	root := t.TempDir()
	primary := filepath.Join(root, "planning.json")
	if err := os.WriteFile(primary, []byte("current"), storage.FilePermissions); err != nil {
		t.Fatalf("write primary: %v", err)
	}
	blocker := filepath.Join(root, "not-a-directory")
	if err := os.WriteFile(blocker, []byte("blocker"), storage.FilePermissions); err != nil {
		t.Fatalf("write blocker: %v", err)
	}

	err := applyArtifactChange(storage.ArtifactChange{
		Path: primary, Data: []byte("next"), BackupPath: filepath.Join(blocker, "planning.json.bak"),
	})
	if err == nil {
		t.Fatal("applyArtifactChange unexpectedly succeeded")
	}
	assertFallbackArtifactBytes(t, primary, []byte("current"))
}

func assertFallbackArtifactBytes(t *testing.T, path string, want []byte) {
	t.Helper()
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if string(got) != string(want) {
		t.Fatalf("%s = %q, want %q", path, got, want)
	}
}
