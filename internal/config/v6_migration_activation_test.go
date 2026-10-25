package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/sglavoie/docket/internal/storage"
	"github.com/sglavoie/docket/internal/transaction"
)

type v6MigrationActivationTestHooks struct {
	snapshot               *preConsolidationV6SnapshotHooks
	afterSnapshotPublished func(*PreConsolidationV6Snapshot)
}

func (h *v6MigrationActivationTestHooks) snapshotCreationCallbacks() preConsolidationV6SnapshotCallbacks {
	if h == nil || h.snapshot == nil {
		return nil
	}
	return h.snapshot
}

func (h *v6MigrationActivationTestHooks) didPublishSnapshot(snapshot *PreConsolidationV6Snapshot) {
	if h != nil && h.afterSnapshotPublished != nil {
		h.afterSnapshotPublished(snapshot)
	}
}

//nolint:gocyclo,govet // One end-to-end rehearsal keeps all migration invariants and scoped setup errors together.
func TestManagerActivatesSnapshotFirstV6MigrationAndIsIdempotent(t *testing.T) {
	options := copyPreConsolidationV6Fixture(t)
	root := options.DataDir
	machinePath, err := MachineStatePath(v5ConsolidationWorkspaceID)
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{
		WorkspaceCatalogPath(root),
		PlanningStatePath(root),
		filepath.Join(root, "projects", "alpha", "tasks.md"),
	} {
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			t.Fatal(readErr)
		}
		if writeErr := os.WriteFile(path+".bak", data, FilePermissions); writeErr != nil {
			t.Fatal(writeErr)
		}
	}
	globalNotePath := filepath.Join(storage.GlobalNotesDir(root), "global-brief.md")
	projectNotePath := filepath.Join(root, "projects", "alpha", "notes", "project-brief.md")
	inboxNoteSource := filepath.Join(root, "projects", "inbox", "notes", "inbox-brief.md")
	if err := os.MkdirAll(filepath.Dir(inboxNoteSource), DirPermissions); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(
		inboxNoteSource,
		mustReadV6ActivationFile(t, globalNotePath),
		FilePermissions,
	); err != nil {
		t.Fatal(err)
	}
	attachmentSource := filepath.Join(
		root, "projects", "alpha", ".attachments", "task-rich", "specification.txt",
	)
	globalNote := mustReadV6ActivationFile(t, globalNotePath)
	projectNote := mustReadV6ActivationFile(t, projectNotePath)
	inboxNote := mustReadV6ActivationFile(t, inboxNoteSource)
	attachment := mustReadV6ActivationFile(t, attachmentSource)

	manager := NewManager(ConfigPathForDataDir(root))
	if err := manager.migrateWorkspaceToV6(&v6MigrationActivationTestHooks{
		snapshot: &preConsolidationV6SnapshotHooks{now: preConsolidationV6TestTime},
	}); err != nil {
		t.Fatalf("activate schema 6: %v", err)
	}

	schema, exists, err := workspaceCatalogPayloadSchema(root)
	if err != nil || !exists || schema != V6ConfigSchemaVersion {
		t.Fatalf("activated catalog schema = %d exists=%v err=%v", schema, exists, err)
	}
	var catalog V6WorkspaceCatalogCandidate
	workspaceID, err := readV6ActivationDomain(WorkspaceCatalogPath(root), "workspace", "", &catalog)
	if err != nil {
		t.Fatal(err)
	}
	if workspaceID != v5ConsolidationWorkspaceID || catalog.SystemUnfiledProjectID != "project-inbox" {
		t.Fatalf("activated catalog identity = workspace %q catalog %+v", workspaceID, catalog)
	}
	protected := findV6CatalogProject(t, catalog, catalog.SystemUnfiledProjectID)
	if protected.Name != V6UnfiledProjectName || protected.Slug != V6UnfiledProjectSlug {
		t.Fatalf("activated protected project = %+v", protected)
	}
	if _, statErr := os.Lstat(filepath.Join(root, "projects", "inbox")); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("legacy Inbox directory remains after activation: %v", statErr)
	}
	unfiledTasks := filepath.Join(root, "projects", V6UnfiledProjectSlug, "tasks.md")
	unfiledDocument, err := storage.ParseV6ProjectDocument(string(mustReadV6ActivationFile(t, unfiledTasks)))
	if err != nil || unfiledDocument.ProjectName != V6UnfiledProjectName {
		t.Fatalf("activated Unfiled document = %+v err=%v", unfiledDocument, err)
	}

	if got := mustReadV6ActivationFile(t, globalNotePath); !bytes.Equal(got, globalNote) {
		t.Fatal("global note bytes changed during schema-6 activation")
	}
	if got := mustReadV6ActivationFile(t, projectNotePath); !bytes.Equal(got, projectNote) {
		t.Fatal("project note bytes changed during schema-6 activation")
	}
	inboxNoteTarget := filepath.Join(root, "projects", V6UnfiledProjectSlug, "notes", "inbox-brief.md")
	if got := mustReadV6ActivationFile(t, inboxNoteTarget); !bytes.Equal(got, inboxNote) {
		t.Fatal("Inbox note bytes changed while moving to Unfiled")
	}
	attachmentTarget := filepath.Join(
		root,
		"projects",
		"alpha",
		".attachments",
		v6AttachmentTaskDirName("task-rich"),
		"specification.txt",
	)
	if got := mustReadV6ActivationFile(t, attachmentTarget); !bytes.Equal(got, attachment) {
		t.Fatal("attachment bytes changed during schema-6 activation")
	}
	if _, statErr := os.Lstat(attachmentSource); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("legacy raw attachment path remains after activation: %v", statErr)
	}
	if _, statErr := os.Lstat(UndoLedgerDir(root)); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("pre-v6 undo/redo ledger remains after activation: %v", statErr)
	}
	for _, path := range []string{
		WorkspaceCatalogPath(root) + ".bak",
		PlanningStatePath(root) + ".bak",
		filepath.Join(root, "projects", "alpha", "tasks.md.bak"),
		machinePath + ".bak",
	} {
		if _, statErr := os.Lstat(path); !errors.Is(statErr, os.ErrNotExist) {
			t.Errorf("pre-v6 backup remains live at %s: %v", path, statErr)
		}
	}

	pairs := preConsolidationV6Pairs(t, root)
	if len(pairs) != 1 {
		t.Fatalf("published pre-v6 snapshot pairs = %v, want one", pairs)
	}
	entries := readPreSimplificationV5Archive(t, pairs[0][0])
	for _, path := range []string{
		"workspace/.docket/undo/ledger.json",
		"workspace/projects/alpha/tasks.md",
		"workspace/projects/alpha/.attachments/task-rich/specification.txt",
	} {
		if _, found := entries[path]; !found {
			t.Errorf("rollback snapshot is missing %q", path)
		}
	}

	if err := manager.MigrateWorkspaceToV6(); err != nil {
		t.Fatalf("idempotent schema-6 activation: %v", err)
	}
	if repeated := preConsolidationV6Pairs(t, root); !reflect.DeepEqual(repeated, pairs) {
		t.Fatalf("idempotent activation changed snapshot pairs: before=%v after=%v", pairs, repeated)
	}
	if _, statErr := os.Stat(TransactionsDir(root)); statErr != nil {
		t.Fatalf("activation transaction directory is unavailable: %v", statErr)
	}
}

