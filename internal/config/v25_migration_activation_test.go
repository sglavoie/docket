package config

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// v25SourceWorkspace returns a private workspace at catalog schema 24, the one
// source the schema-25 boundary reads.
func v25SourceWorkspace(t *testing.T) string {
	t.Helper()
	root := copyMigratedFixture(t, 21)
	if err := NewManager(ConfigPathForDataDir(root)).MigrateWorkspaceToV24(); err != nil {
		t.Fatalf("reach schema 24: %v", err)
	}
	if schema, _, err := WorkspaceCatalogSchema(root); err != nil || schema != V24ConfigSchemaVersion {
		t.Fatalf("source catalog schema = %d (%v), want %d", schema, err, V24ConfigSchemaVersion)
	}
	return root
}

// v25ProjectDocuments reads every project document the catalog names, which
// the boundary must leave untouched.
func v25ProjectDocuments(t *testing.T, root string) map[string][]byte {
	t.Helper()
	catalog, err := readV8PublishedCatalog(WorkspaceCatalogPath(root))
	if err != nil {
		t.Fatal(err)
	}
	documents := map[string][]byte{}
	for _, project := range catalog.Projects {
		path := filepath.Join(project.CatalogPath(root), "tasks.md")
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			t.Fatal(readErr)
		}
		documents[path] = data
	}
	return documents
}

// TestV25MigrationIsTheSchemaNumberAlone is what makes the schema-25 boundary
// catalog-only: the published catalog is the schema-24 source apart from its
// schema number, no project document changes, the undo ledger is dropped, a
// pre-project-alias archive and audit are published, and a repeat run is a
// no-op that publishes nothing further.
func TestV25MigrationIsTheSchemaNumberAlone(t *testing.T) {
	root := v25SourceWorkspace(t)
	catalogPath := WorkspaceCatalogPath(root)
	sourceCatalog, err := os.ReadFile(catalogPath)
	if err != nil {
		t.Fatal(err)
	}
	documents := v25ProjectDocuments(t, root)
	ledger := filepath.Join(UndoLedgerDir(root), "ledger.json")
	if mkdirErr := os.MkdirAll(filepath.Dir(ledger), DirPermissions); mkdirErr != nil {
		t.Fatal(mkdirErr)
	}
	if writeErr := os.WriteFile(ledger, []byte("old history"), FilePermissions); writeErr != nil {
		t.Fatal(writeErr)
	}

	manager := NewManager(ConfigPathForDataDir(root))
	if migrateErr := manager.MigrateWorkspaceToV25(); migrateErr != nil {
		t.Fatal(migrateErr)
	}
	published, err := os.ReadFile(catalogPath)
	if err != nil {
		t.Fatal(err)
	}
	if want := bytes.Replace(sourceCatalog, []byte(`"schema_version": 24,`), []byte(`"schema_version": 25,`), 1); !bytes.Equal(published, want) {
		t.Fatalf("the boundary changed more than the schema number:\nsource:\n%s\npublished:\n%s", sourceCatalog, published)
	}
	if bytes.Contains(published, []byte(`"alias"`)) {
		t.Fatal("the boundary minted an alias key")
	}
	for path, before := range documents {
		after, readErr := os.ReadFile(path)
		if readErr != nil {
			t.Fatal(readErr)
		}
		if !bytes.Equal(before, after) {
			t.Fatalf("the boundary rewrote project document %s", path)
		}
	}
	if schema, _, schemaErr := WorkspaceCatalogSchema(root); schemaErr != nil || schema != V25ConfigSchemaVersion {
		t.Fatalf("catalog schema = %d (%v), want %d", schema, schemaErr, V25ConfigSchemaVersion)
	}
	if _, statErr := os.Stat(ledger); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("old undo survived: %v", statErr)
	}
	if state, inspectErr := inspectV24Legacy(root); inspectErr != nil || state != v24LegacyTombstone {
		t.Fatalf("legacy state = %v (%v), want the schema-24 tombstone kept", state, inspectErr)
	}
	archives, err := filepath.Glob(filepath.Join(PreScopeV7BackupsDir(root), preProjectAliasV25SnapshotPrefix+"*"+preScopeV7ArchiveSuffix))
	if err != nil || len(archives) != 1 {
		t.Fatalf("pre-project-alias archives = %v (%v), want exactly one", archives, err)
	}
	audits, err := filepath.Glob(filepath.Join(PreScopeV7BackupsDir(root), preProjectAliasV25SnapshotPrefix+"*"+preScopeV7AuditSuffix))
	if err != nil || len(audits) != 1 {
		t.Fatalf("pre-project-alias audits = %v (%v), want exactly one", audits, err)
	}
	audit, err := os.ReadFile(audits[0])
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Source catalog schema: 24", "alias", "Undo/redo history is dropped", "tombstone keeps declaring schema 24"} {
		if !strings.Contains(string(audit), want) {
			t.Errorf("audit does not mention %q:\n%s", want, audit)
		}
	}

	if repeatErr := manager.MigrateWorkspaceToV25(); repeatErr != nil {
		t.Fatalf("repeat migration: %v", repeatErr)
	}
	again, _ := filepath.Glob(filepath.Join(PreScopeV7BackupsDir(root), preProjectAliasV25SnapshotPrefix+"*"+preScopeV7ArchiveSuffix))
	if len(again) != 1 {
		t.Fatalf("a repeat run published another pair: %v", again)
	}
	if repeated, _ := os.ReadFile(catalogPath); !bytes.Equal(repeated, published) {
		t.Fatal("a repeat run rewrote the catalog")
	}
}

