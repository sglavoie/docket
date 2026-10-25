package config

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/sglavoie/docket/internal/storage"
)

var preSimplificationV5TestTime = time.Date(2026, 7, 29, 1, 2, 3, 0, time.UTC)

type preSimplificationV5SnapshotHooks struct {
	now            time.Time
	afterInventory func()
	beforePublish  func(archivePath, auditPath string)
}

func (h *preSimplificationV5SnapshotHooks) snapshotCreatedAt() time.Time {
	if h == nil {
		return time.Time{}
	}
	return h.now
}

func (h *preSimplificationV5SnapshotHooks) didInventorySnapshot() {
	if h != nil && h.afterInventory != nil {
		h.afterInventory()
	}
}

func (h *preSimplificationV5SnapshotHooks) willPublishSnapshot(archivePath, auditPath string) {
	if h != nil && h.beforePublish != nil {
		h.beforePublish(archivePath, auditPath)
	}
}

func TestInspectPreSimplificationV5SnapshotsValidatesCanonicalPairs(t *testing.T) {
	t.Run("complete pair", func(t *testing.T) {
		root := t.TempDir()
		dir := PreSimplificationV5BackupsDir(root)
		if err := os.MkdirAll(dir, DirPermissions); err != nil {
			t.Fatalf("create backups dir: %v", err)
		}
		stem := filepath.Join(dir, "pre-simplification-v5-20260729T010203Z")
		for path, content := range map[string]string{
			stem + ".tar.gz":   "archive",
			stem + ".audit.md": "audit",
		} {
			if err := os.WriteFile(path, []byte(content), preSimplificationV5Permissions); err != nil {
				t.Fatalf("write snapshot artifact: %v", err)
			}
		}
		if err := os.WriteFile(filepath.Join(dir, "ordinary.txt"), []byte("ignored"), FilePermissions); err != nil {
			t.Fatalf("write ordinary backup file: %v", err)
		}

		got, err := InspectPreSimplificationV5Snapshots(root)
		if err != nil {
			t.Fatalf("inspect snapshots: %v", err)
		}
		if !got.HasArtifacts || len(got.Pairs) != 1 || len(got.Issues) != 0 {
			t.Fatalf("snapshot inspection = %+v", got)
		}
		if !got.Pairs[0].CreatedAt.Equal(preSimplificationV5TestTime) {
			t.Fatalf("pair timestamp = %s, want %s", got.Pairs[0].CreatedAt, preSimplificationV5TestTime)
		}
	})

	t.Run("malformed evidence", func(t *testing.T) {
		root := t.TempDir()
		dir := PreSimplificationV5BackupsDir(root)
		if err := os.MkdirAll(dir, DirPermissions); err != nil {
			t.Fatalf("create backups dir: %v", err)
		}
		if err := os.WriteFile(filepath.Join(dir, "pre-simplification-v5-not-a-time.tar.gz"), []byte("archive"), FilePermissions); err != nil {
			t.Fatalf("write malformed artifact: %v", err)
		}

		got, err := InspectPreSimplificationV5Snapshots(root)
		if err != nil {
			t.Fatalf("inspect malformed snapshots: %v", err)
		}
		if !got.HasArtifacts || len(got.Pairs) != 0 || len(got.Issues) != 1 ||
			!strings.Contains(got.Issues[0], "malformed") {
			t.Fatalf("malformed snapshot inspection = %+v", got)
		}
	})
}