func TestManagerV6MigrationSnapshotPublicationFailureLeavesWorkspaceUnchanged(t *testing.T) {
	options := copyPreConsolidationV6Fixture(t)
	before := capturePreConsolidationV6WorkspaceBytes(t, options.DataDir)
	wantErr := errors.New("injected pre-publication failure")
	err := NewManager(ConfigPathForDataDir(options.DataDir)).migrateWorkspaceToV6(
		&v6MigrationActivationTestHooks{snapshot: &preConsolidationV6SnapshotHooks{
			now: preConsolidationV6TestTime,
			beforePublish: func(_, _ string) error {
				return wantErr
			},
		}},
	)
	if !errors.Is(err, wantErr) {
		t.Fatalf("pre-publication activation error = %v", err)
	}
	after := capturePreConsolidationV6WorkspaceBytes(t, options.DataDir)
	if !reflect.DeepEqual(after, before) {
		t.Fatalf("pre-publication activation failure changed workspace bytes\nbefore=%v\nafter=%v", before, after)
	}
	if pairs := preConsolidationV6Pairs(t, options.DataDir); len(pairs) != 0 {
		t.Fatalf("pre-publication failure left snapshot pairs: %v", pairs)
	}
}

func TestManagerV6MigrationDetectsStaleSourceAfterSnapshot(t *testing.T) {
	options := copyPreConsolidationV6Fixture(t)
	tasksPath := filepath.Join(options.DataDir, "projects", "alpha", "tasks.md")
	manager := NewManager(ConfigPathForDataDir(options.DataDir))
	err := manager.migrateWorkspaceToV6(&v6MigrationActivationTestHooks{
		snapshot: &preConsolidationV6SnapshotHooks{now: preConsolidationV6TestTime},
		afterSnapshotPublished: func(_ *PreConsolidationV6Snapshot) {
			data := mustReadV6ActivationFile(t, tasksPath)
			if writeErr := os.WriteFile(tasksPath, append(data, '\n'), FilePermissions); writeErr != nil {
				t.Fatal(writeErr)
			}
		},
	})
	if err == nil || !strings.Contains(err.Error(), "source changed while snapshotting") {
		t.Fatalf("stale-source activation error = %v", err)
	}
	schema, exists, schemaErr := workspaceCatalogPayloadSchema(options.DataDir)
	if schemaErr != nil || !exists || schema != V5ConfigSchemaVersion {
		t.Fatalf("stale-source refusal catalog schema = %d exists=%v err=%v", schema, exists, schemaErr)
	}
	pairs := preConsolidationV6Pairs(t, options.DataDir)
	if len(pairs) != 1 || !strings.Contains(err.Error(), pairs[0][0]) || !strings.Contains(err.Error(), pairs[0][1]) {
		t.Fatalf("stale-source error %q does not preserve and name one snapshot pair: %v", err, pairs)
	}
}

