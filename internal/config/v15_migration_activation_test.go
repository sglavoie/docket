package config

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/transaction"
)

type v15MigrationActivationTestHooks struct {
	afterSnapshotPublished func(archivePath, auditPath string)
}

func (h *v15MigrationActivationTestHooks) didPublishSnapshot(archivePath, auditPath string) {
	if h != nil && h.afterSnapshotPublished != nil {
		h.afterSnapshotPublished(archivePath, auditPath)
	}
}

func preDependenciesV15SnapshotNames(t *testing.T, root string) []string {
	t.Helper()
	entries, err := os.ReadDir(PreScopeV7BackupsDir(root))
	if err != nil {
		t.Fatal(err)
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		name := entry.Name()
		if strings.HasPrefix(name, "pre-dependencies-v15-") &&
			(strings.HasSuffix(name, ".tar.gz") || strings.HasSuffix(name, ".audit.md")) {
			names = append(names, name)
		}
	}
	return names
}

//nolint:gocyclo // One end-to-end migration rehearsal keeps the guarded contract together.
func TestManagerSnapshotFirstV15MigrationAddsNoDependenciesAndClearsHistory(t *testing.T) {
	root := copyPreScopeV7Fixture(t).DataDir
	manager := NewManager(ConfigPathForDataDir(root))
	if err := manager.MigrateWorkspaceToV14(); err != nil {
		t.Fatalf("publish schema 14: %v", err)
	}

	catalog, err := readV8PublishedCatalog(WorkspaceCatalogPath(root))
	if err != nil {
		t.Fatal(err)
	}
	if len(catalog.Projects) == 0 {
		t.Fatal("fixture has no projects")
	}
	projectPath := storage.TasksFilePath(catalog.Projects[0].CatalogPath(root))
	beforeProjectBytes, err := os.ReadFile(projectPath)
	if err != nil {
		t.Fatal(err)
	}
	source, err := storage.ParseV13ProjectDocument(string(beforeProjectBytes))
	if err != nil {
		t.Fatal(err)
	}

	ledgerPath := filepath.Join(UndoLedgerDir(root), "ledger.json")
	if mkdirErr := os.MkdirAll(filepath.Dir(ledgerPath), DirPermissions); mkdirErr != nil {
		t.Fatal(mkdirErr)
	}
	if writeErr := os.WriteFile(ledgerPath, []byte("schema-13 history"), FilePermissions); writeErr != nil {
		t.Fatal(writeErr)
	}

	snapshotObserved := false
	if migrateErr := manager.migrateWorkspaceToV15(&v15MigrationActivationTestHooks{
		afterSnapshotPublished: func(archivePath, auditPath string) {
			snapshotObserved = true
			for _, path := range []string{archivePath, auditPath} {
				if _, statErr := os.Stat(path); statErr != nil {
					t.Fatalf("published rollback artifact %s: %v", path, statErr)
				}
			}
			schema, exists, schemaErr := workspaceCatalogPayloadSchema(root)
			if schemaErr != nil || !exists || schema != V14ConfigSchemaVersion {
				t.Fatalf("snapshot hook catalog schema = %d exists=%v err=%v, want %d", schema, exists, schemaErr, V14ConfigSchemaVersion)
			}
			current, readErr := os.ReadFile(projectPath)
			if readErr != nil {
				t.Fatal(readErr)
			}
			if !bytes.Equal(current, beforeProjectBytes) {
				t.Fatal("schema-13 task bytes changed before rollback snapshot publication")
			}
		},
	}); migrateErr != nil {
		t.Fatalf("migrate schema 15: %v", migrateErr)
	}
	if !snapshotObserved {
		t.Fatal("migration activated without observing rollback snapshot publication")
	}

	migrated, err := readV8PublishedCatalog(WorkspaceCatalogPath(root))
	if err != nil {
		t.Fatal(err)
	}
	if migrated.SchemaVersion != V15ConfigSchemaVersion {
		t.Fatalf("catalog schema = %d, want %d", migrated.SchemaVersion, V15ConfigSchemaVersion)
	}
	migratedBytes, err := os.ReadFile(projectPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(migratedBytes), "<!-- tt-schema: 14 -->") {
		t.Fatalf("project did not publish schema 14: %s", migratedBytes)
	}
	document, err := storage.ParseV14ProjectDocument(string(migratedBytes))
	if err != nil {
		t.Fatal(err)
	}
	if len(document.Tasks) != len(source.Tasks) {
		t.Fatalf("migrated tasks = %d, want %d", len(document.Tasks), len(source.Tasks))
	}
	for index, task := range document.Tasks {
		if len(task.Dependencies) != 0 {
			t.Fatalf("migrated task %q dependencies = %+v, want none", task.ID, task.Dependencies)
		}
		if task.ID != source.Tasks[index].ID || task.Title != source.Tasks[index].Title {
			t.Fatalf("migrated task %d changed identity or title", index)
		}
	}
	if strings.Contains(string(migratedBytes), "depends:") {
		t.Fatal("an empty dependency list was not canonically omitted")
	}
	// The whole rewrite must be the schema marker and nothing else: this is what
	// makes the boundary safe for a workspace that never had dependencies.
	if want := strings.Replace(string(beforeProjectBytes), "<!-- tt-schema: 13 -->", "<!-- tt-schema: 14 -->", 1); want != string(migratedBytes) {
		t.Fatalf("migration changed task bytes beyond the schema marker\nbefore: %s\nafter:  %s", beforeProjectBytes, migratedBytes)
	}
	if _, statErr := os.Stat(UndoLedgerDir(root)); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("undo history survived migration: %v", statErr)
	}

	var auditName string
	for _, name := range preDependenciesV15SnapshotNames(t, root) {
		if strings.HasSuffix(name, ".audit.md") {
			auditName = name
		}
	}
	if auditName == "" {
		t.Fatal("rollback snapshot audit missing")
	}
	auditBytes, err := os.ReadFile(filepath.Join(PreScopeV7BackupsDir(root), auditName))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"optional task-level `depends` field",
		"Dependencies retired before this boundary are not " + "recovered",
		"Undo/redo history is dropped at this boundary",
		"`<!-- tt-schema: 13 -->`",
	} {
		if !strings.Contains(string(auditBytes), want) {
			t.Fatalf("audit does not state %q:\n%s", want, auditBytes)
		}
	}
}

