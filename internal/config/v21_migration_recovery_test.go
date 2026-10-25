package config

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/sglavoie/docket/internal/storage"
	"github.com/sglavoie/docket/internal/transaction"
)

// TestManagerV21MigrationValidationFailurePublishesNothing proves the boundary
// reads every project document strictly before it publishes one byte.
func TestManagerV21MigrationValidationFailurePublishesNothing(t *testing.T) {
	root := copyMigratedFixture(t, 20)
	manager := NewManager(ConfigPathForDataDir(root))
	catalog, err := readV8PublishedCatalog(WorkspaceCatalogPath(root))
	if err != nil {
		t.Fatal(err)
	}
	projectPath := storage.TasksFilePath(catalog.Projects[0].CatalogPath(root))
	beforeCatalog, err := os.ReadFile(WorkspaceCatalogPath(root))
	if err != nil {
		t.Fatal(err)
	}
	invalid := []byte("<!-- tt-schema: 99 -->\n# Tasks\n")
	if err := os.WriteFile(projectPath, invalid, FilePermissions); err != nil {
		t.Fatal(err)
	}
	beforeSnapshots := preCheckInsV21SnapshotNames(t, root)

	migrateErr := manager.MigrateWorkspaceToV21()
	if migrateErr == nil || !strings.Contains(migrateErr.Error(), "workspace was not changed") {
		t.Fatalf("invalid migration error = %v, want unchanged refusal", migrateErr)
	}
	if after := preCheckInsV21SnapshotNames(t, root); len(after) != len(beforeSnapshots) {
		t.Fatalf("validation failure published snapshot artifacts: before=%v after=%v", beforeSnapshots, after)
	}
	if current, readErr := os.ReadFile(WorkspaceCatalogPath(root)); readErr != nil {
		t.Fatal(readErr)
	} else if !bytes.Equal(current, beforeCatalog) {
		t.Fatal("validation failure changed catalog bytes")
	}
	if current, readErr := os.ReadFile(projectPath); readErr != nil {
		t.Fatal(readErr)
	} else if !bytes.Equal(current, invalid) {
		t.Fatal("validation failure changed project bytes")
	}
}

// TestManagerV21MigrationAbortsOnWorkspaceMutationAfterSnapshot proves the
// activation guard refuses a candidate the rollback archive cannot restore.
func TestManagerV21MigrationAbortsOnWorkspaceMutationAfterSnapshot(t *testing.T) {
	root := copyMigratedFixture(t, 20)
	manager := NewManager(ConfigPathForDataDir(root))
	catalog, err := readV8PublishedCatalog(WorkspaceCatalogPath(root))
	if err != nil {
		t.Fatal(err)
	}
	if len(catalog.Projects) == 0 {
		t.Fatal("fixture has no projects")
	}
	projectPath := storage.TasksFilePath(catalog.Projects[0].CatalogPath(root))
	catalogBytes, err := os.ReadFile(WorkspaceCatalogPath(root))
	if err != nil {
		t.Fatal(err)
	}

	var mutated []byte
	migrateErr := manager.migrateWorkspaceToV21(&v21MigrationActivationTestHooks{
		afterSnapshotPublished: func(string, string) {
			mutated = rewriteV18TaskTitle(t, projectPath, "Concurrent edit after snapshot")
		},
	})
	if migrateErr == nil {
		t.Fatal("migration activated a candidate the rollback archive does not describe")
	}
	if !strings.Contains(migrateErr.Error(), "changed after the rollback snapshot was published") {
		t.Fatalf("migration error does not name the inventory divergence: %v", migrateErr)
	}

	schema, exists, err := workspaceCatalogPayloadSchema(root)
	if err != nil || !exists || schema != V20ConfigSchemaVersion {
		t.Fatalf("live catalog schema = %d exists=%v err=%v, want %d", schema, exists, err, V20ConfigSchemaVersion)
	}
	if current, readErr := os.ReadFile(WorkspaceCatalogPath(root)); readErr != nil {
		t.Fatal(readErr)
	} else if !bytes.Equal(current, catalogBytes) {
		t.Fatal("aborted v21 migration rewrote the schema-20 catalog")
	}
	current, err := os.ReadFile(projectPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(current, mutated) {
		t.Fatalf("aborted v21 migration rewrote live task bytes:\n%s", current)
	}
	if !strings.Contains(string(current), "<!-- tt-schema: 18 -->") {
		t.Fatalf("aborted v21 migration advanced the task schema marker:\n%s", current)
	}
}

//nolint:govet // Scoped setup and recovery errors keep the interrupted transaction stages readable.
func TestManagerV21MigrationRecoversPreparedActivationOnStartup(t *testing.T) {
	root := copyMigratedFixture(t, 20)
	manager := NewManager(ConfigPathForDataDir(root))
	plan, err := buildV21MigrationPlan(root)
	if err != nil {
		t.Fatalf("build activation plan: %v", err)
	}
	if _, err := createPreCheckInsV21Snapshot(root, plan.seeded); err != nil {
		t.Fatalf("create rollback snapshot: %v", err)
	}

	ops := make([]transaction.Op, 0, len(plan.artifacts)+len(plan.cleanup))
	for _, artifact := range plan.artifacts {
		ops = append(ops, transaction.Op{
			Kind: transaction.OpWrite, Target: artifact.Path, Data: artifact.Data, Mode: artifact.Mode,
		})
	}
	ops = append(ops, plan.cleanup...)
	journal := transaction.NewJournal(TransactionsDir(root), []string{root, LocalDataRoot()}, "")
	manifest, err := journal.Prepare(ops)
	if err != nil {
		t.Fatalf("prepare interrupted activation: %v", err)
	}

	if err := manager.RecoverWorkspace(); err != nil {
		t.Fatalf("recover prepared schema-21 activation on startup: %v", err)
	}
	schema, exists, err := workspaceCatalogPayloadSchema(root)
	if err != nil || !exists || schema != CurrentConfigVersion {
		t.Fatalf("recovered catalog schema = %d exists=%v err=%v, want %d", schema, exists, err, CurrentConfigVersion)
	}
	if _, statErr := os.Stat(journal.ManifestPath(manifest.ID)); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("recovered activation manifest still exists: %v", statErr)
	}
	if snapshots := preCheckInsV21SnapshotNames(t, root); len(snapshots) != 2 {
		t.Fatalf("recovered activation snapshot artifacts = %v, want the one published pair", snapshots)
	}
}