//nolint:govet // Scoped setup and recovery errors keep the interrupted transaction stages readable.
func TestManagerV6MigrationRecoversPreparedActivationBeforeInspection(t *testing.T) {
	options := copyPreConsolidationV6Fixture(t)
	plan, err := buildV6MigrationActivationPlan(V6WorkspaceDryRunOptions{DataDir: options.DataDir})
	if err != nil {
		t.Fatalf("build activation plan: %v", err)
	}
	if _, err := createPreConsolidationV6Snapshot(
		PreConsolidationV6SnapshotOptions{DataDir: options.DataDir},
		&preConsolidationV6SnapshotHooks{now: preConsolidationV6TestTime},
	); err != nil {
		t.Fatalf("create rollback snapshot: %v", err)
	}

	ops := make([]transaction.Op, 0, len(plan.artifacts)+len(plan.cleanupOps))
	for _, artifact := range plan.artifacts {
		ops = append(ops, transaction.Op{
			Kind: transaction.OpWrite, Target: artifact.Path, Data: artifact.Data, Mode: artifact.Mode,
		})
	}
	ops = append(ops, plan.cleanupOps...)
	journal := transaction.NewJournal(
		TransactionsDir(options.DataDir),
		[]string{options.DataDir, LocalDataRoot()}, "",
	)
	manifest, err := journal.Prepare(ops)
	if err != nil {
		t.Fatalf("prepare interrupted activation: %v", err)
	}
	for _, op := range ops {
		if op.Kind != transaction.OpMove {
			continue
		}
		if err := os.MkdirAll(filepath.Dir(op.Target), DirPermissions); err != nil {
			t.Fatalf("prepare interrupted activation move target: %v", err)
		}
	}

	manager := NewManager(ConfigPathForDataDir(options.DataDir))
	if err := manager.MigrateWorkspaceToV6(); err != nil {
		t.Fatalf("recover prepared schema-6 activation: %v", err)
	}
	schema, exists, err := workspaceCatalogPayloadSchema(options.DataDir)
	if err != nil || !exists || schema != V6ConfigSchemaVersion {
		t.Fatalf("recovered catalog schema = %d exists=%v err=%v", schema, exists, err)
	}
	if _, statErr := os.Stat(journal.ManifestPath(manifest.ID)); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("recovered activation manifest still exists: %v", statErr)
	}
	if _, statErr := os.Stat(filepath.Join(options.DataDir, "projects", V6UnfiledProjectSlug)); statErr != nil {
		t.Fatalf("recovered activation did not publish Unfiled: %v", statErr)
	}
}

