package config

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/transaction"
)

type v18MigrationActivationTestHooks struct {
	afterSnapshotPublished func(archivePath, auditPath string)
}

func (h *v18MigrationActivationTestHooks) didPublishSnapshot(archivePath, auditPath string) {
	if h != nil && h.afterSnapshotPublished != nil {
		h.afterSnapshotPublished(archivePath, auditPath)
	}
}

func preResumeNoteV18SnapshotNames(t *testing.T, root string) []string {
	t.Helper()
	entries, err := os.ReadDir(PreScopeV7BackupsDir(root))
	if err != nil {
		t.Fatal(err)
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		name := entry.Name()
		if strings.HasPrefix(name, "pre-resume-note-v18-") &&
			(strings.HasSuffix(name, ".tar.gz") || strings.HasSuffix(name, ".audit.md")) {
			names = append(names, name)
		}
	}
	return names
}

func rewriteV15TaskTitle(t *testing.T, path, title string) []byte {
	t.Helper()
	source, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	document, err := storage.ParseV15ProjectDocument(string(source))
	if err != nil || len(document.Tasks) == 0 {
		t.Fatalf("parse schema-15 project: tasks=%d err=%v", len(document.Tasks), err)
	}
	document.Tasks[0].Title = title
	published, err := storage.WriteV15ProjectDocument(document)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(published), FilePermissions); err != nil {
		t.Fatal(err)
	}
	return []byte(published)
}

// TestManagerSnapshotFirstV18MigrationAdvancesTheMarkerAndClearsHistory is the
// whole claim of this boundary: schema 16 admits a field nothing carries yet, so
// every document must come out byte-identical to its schema-15 source apart from
// the marker line, the catalog must advance beside it, and the undo ledger must
// be gone because its entries still hold schema-15 bytes.
//
//nolint:gocyclo // One end-to-end migration rehearsal keeps the guarded contract together.
func TestManagerSnapshotFirstV18MigrationAdvancesTheMarkerAndClearsHistory(t *testing.T) {
	root := copyPreScopeV7Fixture(t).DataDir
	manager := NewManager(ConfigPathForDataDir(root))
	if err := manager.MigrateWorkspaceToV17(); err != nil {
		t.Fatalf("publish schema 17: %v", err)
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
	beforeLenses, lensesExisted := readLensDefinitionsIfPresent(t, root)

	ledgerPath := filepath.Join(UndoLedgerDir(root), "ledger.json")
	if mkdirErr := os.MkdirAll(filepath.Dir(ledgerPath), DirPermissions); mkdirErr != nil {
		t.Fatal(mkdirErr)
	}
	if writeErr := os.WriteFile(ledgerPath, []byte("schema-17 history"), FilePermissions); writeErr != nil {
		t.Fatal(writeErr)
	}

	snapshotObserved := false
	if migrateErr := manager.migrateWorkspaceToV18(&v18MigrationActivationTestHooks{
		afterSnapshotPublished: func(archivePath, auditPath string) {
			snapshotObserved = true
			for _, path := range []string{archivePath, auditPath} {
				if _, statErr := os.Stat(path); statErr != nil {
					t.Fatalf("published rollback artifact %s: %v", path, statErr)
				}
			}
			schema, exists, schemaErr := workspaceCatalogPayloadSchema(root)
			if schemaErr != nil || !exists || schema != V17ConfigSchemaVersion {
				t.Fatalf("snapshot hook catalog schema = %d exists=%v err=%v, want %d", schema, exists, schemaErr, V17ConfigSchemaVersion)
			}
		},
	}); migrateErr != nil {
		t.Fatalf("migrate schema 18: %v", migrateErr)
	}
	if !snapshotObserved {
		t.Fatal("migration activated without observing rollback snapshot publication")
	}

	migrated, err := readV8PublishedCatalog(WorkspaceCatalogPath(root))
	if err != nil {
		t.Fatal(err)
	}
	if migrated.SchemaVersion != V18ConfigSchemaVersion {
		t.Fatalf("catalog schema = %d, want %d", migrated.SchemaVersion, V18ConfigSchemaVersion)
	}
	migratedBytes, err := os.ReadFile(projectPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(migratedBytes), "<!-- tt-schema: 16 -->") {
		t.Fatalf("task Markdown did not reach schema 16: %s", migratedBytes)
	}
	// The whole rewrite must be the schema marker and nothing else.
	if restored := bytes.Replace(migratedBytes, []byte("<!-- tt-schema: 16 -->"), []byte("<!-- tt-schema: 15 -->"), 1); !bytes.Equal(restored, beforeProjectBytes) {
		t.Fatalf("migration changed task bytes beyond the schema marker\nbefore: %s\nafter:  %s", beforeProjectBytes, migratedBytes)
	}
	document, err := storage.ParseV16ProjectDocument(string(migratedBytes))
	if err != nil {
		t.Fatalf("parse migrated document: %v", err)
	}
	for _, task := range document.Tasks {
		if task.ResumeNote != "" {
			t.Fatalf("migration minted a resume note on task %q: %q", task.ID, task.ResumeNote)
		}
	}
	// This boundary owns no lens axis, so the portable collection must be
	// untouched rather than republished with equivalent contents.
	if lensesExisted {
		afterLenses, stillThere := readLensDefinitionsIfPresent(t, root)
		if !stillThere || !bytes.Equal(beforeLenses, afterLenses) {
			t.Fatal("v18 migration rewrote the portable lens definitions")
		}
	}
	if _, statErr := os.Stat(UndoLedgerDir(root)); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("undo history survived migration: %v", statErr)
	}

	var auditName string
	for _, name := range preResumeNoteV18SnapshotNames(t, root) {
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
		"one optional retained resume note per task",
		"byte-identical to its schema-15 source",
		"Saved lens definitions are not rewritten",
		"Undo/redo history is dropped at this boundary",
	} {
		if !strings.Contains(string(auditBytes), want) {
			t.Fatalf("audit does not state %q:\n%s", want, auditBytes)
		}
	}
}