func TestCreatePreSimplificationV5SnapshotCapturesRollbackDataAndInventoriesAttachments(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	root := filepath.Join(t.TempDir(), "workspace")
	seedPreSimplificationV5Workspace(t, root, testWorkspaceIDA)

	report, err := createPreSimplificationV5Snapshot(root, &preSimplificationV5SnapshotHooks{
		now: preSimplificationV5TestTime,
	})
	if err != nil {
		t.Fatalf("CreatePreSimplificationV5Snapshot: %v", err)
	}
	wantStem := "pre-simplification-v5-20260729T010203Z"
	if got := filepath.Base(report.ArchivePath); got != wantStem+".tar.gz" {
		t.Fatalf("archive name = %q, want timestamped canonical name", got)
	}
	if got := filepath.Base(report.AuditPath); got != wantStem+".audit.md" {
		t.Fatalf("audit name = %q, want timestamped canonical name", got)
	}
	if len(report.ArchiveSHA256) != 64 {
		t.Fatalf("archive SHA-256 = %q", report.ArchiveSHA256)
	}

	entries := readPreSimplificationV5Archive(t, report.ArchivePath)
	for path, want := range map[string]string{
		"workspace/.tt/workspace.json":                          `"kind":"workspace"`,
		"workspace/.tt/planning.json":                           `"kind":"planning"`,
		"workspace/.tt/settings.json":                           `"kind":"settings"`,
		"workspace/.tt/automation.json":                         `"kind":"automation"`,
		"workspace/.tt/global-notes/global.md":                  "Global note",
		"workspace/.tt/trash/deleted/data/tasks.md":             "Deleted task",
		"workspace/projects/alpha/tasks.md":                     "Active task",
		"workspace/projects/alpha/completed.md":                 "Archived task",
		"workspace/projects/alpha/.completed.index.json":        `"version":2`,
		"workspace/projects/alpha/blockers.md":                  "Legacy blocker",
		"workspace/projects/alpha/notes/project.md":             "Project note",
		"workspace/themes/custom.json":                          `"name":"custom"`,
		"machine/runtime/" + testWorkspaceIDA + "/machine.json": `"kind":"machine"`,
	} {
		got, ok := entries[path]
		if !ok || !strings.Contains(string(got), want) {
			t.Errorf("archive entry %q = %q, present=%v; want content containing %q", path, got, ok, want)
		}
	}
	attachmentPath := "workspace/projects/alpha/.attachments/task-hash/specification.txt"
	if _, ok := entries[attachmentPath]; ok {
		t.Fatalf("attachment %q was copied into archive; it must remain in place", attachmentPath)
	}
	if _, statErr := os.Stat(filepath.Join(root, "projects", "alpha", ".attachments", "task-hash", "specification.txt")); statErr != nil {
		t.Fatalf("retained attachment disappeared: %v", statErr)
	}
	if _, ok := entries["workspace/logs/tt.log"]; ok {
		t.Fatal("machine-local log was copied into rollback archive")
	}
	if _, ok := entries["workspace/.tt/backups/older.tar.gz"]; ok {
		t.Fatal("an existing backup was recursively copied into the new archive")
	}

	audit, err := os.ReadFile(report.AuditPath)
	if err != nil {
		t.Fatalf("read audit report: %v", err)
	}
	if internal := entries[preSimplificationV5AuditEntry]; !bytes.Equal(internal, audit) {
		t.Fatal("archive audit and atomically published audit sidecar differ")
	}
	auditText := string(audit)
	for _, want := range []string{
		"Created at: `2026-07-29T01:02:03Z`",
		"Attachment binaries retained in place",
		attachmentPath,
		"workspace/projects/alpha/completed.md",
		"machine/runtime/" + testWorkspaceIDA + "/machine.json",
	} {
		if !strings.Contains(auditText, want) {
			t.Errorf("audit report is missing %q:\n%s", want, auditText)
		}
	}
	if len(report.Attachments) != 1 || report.Attachments[0].Path != attachmentPath {
		t.Fatalf("attachment inventory = %+v", report.Attachments)
	}
	for _, file := range report.ArchivedFiles {
		data, ok := entries[file.Path]
		if !ok {
			t.Errorf("reported archived file %q is absent from archive", file.Path)
			continue
		}
		if got := storage.RawArtifactRevision(data); got != file.SHA256 {
			t.Errorf("reported checksum for %q = %s, archive bytes hash to %s", file.Path, file.SHA256, got)
		}
	}

	if runtime.GOOS != "windows" {
		for _, path := range []string{report.ArchivePath, report.AuditPath} {
			info, err := os.Stat(path)
			if err != nil {
				t.Fatalf("stat immutable snapshot artifact: %v", err)
			}
			if got := info.Mode().Perm(); got != preSimplificationV5Permissions {
				t.Errorf("%s mode = %o, want %o", filepath.Base(path), got, preSimplificationV5Permissions)
			}
		}
	}
}