//nolint:govet // Subtests intentionally reuse scoped errors for independent refusal paths.
func TestManagerV6MigrationRejectsCollisionsAndFutureSchemasBeforeSnapshot(t *testing.T) {
	t.Run("Unfiled collision", func(t *testing.T) {
		options := copyPreConsolidationV6Fixture(t)
		if err := os.MkdirAll(
			filepath.Join(options.DataDir, "projects", V6UnfiledProjectSlug),
			DirPermissions,
		); err != nil {
			t.Fatal(err)
		}
		err := NewManager(ConfigPathForDataDir(options.DataDir)).MigrateWorkspaceToV6()
		if err == nil || !strings.Contains(err.Error(), "filesystem target already exists") {
			t.Fatalf("Unfiled collision error = %v", err)
		}
		if pairs := preConsolidationV6Pairs(t, options.DataDir); len(pairs) != 0 {
			t.Fatalf("collision created rollback snapshot: %v", pairs)
		}
	})

	t.Run("future schema", func(t *testing.T) {
		options := copyPreConsolidationV6Fixture(t)
		path := WorkspaceCatalogPath(options.DataDir)
		data := mustReadV6ActivationFile(t, path)
		envelope, err := decodeEnvelope(data)
		if err != nil {
			t.Fatal(err)
		}
		var payload map[string]any
		if err := json.Unmarshal(envelope.Data, &payload); err != nil {
			t.Fatal(err)
		}
		payload["schema_version"] = float64(V6ConfigSchemaVersion + 1)
		envelope.Data, err = json.Marshal(payload)
		if err != nil {
			t.Fatal(err)
		}
		data, err = json.MarshalIndent(envelope, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, data, FilePermissions); err != nil {
			t.Fatal(err)
		}
		err = NewManager(ConfigPathForDataDir(options.DataDir)).MigrateWorkspaceToV6()
		if err == nil || !strings.Contains(err.Error(), "newer than supported schema 6") {
			t.Fatalf("future-schema activation error = %v", err)
		}
		if pairs := preConsolidationV6Pairs(t, options.DataDir); len(pairs) != 0 {
			t.Fatalf("future-schema refusal created rollback snapshot: %v", pairs)
		}
	})
}

func TestManagerV6MigrationPreservesSupportedV4ToV5ToV6Chain(t *testing.T) {
	root := copyLegacyV4ActivationFixture(t)
	manager := NewManager(ConfigPathForDataDir(root))
	if err := manager.MigrateWorkspaceToV6(); err != nil {
		t.Fatalf("migrate v4 through v5 to v6: %v", err)
	}
	schema, exists, err := workspaceCatalogPayloadSchema(root)
	if err != nil || !exists || schema != V6ConfigSchemaVersion {
		t.Fatalf("chained migration catalog schema = %d exists=%v err=%v", schema, exists, err)
	}
	if _, statErr := os.Stat(filepath.Join(root, "projects", V6UnfiledProjectSlug)); statErr != nil {
		t.Fatalf("chained migration did not publish Unfiled: %v", statErr)
	}
	if _, statErr := os.Lstat(filepath.Join(root, "projects", "inbox")); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("chained migration retained Inbox directory: %v", statErr)
	}
	obsoleteAttachment := filepath.Join(
		root,
		"projects",
		"alpha",
		".attachments",
		"94aeaa63349fb9ecfc9b52011f9eaf07c166cc1d999a00c5b9aad7b0152995a9",
	)
	if _, statErr := os.Lstat(obsoleteAttachment); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("chained migration retained obsolete attachment directory: %v", statErr)
	}
}

