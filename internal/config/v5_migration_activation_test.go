package config

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/transaction"
)

type v5MigrationActivationTestHooks struct {
	afterSnapshotPublished func(*PreSimplificationV5Snapshot)
}

func (h *v5MigrationActivationTestHooks) didPublishSnapshot(snapshot *PreSimplificationV5Snapshot) {
	if h != nil && h.afterSnapshotPublished != nil {
		h.afterSnapshotPublished(snapshot)
	}
}

func copyLegacyV4ActivationFixture(t *testing.T) string {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	root := filepath.Join(t.TempDir(), "workspace")
	if err := os.CopyFS(root, os.DirFS(filepath.Join(legacyV4FixtureRoot, "workspace"))); err != nil {
		t.Fatalf("copy legacy v4 workspace: %v", err)
	}
	machineData, err := os.ReadFile(filepath.Join(
		legacyV4FixtureRoot,
		"machine",
		"runtime",
		legacyV4WorkspaceID,
		"machine.json",
	))
	if err != nil {
		t.Fatalf("read legacy machine fixture: %v", err)
	}
	machinePath, err := MachineStatePath(legacyV4WorkspaceID)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(machinePath), DirPermissions); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(machinePath, machineData, FilePermissions); err != nil {
		t.Fatal(err)
	}
	undoFile := filepath.Join(root, "projects", "alpha", legacyV4ProjectUndoDirName, "old.json")
	if err := os.MkdirAll(filepath.Dir(undoFile), DirPermissions); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(undoFile, []byte("{}"), FilePermissions); err != nil {
		t.Fatal(err)
	}
	backupFile := filepath.Join(root, "projects", "alpha", legacyV4ProjectBackupName)
	if err := os.WriteFile(backupFile, []byte("# legacy backup\n"), FilePermissions); err != nil {
		t.Fatal(err)
	}
	return root
}

func captureV5MigrationTaskAndDomainBytes(t *testing.T, root string) map[string][]byte {
	t.Helper()
	paths := []string{
		WorkspaceCatalogPath(root),
		PlanningStatePath(root),
		SettingsPath(root),
		AutomationPath(root),
	}
	machinePath, err := MachineStatePath(legacyV4WorkspaceID)
	if err != nil {
		t.Fatal(err)
	}
	paths = append(paths, machinePath)
	if err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		if entry.Name() == "tasks.md" || legacyV4ArchiveFileName.MatchString(entry.Name()) {
			paths = append(paths, path)
		}
		return nil
	}); err != nil {
		t.Fatalf("inventory migration task files: %v", err)
	}

	captured := make(map[string][]byte, len(paths))
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read migration source %s: %v", path, err)
		}
		captured[path] = data
	}
	return captured
}

