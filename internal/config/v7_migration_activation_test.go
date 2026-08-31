package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/transaction"
)

var preScopeV7TestTime = time.Date(2026, 8, 5, 9, 10, 11, 0, time.UTC)

type preScopeV7SnapshotTestHooks struct {
	now            time.Time
	afterInventory func()
	beforePublish  func(archivePath, auditPath string) error
}

func (h *preScopeV7SnapshotTestHooks) snapshotCreatedAt() time.Time {
	if h == nil {
		return time.Time{}
	}
	return h.now
}

func (h *preScopeV7SnapshotTestHooks) didInventorySnapshot() {
	if h != nil && h.afterInventory != nil {
		h.afterInventory()
	}
}

func (h *preScopeV7SnapshotTestHooks) willPublishSnapshot(archivePath, auditPath string) error {
	if h == nil || h.beforePublish == nil {
		return nil
	}
	return h.beforePublish(archivePath, auditPath)
}

type v7MigrationActivationTestHooks struct {
	snapshot               *preScopeV7SnapshotTestHooks
	afterSnapshotPublished func(*PreScopeV7Snapshot)
}

func (h *v7MigrationActivationTestHooks) snapshotCreationCallbacks() preScopeV7SnapshotCallbacks {
	if h == nil || h.snapshot == nil {
		return nil
	}
	return h.snapshot
}

func (h *v7MigrationActivationTestHooks) didPublishSnapshot(snapshot *PreScopeV7Snapshot) {
	if h != nil && h.afterSnapshotPublished != nil {
		h.afterSnapshotPublished(snapshot)
	}
}

// copyPreScopeV7Fixture publishes a real schema-6 workspace and then writes the
// retired scope pair back into its catalog. No released migration ever emitted
// those fields, so the only faithful source is the shape a released build saved.
func copyPreScopeV7Fixture(t *testing.T) PreScopeV7SnapshotOptions {
	t.Helper()
	root := copyPreConsolidationV6Fixture(t).DataDir
	if err := NewManager(ConfigPathForDataDir(root)).MigrateWorkspaceToV6(); err != nil {
		t.Fatalf("publish schema-6 fixture: %v", err)
	}
	rewriteV7SourceCatalog(t, root, func(catalog *v7SourceCatalog) {
		catalog.Boundaries = []v7SourceScope{
			{ID: "active", Name: "Active"},
			{ID: "scope-shelf", Name: "On The Shelf"},
		}
		for index := range catalog.Projects {
			scope := "active"
			if catalog.Projects[index].ID == "project-beta" {
				scope = "scope-shelf"
			}
			catalog.Projects[index].BoundaryID = scope
		}
	})
	path := WorkspaceCatalogPath(root)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path+".bak", data, FilePermissions); err != nil {
		t.Fatal(err)
	}
	return PreScopeV7SnapshotOptions{DataDir: root}
}

func rewriteV7SourceCatalog(t *testing.T, root string, mutate func(*v7SourceCatalog)) {
	t.Helper()
	path := WorkspaceCatalogPath(root)
	catalog, err := readV7SourceCatalog(path)
	if err != nil {
		t.Fatalf("read schema-6 catalog: %v", err)
	}
	mutate(&catalog)
	payload, err := json.Marshal(catalog)
	if err != nil {
		t.Fatal(err)
	}
	data, err := json.MarshalIndent(domainEnvelope{
		SchemaVersion: CurrentDomainSchemaVersion,
		WorkspaceID:   catalog.WorkspaceID,
		Kind:          domainCatalog.name(),
		Data:          payload,
	}, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, FilePermissions); err != nil {
		t.Fatal(err)
	}
}

