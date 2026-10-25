package config

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/sglavoie/docket/internal/storage"
)

//nolint:gocyclo // One end-to-end migration rehearsal keeps the guarded contract together.
func TestManagerMigratesAllTagsToSchema8AtomicallyAndClearsHistory(t *testing.T) {
	root := copyMigratedFixture(t, 7)
	manager := NewManager(ConfigPathForDataDir(root))

	catalogPath := WorkspaceCatalogPath(root)
	data, err := os.ReadFile(catalogPath)
	if err != nil {
		t.Fatal(err)
	}
	var envelope domainEnvelope
	if unmarshalErr := json.Unmarshal(data, &envelope); unmarshalErr != nil {
		t.Fatal(unmarshalErr)
	}
	var catalog WorkspaceCatalog
	if unmarshalErr := json.Unmarshal(envelope.Data, &catalog); unmarshalErr != nil {
		t.Fatal(unmarshalErr)
	}
	if len(catalog.Projects) == 0 {
		t.Fatal("fixture has no projects")
	}
	catalog.Projects[0].Tags = []string{" Client / ACME ", "client-acme", "ZETA"}
	payload, _ := json.Marshal(catalog)
	envelope.Data = payload
	data, _ = json.MarshalIndent(envelope, "", "  ")
	if writeErr := os.WriteFile(catalogPath, data, FilePermissions); writeErr != nil {
		t.Fatal(writeErr)
	}

	projectPath := storage.TasksFilePath(catalog.Projects[0].CatalogPath(root))
	projectBytes, err := os.ReadFile(projectPath)
	if err != nil {
		t.Fatal(err)
	}
	document, err := storage.ParseV6ProjectDocument(string(projectBytes))
	if err != nil || len(document.Tasks) == 0 {
		t.Fatalf("parse fixture project: tasks=%d err=%v", len(document.Tasks), err)
	}
	document.Tasks[0].Tags = []string{"First Seen", "FIRST-SEEN", "Needs / Review"}
	legacyDocument, err := storage.WriteV6ProjectDocument(document)
	if err != nil {
		t.Fatal(err)
	}
	if writeErr := os.WriteFile(projectPath, []byte(legacyDocument), FilePermissions); writeErr != nil {
		t.Fatal(writeErr)
	}
	ledgerPath := filepath.Join(UndoLedgerDir(root), "ledger.json")
	if mkdirErr := os.MkdirAll(filepath.Dir(ledgerPath), DirPermissions); mkdirErr != nil {
		t.Fatal(mkdirErr)
	}
	if writeErr := os.WriteFile(ledgerPath, []byte("legacy history"), FilePermissions); writeErr != nil {
		t.Fatal(writeErr)
	}

	if migrateErr := manager.MigrateWorkspaceToV8(); migrateErr != nil {
		t.Fatalf("migrate schema 8: %v", migrateErr)
	}
	migrated, err := readV8PublishedCatalog(catalogPath)
	if err != nil {
		t.Fatal(err)
	}
	if migrated.SchemaVersion != V8ConfigSchemaVersion ||
		!reflect.DeepEqual(migrated.Projects[0].Tags, []string{"client-acme", "zeta"}) {
		t.Fatalf("catalog schema/tags = %d %v", migrated.SchemaVersion, migrated.Projects[0].Tags)
	}
	migratedBytes, err := os.ReadFile(projectPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(migratedBytes), "<!-- tt-schema: 7 -->") {
		t.Fatalf("project did not publish schema 7: %s", migratedBytes)
	}
	migratedDocument, err := storage.ParseV7ProjectDocument(string(migratedBytes))
	if err != nil {
		t.Fatal(err)
	}
	if got := migratedDocument.Tasks[0].Tags; !reflect.DeepEqual(got, []string{"first-seen", "needs-review"}) {
		t.Fatalf("task tags = %v", got)
	}
	if _, statErr := os.Stat(UndoLedgerDir(root)); !os.IsNotExist(statErr) {
		t.Fatalf("undo history survived migration: %v", statErr)
	}
	entries, err := os.ReadDir(PreScopeV7BackupsDir(root))
	if err != nil {
		t.Fatal(err)
	}
	foundArchive, foundAudit := false, false
	for _, entry := range entries {
		foundArchive = foundArchive || strings.HasPrefix(entry.Name(), "pre-tag-v8-") && strings.HasSuffix(entry.Name(), ".tar.gz")
		foundAudit = foundAudit || strings.HasPrefix(entry.Name(), "pre-tag-v8-") && strings.HasSuffix(entry.Name(), ".audit.md")
	}
	if !foundArchive || !foundAudit {
		t.Fatalf("rollback snapshot pair missing: %+v", entries)
	}
	if migrateErr := manager.MigrateWorkspaceToV8(); migrateErr != nil {
		t.Fatalf("idempotent migration: %v", migrateErr)
	}
}

func readV8PublishedCatalog(path string) (WorkspaceCatalog, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return WorkspaceCatalog{}, err
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	var envelope domainEnvelope
	if err := decoder.Decode(&envelope); err != nil {
		return WorkspaceCatalog{}, err
	}
	var catalog WorkspaceCatalog
	if err := json.Unmarshal(envelope.Data, &catalog); err != nil {
		return WorkspaceCatalog{}, err
	}
	return catalog, nil
}