func TestManagerV15MigrationValidationFailurePublishesNothing(t *testing.T) {
	root := copyPreScopeV7Fixture(t).DataDir
	manager := NewManager(ConfigPathForDataDir(root))
	if err := manager.MigrateWorkspaceToV14(); err != nil {
		t.Fatalf("publish schema 14: %v", err)
	}
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
	beforeSnapshots := preDependenciesV15SnapshotNames(t, root)

	migrateErr := manager.MigrateWorkspaceToV15()
	if migrateErr == nil || !strings.Contains(migrateErr.Error(), "workspace was not changed") {
		t.Fatalf("invalid migration error = %v, want unchanged refusal", migrateErr)
	}
	if after := preDependenciesV15SnapshotNames(t, root); len(after) != len(beforeSnapshots) {
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

// TestManagerV15MigrationRefusesADocumentCarryingDepends proves the boundary
// validates every source document as strict schema 13 before writing: a task
// file that already carries the schema-14 key is not silently accepted.
func TestManagerV15MigrationRefusesADocumentCarryingDepends(t *testing.T) {
	root := copyPreScopeV7Fixture(t).DataDir
	manager := NewManager(ConfigPathForDataDir(root))
	if err := manager.MigrateWorkspaceToV14(); err != nil {
		t.Fatalf("publish schema 14: %v", err)
	}
	catalog, err := readV8PublishedCatalog(WorkspaceCatalogPath(root))
	if err != nil {
		t.Fatal(err)
	}
	projectPath := storage.TasksFilePath(catalog.Projects[0].CatalogPath(root))
	source, err := os.ReadFile(projectPath)
	if err != nil {
		t.Fatal(err)
	}
	smuggled := strings.Replace(string(source), " -->", ", depends: p:t -->", 1)
	if smuggled == string(source) {
		t.Fatal("fixture has no task metadata comment to smuggle a depends key into")
	}
	if err := os.WriteFile(projectPath, []byte(smuggled), FilePermissions); err != nil {
		t.Fatal(err)
	}

	migrateErr := manager.MigrateWorkspaceToV15()
	if migrateErr == nil || !strings.Contains(migrateErr.Error(), "workspace was not changed") {
		t.Fatalf("smuggled depends error = %v, want unchanged refusal", migrateErr)
	}
	if current, readErr := os.ReadFile(projectPath); readErr != nil {
		t.Fatal(readErr)
	} else if string(current) != smuggled {
		t.Fatal("refused migration changed project bytes")
	}
}

//nolint:govet // Scoped setup and recovery errors keep the interrupted transaction stages readable.
func TestManagerV15MigrationRecoversPreparedActivationOnStartup(t *testing.T) {
	root := copyPreScopeV7Fixture(t).DataDir
	manager := NewManager(ConfigPathForDataDir(root))
	if err := manager.MigrateWorkspaceToV14(); err != nil {
		t.Fatalf("publish schema 14: %v", err)
	}
	plan, err := buildV15MigrationPlan(root)
	if err != nil {
		t.Fatalf("build activation plan: %v", err)
	}
	if _, err := createPreDependenciesV15Snapshot(root); err != nil {
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
		t.Fatalf("recover prepared schema-15 activation on startup: %v", err)
	}
	// Startup rolls the prepared manifest forward and then continues the chain,
	// so the catalog lands on the newest supported schema. The single published
	// snapshot pair asserted below is what proves the v15 boundary ran once and
	// was never re-entered.
	schema, exists, err := workspaceCatalogPayloadSchema(root)
	if err != nil || !exists || schema != CurrentConfigVersion {
		t.Fatalf("recovered catalog schema = %d exists=%v err=%v, want %d", schema, exists, err, CurrentConfigVersion)
	}
	if _, statErr := os.Stat(journal.ManifestPath(manifest.ID)); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("recovered activation manifest still exists: %v", statErr)
	}
	if snapshots := preDependenciesV15SnapshotNames(t, root); len(snapshots) != 2 {
		t.Fatalf("recovered activation snapshot artifacts = %v, want the one published pair", snapshots)
	}
}

// TestManagerV15MigrationIsIdempotent proves a second activation over an
// already-migrated workspace writes nothing and publishes no second snapshot.
func TestManagerV15MigrationIsIdempotent(t *testing.T) {
	root := copyPreScopeV7Fixture(t).DataDir
	manager := NewManager(ConfigPathForDataDir(root))
	if err := manager.MigrateWorkspaceToV15(); err != nil {
		t.Fatalf("migrate schema 15: %v", err)
	}
	catalog, err := readV8PublishedCatalog(WorkspaceCatalogPath(root))
	if err != nil {
		t.Fatal(err)
	}
	projectPath := storage.TasksFilePath(catalog.Projects[0].CatalogPath(root))
	firstTasks, err := os.ReadFile(projectPath)
	if err != nil {
		t.Fatal(err)
	}
	firstCatalog, err := os.ReadFile(WorkspaceCatalogPath(root))
	if err != nil {
		t.Fatal(err)
	}
	firstSnapshots := preDependenciesV15SnapshotNames(t, root)

	if migrateErr := manager.MigrateWorkspaceToV15(); migrateErr != nil {
		t.Fatalf("idempotent migration: %v", migrateErr)
	}

	secondTasks, err := os.ReadFile(projectPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(firstTasks, secondTasks) {
		t.Fatalf("second migration rewrote the task document:\n%s", secondTasks)
	}
	secondCatalog, err := os.ReadFile(WorkspaceCatalogPath(root))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(firstCatalog, secondCatalog) {
		t.Fatal("second migration rewrote the catalog")
	}
	if second := preDependenciesV15SnapshotNames(t, root); len(second) != len(firstSnapshots) {
		t.Fatalf("second migration published snapshots %v, want %v", second, firstSnapshots)
	}
}

// TestManagerV15MigrationRefusesANewerWorkspace keeps the startup chain honest
// about a catalog this build cannot read.
func TestManagerV15MigrationRefusesANewerWorkspace(t *testing.T) {
	root := copyPreScopeV7Fixture(t).DataDir
	manager := NewManager(ConfigPathForDataDir(root))
	if err := manager.MigrateWorkspaceToV15(); err != nil {
		t.Fatalf("migrate schema 15: %v", err)
	}
	catalogPath := WorkspaceCatalogPath(root)
	source, err := os.ReadFile(catalogPath)
	if err != nil {
		t.Fatal(err)
	}
	// The marker must land past CurrentConfigVersion, not merely past schema 15:
	// the guard now refuses on what this build supports, so an intermediate
	// schema the chain can still reach is not a newer workspace.
	newer := bytes.Replace(source, []byte("\"schema_version\": 15"), []byte("\"schema_version\": 21"), 1)
	if bytes.Equal(newer, source) {
		t.Fatalf("catalog does not carry the expected schema marker:\n%s", source)
	}
	if err := os.WriteFile(catalogPath, newer, FilePermissions); err != nil {
		t.Fatal(err)
	}
	migrateErr := manager.MigrateWorkspaceToV15()
	if migrateErr == nil || !strings.Contains(migrateErr.Error(), "newer than supported schema") {
		t.Fatalf("newer workspace error = %v, want an upgrade refusal", migrateErr)
	}
}