func preScopeV7Pairs(t *testing.T, root string) [][2]string {
	t.Helper()
	dir := PreScopeV7BackupsDir(root)
	entries, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	archives := make(map[string]string)
	audits := make(map[string]string)
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasPrefix(name, preScopeV7SnapshotPrefix) {
			continue
		}
		switch {
		case strings.HasSuffix(name, preScopeV7ArchiveSuffix):
			archives[strings.TrimSuffix(name, preScopeV7ArchiveSuffix)] = filepath.Join(dir, name)
		case strings.HasSuffix(name, preScopeV7AuditSuffix):
			audits[strings.TrimSuffix(name, preScopeV7AuditSuffix)] = filepath.Join(dir, name)
		}
	}
	stems := make([]string, 0, len(archives))
	for stem := range archives {
		if audits[stem] != "" {
			stems = append(stems, stem)
		}
	}
	sort.Strings(stems)
	pairs := make([][2]string, 0, len(stems))
	for _, stem := range stems {
		pairs = append(pairs, [2]string{archives[stem], audits[stem]})
	}
	return pairs
}

func loadV7ActivatedCatalog(t *testing.T, root string) WorkspaceCatalog {
	t.Helper()
	data, err := os.ReadFile(WorkspaceCatalogPath(root))
	if err != nil {
		t.Fatal(err)
	}
	catalog, err := decodeV7PublishedCatalog(data)
	if err != nil {
		t.Fatalf("decode activated catalog: %v", err)
	}
	return catalog
}

//nolint:gocyclo // One end-to-end rehearsal keeps every schema-7 activation invariant together.
func TestManagerActivatesSnapshotFirstV7MigrationAndIsIdempotent(t *testing.T) {
	options := copyPreScopeV7Fixture(t)
	root := options.DataDir

	manager := NewManager(ConfigPathForDataDir(root))
	if err := manager.migrateWorkspaceToV7(&v7MigrationActivationTestHooks{
		snapshot: &preScopeV7SnapshotTestHooks{now: preScopeV7TestTime},
	}); err != nil {
		t.Fatalf("activate schema 7: %v", err)
	}

	schema, exists, err := workspaceCatalogPayloadSchema(root)
	if err != nil || !exists || schema != V7ConfigSchemaVersion {
		t.Fatalf("activated catalog schema = %d exists=%v err=%v", schema, exists, err)
	}
	catalog := loadV7ActivatedCatalog(t, root)
	if err := catalog.Validate(); err != nil {
		t.Fatalf("activated catalog does not validate: %v", err)
	}
	parked := 0
	for _, project := range catalog.Projects {
		if project.ID == "project-beta" {
			if !slices.Contains(project.Tags, "parked") || !slices.Contains(project.Tags, "on-the-shelf") {
				t.Errorf("parked project tags = %v, want the reserved tag and the retired grouping name", project.Tags)
			}
		}
		if slices.Contains(project.Tags, "parked") {
			parked++
		}
	}
	if parked != 1 {
		t.Fatalf("parked projects after activation = %d, want exactly the one grouped project", parked)
	}
	if catalog.SystemUnfiledProjectID == "" {
		t.Fatal("activation lost the protected Unfiled reference")
	}

	if _, statErr := os.Lstat(WorkspaceCatalogPath(root) + ".bak"); !errors.Is(statErr, os.ErrNotExist) {
		t.Errorf("schema-6 catalog sidecar remains live: %v", statErr)
	}

	pairs := preScopeV7Pairs(t, root)
	if len(pairs) != 1 {
		t.Fatalf("published pre-v7 snapshot pairs = %v, want one", pairs)
	}
	wantStem := "pre-scope-v7-20260805T091011Z"
	if got := filepath.Base(pairs[0][0]); got != wantStem+preScopeV7ArchiveSuffix {
		t.Errorf("archive name = %q, want canonical UTC name", got)
	}
	entries := readPreSimplificationV5Archive(t, pairs[0][0])
	for _, path := range []string{
		"workspace/.tt/workspace.json",
		"workspace/.tt/planning.json",
		"workspace/projects/alpha/tasks.md",
		preScopeV7AuditEntry,
	} {
		if _, found := entries[path]; !found {
			t.Errorf("rollback snapshot is missing %q", path)
		}
	}
	for path := range entries {
		if strings.Contains(path, "/backups/") {
			t.Errorf("rollback snapshot archived a protected backup artifact %q", path)
		}
	}
	audit := string(entries[preScopeV7AuditEntry])
	for _, marker := range []string{
		"Schema-7 candidate validation: complete",
		"Retired grouping definitions discarded: 2",
		"Projects parked by the reserved `parked` tag: 1",
		"## Schema-7 project tags",
		"## Conversion decisions",
	} {
		if !strings.Contains(audit, marker) {
			t.Errorf("migration audit is missing %q", marker)
		}
	}

	if err := manager.MigrateWorkspaceToV7(); err != nil {
		t.Fatalf("idempotent schema-7 activation: %v", err)
	}
	if repeated := preScopeV7Pairs(t, root); !reflect.DeepEqual(repeated, pairs) {
		t.Fatalf("idempotent activation changed snapshot pairs: before=%v after=%v", pairs, repeated)
	}
	if after := loadV7ActivatedCatalog(t, root); !reflect.DeepEqual(after, catalog) {
		t.Fatal("idempotent activation rewrote the schema-7 catalog")
	}
}

