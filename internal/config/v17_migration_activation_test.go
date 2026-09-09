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

type v17MigrationActivationTestHooks struct {
	afterSnapshotPublished func(archivePath, auditPath string)
}

func (h *v17MigrationActivationTestHooks) didPublishSnapshot(archivePath, auditPath string) {
	if h != nil && h.afterSnapshotPublished != nil {
		h.afterSnapshotPublished(archivePath, auditPath)
	}
}

func preStoredReviewV17SnapshotNames(t *testing.T, root string) []string {
	t.Helper()
	entries, err := os.ReadDir(PreScopeV7BackupsDir(root))
	if err != nil {
		t.Fatal(err)
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		name := entry.Name()
		if strings.HasPrefix(name, "pre-stored-review-v17-") &&
			(strings.HasSuffix(name, ".tar.gz") || strings.HasSuffix(name, ".audit.md")) {
			names = append(names, name)
		}
	}
	return names
}

// tagFirstV14TaskForReview rewrites one schema-14 project document so its first
// task carries the review tag, which is the exact shape the v17 boundary lifts.
func tagFirstV14TaskForReview(t *testing.T, path string) {
	t.Helper()
	source, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	document, err := storage.ParseV14ProjectDocument(string(source))
	if err != nil {
		t.Fatalf("parse schema-14 project document: %v", err)
	}
	if len(document.Tasks) == 0 {
		t.Fatalf("fixture project %s has no tasks", path)
	}
	document.Tasks[0].Tags = append([]string{"keepme", storage.LiftedReviewTaskTag}, document.Tasks[0].Tags...)
	rewritten, err := storage.WriteV14ProjectDocument(document)
	if err != nil {
		t.Fatalf("rewrite schema-14 project document: %v", err)
	}
	if err := os.WriteFile(path, []byte(rewritten), FilePermissions); err != nil {
		t.Fatal(err)
	}
}

//nolint:gocyclo // One end-to-end migration rehearsal keeps the guarded contract together.
func TestManagerSnapshotFirstV17MigrationLiftsReviewAndClearsHistory(t *testing.T) {
	root := copyPreScopeV7Fixture(t).DataDir
	manager := NewManager(ConfigPathForDataDir(root))
	if err := manager.MigrateWorkspaceToV16(); err != nil {
		t.Fatalf("publish schema 16: %v", err)
	}

	catalog, err := readV8PublishedCatalog(WorkspaceCatalogPath(root))
	if err != nil {
		t.Fatal(err)
	}
	if len(catalog.Projects) == 0 {
		t.Fatal("fixture has no projects")
	}
	projectPath := storage.TasksFilePath(catalog.Projects[0].CatalogPath(root))
	tagFirstV14TaskForReview(t, projectPath)

	ledgerPath := filepath.Join(UndoLedgerDir(root), "ledger.json")
	if mkdirErr := os.MkdirAll(filepath.Dir(ledgerPath), DirPermissions); mkdirErr != nil {
		t.Fatal(mkdirErr)
	}
	if writeErr := os.WriteFile(ledgerPath, []byte("schema-16 history"), FilePermissions); writeErr != nil {
		t.Fatal(writeErr)
	}

	snapshotObserved := false
	if migrateErr := manager.migrateWorkspaceToV17(&v17MigrationActivationTestHooks{
		afterSnapshotPublished: func(archivePath, auditPath string) {
			snapshotObserved = true
			for _, path := range []string{archivePath, auditPath} {
				if _, statErr := os.Stat(path); statErr != nil {
					t.Fatalf("published rollback artifact %s: %v", path, statErr)
				}
			}
			schema, exists, schemaErr := workspaceCatalogPayloadSchema(root)
			if schemaErr != nil || !exists || schema != V16ConfigSchemaVersion {
				t.Fatalf("snapshot hook catalog schema = %d exists=%v err=%v, want %d", schema, exists, schemaErr, V16ConfigSchemaVersion)
			}
		},
	}); migrateErr != nil {
		t.Fatalf("migrate schema 17: %v", migrateErr)
	}
	if !snapshotObserved {
		t.Fatal("migration activated without observing rollback snapshot publication")
	}

	migrated, err := readV8PublishedCatalog(WorkspaceCatalogPath(root))
	if err != nil {
		t.Fatal(err)
	}
	if migrated.SchemaVersion != V17ConfigSchemaVersion {
		t.Fatalf("catalog schema = %d, want %d", migrated.SchemaVersion, V17ConfigSchemaVersion)
	}
	migratedBytes, err := os.ReadFile(projectPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(migratedBytes), "<!-- tt-schema: 15 -->") {
		t.Fatalf("task Markdown did not reach schema 15: %s", migratedBytes)
	}
	document, err := storage.ParseV15ProjectDocument(string(migratedBytes))
	if err != nil {
		t.Fatalf("parse migrated document: %v", err)
	}
	if !document.Tasks[0].Review {
		t.Fatalf("review tag did not become the stored flag: %+v", document.Tasks[0])
	}
	for _, tag := range document.Tasks[0].Tags {
		if tag == storage.LiftedReviewTaskTag {
			t.Fatalf("lifted task still carries the review tag: %+v", document.Tasks[0].Tags)
		}
	}
	// The lift removes exactly the one tag it claims and leaves the rest alone.
	if !containsString(document.Tasks[0].Tags, "keepme") {
		t.Fatalf("lift dropped an unrelated tag: %+v", document.Tasks[0].Tags)
	}
	if _, statErr := os.Stat(UndoLedgerDir(root)); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("undo history survived migration: %v", statErr)
	}

	var auditName string
	for _, name := range preStoredReviewV17SnapshotNames(t, root) {
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
		"lifts review off the tag list",
		"not a lifecycle state",
		"`=review`",
		"`f=`",
		"Undo/redo history is dropped at this boundary",
	} {
		if !strings.Contains(string(auditBytes), want) {
			t.Fatalf("audit does not state %q:\n%s", want, auditBytes)
		}
	}
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

