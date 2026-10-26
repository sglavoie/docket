package config

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWorkspaceBackupCanBeVerifiedAfterSourceChanges(t *testing.T) {
	fixture := newEnrollFixture(t)
	projectFile := filepath.Join(fixture.original, "projects", "unfiled", "notes", "handoff.md")
	if err := os.MkdirAll(filepath.Dir(projectFile), DirPermissions); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(projectFile, []byte("Keep the handoff"), FilePermissions); err != nil {
		t.Fatal(err)
	}
	report, err := CreateWorkspaceBackup(fixture.original, filepath.Join(fixture.root, "backups"))
	if err != nil {
		t.Fatal(err)
	}
	if report.Files < 3 || report.Source != fixture.original {
		t.Fatalf("backup report = %+v", report)
	}
	if err = os.WriteFile(projectFile, []byte("Newer source text"), FilePermissions); err != nil {
		t.Fatal(err)
	}
	verified, err := VerifyWorkspaceBackup(report.Path)
	if err != nil || verified.Files != report.Files {
		t.Fatalf("verify changed-source backup = %+v, %v", verified, err)
	}
	copyText, err := os.ReadFile(filepath.Join(report.Path, "projects", "unfiled", "notes", "handoff.md"))
	if err != nil || string(copyText) != "Keep the handoff" {
		t.Fatalf("snapshot did not retain earlier bytes: %q, %v", copyText, err)
	}
}

func TestWorkspaceBackupVerificationFindsMissingChangedAndUnexpectedFiles(t *testing.T) {
	for _, change := range []string{"changed", "missing", "unexpected", "symlink"} {
		t.Run(change, func(t *testing.T) {
			fixture := newEnrollFixture(t)
			report, err := CreateWorkspaceBackup(fixture.original, filepath.Join(fixture.root, "backups"))
			if err != nil {
				t.Fatal(err)
			}
			catalog := WorkspaceCatalogPath(report.Path)
			switch change {
			case "changed":
				err = os.WriteFile(catalog, []byte("broken"), FilePermissions)
			case "missing":
				err = os.Remove(catalog)
			case "unexpected":
				err = os.WriteFile(filepath.Join(report.Path, "unlisted.txt"), []byte("extra"), FilePermissions)
			case "symlink":
				err = os.Symlink(catalog, filepath.Join(report.Path, "link"))
			}
			if err != nil {
				t.Fatal(err)
			}
			if _, err := VerifyWorkspaceBackup(report.Path); err == nil {
				t.Fatal("damaged backup passed verification")
			}
		})
	}
}

func TestWorkspaceBackupRefusesOverlappingDestination(t *testing.T) {
	fixture := newEnrollFixture(t)
	for _, root := range []string{fixture.original, filepath.Join(fixture.original, "backups"), fixture.root} {
		if _, err := CreateWorkspaceBackup(fixture.original, root); err == nil || !strings.Contains(err.Error(), "backup placement") || strings.Contains(err.Error(), "move") {
			t.Fatalf("overlapping root %s: %v", root, err)
		}
	}
}

func TestWorkspaceBackupDoesNotReplaceAWorkspaceSnapshotManifest(t *testing.T) {
	fixture := newEnrollFixture(t)
	path := filepath.Join(fixture.original, snapshotManifest)
	if err := os.WriteFile(path, []byte("existing user file"), FilePermissions); err != nil {
		t.Fatal(err)
	}
	if _, err := CreateWorkspaceBackup(fixture.original, fixture.snapshots); err == nil || !strings.Contains(err.Error(), "reserved") {
		t.Fatalf("reserved source file was accepted: %v", err)
	}
	if data, err := os.ReadFile(path); err != nil || string(data) != "existing user file" {
		t.Fatalf("source file changed: %q, %v", data, err)
	}
}

func TestWorkspaceBackupVerificationNamesAMissingBackupPlainly(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nowhere")
	_, err := VerifyWorkspaceBackup(dir)
	if err == nil {
		t.Fatal("verifying a directory with no backup succeeded")
	}
	if !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("error does not unwrap to fs.ErrNotExist: %v", err)
	}
	if !strings.HasPrefix(err.Error(), "no backup at ") || !strings.Contains(err.Error(), "(snapshot.json is missing)") ||
		strings.Contains(err.Error(), "no such file") {
		t.Fatalf("error = %q", err)
	}
}

func TestWorkspaceBackupCopiesAPreV24WorkspaceWithoutUpgradingIt(t *testing.T) {
	root := legacyV24Workspace(t)
	before := v24Tree(t, root)
	report, err := CreateWorkspaceBackup(root, filepath.Join(t.TempDir(), "backups"))
	if err != nil {
		t.Fatalf("backup of a pre-v24 workspace: %v", err)
	}
	if _, statErr := os.Stat(InternalDir(root)); !errors.Is(statErr, fs.ErrNotExist) {
		t.Fatalf("backup created %s in the source: %v", InternalDir(root), statErr)
	}
	after := v24Tree(t, root)
	if _, held := before[".tt/transactions/commit.lock"]; !held {
		delete(after, ".tt/transactions/commit.lock")
	}
	if len(after) != len(before) {
		t.Fatalf("backup changed the source tree: %d files before, %d after", len(before), len(after))
	}
	copied, err := os.ReadFile(filepath.Join(report.Path, ".tt", "global-notes", "2026-10-25-review.md"))
	if err != nil || string(copied) != "# Review\n" {
		t.Fatalf("backup did not carry .tt/ as it stands: %q, %v", copied, err)
	}
	if _, err = VerifyWorkspaceBackup(report.Path); err != nil {
		t.Fatalf("verify pre-v24 backup: %v", err)
	}
}

func TestWorkspaceBackupRefusesALoneTombstone(t *testing.T) {
	root := legacyV24Workspace(t)
	if err := NewManager(ConfigPathForDataDir(root)).RecoverWorkspace(); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(InternalDir(root)); err != nil {
		t.Fatal(err)
	}
	_, err := CreateWorkspaceBackup(root, filepath.Join(t.TempDir(), "backups"))
	var moved *WorkspaceMovedElsewhereError
	if !errors.As(err, &moved) {
		t.Fatalf("backup of a lone tombstone = %v, want WorkspaceMovedElsewhereError", err)
	}
	if dirExists(InternalDir(root)) {
		t.Fatal("refusing a lone tombstone created .docket/")
	}
}