func TestManagerV7MigrationSnapshotPublicationFailureLeavesWorkspaceUnchanged(t *testing.T) {
	options := copyPreScopeV7Fixture(t)
	before := capturePreConsolidationV6WorkspaceBytes(t, options.DataDir)
	wantErr := errors.New("injected pre-publication failure")
	err := NewManager(ConfigPathForDataDir(options.DataDir)).migrateWorkspaceToV7(
		&v7MigrationActivationTestHooks{snapshot: &preScopeV7SnapshotTestHooks{
			now:           preScopeV7TestTime,
			beforePublish: func(_, _ string) error { return wantErr },
		}},
	)
	if !errors.Is(err, wantErr) {
		t.Fatalf("pre-publication activation error = %v", err)
	}
	after := capturePreConsolidationV6WorkspaceBytes(t, options.DataDir)
	if !reflect.DeepEqual(after, before) {
		t.Fatal("pre-publication activation failure changed workspace bytes")
	}
	if pairs := preScopeV7Pairs(t, options.DataDir); len(pairs) != 0 {
		t.Fatalf("pre-publication failure left snapshot pairs: %v", pairs)
	}
}

func TestManagerV7MigrationDetectsStaleSourceAfterSnapshot(t *testing.T) {
	options := copyPreScopeV7Fixture(t)
	tasksPath := filepath.Join(options.DataDir, "projects", "alpha", "tasks.md")
	err := NewManager(ConfigPathForDataDir(options.DataDir)).migrateWorkspaceToV7(&v7MigrationActivationTestHooks{
		snapshot: &preScopeV7SnapshotTestHooks{now: preScopeV7TestTime},
		afterSnapshotPublished: func(_ *PreScopeV7Snapshot) {
			data, readErr := os.ReadFile(tasksPath)
			if readErr != nil {
				t.Fatal(readErr)
			}
			if writeErr := os.WriteFile(tasksPath, append(data, '\n'), FilePermissions); writeErr != nil {
				t.Fatal(writeErr)
			}
		},
	})
	if err == nil || !strings.Contains(err.Error(), "source changed while snapshotting") {
		t.Fatalf("stale-source activation error = %v", err)
	}
	schema, exists, schemaErr := workspaceCatalogPayloadSchema(options.DataDir)
	if schemaErr != nil || !exists || schema != V6ConfigSchemaVersion {
		t.Fatalf("stale-source refusal catalog schema = %d exists=%v err=%v", schema, exists, schemaErr)
	}
	pairs := preScopeV7Pairs(t, options.DataDir)
	if len(pairs) != 1 || !strings.Contains(err.Error(), pairs[0][0]) || !strings.Contains(err.Error(), pairs[0][1]) {
		t.Fatalf("stale-source error %q does not preserve and name one snapshot pair: %v", err, pairs)
	}
}

