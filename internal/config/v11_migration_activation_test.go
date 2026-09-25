package config

import (
	"bytes"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/storage"
)

type v11MigrationActivationTestHooks struct {
	afterSnapshotPublished func(archivePath, auditPath string)
}

func (h *v11MigrationActivationTestHooks) didPublishSnapshot(archivePath, auditPath string) {
	if h != nil && h.afterSnapshotPublished != nil {
		h.afterSnapshotPublished(archivePath, auditPath)
	}
}

// bareDeferralDate matches a calendar-day deferral value: an instant always
// carries the `T` separator, so anything else after the date is a value the
// migration failed to lift.
var bareDeferralDate = regexp.MustCompile(`deferred: [0-9]{4}-[0-9]{2}-[0-9]{2}([^T]|$)`)

// TestManagerV11MigrationAbortsOnWorkspaceMutationAfterSnapshot proves the
// activation guard refuses a candidate the rollback archive cannot restore: a
// task document written after snapshot publication leaves the live schema-10
// workspace exactly as the mutation left it, still carrying the schema-9
// Markdown marker and the pre-lift calendar-day deferral.
func TestManagerV11MigrationAbortsOnWorkspaceMutationAfterSnapshot(t *testing.T) {
	root := copyMigratedFixture(t, 10)
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
	deferPreInstantTask(t, projectPath, "2030-01-02")

	var mutated []byte
	migrateErr := manager.migrateWorkspaceToV11(&v11MigrationActivationTestHooks{
		afterSnapshotPublished: func(string, string) {
			mutated = rewriteV11TaskTitle(t, projectPath, "Concurrent edit after snapshot")
		},
	})
	if migrateErr == nil {
		t.Fatal("migration activated a candidate the rollback archive does not describe")
	}
	if !strings.Contains(migrateErr.Error(), "changed after the rollback snapshot was published") {
		t.Fatalf("migration error does not name the inventory divergence: %v", migrateErr)
	}

	schema, exists, err := workspaceCatalogPayloadSchema(root)
	if err != nil || !exists || schema != V10ConfigSchemaVersion {
		t.Fatalf("live catalog schema = %d exists=%v err=%v, want %d", schema, exists, err, V10ConfigSchemaVersion)
	}
	if current, readErr := os.ReadFile(WorkspaceCatalogPath(root)); readErr != nil {
		t.Fatal(readErr)
	} else if !bytes.Equal(current, catalogBytes) {
		t.Fatal("aborted v11 migration rewrote the schema-10 catalog")
	}
	current, err := os.ReadFile(projectPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(current, mutated) {
		t.Fatalf("aborted v11 migration rewrote live task bytes:\n%s", current)
	}
	if !strings.Contains(string(current), "<!-- tt-schema: 9 -->") {
		t.Fatalf("live project document left schema 9: %s", current)
	}
	if !strings.Contains(string(current), "deferred: 2030-01-02") || !bareDeferralDate.MatchString(string(current)) {
		t.Fatalf("aborted v11 migration lifted the deferral value: %s", current)
	}
}

// deferPreInstantTask gives the first task a schema-9 calendar-day deferral so
// the lift has something to convert and the abort has something to preserve.
func deferPreInstantTask(t *testing.T, path, date string) {
	t.Helper()
	document := readV9TaskDocument(t, path)
	document.Tasks[0].SnoozedUntil = date
	writeV9TaskDocument(t, path, document)
}

func rewriteV11TaskTitle(t *testing.T, path, title string) []byte {
	t.Helper()
	document := readV9TaskDocument(t, path)
	document.Tasks[0].Title = title
	return writeV9TaskDocument(t, path, document)
}

func readV9TaskDocument(t *testing.T, path string) storage.V9ProjectDocument {
	t.Helper()
	source, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	document, err := storage.ParseV9ProjectDocument(string(source))
	if err != nil || len(document.Tasks) == 0 {
		t.Fatalf("parse schema-9 project: tasks=%d err=%v", len(document.Tasks), err)
	}
	return document
}

func writeV9TaskDocument(t *testing.T, path string, document storage.V9ProjectDocument) []byte {
	t.Helper()
	published, err := storage.WriteV9ProjectDocument(document)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(published), FilePermissions); err != nil {
		t.Fatal(err)
	}
	return []byte(published)
}