func readLensDefinitionsIfPresent(t *testing.T, root string) ([]byte, bool) {
	t.Helper()
	source, err := os.ReadFile(LensDefinitionsPath(root))
	if errors.Is(err, os.ErrNotExist) {
		return nil, false
	}
	if err != nil {
		t.Fatal(err)
	}
	return source, true
}

// TestManagerV18MigrationValidationFailurePublishesNothing proves the boundary
// reads every project document strictly before it publishes one byte.
func TestManagerV18MigrationValidationFailurePublishesNothing(t *testing.T) {
	root := copyPreScopeV7Fixture(t).DataDir
	manager := NewManager(ConfigPathForDataDir(root))
	if err := manager.MigrateWorkspaceToV17(); err != nil {
		t.Fatalf("publish schema 17: %v", err)
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
	beforeSnapshots := preResumeNoteV18SnapshotNames(t, root)

	migrateErr := manager.MigrateWorkspaceToV18()
	if migrateErr == nil || !strings.Contains(migrateErr.Error(), "workspace was not changed") {
		t.Fatalf("invalid migration error = %v, want unchanged refusal", migrateErr)
	}
	if after := preResumeNoteV18SnapshotNames(t, root); len(after) != len(beforeSnapshots) {
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

// TestManagerV18MigrationAbortsOnWorkspaceMutationAfterSnapshot proves the
// activation guard refuses a candidate the rollback archive cannot restore.
func TestManagerV18MigrationAbortsOnWorkspaceMutationAfterSnapshot(t *testing.T) {
	root := copyPreScopeV7Fixture(t).DataDir
	manager := NewManager(ConfigPathForDataDir(root))
	if err := manager.MigrateWorkspaceToV17(); err != nil {
		t.Fatalf("publish schema 17: %v", err)
	}
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
	migrateErr := manager.migrateWorkspaceToV18(&v18MigrationActivationTestHooks{
		afterSnapshotPublished: func(string, string) {
			mutated = rewriteV15TaskTitle(t, projectPath, "Concurrent edit after snapshot")
		},
	})
	if migrateErr == nil {
		t.Fatal("migration activated a candidate the rollback archive does not describe")
	}
	if !strings.Contains(migrateErr.Error(), "changed after the rollback snapshot was published") {
		t.Fatalf("migration error does not name the inventory divergence: %v", migrateErr)
	}

	schema, exists, err := workspaceCatalogPayloadSchema(root)
	if err != nil || !exists || schema != V17ConfigSchemaVersion {
		t.Fatalf("live catalog schema = %d exists=%v err=%v, want %d", schema, exists, err, V17ConfigSchemaVersion)
	}
	if current, readErr := os.ReadFile(WorkspaceCatalogPath(root)); readErr != nil {
		t.Fatal(readErr)
	} else if !bytes.Equal(current, catalogBytes) {
		t.Fatal("aborted v18 migration rewrote the schema-17 catalog")
	}
	current, err := os.ReadFile(projectPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(current, mutated) {
		t.Fatalf("aborted v18 migration rewrote live task bytes:\n%s", current)
	}
	if !strings.Contains(string(current), "<!-- tt-schema: 15 -->") {
		t.Fatalf("aborted v18 migration advanced the task schema marker:\n%s", current)
	}
}

//nolint:govet // Scoped setup and recovery errors keep the interrupted transaction stages readable.
func TestManagerV18MigrationRecoversPreparedActivationOnStartup(t *testing.T) {
	root := copyPreScopeV7Fixture(t).DataDir
	manager := NewManager(ConfigPathForDataDir(root))
	if err := manager.MigrateWorkspaceToV17(); err != nil {
		t.Fatalf("publish schema 17: %v", err)
	}
	plan, err := buildV18MigrationPlan(root)
	if err != nil {
		t.Fatalf("build activation plan: %v", err)
	}
	if _, err := createPreResumeNoteV18Snapshot(root); err != nil {
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
		t.Fatalf("recover prepared schema-18 activation on startup: %v", err)
	}
	schema, exists, err := workspaceCatalogPayloadSchema(root)
	if err != nil || !exists || schema != CurrentConfigVersion {
		t.Fatalf("recovered catalog schema = %d exists=%v err=%v, want %d", schema, exists, err, CurrentConfigVersion)
	}
	if _, statErr := os.Stat(journal.ManifestPath(manifest.ID)); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("recovered activation manifest still exists: %v", statErr)
	}
	if snapshots := preResumeNoteV18SnapshotNames(t, root); len(snapshots) != 2 {
		t.Fatalf("recovered activation snapshot artifacts = %v, want the one published pair", snapshots)
	}
}

// TestManagerV18MigrationIsIdempotent proves a second activation over an
// already-migrated workspace writes nothing and publishes no second snapshot.
func TestManagerV18MigrationIsIdempotent(t *testing.T) {
	root := copyPreScopeV7Fixture(t).DataDir
	manager := NewManager(ConfigPathForDataDir(root))
	if err := manager.MigrateWorkspaceToV18(); err != nil {
		t.Fatalf("migrate schema 18: %v", err)
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
	firstSnapshots := preResumeNoteV18SnapshotNames(t, root)

	if migrateErr := manager.MigrateWorkspaceToV18(); migrateErr != nil {
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
	if second := preResumeNoteV18SnapshotNames(t, root); len(second) != len(firstSnapshots) {
		t.Fatalf("second migration published snapshots %v, want %v", second, firstSnapshots)
	}
}

// TestManagerV18MigrationRefusesANewerWorkspace keeps the startup chain honest
// about a catalog this build cannot read.
func TestManagerV18MigrationRefusesANewerWorkspace(t *testing.T) {
	root := copyPreScopeV7Fixture(t).DataDir
	manager := NewManager(ConfigPathForDataDir(root))
	if err := manager.MigrateWorkspaceToV18(); err != nil {
		t.Fatalf("migrate schema 18: %v", err)
	}
	catalogPath := WorkspaceCatalogPath(root)
	source, err := os.ReadFile(catalogPath)
	if err != nil {
		t.Fatal(err)
	}
	newer := bytes.Replace(source, []byte("\"schema_version\": 18"), []byte("\"schema_version\": 21"), 1)
	if bytes.Equal(newer, source) {
		t.Fatalf("catalog does not carry the expected schema marker:\n%s", source)
	}
	if err := os.WriteFile(catalogPath, newer, FilePermissions); err != nil {
		t.Fatal(err)
	}
	migrateErr := manager.MigrateWorkspaceToV18()
	if migrateErr == nil || !strings.Contains(migrateErr.Error(), "newer than supported schema") {
		t.Fatalf("newer workspace error = %v, want an upgrade refusal", migrateErr)
	}
}

// TestV18MigrationCarriesArchivedAndParkedProjects keeps the boundary indifferent
// to a project's lifecycle: every cataloged project is migrated, not just the ones
// the Outliner would show.
func TestV18MigrationCarriesArchivedAndParkedProjects(t *testing.T) {
	root := copyPreScopeV7Fixture(t).DataDir
	manager := NewManager(ConfigPathForDataDir(root))
	if err := manager.MigrateWorkspaceToV17(); err != nil {
		t.Fatalf("publish schema 17: %v", err)
	}
	catalog, err := readV8PublishedCatalog(WorkspaceCatalogPath(root))
	if err != nil {
		t.Fatal(err)
	}
	markArchivedAndParked(t, root, &catalog)

	if migrateErr := manager.MigrateWorkspaceToV18(); migrateErr != nil {
		t.Fatalf("migrate schema 18: %v", migrateErr)
	}
	migrated, err := readV8PublishedCatalog(WorkspaceCatalogPath(root))
	if err != nil {
		t.Fatal(err)
	}
	for _, project := range migrated.Projects {
		path := storage.TasksFilePath(project.CatalogPath(root))
		source, readErr := os.ReadFile(path)
		if readErr != nil {
			t.Fatalf("read migrated project %s: %v", project.Slug, readErr)
		}
		if !strings.Contains(string(source), "<!-- tt-schema: 16 -->") {
			t.Fatalf("project %s did not reach schema 16:\n%s", project.Slug, source)
		}
	}
}

// markArchivedAndParked republishes the catalog with one ordinary project
// archived and another parked, which is the read-time-filtered pair a migration
// must still carry. The protected Unfiled project can be neither, so it is
// skipped rather than marked.
func markArchivedAndParked(t *testing.T, root string, catalog *WorkspaceCatalog) {
	t.Helper()
	markable := make([]*CatalogProject, 0, len(catalog.Projects))
	for index := range catalog.Projects {
		if catalog.Projects[index].ID == catalog.SystemUnfiledProjectID {
			continue
		}
		markable = append(markable, &catalog.Projects[index])
	}
	if len(markable) < 2 {
		t.Fatalf("fixture has %d markable projects; this case needs two", len(markable))
	}
	markable[0].Archived = true
	parked, err := models.NormalizeProjectTags(append(markable[1].Tags, "parked"))
	if err != nil {
		t.Fatal(err)
	}
	markable[1].Tags = parked
	published, err := encodeMigrationCatalogEnvelope(catalog, V17ConfigSchemaVersion)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(WorkspaceCatalogPath(root), published, FilePermissions); err != nil {
		t.Fatal(err)
	}
}

// loadMigratedProjectWithTasks opens the first cataloged project that actually
// holds a task, so the undo rehearsal has content to edit rather than an empty
// document whose commit would write nothing worth reversing.
func loadMigratedProjectWithTasks(t *testing.T, root string, catalog WorkspaceCatalog) (*models.Project, string) {
	t.Helper()
	for _, entry := range catalog.Projects {
		project, err := models.NewProject(entry.ID, entry.Name, "")
		if err != nil {
			t.Fatal(err)
		}
		project.Path = entry.CatalogPath(root)
		if loadErr := storage.LoadProject(project); loadErr != nil {
			t.Fatalf("load migrated project %s: %v", entry.Slug, loadErr)
		}
		if len(project.Tasks) > 0 {
			return project, storage.TasksFilePath(project.Path)
		}
	}
	t.Fatal("no migrated project holds a task")
	return nil, ""
}
