package config

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/storage"
)

// TestV23MigrationIsTheMarkerAlone is what makes the schema-23 boundary a
// marker-only rewrite: every project document is byte-identical to its
// schema-19 source apart from the marker, the undo ledger is dropped, a
// pre-hidden archive and audit are published, and a repeat run is a no-op.
func TestV23MigrationIsTheMarkerAlone(t *testing.T) {
	root := copyMigratedFixture(t, 21)
	manager := NewManager(ConfigPathForDataDir(root))
	catalog, err := readV8PublishedCatalog(WorkspaceCatalogPath(root))
	if err != nil {
		t.Fatal(err)
	}
	before := map[string][]byte{}
	for _, project := range catalog.Projects {
		path := storage.TasksFilePath(project.CatalogPath(root))
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			t.Fatal(readErr)
		}
		before[path] = data
	}
	ledger := filepath.Join(UndoLedgerDir(root), "ledger.json")
	if mkdirErr := os.MkdirAll(filepath.Dir(ledger), DirPermissions); mkdirErr != nil {
		t.Fatal(mkdirErr)
	}
	if writeErr := os.WriteFile(ledger, []byte("old history"), FilePermissions); writeErr != nil {
		t.Fatal(writeErr)
	}

	if migrateErr := manager.MigrateWorkspaceToV23(); migrateErr != nil {
		t.Fatal(migrateErr)
	}
	for path, source := range before {
		after, readErr := os.ReadFile(path)
		if readErr != nil {
			t.Fatal(readErr)
		}
		if !bytes.Equal(bytes.Replace(after, []byte("<!-- tt-schema: 20 -->"), []byte("<!-- tt-schema: 19 -->"), 1), source) {
			t.Fatalf("migration changed task values in %s:\n%s", path, after)
		}
		if bytes.Contains(after, []byte("hidden:")) {
			t.Fatalf("migration minted a hidden flag in %s", path)
		}
		if _, parseErr := storage.ParseV20ProjectDocument(string(after)); parseErr != nil {
			t.Fatal(parseErr)
		}
	}
	if schema, _, schemaErr := WorkspaceCatalogSchema(root); schemaErr != nil || schema != V23ConfigSchemaVersion {
		t.Fatalf("catalog schema = %d (%v), want %d", schema, schemaErr, V23ConfigSchemaVersion)
	}
	if _, statErr := os.Stat(ledger); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("old undo survived: %v", statErr)
	}
	archives, err := filepath.Glob(filepath.Join(PreScopeV7BackupsDir(root), preHiddenV23SnapshotPrefix+"*"+preScopeV7ArchiveSuffix))
	if err != nil || len(archives) != 1 {
		t.Fatalf("pre-hidden archives = %v (%v), want exactly one", archives, err)
	}
	audits, err := filepath.Glob(filepath.Join(PreScopeV7BackupsDir(root), preHiddenV23SnapshotPrefix+"*"+preScopeV7AuditSuffix))
	if err != nil || len(audits) != 1 {
		t.Fatalf("pre-hidden audits = %v (%v), want exactly one", audits, err)
	}
	if err := manager.MigrateWorkspaceToV23(); err != nil {
		t.Fatalf("repeat migration: %v", err)
	}
	again, _ := filepath.Glob(filepath.Join(PreScopeV7BackupsDir(root), preHiddenV23SnapshotPrefix+"*"+preScopeV7ArchiveSuffix))
	if len(again) != 1 {
		t.Fatalf("a repeat run published another archive: %v", again)
	}
}

// TestV23MigrationCarriesALegacyEnrolledCatalog proves a working copy enrolled
// at the retired schema 22 activates schema 23 like a schema-21 workspace and
// keeps its sync block, which from here on alone marks enrollment.
func TestV23MigrationCarriesALegacyEnrolledCatalog(t *testing.T) {
	root := copyMigratedFixture(t, 21)
	catalogPath := WorkspaceCatalogPath(root)
	_, catalog, err := readMigrationSourceCatalog(catalogPath, V21ConfigSchemaVersion)
	if err != nil {
		t.Fatal(err)
	}
	enrolledAt := time.Date(2026, time.October, 6, 12, 0, 0, 0, time.UTC)
	catalog.SchemaVersion = V22ConfigSchemaVersion
	catalog.Sync = &CatalogSync{EnrolledAt: enrolledAt, Exchange: SyncExchangeFormat}
	encoded, err := encodeMigrationCatalogEnvelope(&catalog, V22ConfigSchemaVersion)
	if err != nil {
		t.Fatal(err)
	}
	if writeErr := os.WriteFile(catalogPath, encoded, FilePermissions); writeErr != nil {
		t.Fatal(writeErr)
	}

	if migrateErr := NewManager(ConfigPathForDataDir(root)).MigrateWorkspaceToV23(); migrateErr != nil {
		t.Fatal(migrateErr)
	}
	_, migrated, err := readMigrationSourceCatalog(catalogPath, V23ConfigSchemaVersion)
	if err != nil {
		t.Fatal(err)
	}
	if !migrated.Enrolled() || !migrated.Sync.EnrolledAt.Equal(enrolledAt) || migrated.Sync.Exchange != SyncExchangeFormat {
		t.Fatalf("migrated catalog lost its sync block: %+v", migrated.Sync)
	}
	if err := migrated.Validate(); err != nil {
		t.Fatalf("migrated enrolled catalog: %v", err)
	}
	audits, _ := filepath.Glob(filepath.Join(PreScopeV7BackupsDir(root), preHiddenV23SnapshotPrefix+"*"+preScopeV7AuditSuffix))
	if len(audits) != 1 {
		t.Fatalf("pre-hidden audits = %v", audits)
	}
	audit, err := os.ReadFile(audits[0])
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(audit), "Source catalog schema: 22") {
		t.Fatalf("audit does not name the enrolled source:\n%s", audit)
	}
}

// TestV23MigrationRefusesAnInvalidDocumentUnchanged keeps validate-first: a
// project document the schema-19 reader refuses stops the boundary before any
// archive or byte is published.
func TestV23MigrationRefusesAnInvalidDocumentUnchanged(t *testing.T) {
	root := copyMigratedFixture(t, 21)
	catalog, err := readV8PublishedCatalog(WorkspaceCatalogPath(root))
	if err != nil {
		t.Fatal(err)
	}
	path := storage.TasksFilePath(catalog.Projects[0].CatalogPath(root))
	source, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	corrupt := bytes.Replace(source, []byte("<!-- tt-schema: 19 -->"), []byte("<!-- tt-schema: 20 -->"), 1)
	if writeErr := os.WriteFile(path, corrupt, FilePermissions); writeErr != nil {
		t.Fatal(writeErr)
	}
	catalogBefore, err := os.ReadFile(WorkspaceCatalogPath(root))
	if err != nil {
		t.Fatal(err)
	}
	err = NewManager(ConfigPathForDataDir(root)).MigrateWorkspaceToV23()
	if err == nil || !strings.Contains(err.Error(), "workspace was not changed") {
		t.Fatalf("MigrateWorkspaceToV23 = %v, want a validation refusal", err)
	}
	catalogAfter, err := os.ReadFile(WorkspaceCatalogPath(root))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(catalogBefore, catalogAfter) {
		t.Fatal("a refused migration rewrote the catalog")
	}
	archives, _ := filepath.Glob(filepath.Join(PreScopeV7BackupsDir(root), preHiddenV23SnapshotPrefix+"*"))
	if len(archives) != 0 {
		t.Fatalf("a refused migration published %v", archives)
	}
}