// TestV25MigrationReachesTheSchemaThroughTheChain runs the boundary on an older
// workspace, which must climb every predecessor first.
func TestV25MigrationReachesTheSchemaThroughTheChain(t *testing.T) {
	root := copyMigratedFixture(t, 21)
	if err := NewManager(ConfigPathForDataDir(root)).MigrateWorkspaceToV25(); err != nil {
		t.Fatal(err)
	}
	if schema, _, err := WorkspaceCatalogSchema(root); err != nil || schema != V25ConfigSchemaVersion {
		t.Fatalf("catalog schema = %d (%v), want %d", schema, err, V25ConfigSchemaVersion)
	}
}

// TestV25MigrationRefusesANewerCatalog leaves a catalog beyond this build
// untouched and says so.
func TestV25MigrationRefusesANewerCatalog(t *testing.T) {
	root := v25SourceWorkspace(t)
	catalogPath := WorkspaceCatalogPath(root)
	source, err := os.ReadFile(catalogPath)
	if err != nil {
		t.Fatal(err)
	}
	newer := bytes.Replace(source, []byte(`"schema_version": 24,`), []byte(`"schema_version": 26,`), 1)
	if writeErr := os.WriteFile(catalogPath, newer, FilePermissions); writeErr != nil {
		t.Fatal(writeErr)
	}
	err = NewManager(ConfigPathForDataDir(root)).MigrateWorkspaceToV25()
	if err == nil || !strings.Contains(err.Error(), "newer than supported") {
		t.Fatalf("MigrateWorkspaceToV25 = %v, want a newer-than-supported refusal", err)
	}
	if after, _ := os.ReadFile(catalogPath); !bytes.Equal(after, newer) {
		t.Fatal("a refused migration rewrote the catalog")
	}
	if archives, _ := filepath.Glob(filepath.Join(PreScopeV7BackupsDir(root), preProjectAliasV25SnapshotPrefix+"*")); len(archives) != 0 {
		t.Fatalf("a refused migration published %v", archives)
	}
}

