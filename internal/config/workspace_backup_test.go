package config

import (
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
		if _, err := CreateWorkspaceBackup(fixture.original, root); err == nil || !strings.Contains(err.Error(), "backup placement") {
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