func TestCreatePreSimplificationV5SnapshotCoversLegacyV4FixtureCorpus(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	root := filepath.Join(t.TempDir(), "workspace")
	fixtureWorkspace := filepath.Join(legacyV4FixtureRoot, "workspace")
	if err := os.CopyFS(root, os.DirFS(fixtureWorkspace)); err != nil {
		t.Fatalf("copy legacy v4 fixture workspace: %v", err)
	}
	machineFixture, err := os.ReadFile(filepath.Join(
		legacyV4FixtureRoot,
		"machine",
		"runtime",
		legacyV4WorkspaceID,
		"machine.json",
	))
	if err != nil {
		t.Fatalf("read legacy v4 machine fixture: %v", err)
	}
	machinePath, err := MachineStatePath(legacyV4WorkspaceID)
	if err != nil {
		t.Fatal(err)
	}
	if mkdirErr := os.MkdirAll(filepath.Dir(machinePath), DirPermissions); mkdirErr != nil {
		t.Fatal(mkdirErr)
	}
	if writeErr := os.WriteFile(machinePath, machineFixture, FilePermissions); writeErr != nil {
		t.Fatal(writeErr)
	}

	report, err := createPreSimplificationV5Snapshot(root, &preSimplificationV5SnapshotHooks{now: preSimplificationV5TestTime})
	if err != nil {
		t.Fatalf("snapshot legacy v4 fixture: %v", err)
	}
	entries := readPreSimplificationV5Archive(t, report.ArchivePath)
	for _, path := range []string{
		"workspace/.tt/automation.json",
		"workspace/.tt/global-notes/global-reference.md",
		"workspace/.tt/trash/project-deleted__1769947200000000000/data/tasks.md",
		"workspace/.tt/trash/project-deleted__1769947200000000000/tombstone.json",
		"workspace/projects/_sandbox/tasks.md",
		"workspace/projects/alpha/.completed.index.json",
		"workspace/projects/alpha/blockers.md",
		"workspace/projects/alpha/completed.md",
		"workspace/projects/alpha/notes/project-plan.md",
		"workspace/projects/alpha/tasks.md",
		"workspace/projects/beta/completed.md",
		"workspace/projects/internal-fixture/tasks.md",
		"machine/runtime/" + legacyV4WorkspaceID + "/machine.json",
	} {
		if _, ok := entries[path]; !ok {
			t.Errorf("legacy v4 rollback archive is missing %q", path)
		}
	}
	if len(report.Attachments) != 2 {
		t.Fatalf("legacy v4 attachment inventory count = %d, want 2: %+v", len(report.Attachments), report.Attachments)
	}
	for _, attachment := range report.Attachments {
		if _, ok := entries[attachment.Path]; ok {
			t.Errorf("legacy v4 attachment %q was copied instead of retained in place", attachment.Path)
		}
		if attachment.Size == 0 || len(attachment.SHA256) != 64 {
			t.Errorf("legacy v4 attachment inventory is incomplete: %+v", attachment)
		}
	}
}
func seedPreSimplificationV5Workspace(t *testing.T, root, workspaceID string) {
	t.Helper()
	writeFileTree(t, root, map[string]string{
		".tt/workspace.json":                                      workspaceEnvelopeFixture(workspaceID, "workspace"),
		".tt/planning.json":                                       workspaceEnvelopeFixture(workspaceID, "planning"),
		".tt/settings.json":                                       workspaceEnvelopeFixture(workspaceID, "settings"),
		".tt/automation.json":                                     workspaceEnvelopeFixture(workspaceID, "automation"),
		".tt/global-notes/global.md":                              "Global note\n",
		".tt/trash/deleted/data/tasks.md":                         "Deleted task\n",
		".tt/backups/older.tar.gz":                                "older snapshot",
		"projects/alpha/tasks.md":                                 "Active task\n",
		"projects/alpha/completed.md":                             "Archived task\n",
		"projects/alpha/.completed.index.json":                    `{"version":2}`,
		"projects/alpha/blockers.md":                              "Legacy blocker\n",
		"projects/alpha/notes/project.md":                         "Project note\n",
		"projects/alpha/.attachments/task-hash/specification.txt": "Attachment bytes\n",
		"themes/custom.json":                                      `{"name":"custom"}`,
		"logs/tt.log":                                             "runtime log",
	})
	machinePath, err := MachineStatePath(workspaceID)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(machinePath), DirPermissions); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(machinePath, []byte(workspaceEnvelopeFixture(workspaceID, "machine")), FilePermissions); err != nil {
		t.Fatal(err)
	}
}

func seedDeterministicPreSimplificationV5Workspace(t *testing.T, root string) {
	t.Helper()
	writeFileTree(t, root, map[string]string{
		"projects/alpha/tasks.md": "Active task\n",
		".tt/settings.json":       `{"stable":true}`,
	})
}

func workspaceEnvelopeFixture(workspaceID, kind string) string {
	return `{"schema_version":1,"workspace_id":"` + workspaceID + `","kind":"` + kind + `","data":{}}`
}

func readPreSimplificationV5Archive(t *testing.T, path string) map[string][]byte {
	t.Helper()
	file, err := os.Open(path)
	if err != nil {
		t.Fatalf("open snapshot archive: %v", err)
	}
	defer func() { _ = file.Close() }()
	gzipReader, err := gzip.NewReader(file)
	if err != nil {
		t.Fatalf("open snapshot gzip: %v", err)
	}
	defer func() { _ = gzipReader.Close() }()
	tarReader := tar.NewReader(gzipReader)
	entries := make(map[string][]byte)
	for {
		header, err := tarReader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatalf("read snapshot tar: %v", err)
		}
		data, err := io.ReadAll(tarReader)
		if err != nil {
			t.Fatalf("read snapshot entry %s: %v", header.Name, err)
		}
		if _, exists := entries[header.Name]; exists {
			t.Fatalf("duplicate snapshot entry %q", header.Name)
		}
		entries[header.Name] = data
	}
	return entries
}

func assertNoPreSimplificationV5Pair(t *testing.T, root string) {
	t.Helper()
	stem := filepath.Join(
		PreSimplificationV5BackupsDir(root),
		"pre-simplification-v5-20260729T010203Z",
	)
	for _, suffix := range []string{".tar.gz", ".audit.md"} {
		if _, err := os.Stat(stem + suffix); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("failed snapshot left partial artifact %s: %v", stem+suffix, err)
		}
	}
}
