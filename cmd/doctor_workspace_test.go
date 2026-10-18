package cmd

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/testutil"
)

//nolint:govet // The test-local data directory is intentionally independent of the command flag.
func TestRunDoctorFreshWorkspaceBootstrapsProtectedUnfiledIdempotently(t *testing.T) {
	dataDir := t.TempDir()
	setCommandDataDir(t, dataDir)

	oldRepo := storage.DefaultRepository
	storage.DefaultRepository = storage.NewFileSystemRepository()
	t.Cleanup(func() { storage.DefaultRepository = oldRepo })

	first, err := LoadCLIWorkspace()
	if err != nil {
		t.Fatalf("first LoadCLIWorkspace: %v", err)
	}
	unfiled := first.Runtime.SystemUnfiledProject()
	if unfiled == nil {
		t.Fatal("fresh workspace has no protected system Unfiled")
	}
	if unfiled.ID != first.Runtime.Catalog().SystemUnfiledProjectID ||
		unfiled.Name != config.SystemUnfiledName ||
		unfiled.Slug != config.SystemUnfiledSlug {
		t.Fatalf("fresh protected Unfiled = %+v, catalog id = %q", unfiled, first.Runtime.Catalog().SystemUnfiledProjectID)
	}
	taskPath := storage.TasksFilePath(unfiled.Path)
	before, err := os.ReadFile(taskPath)
	if err != nil {
		t.Fatalf("read fresh Unfiled task document: %v", err)
	}
	document, err := storage.ParseProjectDocument(string(before))
	if err != nil {
		t.Fatalf("parse fresh Unfiled task document: %v", err)
	}
	if len(document.Tasks) != 0 {
		t.Fatalf("fresh Unfiled has %d tasks, want 0", len(document.Tasks))
	}

	second, err := LoadCLIWorkspace()
	if err != nil {
		t.Fatalf("second LoadCLIWorkspace: %v", err)
	}
	if got := second.Runtime.Catalog().SystemUnfiledProjectID; got != unfiled.ID {
		t.Fatalf("repeat bootstrap changed Unfiled id from %q to %q", unfiled.ID, got)
	}
	after, err := os.ReadFile(taskPath)
	if err != nil {
		t.Fatalf("read Unfiled task document after repeat load: %v", err)
	}
	if !bytes.Equal(after, before) {
		t.Fatal("repeat workspace load rewrote the canonical Unfiled task document")
	}

	command := &cobra.Command{}
	command.Flags().Bool("fix", false, "")
	command.Flags().Bool("json", false, "")
	stdout, stderr, runErr := captureDoctorOutput(t, func() error {
		return runDoctor(command, nil)
	})
	if runErr != nil {
		t.Fatalf("doctor on fresh workspace: %v\n%s", runErr, stdout)
	}
	if stderr != "" {
		t.Fatalf("doctor stderr = %q, want empty", stderr)
	}
	// Text mode skips empty sections, so no zero-count header may appear.
	if strings.Contains(stdout, ": 0\n") ||
		!strings.Contains(stdout, fmt.Sprintf("[Migration] workspace schema %d is current", config.CurrentConfigVersion)) ||
		!strings.Contains(stdout, "[Undo] workspace ledger is valid (0 undo entries, 0 redo entries)") ||
		!strings.Contains(stdout, "All checks passed") {
		t.Fatalf("doctor did not report a healthy fresh workspace:\n%s", stdout)
	}
	// A workspace created at the current schema crossed no boundary, so it has no
	// missing early snapshot to explain.
	if strings.Contains(stdout, "clean schema-") {
		t.Fatalf("doctor narrated migration history a fresh workspace never had:\n%s", stdout)
	}
	if _, err := os.Lstat(config.PreSimplificationV5BackupsDir(dataDir)); !os.IsNotExist(err) {
		t.Fatalf("doctor created or changed the migration backups directory: %v", err)
	}
}