//nolint:govet // Restore-loop errors are scoped independently of snapshot creation.
func TestManagerV6MigrationRestoredSnapshotCanBeReactivated(t *testing.T) {
	options := copyPreConsolidationV6Fixture(t)
	report, err := createPreConsolidationV6Snapshot(options, &preConsolidationV6SnapshotHooks{
		now: preConsolidationV6TestTime,
	})
	if err != nil {
		t.Fatalf("create rollback snapshot: %v", err)
	}
	entries := readPreSimplificationV5Archive(t, report.ArchivePath)
	restoredRoot := filepath.Join(t.TempDir(), "restored-workspace")
	for archivePath, data := range entries {
		if !strings.HasPrefix(archivePath, "workspace/") {
			continue
		}
		rel := strings.TrimPrefix(archivePath, "workspace/")
		if rel == "" || rel == preConsolidationV6AuditEntry {
			continue
		}
		target := filepath.Join(restoredRoot, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(target), DirPermissions); err != nil {
			t.Fatalf("create restored parent for %s: %v", rel, err)
		}
		if err := os.WriteFile(target, data, FilePermissions); err != nil {
			t.Fatalf("restore %s: %v", rel, err)
		}
	}

	for _, rel := range []string{
		".docket/workspace.json",
		".docket/planning.json",
		".docket/global-notes/global-brief.md",
		".docket/undo/ledger.json",
		"projects/alpha/tasks.md",
		"projects/alpha/notes/project-brief.md",
		"projects/alpha/.attachments/task-rich/specification.txt",
	} {
		if _, statErr := os.Stat(filepath.Join(restoredRoot, filepath.FromSlash(rel))); statErr != nil {
			t.Fatalf("restored rollback artifact %s: %v", rel, statErr)
		}
	}

	manager := NewManager(ConfigPathForDataDir(restoredRoot))
	if err := manager.MigrateWorkspaceToV6(); err != nil {
		t.Fatalf("reactivate restored schema-5 snapshot: %v", err)
	}
	schema, exists, err := workspaceCatalogPayloadSchema(restoredRoot)
	if err != nil || !exists || schema != V6ConfigSchemaVersion {
		t.Fatalf("restored migration schema = %d exists=%v err=%v", schema, exists, err)
	}
	if pairs := preConsolidationV6Pairs(t, restoredRoot); len(pairs) != 1 {
		t.Fatalf("restored migration snapshot pairs = %v, want one fresh pair", pairs)
	}
	if _, statErr := os.Lstat(UndoLedgerDir(restoredRoot)); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("restored pre-v6 undo ledger survived activation: %v", statErr)
	}
	for _, rel := range []string{
		".docket/global-notes/global-brief.md",
		"projects/alpha/notes/project-brief.md",
		"projects/alpha/.attachments/" + v6AttachmentTaskDirName("task-rich") + "/specification.txt",
	} {
		if _, statErr := os.Stat(filepath.Join(restoredRoot, filepath.FromSlash(rel))); statErr != nil {
			t.Fatalf("reactivated restored artifact %s: %v", rel, statErr)
		}
	}
}

func readV6ActivationDomain(path, kind, workspaceID string, out any) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return decodeV5CandidateDomain(path, kind, workspaceID, data, out)
}

func mustReadV6ActivationFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return data
}

func preConsolidationV6Pairs(t *testing.T, root string) [][2]string {
	t.Helper()
	dir := PreConsolidationV6BackupsDir(root)
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
		switch {
		case strings.HasPrefix(name, preConsolidationV6SnapshotPrefix) && strings.HasSuffix(name, preConsolidationV6ArchiveSuffix):
			stem := strings.TrimSuffix(name, preConsolidationV6ArchiveSuffix)
			archives[stem] = filepath.Join(dir, name)
		case strings.HasPrefix(name, preConsolidationV6SnapshotPrefix) && strings.HasSuffix(name, preConsolidationV6AuditSuffix):
			stem := strings.TrimSuffix(name, preConsolidationV6AuditSuffix)
			audits[stem] = filepath.Join(dir, name)
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