// TestV25MigrationRefusesAnAliasBeforeItsSchema keeps validate-first: a
// schema-24 catalog that already carries an alias is one no build could have
// written, so the boundary refuses it before publishing any archive or byte.
func TestV25MigrationRefusesAnAliasBeforeItsSchema(t *testing.T) {
	root := v25SourceWorkspace(t)
	catalogPath := WorkspaceCatalogPath(root)
	source, err := os.ReadFile(catalogPath)
	if err != nil {
		t.Fatal(err)
	}
	smuggled := bytes.Replace(source, []byte(`"name": "Unfiled",`), []byte(`"name": "Unfiled",`+"\n"+`        "alias": "UF",`), 1)
	if bytes.Equal(smuggled, source) {
		t.Fatal("fixture has no Unfiled entry to edit")
	}
	if writeErr := os.WriteFile(catalogPath, smuggled, FilePermissions); writeErr != nil {
		t.Fatal(writeErr)
	}
	err = NewManager(ConfigPathForDataDir(root)).MigrateWorkspaceToV25()
	if err == nil || !strings.Contains(err.Error(), "workspace was not changed") || !strings.Contains(err.Error(), "requires schema version 25") {
		t.Fatalf("MigrateWorkspaceToV25 = %v, want a validation refusal naming the alias", err)
	}
	if after, _ := os.ReadFile(catalogPath); !bytes.Equal(after, smuggled) {
		t.Fatal("a refused migration rewrote the catalog")
	}
	if archives, _ := filepath.Glob(filepath.Join(PreScopeV7BackupsDir(root), preProjectAliasV25SnapshotPrefix+"*")); len(archives) != 0 {
		t.Fatalf("a refused migration published %v", archives)
	}
}

type v25TestHooks struct {
	afterSnapshot func()
}

func (h *v25TestHooks) didPublishSnapshot(string, string) {
	if h.afterSnapshot != nil {
		h.afterSnapshot()
	}
}

// TestV25MigrationAbortsOnWorkspaceMutationAfterSnapshot refuses to activate a
// workspace that no longer matches the archive it just published, so no write
// is migrated without a recoverable copy of itself.
func TestV25MigrationAbortsOnWorkspaceMutationAfterSnapshot(t *testing.T) {
	root := v25SourceWorkspace(t)
	documents := v25ProjectDocuments(t, root)
	var target string
	for path := range documents {
		target = path
		break
	}
	hooks := &v25TestHooks{afterSnapshot: func() {
		if err := os.WriteFile(target, append(documents[target], []byte("\n")...), FilePermissions); err != nil {
			t.Fatal(err)
		}
	}}
	err := NewManager(ConfigPathForDataDir(root)).migrateWorkspaceToV25(hooks)
	if err == nil || !strings.Contains(err.Error(), "changed after the rollback snapshot was published") {
		t.Fatalf("migrateWorkspaceToV25 = %v, want an abort naming the change", err)
	}
	if schema, _, schemaErr := WorkspaceCatalogSchema(root); schemaErr != nil || schema != V24ConfigSchemaVersion {
		t.Fatalf("catalog schema = %d (%v), want the source schema %d kept", schema, schemaErr, V24ConfigSchemaVersion)
	}
}

// TestV25MigrationConcurrentOpenersActivateOnce races two openers over one
// schema-24 workspace: one activates and the other waits, then finds the
// boundary crossed and publishes nothing of its own.
func TestV25MigrationConcurrentOpenersActivateOnce(t *testing.T) {
	root := v25SourceWorkspace(t)
	errs := make(chan error, 2)
	for range 2 {
		go func() { errs <- NewManager(ConfigPathForDataDir(root)).MigrateWorkspaceToV25() }()
	}
	for range 2 {
		if err := <-errs; err != nil {
			t.Fatalf("concurrent migration: %v", err)
		}
	}
	if schema, _, err := WorkspaceCatalogSchema(root); err != nil || schema != V25ConfigSchemaVersion {
		t.Fatalf("catalog schema = %d (%v), want %d", schema, err, V25ConfigSchemaVersion)
	}
	if pair, _ := filepath.Glob(filepath.Join(PreScopeV7BackupsDir(root), preProjectAliasV25SnapshotPrefix+"*"+preScopeV7ArchiveSuffix)); len(pair) != 1 {
		t.Fatalf("pre-project-alias artifacts = %v, want exactly one archive", pair)
	}
}