// TestManagerV21MigrationIsIdempotent proves a second activation over an
// already-migrated workspace writes nothing and publishes no second snapshot. It
// is also what makes the seeding pass safe to re-run: a task that already carries
// the check-in this boundary would mint is left exactly as it is.
func TestManagerV21MigrationIsIdempotent(t *testing.T) {
	root := copyPreScopeV7Fixture(t).DataDir
	manager := NewManager(ConfigPathForDataDir(root))
	if err := manager.MigrateWorkspaceToV21(); err != nil {
		t.Fatalf("migrate schema 21: %v", err)
	}
	catalogBytes, err := os.ReadFile(WorkspaceCatalogPath(root))
	if err != nil {
		t.Fatal(err)
	}
	snapshots := preCheckInsV21SnapshotNames(t, root)
	if len(snapshots) != 2 {
		t.Fatalf("published snapshot artifacts = %v, want one pair", snapshots)
	}

	if err := manager.MigrateWorkspaceToV21(); err != nil {
		t.Fatalf("second migration: %v", err)
	}
	if current, readErr := os.ReadFile(WorkspaceCatalogPath(root)); readErr != nil {
		t.Fatal(readErr)
	} else if !bytes.Equal(current, catalogBytes) {
		t.Fatal("second v21 activation rewrote the catalog")
	}
	if after := preCheckInsV21SnapshotNames(t, root); len(after) != len(snapshots) {
		t.Fatalf("second v21 activation published snapshots: %v", after)
	}
}

// TestManagerV21MigrationRefusesANewerWorkspace keeps the guard honest about a
// catalog this build cannot read, which is the same refusal an old binary owes a
// schema-21 workspace.
func TestManagerV21MigrationRefusesANewerWorkspace(t *testing.T) {
	root := copyMigratedFixture(t, 21)
	manager := NewManager(ConfigPathForDataDir(root))
	catalogPath := WorkspaceCatalogPath(root)
	source, err := os.ReadFile(catalogPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(catalogPath, newerThanSupportedCatalog(t, source, V21ConfigSchemaVersion), FilePermissions); err != nil {
		t.Fatal(err)
	}
	migrateErr := manager.MigrateWorkspaceToV21()
	if migrateErr == nil || !strings.Contains(migrateErr.Error(), "newer than supported schema") {
		t.Fatalf("newer workspace error = %v, want an upgrade refusal", migrateErr)
	}
}

// newerThanSupportedCatalog rewrites a catalog's schema marker to one version
// past what this build supports, so the caller can check that the guard refuses
// it.
//
// The replacement is derived from CurrentConfigVersion rather than spelled,
// because every one of these guards was written with a literal that was newer at
// the time and stopped being newer the moment that version was allocated. A
// guard that has quietly become a supported version tests nothing and still
// passes for the boundary that owns it, which is exactly how five of them failed
// together at this boundary instead of one of them failing at each.
func newerThanSupportedCatalog(t *testing.T, source []byte, currentSchema int) []byte {
	t.Helper()
	newer := bytes.Replace(source,
		fmt.Appendf(nil, "%q: %d", "schema_version", currentSchema),
		fmt.Appendf(nil, "%q: %d", "schema_version", SupportedConfigVersion+1), 1)
	if bytes.Equal(newer, source) {
		t.Fatalf("catalog does not carry the expected schema marker:\n%s", source)
	}
	return newer
}