// TestManagerV17MigrationValidationFailurePublishesNothing proves the boundary
// reads every project document strictly before it publishes one byte.
func TestManagerV17MigrationValidationFailurePublishesNothing(t *testing.T) {
	root := copyPreScopeV7Fixture(t).DataDir
	manager := NewManager(ConfigPathForDataDir(root))
	if err := manager.MigrateWorkspaceToV16(); err != nil {
		t.Fatalf("publish schema 16: %v", err)
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
	beforeSnapshots := preStoredReviewV17SnapshotNames(t, root)

	migrateErr := manager.MigrateWorkspaceToV17()
	if migrateErr == nil || !strings.Contains(migrateErr.Error(), "workspace was not changed") {
		t.Fatalf("invalid migration error = %v, want unchanged refusal", migrateErr)
	}
	if after := preStoredReviewV17SnapshotNames(t, root); len(after) != len(beforeSnapshots) {
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

//nolint:govet // Scoped setup and recovery errors keep the interrupted transaction stages readable.
func TestManagerV17MigrationRecoversPreparedActivationOnStartup(t *testing.T) {
	root := copyPreScopeV7Fixture(t).DataDir
	manager := NewManager(ConfigPathForDataDir(root))
	if err := manager.MigrateWorkspaceToV16(); err != nil {
		t.Fatalf("publish schema 16: %v", err)
	}
	plan, err := buildV17MigrationPlan(root)
	if err != nil {
		t.Fatalf("build activation plan: %v", err)
	}
	if _, err := createPreStoredReviewV17Snapshot(root); err != nil {
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
		t.Fatalf("recover prepared schema-17 activation on startup: %v", err)
	}
	schema, exists, err := workspaceCatalogPayloadSchema(root)
	if err != nil || !exists || schema != CurrentConfigVersion {
		t.Fatalf("recovered catalog schema = %d exists=%v err=%v, want %d", schema, exists, err, CurrentConfigVersion)
	}
	if _, statErr := os.Stat(journal.ManifestPath(manifest.ID)); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("recovered activation manifest still exists: %v", statErr)
	}
	if snapshots := preStoredReviewV17SnapshotNames(t, root); len(snapshots) != 2 {
		t.Fatalf("recovered activation snapshot artifacts = %v, want the one published pair", snapshots)
	}
}

// TestManagerV17MigrationIsIdempotent proves a second activation over an
// already-migrated workspace writes nothing and publishes no second snapshot.
func TestManagerV17MigrationIsIdempotent(t *testing.T) {
	root := copyPreScopeV7Fixture(t).DataDir
	manager := NewManager(ConfigPathForDataDir(root))
	if err := manager.MigrateWorkspaceToV17(); err != nil {
		t.Fatalf("migrate schema 17: %v", err)
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
	firstSnapshots := preStoredReviewV17SnapshotNames(t, root)

	if migrateErr := manager.MigrateWorkspaceToV17(); migrateErr != nil {
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
	if second := preStoredReviewV17SnapshotNames(t, root); len(second) != len(firstSnapshots) {
		t.Fatalf("second migration published snapshots %v, want %v", second, firstSnapshots)
	}
}

// TestManagerV17MigrationRefusesANewerWorkspace keeps the startup chain honest
// about a catalog this build cannot read.
func TestManagerV17MigrationRefusesANewerWorkspace(t *testing.T) {
	root := copyPreScopeV7Fixture(t).DataDir
	manager := NewManager(ConfigPathForDataDir(root))
	if err := manager.MigrateWorkspaceToV17(); err != nil {
		t.Fatalf("migrate schema 17: %v", err)
	}
	catalogPath := WorkspaceCatalogPath(root)
	source, err := os.ReadFile(catalogPath)
	if err != nil {
		t.Fatal(err)
	}
	newer := bytes.Replace(source, []byte("\"schema_version\": 17"), []byte("\"schema_version\": 20"), 1)
	if bytes.Equal(newer, source) {
		t.Fatalf("catalog does not carry the expected schema marker:\n%s", source)
	}
	if err := os.WriteFile(catalogPath, newer, FilePermissions); err != nil {
		t.Fatal(err)
	}
	migrateErr := manager.MigrateWorkspaceToV17()
	if migrateErr == nil || !strings.Contains(migrateErr.Error(), "newer than supported schema") {
		t.Fatalf("newer workspace error = %v, want an upgrade refusal", migrateErr)
	}
}

// TestV17MigrationLeavesEveryBuiltinLensNonEmpty is the per-template assertion
// the boundary owes: a workspace that migrates must still be able to name every
// built-in lens, and every one of them must still carry a definition.
func TestV17MigrationLeavesEveryBuiltinLensNonEmpty(t *testing.T) {
	root := copyPreScopeV7Fixture(t).DataDir
	manager := NewManager(ConfigPathForDataDir(root))
	if err := manager.MigrateWorkspaceToV17(); err != nil {
		t.Fatalf("migrate schema 17: %v", err)
	}
	for _, lens := range BuiltinNamedLensTemplates() {
		t.Run(lens.Name, func(t *testing.T) {
			if err := lens.Validate(); err != nil {
				t.Fatalf("built-in template does not validate after migration: %v", err)
			}
			if lens.Mode == "" {
				t.Fatal("built-in template names no mode")
			}
			// The query is the whole predicate now, so a built-in that narrows
			// nothing is a built-in with an empty one.
			if lens.Query == "" {
				t.Fatal("built-in template narrows nothing")
			}
		})
	}
}