// TestCatalogAliasIsGatedBySchema holds the catalog to the schema that carries
// an alias: none before 25, and only what the model gate admits at 25. Empty is
// always absent and always valid.
func TestCatalogAliasIsGatedBySchema(t *testing.T) {
	stamp := time.Date(2026, 11, 1, 8, 0, 0, 0, time.UTC)
	catalogAt := func(schema int, alias string) *WorkspaceCatalog {
		return &WorkspaceCatalog{
			SchemaVersion: schema,
			Projects:      []CatalogProject{{ID: "p1", Slug: "alpha", Name: "Alpha", Alias: alias, CreatedAt: stamp, UpdatedAt: stamp, ColorSlot: 1}},
		}
	}
	for _, tc := range []struct {
		name    string
		schema  int
		alias   string
		wantErr string
	}{
		{name: "empty at 24", schema: V24ConfigSchemaVersion},
		{name: "empty at 25", schema: V25ConfigSchemaVersion},
		{name: "set at 25", schema: V25ConfigSchemaVersion, alias: "IA"},
		{name: "set at 24", schema: V24ConfigSchemaVersion, alias: "IA", wantErr: "requires schema version 25"},
		{name: "spaced at 25", schema: V25ConfigSchemaVersion, alias: "I A", wantErr: "invalid alias"},
		{name: "bracketed at 25", schema: V25ConfigSchemaVersion, alias: "[IA]", wantErr: "invalid alias"},
		{name: "untrimmed at 25", schema: V25ConfigSchemaVersion, alias: " IA", wantErr: "invalid alias"},
		{name: "too wide at 25", schema: V25ConfigSchemaVersion, alias: "thirteen-cols", wantErr: "invalid alias"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := catalogAt(tc.schema, tc.alias).validateAtSchema(V25ConfigSchemaVersion)
			switch {
			case tc.wantErr == "" && err != nil:
				t.Fatalf("validate = %v, want valid", err)
			case tc.wantErr != "" && (err == nil || !strings.Contains(err.Error(), tc.wantErr)):
				t.Fatalf("validate = %v, want an error containing %q", err, tc.wantErr)
			}
		})
	}
}

// TestUpsertProjectCarriesAndGatesTheAlias round-trips an alias through the
// catalog's model seam and refuses one the gate rejects before the catalog
// changes.
func TestUpsertProjectCarriesAndGatesTheAlias(t *testing.T) {
	stamp := time.Date(2026, 11, 1, 8, 0, 0, 0, time.UTC)
	catalog := NewWorkspaceCatalog()
	catalog.Projects = []CatalogProject{{ID: "p1", Slug: "alpha", Name: "Alpha", CreatedAt: stamp, UpdatedAt: stamp, ColorSlot: 1}}
	project := catalog.Projects[0].ToModel(t.TempDir())
	project.Alias = "web"
	if err := catalog.UpsertProject(project); err != nil {
		t.Fatal(err)
	}
	if got := catalog.Projects[0].Alias; got != "web" {
		t.Fatalf("catalog alias = %q, want %q", got, "web")
	}
	if got := catalog.Projects[0].ToModel(t.TempDir()).Alias; got != "web" {
		t.Fatalf("model alias = %q, want %q", got, "web")
	}
	project.Alias = "two words"
	if err := catalog.UpsertProject(project); err == nil || !strings.Contains(err.Error(), "invalid alias") {
		t.Fatalf("UpsertProject = %v, want an invalid-alias refusal", err)
	}
	if got := catalog.Projects[0].Alias; got != "web" {
		t.Fatalf("a refused upsert changed the alias to %q", got)
	}
}