// useFixedLocalTimezone pins time.Local for one test so the instant the lift
// produces is deterministic rather than a property of the machine running the
// suite. The lift reads time.Local by design: that is the location the bare
// date was always implicitly parsed in.
func useFixedLocalTimezone(t *testing.T, name string) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation(name)
	if err != nil {
		t.Fatal(err)
	}
	previous := time.Local
	time.Local = loc
	t.Cleanup(func() { time.Local = previous })
	return loc
}

//nolint:gocyclo // One end-to-end migration rehearsal keeps the guarded contract together.
func TestManagerSnapshotFirstV11MigrationLiftsDeferralInstantAndClearsHistory(t *testing.T) {
	loc := useFixedLocalTimezone(t, "America/Los_Angeles")
	root := copyMigratedFixture(t, 10)
	manager := NewManager(ConfigPathForDataDir(root))

	catalog, err := readV8PublishedCatalog(WorkspaceCatalogPath(root))
	if err != nil {
		t.Fatal(err)
	}
	if len(catalog.Projects) == 0 {
		t.Fatal("fixture has no projects")
	}
	projectPath := storage.TasksFilePath(catalog.Projects[0].CatalogPath(root))
	deferPreInstantTask(t, projectPath, "2030-01-02")
	source := readV9TaskDocument(t, projectPath)
	wantInstant := time.Date(2030, time.January, 2, 0, 0, 0, 0, loc).Format(time.RFC3339Nano)

	ledgerPath := filepath.Join(UndoLedgerDir(root), "ledger.json")
	if mkdirErr := os.MkdirAll(filepath.Dir(ledgerPath), DirPermissions); mkdirErr != nil {
		t.Fatal(mkdirErr)
	}
	if writeErr := os.WriteFile(ledgerPath, []byte("schema-9 history"), FilePermissions); writeErr != nil {
		t.Fatal(writeErr)
	}

	if migrateErr := manager.MigrateWorkspaceToV11(); migrateErr != nil {
		t.Fatalf("migrate schema 11: %v", migrateErr)
	}

	migrated, err := readV8PublishedCatalog(WorkspaceCatalogPath(root))
	if err != nil {
		t.Fatal(err)
	}
	if migrated.SchemaVersion != V11ConfigSchemaVersion {
		t.Fatalf("catalog schema = %d, want %d", migrated.SchemaVersion, V11ConfigSchemaVersion)
	}
	migratedBytes, err := os.ReadFile(projectPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(migratedBytes), "<!-- tt-schema: 10 -->") {
		t.Fatalf("project did not publish schema 10: %s", migratedBytes)
	}
	if !strings.Contains(string(migratedBytes), "deferred: "+wantInstant) {
		t.Fatalf("project did not publish the deferral instant %q: %s", wantInstant, migratedBytes)
	}
	if bareDeferralDate.MatchString(string(migratedBytes)) {
		t.Fatalf("project retained a calendar-day deferral: %s", migratedBytes)
	}
	document, err := storage.ParseV10ProjectDocument(string(migratedBytes))
	if err != nil {
		t.Fatal(err)
	}
	if len(document.Tasks) != len(source.Tasks) {
		t.Fatalf("migrated tasks = %d, want %d", len(document.Tasks), len(source.Tasks))
	}
	for i, task := range document.Tasks {
		want := source.Tasks[i].SnoozedUntil
		if want != "" {
			day, parseErr := time.ParseInLocation("2006-01-02", want, loc)
			if parseErr != nil {
				t.Fatal(parseErr)
			}
			want = day.Format(time.RFC3339Nano)
		}
		if task.SnoozedUntil != want {
			t.Fatalf("task %q deferral = %q, want %q", task.ID, task.SnoozedUntil, want)
		}
	}
	if _, statErr := os.Stat(UndoLedgerDir(root)); !os.IsNotExist(statErr) {
		t.Fatalf("undo history survived migration: %v", statErr)
	}

	entries, err := os.ReadDir(PreScopeV7BackupsDir(root))
	if err != nil {
		t.Fatal(err)
	}
	var auditName string
	foundArchive := false
	for _, entry := range entries {
		if !strings.HasPrefix(entry.Name(), "pre-defer-instant-v11-") {
			continue
		}
		if strings.HasSuffix(entry.Name(), ".tar.gz") {
			foundArchive = true
		}
		if strings.HasSuffix(entry.Name(), ".audit.md") {
			auditName = entry.Name()
		}
	}
	if !foundArchive || auditName == "" {
		t.Fatalf("rollback snapshot pair missing: %+v", entries)
	}
	auditBytes, err := os.ReadFile(filepath.Join(PreScopeV7BackupsDir(root), auditName))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"canonical RFC3339Nano instant on 2 tasks across 2 projects",
		"still expires at exactly the moment it expired at before",
		"stops waking at each machine's own midnight",
		"Undo/redo history is dropped at this boundary",
		"| `project-archive` |",
	} {
		if !strings.Contains(string(auditBytes), want) {
			t.Fatalf("audit does not state %q:\n%s", want, auditBytes)
		}
	}
}