func TestCheckProtectedUnfiledReportsMissingAndInvalidIdentity(t *testing.T) {
	results := &doctorResults{}
	checkProtectedUnfiled(config.NewWorkspaceCatalog(), results)
	if len(results.workspaceIssues) != 1 || !strings.Contains(results.workspaceIssues[0].detail, "ID is missing") {
		t.Fatalf("missing Unfiled issues = %+v", results.workspaceIssues)
	}

	catalog := config.NewWorkspaceCatalog()
	project := testutil.NewTestProject("Wrong Name")
	project.Slug = "wrong-name"
	if err := catalog.UpsertProject(project); err != nil {
		t.Fatalf("seed project: %v", err)
	}
	catalog.SystemUnfiledProjectID = project.ID
	results = &doctorResults{}
	checkProtectedUnfiled(catalog, results)
	if len(results.workspaceIssues) != 1 || !strings.Contains(results.workspaceIssues[0].detail, "want \"Unfiled\"/\"unfiled\"") {
		t.Fatalf("invalid Unfiled identity issues = %+v", results.workspaceIssues)
	}
}

//nolint:govet // The test-local data directory is intentionally independent of the command flag.
func TestCheckMigrationSnapshotsReportsIncompletePairWithoutMutation(t *testing.T) {
	dataDir := t.TempDir()
	backupsDir := config.PreSimplificationV5BackupsDir(dataDir)
	if err := os.MkdirAll(backupsDir, 0o700); err != nil {
		t.Fatalf("create backups dir: %v", err)
	}
	archivePath := filepath.Join(backupsDir, "pre-simplification-v5-20260729T010203Z.tar.gz")
	want := []byte("immutable archive")
	if err := os.WriteFile(archivePath, want, 0o400); err != nil {
		t.Fatalf("write archive: %v", err)
	}

	results := &doctorResults{}
	checkMigrationSnapshots(dataDir, config.V5ConfigSchemaVersion, results)
	if len(results.workspaceIssues) != 1 ||
		!strings.Contains(results.workspaceIssues[0].detail, "missing its audit report") {
		t.Fatalf("snapshot issues = %+v", results.workspaceIssues)
	}
	got, err := os.ReadFile(archivePath)
	if err != nil {
		t.Fatalf("read archive after doctor check: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Fatal("snapshot check changed the rollback archive")
	}
	auditPath := filepath.Join(backupsDir, "pre-simplification-v5-20260729T010203Z.audit.md")
	if _, err := os.Lstat(auditPath); !os.IsNotExist(err) {
		t.Fatalf("snapshot check recreated a missing audit report: %v", err)
	}
}

// TestCheckMigrationSnapshotsExplainsMissingEarlySnapshotsOnlyAfterMigrating
// keeps the "clean schema-N" notes for the workspace they describe: one whose
// history shows a migration, so a missing early snapshot needs explaining.
func TestCheckMigrationSnapshotsExplainsMissingEarlySnapshotsOnlyAfterMigrating(t *testing.T) {
	fresh := &doctorResults{}
	checkMigrationSnapshots(t.TempDir(), config.CurrentConfigVersion, fresh)
	for _, note := range fresh.workspaceNotes {
		if strings.Contains(note.detail, "clean schema-") {
			t.Fatalf("never-migrated workspace notes = %+v", fresh.workspaceNotes)
		}
	}

	migratedDir := t.TempDir()
	backupsDir := config.PreSimplificationV5BackupsDir(migratedDir)
	if err := os.MkdirAll(backupsDir, 0o700); err != nil {
		t.Fatalf("create backups dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(backupsDir, "pre-simplification-v5-20260729T010203Z.tar.gz"), []byte("archive"), 0o400); err != nil {
		t.Fatalf("write archive: %v", err)
	}
	migrated := &doctorResults{}
	checkMigrationSnapshots(migratedDir, config.CurrentConfigVersion, migrated)
	var details []string
	for _, note := range migrated.workspaceNotes {
		details = append(details, note.detail)
	}
	joined := strings.Join(details, "\n")
	for _, want := range []string{
		"clean schema-6 workspace; no pre-v6 snapshot recorded",
		"clean schema-7 workspace; no pre-v7 snapshot recorded",
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("migrated workspace notes lack %q:\n%s", want, joined)
		}
	}
}

//nolint:govet // The test-local data directory is intentionally independent of the command flag.
func TestCheckWorkspaceUndoLedgerReportsCorruption(t *testing.T) {
	dataDir := t.TempDir()
	undoDir := config.UndoLedgerDir(dataDir)
	if err := os.MkdirAll(undoDir, 0o700); err != nil {
		t.Fatalf("create undo dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(undoDir, "ledger.json"), []byte("{not-json"), 0o600); err != nil {
		t.Fatalf("write corrupt ledger: %v", err)
	}

	results := &doctorResults{}
	checkWorkspaceUndoLedger(dataDir, results)
	if len(results.workspaceIssues) != 1 ||
		!strings.Contains(results.workspaceIssues[0].detail, "failed to parse undo ledger") {
		t.Fatalf("undo issues = %+v", results.workspaceIssues)
	}
}

//nolint:govet // The test-local data directory is intentionally independent of the command flag.
func TestCheckRetainedV5DomainsDoesNotCreateRetiredFiles(t *testing.T) {
	dataDir := t.TempDir()
	if err := os.MkdirAll(config.InternalDir(dataDir), 0o700); err != nil {
		t.Fatalf("create internal dir: %v", err)
	}
	for _, path := range []string{
		config.WorkspaceCatalogPath(dataDir),
		config.PlanningStatePath(dataDir),
	} {
		if err := os.WriteFile(path, []byte("{}"), 0o600); err != nil {
			t.Fatalf("write retained domain: %v", err)
		}
	}

	results := &doctorResults{}
	checkRetainedDomainArtifacts(dataDir, config.V5ConfigSchemaVersion, results)
	if len(results.workspaceIssues) != 0 {
		t.Fatalf("retained v5 domain issues = %+v", results.workspaceIssues)
	}
	for _, path := range []string{config.SettingsPath(dataDir), config.AutomationPath(dataDir)} {
		if _, err := os.Lstat(path); !os.IsNotExist(err) {
			t.Fatalf("domain check recreated retired file %s: %v", path, err)
		}
	}
}

// TestRunDoctorRefusesAMissingWorkspaceWithoutCreatingOne keeps the read-only
// diagnostic from bootstrapping a workspace it was pointed at by mistake: a
// directory that does not exist, and one that exists but holds no workspace.
func TestRunDoctorRefusesAMissingWorkspaceWithoutCreatingOne(t *testing.T) {
	for _, tc := range []struct {
		name   string
		create bool
	}{
		{"nonexistent", false},
		{"empty", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("HOME", t.TempDir())
			dir := filepath.Join(t.TempDir(), "workspace")
			if tc.create {
				if err := os.Mkdir(dir, 0o700); err != nil {
					t.Fatalf("mkdir: %v", err)
				}
			}
			setCommandDataDir(t, dir)

			stdout, _, err := captureDoctorOutput(t, func() error {
				return runDoctor(newDoctorCommand(t), nil)
			})
			if err == nil || !strings.Contains(err.Error(), "no tt workspace at "+dir) {
				t.Fatalf("doctor error = %v, want a refusal naming %s\n%s", err, dir, stdout)
			}
			var issues *doctorIssuesError
			if errors.As(err, &issues) {
				t.Fatalf("a missing workspace is not an issues verdict: %v", err)
			}
			if strings.Contains(stdout, "All checks passed") {
				t.Fatalf("doctor passed a workspace that does not exist:\n%s", stdout)
			}
			if tc.create {
				entries, readErr := os.ReadDir(dir)
				if readErr != nil || len(entries) != 0 {
					t.Fatalf("doctor wrote into the empty directory: %v %v", entries, readErr)
				}
			} else if _, statErr := os.Stat(dir); !os.IsNotExist(statErr) {
				t.Fatalf("doctor created the missing directory: %v", statErr)
			}
		})
	}
}

// TestDataDirThatIsAFileIsRefusedByName names the override rather than the
// first path beneath it that fails with ENOTDIR.
func TestDataDirThatIsAFileIsRefusedByName(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	file := filepath.Join(t.TempDir(), "afile")
	if err := os.WriteFile(file, []byte("x"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	setCommandDataDir(t, file)

	for name, run := range map[string]func() error{
		"doctor":             func() error { return runDoctor(newDoctorCommand(t), nil) },
		"workspace location": func() error { return runWorkspaceLocation(nil, nil) },
		"load":               func() error { _, err := LoadCLIWorkspace(); return err },
	} {
		_, _, err := captureDoctorOutput(t, run)
		if err == nil || err.Error() != "--data-dir "+file+" is not a directory" {
			t.Errorf("%s error = %v, want the --data-dir refusal", name, err)
		}
	}

	setCommandDataDir(t, "")
	t.Setenv("TT_DATA_DIR", file)
	if _, err := LoadCLIWorkspace(); err == nil || err.Error() != "TT_DATA_DIR="+file+" is not a directory" {
		t.Errorf("TT_DATA_DIR load error = %v, want the TT_DATA_DIR refusal", err)
	}
}
