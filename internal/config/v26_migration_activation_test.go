package config

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// v26SourceWorkspace returns a private workspace at catalog schema 25, the one
// source the schema-26 boundary reads. The legacy fixture it climbs from holds
// a someday task, so the boundary has work to retire.
func v26SourceWorkspace(t *testing.T) string {
	t.Helper()
	root := copyMigratedFixture(t, 21)
	if err := NewManager(ConfigPathForDataDir(root)).MigrateWorkspaceToV25(); err != nil {
		t.Fatalf("reach schema 25: %v", err)
	}
	if schema, _, err := WorkspaceCatalogSchema(root); err != nil || schema != V25ConfigSchemaVersion {
		t.Fatalf("source catalog schema = %d (%v), want %d", schema, err, V25ConfigSchemaVersion)
	}
	return root
}

// TestV26MigrationRetiresSomedayAndMovesNothingElse is the schema-26 boundary's
// contract: a someday task becomes Backlog with its clocks and every other value
// byte for byte, every other document changes at its marker alone, the catalog
// at its number alone, the undo ledger is dropped, one archive and audit pair is
// published naming the count, and a repeat run publishes nothing further.
func TestV26MigrationRetiresSomedayAndMovesNothingElse(t *testing.T) {
	root := v26SourceWorkspace(t)
	catalogPath := WorkspaceCatalogPath(root)
	sourceCatalog, err := os.ReadFile(catalogPath)
	if err != nil {
		t.Fatal(err)
	}
	documents := v25ProjectDocuments(t, root)
	someday := 0
	for _, data := range documents {
		someday += bytes.Count(data, []byte("state: someday,"))
	}
	if someday == 0 {
		t.Fatal("fixture holds no someday task for the boundary to retire")
	}
	ledger := filepath.Join(UndoLedgerDir(root), "ledger.json")
	if mkdirErr := os.MkdirAll(filepath.Dir(ledger), DirPermissions); mkdirErr != nil {
		t.Fatal(mkdirErr)
	}
	if writeErr := os.WriteFile(ledger, []byte("old history"), FilePermissions); writeErr != nil {
		t.Fatal(writeErr)
	}

	manager := NewManager(ConfigPathForDataDir(root))
	if migrateErr := manager.MigrateWorkspaceToV26(); migrateErr != nil {
		t.Fatal(migrateErr)
	}
	published, err := os.ReadFile(catalogPath)
	if err != nil {
		t.Fatal(err)
	}
	if want := bytes.Replace(sourceCatalog, []byte(`"schema_version": 25,`), []byte(`"schema_version": 26,`), 1); !bytes.Equal(published, want) {
		t.Fatalf("the boundary changed the catalog beyond its schema number:\nsource:\n%s\npublished:\n%s", sourceCatalog, published)
	}
	for path, before := range documents {
		after, readErr := os.ReadFile(path)
		if readErr != nil {
			t.Fatal(readErr)
		}
		want := bytes.Replace(before, []byte("<!-- tt-schema: 20 -->"), []byte("<!-- tt-schema: 21 -->"), 1)
		want = bytes.ReplaceAll(want, []byte("state: someday,"), []byte("state: backlog,"))
		if !bytes.Equal(after, want) {
			t.Fatalf("project document %s changed beyond its marker and the retired state:\nwant:\n%s\ngot:\n%s", path, want, after)
		}
		if bytes.Contains(after, []byte("state: someday")) {
			t.Fatalf("project document %s still holds a someday task", path)
		}
	}
	if schema, _, schemaErr := WorkspaceCatalogSchema(root); schemaErr != nil || schema != V26ConfigSchemaVersion {
		t.Fatalf("catalog schema = %d (%v), want %d", schema, schemaErr, V26ConfigSchemaVersion)
	}
	if _, statErr := os.Stat(ledger); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("old undo survived: %v", statErr)
	}
	if state, inspectErr := inspectV24Legacy(root); inspectErr != nil || state != v24LegacyTombstone {
		t.Fatalf("legacy state = %v (%v), want the schema-24 tombstone kept", state, inspectErr)
	}
	archives, err := filepath.Glob(filepath.Join(PreScopeV7BackupsDir(root), preRetireSomedayV26SnapshotPrefix+"*"+preScopeV7ArchiveSuffix))
	if err != nil || len(archives) != 1 {
		t.Fatalf("pre-retire-someday archives = %v (%v), want exactly one", archives, err)
	}
	audits, err := filepath.Glob(filepath.Join(PreScopeV7BackupsDir(root), preRetireSomedayV26SnapshotPrefix+"*"+preScopeV7AuditSuffix))
	if err != nil || len(audits) != 1 {
		t.Fatalf("pre-retire-someday audits = %v (%v), want exactly one", audits, err)
	}
	audit, err := os.ReadFile(audits[0])
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Source catalog schema: 25", fmt.Sprintf("Tasks moved from someday to backlog: %d", someday), "`state_changed`, `updated`", "Undo/redo history is dropped", "tombstone keeps declaring schema 24"} {
		if !strings.Contains(string(audit), want) {
			t.Errorf("audit does not mention %q:\n%s", want, audit)
		}
	}

	if repeatErr := manager.MigrateWorkspaceToV26(); repeatErr != nil {
		t.Fatalf("repeat migration: %v", repeatErr)
	}
	again, _ := filepath.Glob(filepath.Join(PreScopeV7BackupsDir(root), preRetireSomedayV26SnapshotPrefix+"*"+preScopeV7ArchiveSuffix))
	if len(again) != 1 {
		t.Fatalf("a repeat run published another pair: %v", again)
	}
}

// TestV26MigrationReachesTheSchemaThroughTheChain runs the boundary on an older
// workspace, which must climb every predecessor first.
func TestV26MigrationReachesTheSchemaThroughTheChain(t *testing.T) {
	root := copyMigratedFixture(t, 21)
	if err := NewManager(ConfigPathForDataDir(root)).MigrateWorkspaceToV26(); err != nil {
		t.Fatal(err)
	}
	if schema, _, err := WorkspaceCatalogSchema(root); err != nil || schema != V26ConfigSchemaVersion {
		t.Fatalf("catalog schema = %d (%v), want %d", schema, err, V26ConfigSchemaVersion)
	}
}

// TestV26MigrationRefusesAnUnreadableDocument keeps validate-first: a project
// document schema 20 cannot read is refused before any archive or byte.
func TestV26MigrationRefusesAnUnreadableDocument(t *testing.T) {
	root := v26SourceWorkspace(t)
	broken := []byte("# Project: Broken\n<!-- tt-schema: 19 -->\n")
	for path := range v25ProjectDocuments(t, root) {
		if err := os.WriteFile(path, broken, FilePermissions); err != nil {
			t.Fatal(err)
		}
		break
	}
	catalogPath := WorkspaceCatalogPath(root)
	source, err := os.ReadFile(catalogPath)
	if err != nil {
		t.Fatal(err)
	}
	err = NewManager(ConfigPathForDataDir(root)).MigrateWorkspaceToV26()
	if err == nil || !strings.Contains(err.Error(), "workspace was not changed") {
		t.Fatalf("MigrateWorkspaceToV26 = %v, want a validation refusal", err)
	}
	if after, _ := os.ReadFile(catalogPath); !bytes.Equal(after, source) {
		t.Fatal("a refused migration rewrote the catalog")
	}
	if archives, _ := filepath.Glob(filepath.Join(PreScopeV7BackupsDir(root), preRetireSomedayV26SnapshotPrefix+"*")); len(archives) != 0 {
		t.Fatalf("a refused migration published %v", archives)
	}
}