// TestOrdinaryCatalogPathsStillRefuseAnUnactivatedSchema proves the migration
// publish seam did not widen the guard: only encodeMigrationCatalogEnvelope
// tolerates a catalog above the schema this build activates, and every ordinary
// save and load still refuses one. The subject schema is one past whatever this
// build activates, so the proof survives the next bump without an edit.
func TestOrdinaryCatalogPathsStillRefuseAnUnactivatedSchema(t *testing.T) {
	catalog := &WorkspaceCatalog{
		SchemaVersion: CurrentConfigVersion + 1,
		WorkspaceID:   testWorkspaceID,
		Projects:      []CatalogProject{{ID: "alpha", Slug: "alpha", ColorSlot: 1}},
	}
	if err := catalog.Validate(); err == nil {
		t.Fatal("ordinary catalog validation accepted a schema this build does not activate")
	}
	if err := catalog.validateForLoad(); err == nil {
		t.Fatal("catalog load validation accepted a schema this build does not activate")
	}
	if _, err := encodeDomainEnvelope(domainCatalog, catalog.WorkspaceID, catalog); err == nil {
		t.Fatal("ordinary catalog encoding accepted a schema this build does not activate")
	}
	if _, err := encodeMigrationCatalogEnvelope(catalog, CurrentConfigVersion+1); err != nil {
		t.Fatalf("migration catalog encoding refused its own target schema: %v", err)
	}

	catalog.SchemaVersion = CurrentConfigVersion
	ordinaryBytes, err := encodeDomainEnvelope(domainCatalog, catalog.WorkspaceID, catalog)
	if err != nil {
		t.Fatal(err)
	}
	migrationBytes, err := encodeMigrationCatalogEnvelope(catalog, CurrentConfigVersion)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(ordinaryBytes, migrationBytes) {
		t.Fatalf("migration envelope bytes diverge from an ordinary save:\n%s\n%s", migrationBytes, ordinaryBytes)
	}
	if _, err := encodeMigrationCatalogEnvelope(catalog, CurrentConfigVersion-1); err == nil {
		t.Fatal("migration catalog encoding accepted a catalog above its own target schema")
	}
}

// TestManagerV11MigrationIsIdempotent proves a second activation over an
// already-migrated workspace writes nothing: neither the task document nor the
// catalog moves, and no second rollback snapshot pair is published.
func TestManagerV11MigrationIsIdempotent(t *testing.T) {
	root := copyPreScopeV7Fixture(t).DataDir
	manager := NewManager(ConfigPathForDataDir(root))
	if err := manager.MigrateWorkspaceToV11(); err != nil {
		t.Fatalf("migrate schema 11: %v", err)
	}
	catalog, err := readV8PublishedCatalog(WorkspaceCatalogPath(root))
	if err != nil {
		t.Fatal(err)
	}
	if len(catalog.Projects) == 0 {
		t.Fatal("fixture has no projects")
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
	firstSnapshots := deferInstantSnapshotNames(t, root)

	if migrateErr := manager.MigrateWorkspaceToV11(); migrateErr != nil {
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
	if second := deferInstantSnapshotNames(t, root); len(second) != len(firstSnapshots) {
		t.Fatalf("second migration published snapshots %v, want %v", second, firstSnapshots)
	}
}

func deferInstantSnapshotNames(t *testing.T, root string) []string {
	t.Helper()
	entries, err := os.ReadDir(PreScopeV7BackupsDir(root))
	if err != nil {
		t.Fatal(err)
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), "pre-defer-instant-v11-") {
			names = append(names, entry.Name())
		}
	}
	return names
}
