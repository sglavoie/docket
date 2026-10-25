package config

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/sglavoie/docket/internal/storage"
)

var preConsolidationV6TestTime = time.Date(2026, 8, 1, 18, 19, 20, 0, time.UTC)

type preConsolidationV6SnapshotHooks struct {
	now            time.Time
	afterInventory func()
	beforePublish  func(archivePath, auditPath string) error
}

func (h *preConsolidationV6SnapshotHooks) snapshotCreatedAt() time.Time {
	if h == nil {
		return time.Time{}
	}
	return h.now
}

func (h *preConsolidationV6SnapshotHooks) didInventorySnapshot() {
	if h != nil && h.afterInventory != nil {
		h.afterInventory()
	}
}

func (h *preConsolidationV6SnapshotHooks) willPublishSnapshot(archivePath, auditPath string) error {
	if h == nil || h.beforePublish == nil {
		return nil
	}
	return h.beforePublish(archivePath, auditPath)
}

func TestCreatePreConsolidationV6SnapshotCapturesCompleteWorkspaceAndAudit(t *testing.T) {
	options := copyPreConsolidationV6Fixture(t)
	report, err := createPreConsolidationV6Snapshot(options, &preConsolidationV6SnapshotHooks{
		now: preConsolidationV6TestTime,
	})
	if err != nil {
		t.Fatalf("CreatePreConsolidationV6Snapshot: %v", err)
	}
	wantStem := "pre-consolidation-v6-20260801T181920Z"
	if got := filepath.Base(report.ArchivePath); got != wantStem+preConsolidationV6ArchiveSuffix {
		t.Fatalf("archive name = %q, want canonical UTC name", got)
	}
	if got := filepath.Base(report.AuditPath); got != wantStem+preConsolidationV6AuditSuffix {
		t.Fatalf("audit name = %q, want canonical UTC name", got)
	}
	if len(report.ArchiveSHA256) != 64 {
		t.Fatalf("archive SHA-256 = %q", report.ArchiveSHA256)
	}

	entries := readPreSimplificationV5Archive(t, report.ArchivePath)
	wantPaths := []string{
		"workspace/.docket/workspace.json",
		"workspace/.docket/planning.json",
		"workspace/.docket/global-notes/global-brief.md",
		"workspace/.docket/transactions/source-state.snapshot",
		"workspace/.docket/undo/ledger.json",
		"workspace/projects/alpha/tasks.md",
		"workspace/projects/alpha/notes/project-brief.md",
		"workspace/projects/alpha/.attachments/task-rich/specification.txt",
		"machine/runtime/" + v5ConsolidationWorkspaceID + "/machine.json",
		"machine/runtime/" + v5ConsolidationWorkspaceID + "/machine.json.bak",
		preConsolidationV6AuditEntry,
	}
	for _, path := range wantPaths {
		if _, exists := entries[path]; !exists {
			t.Errorf("pre-v6 archive is missing %q", path)
		}
	}
	undoBlobCount := 0
	for path := range entries {
		if strings.HasPrefix(path, "workspace/.docket/undo/blobs/") {
			undoBlobCount++
		}
	}
	if undoBlobCount != 3 {
		t.Errorf("archived undo blobs = %d, want 3", undoBlobCount)
	}

	for _, file := range report.Files {
		data, exists := entries[file.Path]
		if !exists {
			t.Errorf("reported snapshot file %q is absent from archive", file.Path)
			continue
		}
		if got := storage.RawArtifactRevision(data); got != file.SHA256 {
			t.Errorf("reported hash for %q = %s, archive bytes hash to %s", file.Path, file.SHA256, got)
		}
	}
	audit, err := os.ReadFile(report.AuditPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(entries[preConsolidationV6AuditEntry], audit) {
		t.Fatal("archive audit and published audit sidecar differ")
	}
	for _, marker := range []string{
		"Schema-6 candidate validation: complete",
		"Transaction-state files: 1",
		"Undo-ledger files: 4",
		"Dependency references discarded: 2",
		"Recurrences discarded: 1",
		"Decisions converted to comments: 2",
		"`project-alpha` | `task-rich` | `depends_on[0]`",
		"`project-alpha` | `task-rich` | `recurrence`",
		"2 decisions",
		"## Planning repairs",
		"task-missing",
		"## Unfiled changes",
		"system_unfiled_project_id",
		"## Custom-context removals",
		"Deep Work",
		"| `next` | `backlog` | add to Next",
		"| 5 | 3 | Low |",
	} {
		if !strings.Contains(string(audit), marker) {
			t.Errorf("migration audit is missing %q", marker)
		}
	}
	if runtime.GOOS != "windows" {
		for _, path := range []string{report.ArchivePath, report.AuditPath} {
			info, err := os.Stat(path)
			if err != nil {
				t.Fatal(err)
			}
			if got := info.Mode().Perm(); got != preConsolidationV6Permissions {
				t.Errorf("%s mode = %o, want %o", filepath.Base(path), got, preConsolidationV6Permissions)
			}
		}
	}
}

func TestCreatePreConsolidationV6SnapshotIsByteDeterministic(t *testing.T) {
	firstOptions := copyPreConsolidationV6Fixture(t)
	first, err := createPreConsolidationV6Snapshot(firstOptions, &preConsolidationV6SnapshotHooks{now: preConsolidationV6TestTime})
	if err != nil {
		t.Fatal(err)
	}
	firstArchive, err := os.ReadFile(first.ArchivePath)
	if err != nil {
		t.Fatal(err)
	}
	firstAudit, err := os.ReadFile(first.AuditPath)
	if err != nil {
		t.Fatal(err)
	}

	secondOptions := copyPreConsolidationV6Fixture(t)
	second, err := createPreConsolidationV6Snapshot(secondOptions, &preConsolidationV6SnapshotHooks{now: preConsolidationV6TestTime})
	if err != nil {
		t.Fatal(err)
	}
	secondArchive, err := os.ReadFile(second.ArchivePath)
	if err != nil {
		t.Fatal(err)
	}
	secondAudit, err := os.ReadFile(second.AuditPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(firstArchive, secondArchive) || !bytes.Equal(firstAudit, secondAudit) {
		t.Fatal("identical schema-5 sources and timestamps produced different pre-v6 artifacts")
	}
	if first.ArchiveSHA256 != second.ArchiveSHA256 {
		t.Fatalf("archive hashes differ: %s != %s", first.ArchiveSHA256, second.ArchiveSHA256)
	}
}

func TestCreatePreConsolidationV6SnapshotValidationFailureLeavesWorkspaceUnchanged(t *testing.T) {
	options := copyPreConsolidationV6Fixture(t)
	attachment := filepath.Join(
		options.DataDir, "projects", "alpha", ".attachments", "task-rich", "specification.txt",
	)
	if err := os.Remove(attachment); err != nil {
		t.Fatal(err)
	}
	before := capturePreConsolidationV6WorkspaceBytes(t, options.DataDir)
	_, err := createPreConsolidationV6Snapshot(options, &preConsolidationV6SnapshotHooks{now: preConsolidationV6TestTime})
	if err == nil || !strings.Contains(err.Error(), "candidate validation failed") {
		t.Fatalf("validation error = %v", err)
	}
	after := capturePreConsolidationV6WorkspaceBytes(t, options.DataDir)
	if !reflect.DeepEqual(before, after) {
		t.Fatalf("candidate validation failure changed workspace bytes\nbefore=%v\nafter=%v", before, after)
	}
	assertNoPreConsolidationV6Pair(t, options.DataDir)
}

func TestCreatePreConsolidationV6SnapshotPublicationFailureLeavesWorkspaceUnchanged(t *testing.T) {
	options := copyPreConsolidationV6Fixture(t)
	before := capturePreConsolidationV6WorkspaceBytes(t, options.DataDir)
	wantErr := errors.New("injected snapshot build failure")
	_, err := createPreConsolidationV6Snapshot(options, &preConsolidationV6SnapshotHooks{
		now: preConsolidationV6TestTime,
		beforePublish: func(_, _ string) error {
			return wantErr
		},
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("publication error = %v, want injected failure", err)
	}
	after := capturePreConsolidationV6WorkspaceBytes(t, options.DataDir)
	if !reflect.DeepEqual(before, after) {
		t.Fatalf("pre-publication failure changed workspace bytes\nbefore=%v\nafter=%v", before, after)
	}
	assertNoPreConsolidationV6Pair(t, options.DataDir)
}

func TestCreatePreConsolidationV6SnapshotSourceDivergencePublishesNothing(t *testing.T) {
	options := copyPreConsolidationV6Fixture(t)
	tasksPath := filepath.Join(options.DataDir, "projects", "alpha", "tasks.md")
	_, err := createPreConsolidationV6Snapshot(options, &preConsolidationV6SnapshotHooks{
		now: preConsolidationV6TestTime,
		afterInventory: func() {
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
		t.Fatalf("source divergence error = %v", err)
	}
	assertNoPreConsolidationV6Pair(t, options.DataDir)
}

func TestPreConsolidationV6SnapshotCollisionAndMutationGuard(t *testing.T) {
	options := copyPreConsolidationV6Fixture(t)
	backupsDir := PreConsolidationV6BackupsDir(options.DataDir)
	if err := os.MkdirAll(backupsDir, DirPermissions); err != nil {
		t.Fatal(err)
	}
	protected := filepath.Join(backupsDir, "pre-consolidation-v6-20260801T181920Z.tar.gz")
	if err := os.WriteFile(protected, []byte("existing"), FilePermissions); err != nil {
		t.Fatal(err)
	}
	_, err := createPreConsolidationV6Snapshot(options, &preConsolidationV6SnapshotHooks{now: preConsolidationV6TestTime})
	if err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("collision error = %v", err)
	}
	data, err := os.ReadFile(protected)
	if err != nil || string(data) != "existing" {
		t.Fatalf("collision changed protected artifact: bytes=%q err=%v", data, err)
	}
	if !IsPreConsolidationV6SnapshotPath(options.DataDir, protected) {
		t.Fatal("canonical pre-v6 snapshot path was not recognized")
	}
	if err := RefusePreConsolidationV6SnapshotMutation(options.DataDir, protected); !errors.Is(err, ErrImmutablePreConsolidationV6Snapshot) {
		t.Fatalf("mutation guard error = %v", err)
	}
}

func copyPreConsolidationV6Fixture(t *testing.T) PreConsolidationV6SnapshotOptions {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	root := filepath.Join(t.TempDir(), "workspace")
	if err := os.CopyFS(root, os.DirFS(filepath.Join(v5ConsolidationConfigFixtureRoot, "workspace"))); err != nil {
		t.Fatalf("copy v5 consolidation workspace: %v", err)
	}
	machineData, err := os.ReadFile(filepath.Join(
		v5ConsolidationConfigFixtureRoot,
		"machine", "runtime", v5ConsolidationWorkspaceID, machineStateFileName,
	))
	if err != nil {
		t.Fatal(err)
	}
	machinePath, err := MachineStatePath(v5ConsolidationWorkspaceID)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(machinePath), DirPermissions); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(machinePath, machineData, FilePermissions); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(machinePath+".bak", append([]byte(nil), machineData...), FilePermissions); err != nil {
		t.Fatal(err)
	}
	writeFileTree(t, root, map[string]string{
		".docket/transactions/commit.lock":           "",
		".docket/transactions/source-state.snapshot": "prepared migration boundary\n",
	})
	return PreConsolidationV6SnapshotOptions{DataDir: root}
}

func capturePreConsolidationV6WorkspaceBytes(t *testing.T, root string) map[string][]byte {
	t.Helper()
	files := make(map[string][]byte)
	if err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		files[filepath.ToSlash(rel)] = data
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return files
}

func assertNoPreConsolidationV6Pair(t *testing.T, root string) {
	t.Helper()
	stem := filepath.Join(
		PreConsolidationV6BackupsDir(root),
		"pre-consolidation-v6-20260801T181920Z",
	)
	for _, suffix := range []string{preConsolidationV6ArchiveSuffix, preConsolidationV6AuditSuffix} {
		if _, err := os.Stat(stem + suffix); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("failed snapshot left partial artifact %s: %v", stem+suffix, err)
		}
	}
}