//nolint:gocyclo // This rehearsal verifies the complete snapshot-first activation contract.
func TestManagerActivatesSnapshotFirstV5MigrationAndIsIdempotent(t *testing.T) {
	root := copyLegacyV4ActivationFixture(t)
	for path, contents := range map[string]string{
		filepath.Join(InternalDir(root), "themes", "custom.json"): `{"name":"custom"}`,
		filepath.Join(root, "themes", "legacy.json"):              `{"name":"legacy"}`,
	} {
		if err := os.MkdirAll(filepath.Dir(path), DirPermissions); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(contents), FilePermissions); err != nil {
			t.Fatal(err)
		}
	}
	manager := NewManager(ConfigPathForDataDir(root))

	if err := manager.migrateWorkspaceToV5(nil); err != nil {
		t.Fatalf("activate v5 migration: %v", err)
	}
	catalog, workspaceID, err := readV5CatalogCandidate(WorkspaceCatalogPath(root))
	if err != nil {
		t.Fatalf("load migrated v5 catalog: %v", err)
	}
	machine, _, _, err := readV5MachineCandidate(V6WorkspaceDryRunOptions{DataDir: root}, workspaceID)
	if err != nil {
		t.Fatalf("load migrated v5 machine state: %v", err)
	}
	if catalog.SchemaVersion != V5ConfigSchemaVersion {
		t.Fatalf("catalog schema = %d, want %d", catalog.SchemaVersion, V5ConfigSchemaVersion)
	}
	if len(catalog.Projects) != 4 {
		t.Fatalf("retained catalog projects = %d, want 4", len(catalog.Projects))
	}
	if machine.Theme != "dark" {
		t.Fatalf("migrated theme = %q, want dark", machine.Theme)
	}

	for _, path := range []string{
		SettingsPath(root),
		AutomationPath(root),
		filepath.Join(InternalDir(root), "trash"),
		filepath.Join(InternalDir(root), "themes"),
		filepath.Join(root, "themes"),
		filepath.Join(root, "projects", "_sandbox"),
		filepath.Join(root, "projects", "internal-fixture"),
		filepath.Join(root, "projects", "alpha", legacyV4BlockersFileName),
		filepath.Join(root, "projects", "alpha", "completed.md"),
		filepath.Join(root, "projects", "alpha", legacyV4ArchiveIndexFileName),
		filepath.Join(root, "projects", "alpha", legacyV4ProjectUndoDirName),
		filepath.Join(root, "projects", "alpha", legacyV4ProjectBackupName),
		filepath.Join(root, "projects", "beta", "completed.md"),
	} {
		if _, statErr := os.Lstat(path); !errors.Is(statErr, os.ErrNotExist) {
			t.Errorf("legacy artifact still exists %s: %v", path, statErr)
		}
	}

	alphaBytes, err := os.ReadFile(filepath.Join(root, "projects", "alpha", "tasks.md"))
	if err != nil {
		t.Fatal(err)
	}
	alpha, err := storage.ParseV5ProjectDocument(string(alphaBytes))
	if err != nil {
		t.Fatalf("parse migrated alpha tasks: %v", err)
	}
	var sawBlocker, sawArchived bool
	for _, task := range alpha.Tasks {
		sawBlocker = sawBlocker || task.ID == "synthetic-blocker-open" &&
			task.State == "inbox" && len(task.Tags) == 1 && task.Tags[0] == "blocker"
		sawArchived = sawArchived || task.ID == "synthetic-alpha-archived" && task.State == "done"
	}
	if !sawBlocker || !sawArchived {
		t.Fatalf("migrated tasks missing blocker/archive conversion: blocker=%v archived=%v", sawBlocker, sawArchived)
	}

	snapshots, err := InspectPreSimplificationV5Snapshots(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshots.Pairs) != 1 || len(snapshots.Issues) != 0 {
		t.Fatalf("snapshot inspection = %+v, want one complete pair", snapshots)
	}
	archiveEntries := readPreSimplificationV5Archive(t, snapshots.Pairs[0].ArchivePath)
	for _, path := range []string{
		"workspace/.tt/themes/custom.json",
		"workspace/themes/legacy.json",
	} {
		if _, ok := archiveEntries[path]; !ok {
			t.Errorf("migration snapshot missing retired custom theme %q", path)
		}
	}
	audit, err := os.ReadFile(snapshots.Pairs[0].AuditPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, marker := range []string{
		"## Conversion and repair report",
		"synthetic-duplicate-active",
		"Omitted duplicate tasks: 1",
		"standalone_blocker",
	} {
		if !strings.Contains(string(audit), marker) {
			t.Errorf("migration audit missing %q", marker)
		}
	}
	if recoverErr := manager.RecoverWorkspace(); recoverErr != nil {
		t.Fatalf("repeated migrated startup: %v", recoverErr)
	}
	repeated, err := InspectPreSimplificationV5Snapshots(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(repeated.Pairs) != 1 {
		t.Fatalf("idempotent startup created %d snapshot pairs, want 1", len(repeated.Pairs))
	}
}

func TestManagerV5MigrationAfterSnapshotFailuresPreserveTaskAndDomainBytes(t *testing.T) {
	tests := []struct {
		name        string
		wantFailure string
		force       func(t *testing.T, root string)
	}{
		{
			name:        "inventory comparison",
			wantFailure: "source changed after snapshot",
			force: func(t *testing.T, root string) {
				t.Helper()
				path := filepath.Join(
					root,
					"projects",
					"alpha",
					".attachments",
					"d339d9894d9d35bf3dff069a22086be037c7c98fb1d7fc57ea77c74e25098e87",
					"specification.txt",
				)
				data, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, append(data, '\n'), FilePermissions); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name:        "commit failure",
			wantFailure: "creating transactions directory",
			force: func(t *testing.T, root string) {
				t.Helper()
				transactionsDir := TransactionsDir(root)
				if err := os.RemoveAll(transactionsDir); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(transactionsDir, []byte("not a directory"), FilePermissions); err != nil {
					t.Fatal(err)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := copyLegacyV4ActivationFixture(t)
			before := captureV5MigrationTaskAndDomainBytes(t, root)
			manager := NewManager(ConfigPathForDataDir(root))

			err := manager.migrateWorkspaceToV5(&v5MigrationActivationTestHooks{
				afterSnapshotPublished: func(_ *PreSimplificationV5Snapshot) {
					tt.force(t, root)
				},
			})
			if err == nil || !strings.Contains(err.Error(), tt.wantFailure) {
				t.Fatalf("post-snapshot migration error = %v, want %q", err, tt.wantFailure)
			}
			after := captureV5MigrationTaskAndDomainBytes(t, root)
			if !reflect.DeepEqual(after, before) {
				t.Fatal("post-snapshot migration failure changed task or domain bytes")
			}

			snapshots, inspectErr := InspectPreSimplificationV5Snapshots(root)
			if inspectErr != nil {
				t.Fatal(inspectErr)
			}
			if len(snapshots.Pairs) != 1 || len(snapshots.Issues) != 0 {
				t.Fatalf("post-snapshot failure artifacts = %+v, want exactly one complete pair", snapshots)
			}
			pair := snapshots.Pairs[0]
			if !strings.Contains(err.Error(), pair.ArchivePath) ||
				!strings.Contains(err.Error(), pair.AuditPath) {
				t.Fatalf(
					"post-snapshot migration error %q does not name archive %q and audit %q",
					err,
					pair.ArchivePath,
					pair.AuditPath,
				)
			}
		})
	}
}

func TestManagerV5MigrationAcceptsOptionalDomainArtifacts(t *testing.T) {
	tests := []struct {
		name               string
		prepare            func(t *testing.T, root string) string
		auditMarkers       []string
		wantDefaultMachine bool
		wantLegacyMachine  bool
	}{
		{
			name: "machine absent",
			prepare: func(t *testing.T, _ string) string {
				t.Helper()
				if err := os.RemoveAll(LocalDataRoot()); err != nil {
					t.Fatal(err)
				}
				return ""
			},
			auditMarkers:       []string{"empty v4 machine state"},
			wantDefaultMachine: true,
		},
		{
			name: "machine at legacy path",
			prepare: func(t *testing.T, root string) string {
				t.Helper()
				canonicalPath, err := MachineStatePath(legacyV4WorkspaceID)
				if err != nil {
					t.Fatal(err)
				}
				data, err := os.ReadFile(canonicalPath)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.Remove(canonicalPath); err != nil {
					t.Fatal(err)
				}
				legacyPaths := legacyMachineStatePaths(root)
				legacyPath := legacyPaths[len(legacyPaths)-1]
				if err := os.MkdirAll(filepath.Dir(legacyPath), DirPermissions); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(legacyPath, data, FilePermissions); err != nil {
					t.Fatal(err)
				}
				return legacyPath
			},
			auditMarkers:      []string{"legacy machine state was converted into the canonical workspace-keyed domain"},
			wantLegacyMachine: true,
		},
		{
			name: "settings absent",
			prepare: func(t *testing.T, root string) string {
				t.Helper()
				if err := os.Remove(SettingsPath(root)); err != nil {
					t.Fatal(err)
				}
				return ""
			},
			auditMarkers: []string{"empty v4 settings state"},
		},
		{
			name: "automation absent",
			prepare: func(t *testing.T, root string) string {
				t.Helper()
				if err := os.Remove(AutomationPath(root)); err != nil {
					t.Fatal(err)
				}
				return ""
			},
			auditMarkers: []string{"empty v4 automation state"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := copyLegacyV4ActivationFixture(t)
			legacyMachinePath := tt.prepare(t, root)
			manager := NewManager(ConfigPathForDataDir(root))

			if err := manager.migrateWorkspaceToV5(nil); err != nil {
				t.Fatalf("activate migration with optional artifact unavailable: %v", err)
			}
			catalog, workspaceID, err := readV5CatalogCandidate(WorkspaceCatalogPath(root))
			if err != nil {
				t.Fatalf("load migrated v5 catalog: %v", err)
			}
			machine, _, _, err := readV5MachineCandidate(V6WorkspaceDryRunOptions{DataDir: root}, workspaceID)
			if err != nil {
				t.Fatalf("load migrated v5 machine state: %v", err)
			}
			if catalog.SchemaVersion != V5ConfigSchemaVersion {
				t.Fatalf("catalog schema = %d, want %d", catalog.SchemaVersion, V5ConfigSchemaVersion)
			}
			if tt.wantDefaultMachine {
				if len(machine.CustomContexts) != 0 || machine.Theme != "dark" {
					t.Fatalf("default machine state = %+v, want no custom contexts and dark theme", machine)
				}
			}
			if tt.wantLegacyMachine {
				want := []string{"project-alpha", "project-beta"}
				if got := machine.CustomContexts["synthetic-focus"]; !reflect.DeepEqual(got, want) {
					t.Fatalf("legacy machine context = %v, want %v", got, want)
				}
			}

			snapshots, err := InspectPreSimplificationV5Snapshots(root)
			if err != nil {
				t.Fatal(err)
			}
			if len(snapshots.Pairs) != 1 || len(snapshots.Issues) != 0 {
				t.Fatalf("snapshot inspection = %+v, want one complete pair", snapshots)
			}
			audit, err := os.ReadFile(snapshots.Pairs[0].AuditPath)
			if err != nil {
				t.Fatal(err)
			}
			for _, marker := range tt.auditMarkers {
				if !strings.Contains(string(audit), marker) {
					t.Errorf("migration audit missing %q", marker)
				}
			}
			if legacyMachinePath != "" {
				rel, err := filepath.Rel(LocalDataRoot(), legacyMachinePath)
				if err != nil {
					t.Fatal(err)
				}
				wantEntry := filepath.ToSlash(filepath.Join("machine", rel))
				archiveEntries := readPreSimplificationV5Archive(t, snapshots.Pairs[0].ArchivePath)
				if _, ok := archiveEntries[wantEntry]; !ok {
					t.Errorf("migration snapshot missing legacy machine state %q", wantEntry)
				}
			}
		})
	}
}

func TestManagerV5MigrationCorruptInputRefusesBeforeSnapshot(t *testing.T) {
	root := copyLegacyV4ActivationFixture(t)
	corrupt, err := os.ReadFile(filepath.Join(legacyV4FixtureRoot, "unsupported", "corrupt-note.md"))
	if err != nil {
		t.Fatal(err)
	}
	notePath := filepath.Join(root, ".tt", "global-notes", "global-reference.md")
	originalWorkspace, err := os.ReadFile(WorkspaceCatalogPath(root))
	if err != nil {
		t.Fatal(err)
	}
	if writeErr := os.WriteFile(notePath, corrupt, FilePermissions); writeErr != nil {
		t.Fatal(writeErr)
	}

	manager := NewManager(ConfigPathForDataDir(root))
	err = manager.RecoverWorkspace()
	if err == nil || !strings.Contains(err.Error(), "workspace was not changed") {
		t.Fatalf("corrupt migration error = %v, want unchanged refusal", err)
	}
	afterWorkspace, readErr := os.ReadFile(WorkspaceCatalogPath(root))
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(afterWorkspace) != string(originalWorkspace) {
		t.Fatal("corrupt-input refusal changed workspace catalog bytes")
	}
	snapshots, inspectErr := InspectPreSimplificationV5Snapshots(root)
	if inspectErr != nil {
		t.Fatal(inspectErr)
	}
	if snapshots.HasArtifacts {
		t.Fatalf("corrupt-input refusal created snapshot artifacts: %+v", snapshots)
	}
}

func TestManagerV5MigrationRejectsFutureTaskSchemaBeforeSnapshot(t *testing.T) {
	root := copyLegacyV4ActivationFixture(t)
	future, err := os.ReadFile(filepath.Join(legacyV4FixtureRoot, "unsupported", "future-tasks.md"))
	if err != nil {
		t.Fatal(err)
	}
	if writeErr := os.WriteFile(filepath.Join(root, "projects", "alpha", "tasks.md"), future, FilePermissions); writeErr != nil {
		t.Fatal(writeErr)
	}

	err = NewManager(ConfigPathForDataDir(root)).RecoverWorkspace()
	if err == nil || !strings.Contains(err.Error(), "unsupported task schema 99") {
		t.Fatalf("future task migration error = %v", err)
	}
	snapshots, inspectErr := InspectPreSimplificationV5Snapshots(root)
	if inspectErr != nil {
		t.Fatal(inspectErr)
	}
	if snapshots.HasArtifacts {
		t.Fatalf("future task refusal created snapshot artifacts: %+v", snapshots)
	}
}

func TestManagerV5MigrationRecoversPreparedActivationBeforeInspection(t *testing.T) {
	root := copyLegacyV4ActivationFixture(t)
	plan, err := buildV5MigrationActivationPlan(root, time.Local)
	if err != nil {
		t.Fatalf("build activation plan: %v", err)
	}
	if _, snapshotErr := createPreSimplificationV5SnapshotWithMigrationAudit(root, plan.audit); snapshotErr != nil {
		t.Fatalf("create migration snapshot: %v", snapshotErr)
	}

	ops := make([]transaction.Op, 0, len(plan.artifacts)+len(plan.cleanupOps))
	for _, artifact := range plan.artifacts {
		ops = append(ops, transaction.Op{
			Kind:   transaction.OpWrite,
			Target: artifact.Path,
			Data:   artifact.Data,
			Mode:   artifact.Mode,
		})
	}
	ops = append(ops, plan.cleanupOps...)
	journal := transaction.NewJournal(
		TransactionsDir(root),
		[]string{root, LocalDataRoot()}, "",
	)
	manifest, err := journal.Prepare(ops)
	if err != nil {
		t.Fatalf("prepare interrupted activation: %v", err)
	}
	if _, statErr := os.Stat(journal.ManifestPath(manifest.ID)); statErr != nil {
		t.Fatalf("prepared migration has no manifest: %v", statErr)
	}

	manager := NewManager(ConfigPathForDataDir(root))
	if recoverErr := manager.RecoverWorkspace(); recoverErr != nil {
		t.Fatalf("recover prepared activation: %v", recoverErr)
	}
	schema, exists, err := workspaceCatalogPayloadSchema(root)
	if err != nil {
		t.Fatal(err)
	}
	if !exists || schema != CurrentConfigVersion {
		t.Fatalf("recovered catalog schema = %d, exists=%v", schema, exists)
	}
	if _, statErr := os.Stat(journal.ManifestPath(manifest.ID)); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("recovered migration manifest still exists: %v", statErr)
	}
	if _, statErr := os.Stat(filepath.Join(root, "projects", "alpha", "completed.md")); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("recovered migration retained completed archive: %v", statErr)
	}
	snapshots, err := InspectPreSimplificationV5Snapshots(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshots.Pairs) != 1 || len(snapshots.Issues) != 0 {
		t.Fatalf("recovered migration snapshot inspection = %+v", snapshots)
	}
}