//nolint:govet // Scoped setup and recovery errors keep the interrupted transaction stages readable.
func TestManagerV7MigrationRecoversPreparedActivationBeforeInspection(t *testing.T) {
	options := copyPreScopeV7Fixture(t)
	plan, err := buildV7MigrationActivationPlan(V7WorkspaceDryRunOptions(options))
	if err != nil {
		t.Fatalf("build activation plan: %v", err)
	}
	if _, err := createPreScopeV7Snapshot(options, &preScopeV7SnapshotTestHooks{now: preScopeV7TestTime}); err != nil {
		t.Fatalf("create rollback snapshot: %v", err)
	}

	ops := make([]transaction.Op, 0, len(plan.artifacts)+len(plan.cleanupOps))
	for _, artifact := range plan.artifacts {
		ops = append(ops, transaction.Op{
			Kind: transaction.OpWrite, Target: artifact.Path, Data: artifact.Data, Mode: artifact.Mode,
		})
	}
	ops = append(ops, plan.cleanupOps...)
	journal := transaction.NewJournal(TransactionsDir(options.DataDir), []string{options.DataDir, LocalDataRoot()}, "")
	manifest, err := journal.Prepare(ops)
	if err != nil {
		t.Fatalf("prepare interrupted activation: %v", err)
	}

	if err := NewManager(ConfigPathForDataDir(options.DataDir)).MigrateWorkspaceToV7(); err != nil {
		t.Fatalf("recover prepared schema-7 activation: %v", err)
	}
	schema, exists, err := workspaceCatalogPayloadSchema(options.DataDir)
	if err != nil || !exists || schema != V7ConfigSchemaVersion {
		t.Fatalf("recovered catalog schema = %d exists=%v err=%v", schema, exists, err)
	}
	if _, statErr := os.Stat(journal.ManifestPath(manifest.ID)); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("recovered activation manifest still exists: %v", statErr)
	}
	if pairs := preScopeV7Pairs(t, options.DataDir); len(pairs) != 1 {
		t.Fatalf("recovered activation snapshot pairs = %v, want the one prepared pair", pairs)
	}
}

func TestManagerV7MigrationRejectsFutureSchemasBeforeSnapshot(t *testing.T) {
	options := copyPreScopeV7Fixture(t)
	rewriteV7SourceCatalog(t, options.DataDir, func(catalog *v7SourceCatalog) {
		catalog.SchemaVersion = V7ConfigSchemaVersion + 1
	})
	err := NewManager(ConfigPathForDataDir(options.DataDir)).MigrateWorkspaceToV7()
	if err == nil || !strings.Contains(err.Error(), "newer than supported schema 7") {
		t.Fatalf("future-schema activation error = %v", err)
	}
	if pairs := preScopeV7Pairs(t, options.DataDir); len(pairs) != 0 {
		t.Fatalf("future-schema refusal created rollback snapshot: %v", pairs)
	}
}

func TestManagerV7MigrationPreservesSupportedV4ThroughV7Chain(t *testing.T) {
	root := copyLegacyV4ActivationFixture(t)
	if err := NewManager(ConfigPathForDataDir(root)).MigrateWorkspaceToV7(); err != nil {
		t.Fatalf("migrate v4 through v5 and v6 to v7: %v", err)
	}
	schema, exists, err := workspaceCatalogPayloadSchema(root)
	if err != nil || !exists || schema != V7ConfigSchemaVersion {
		t.Fatalf("chained migration catalog schema = %d exists=%v err=%v", schema, exists, err)
	}
	catalog := loadV7ActivatedCatalog(t, root)
	if err := catalog.Validate(); err != nil {
		t.Fatalf("chained migration catalog does not validate: %v", err)
	}
	for _, project := range catalog.Projects {
		if slices.Contains(project.Tags, "parked") {
			t.Errorf("chained migration parked project %q with no retired grouping on disk", project.ID)
		}
	}
	if len(preScopeV7Pairs(t, root)) != 1 {
		t.Fatal("chained migration did not publish exactly one pre-v7 snapshot pair")
	}
}
