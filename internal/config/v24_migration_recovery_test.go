package config

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"
)

var errSimulatedV24Crash = errors.New("simulated crash")

type v24LayoutTestHooks struct {
	afterSnapshotPublished func(archivePath, auditPath string)
	stopAt                 v24LayoutStep
}

func waitForNextSecond() {
	start := time.Now().Unix()
	for time.Now().Unix() == start {
		time.Sleep(10 * time.Millisecond)
	}
}

func (h *v24LayoutTestHooks) didPublishSnapshot(archivePath, auditPath string) {
	if h.afterSnapshotPublished != nil {
		h.afterSnapshotPublished(archivePath, auditPath)
	}
}

func (h *v24LayoutTestHooks) interrupt(step v24LayoutStep) error {
	if step == h.stopAt {
		return errSimulatedV24Crash
	}
	return nil
}

// legacyV24Workspace is a schema-23 workspace laid out the way every build
// before schema 24 wrote it: internal state under .tt/, with an undo ledger, a
// conflict bundle, and a device's sync history beside the domains.
func legacyV24Workspace(t *testing.T) string {
	t.Helper()
	root := copyMigratedFixture(t, 21)
	if err := NewManager(ConfigPathForDataDir(root)).MigrateWorkspaceToV23(); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(LegacyInternalDir(root)); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(InternalDir(root), LegacyInternalDir(root)); err != nil {
		t.Fatal(err)
	}
	for rel, data := range map[string]string{
		"undo/ledger.json":                  `{"schema_version":1}`,
		"conflicts/bundle-1/conflict.json":  `{"recorded":"frozen bytes"}`,
		"sync/devices/desk/state.json":      `{"format":"tt-sync-state"}`,
		"global-notes/2026-10-25-review.md": "# Review\n",
	} {
		path := filepath.Join(LegacyInternalDir(root), filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), DirPermissions); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(data), FilePermissions); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

// v24Tree lists every regular file under dir with its bytes, by slash path.
func v24Tree(t *testing.T, dir string) map[string]string {
	t.Helper()
	tree := map[string]string{}
	err := filepath.WalkDir(dir, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil || !entry.Type().IsRegular() {
			return walkErr
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(dir, path)
		tree[filepath.ToSlash(rel)] = string(data)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return tree
}

// requireV24Activated holds a recovered workspace to the finished layout: the
// legacy bytes under .docket/ at schema 24, no undo ledger, one archive/audit
// pair, the three-file tombstone at .tt/, and no stage or retired directory.
func requireV24Activated(t *testing.T, root string, legacy map[string]string) {
	t.Helper()
	schema, exists, err := workspaceCatalogPayloadSchema(root)
	if err != nil || !exists || schema != V24ConfigSchemaVersion {
		t.Fatalf("catalog schema = %d (exists %v, err %v), want %d", schema, exists, err, V24ConfigSchemaVersion)
	}
	moved := v24Tree(t, InternalDir(root))
	for rel, data := range legacy {
		top := strings.SplitN(rel, "/", 2)[0]
		switch {
		case top == "undo":
			if _, ok := moved[rel]; ok {
				t.Errorf("undo entry %s rode the move", rel)
			}
		case rel == "workspace.json", top == "backups", top == "transactions", strings.HasSuffix(rel, ".lock"):
		default:
			if moved[rel] != data {
				t.Errorf(".docket/%s = %q, want the pre-v24 bytes %q", rel, moved[rel], data)
			}
		}
	}
	var pair []string
	for rel := range moved {
		if strings.HasPrefix(rel, "backups/"+preDocketLayoutV24SnapshotPrefix) {
			pair = append(pair, rel)
		}
	}
	sort.Strings(pair)
	if len(pair)%2 != 0 || len(pair) == 0 || !strings.HasSuffix(pair[0], preScopeV7AuditSuffix) || !strings.HasSuffix(pair[1], preScopeV7ArchiveSuffix) {
		t.Errorf("v24 archive/audit pair = %v", pair)
	}
	tombstone := v24Tree(t, LegacyInternalDir(root))
	if len(tombstone) != 3 || tombstone["planning.json"] != v24TombstoneNote || tombstone["transactions/commit.lock"] != "" {
		t.Errorf("tombstone = %v", tombstone)
	}
	if state, inspectErr := inspectV24Legacy(root); inspectErr != nil || state != v24LegacyTombstone {
		t.Errorf("legacy state = %v, %v; want the tombstone", state, inspectErr)
	}
	for _, leftover := range []string{v24StageDirName, v24TombstoneStageDirName, v24RetiredDirName} {
		if _, statErr := os.Lstat(filepath.Join(root, leftover)); !errors.Is(statErr, fs.ErrNotExist) {
			t.Errorf("%s is still present: %v", leftover, statErr)
		}
	}
}

// TestManagerV24ActivationMovesLegacyStateAndLeavesATombstone is the whole
// boundary through the entry point every frontend takes.
func TestManagerV24ActivationMovesLegacyStateAndLeavesATombstone(t *testing.T) {
	root := legacyV24Workspace(t)
	legacy := v24Tree(t, LegacyInternalDir(root))
	if err := NewManager(ConfigPathForDataDir(root)).RecoverWorkspace(); err != nil {
		t.Fatalf("recover: %v", err)
	}
	requireV24Activated(t, root, legacy)
	audit := ""
	for rel, data := range v24Tree(t, PreScopeV7BackupsDir(root)) {
		if strings.HasSuffix(rel, preScopeV7AuditSuffix) && strings.HasPrefix(rel, preDocketLayoutV24SnapshotPrefix) {
			audit = data
		}
	}
	for _, want := range []string{"Source catalog schema: 23", "tombstone", "Undo/redo history is dropped"} {
		if !strings.Contains(audit, want) {
			t.Errorf("audit does not state %q:\n%s", want, audit)
		}
	}

	catalog, err := os.ReadFile(WorkspaceCatalogPath(root))
	if err != nil {
		t.Fatal(err)
	}
	if err := NewManager(ConfigPathForDataDir(root)).RecoverWorkspace(); err != nil {
		t.Fatalf("second recover: %v", err)
	}
	if again, _ := os.ReadFile(WorkspaceCatalogPath(root)); !bytes.Equal(again, catalog) {
		t.Fatal("a second recovery rewrote the schema-24 catalog")
	}
	requireV24Activated(t, root, legacy)
}

// TestManagerV24RecoveryFinishesOrRollsBackFromEveryCrashPoint interrupts the
// activation after each durable step and requires the next opening to land on
// the same finished layout: before the commit point by discarding the stage,
// after it by finishing forward.
func TestManagerV24RecoveryFinishesOrRollsBackFromEveryCrashPoint(t *testing.T) {
	for _, step := range []v24LayoutStep{v24StepStaged, v24StepActivated, v24StepLegacyRetired, v24StepTombstonePlaced} {
		t.Run(string(step), func(t *testing.T) {
			root := legacyV24Workspace(t)
			legacy := v24Tree(t, LegacyInternalDir(root))
			manager := NewManager(ConfigPathForDataDir(root))
			if err := manager.activateV24Layout(&v24LayoutTestHooks{stopAt: step}); !errors.Is(err, errSimulatedV24Crash) {
				t.Fatalf("interrupted activation error = %v", err)
			}
			if step == v24StepStaged {
				if dirExists(InternalDir(root)) {
					t.Fatal("a crash before the commit point left .docket/ in place")
				}
				if got := v24Tree(t, LegacyInternalDir(root)); got["workspace.json"] != legacy["workspace.json"] {
					t.Fatal("a crash before the commit point changed .tt/")
				}
			}
			if step == v24StepStaged {
				// The retry publishes a second archive/audit pair, which a
				// crash in the same second would collide with.
				waitForNextSecond()
			}
			if err := NewManager(ConfigPathForDataDir(root)).RecoverWorkspace(); err != nil {
				t.Fatalf("recover after %s: %v", step, err)
			}
			requireV24Activated(t, root, legacy)
		})
	}
}

// TestManagerV24RecoveryWritesAMissingTombstone covers a .docket/ whose .tt/ is
// gone entirely, which an interrupted retirement or a person deleting the
// tombstone both leave.
func TestManagerV24RecoveryWritesAMissingTombstone(t *testing.T) {
	root := legacyV24Workspace(t)
	legacy := v24Tree(t, LegacyInternalDir(root))
	if err := NewManager(ConfigPathForDataDir(root)).RecoverWorkspace(); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(LegacyInternalDir(root)); err != nil {
		t.Fatal(err)
	}
	if err := NewManager(ConfigPathForDataDir(root)).RecoverWorkspace(); err != nil {
		t.Fatal(err)
	}
	requireV24Activated(t, root, legacy)
}

// TestManagerV24KeepsALiveLegacyDirectoryBesideAnActivatedOne keeps rather than
// deletes a .tt/ that reappeared beside .docket/: it may hold writes an older
// build made while the move was syncing.
func TestManagerV24KeepsALiveLegacyDirectoryBesideAnActivatedOne(t *testing.T) {
	root := legacyV24Workspace(t)
	if err := NewManager(ConfigPathForDataDir(root)).RecoverWorkspace(); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(LegacyInternalDir(root)); err != nil {
		t.Fatal(err)
	}
	stray := filepath.Join(LegacyInternalDir(root), "global-notes", "written-by-an-older-build.md")
	if err := os.MkdirAll(filepath.Dir(stray), DirPermissions); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(stray, []byte("late note\n"), FilePermissions); err != nil {
		t.Fatal(err)
	}
	if err := NewManager(ConfigPathForDataDir(root)).RecoverWorkspace(); err != nil {
		t.Fatal(err)
	}
	kept := ""
	for rel, data := range v24Tree(t, PreScopeV7BackupsDir(root)) {
		if strings.HasPrefix(rel, v24KeptDirPrefix) && strings.HasSuffix(rel, "written-by-an-older-build.md") {
			kept = data
		}
	}
	if kept != "late note\n" {
		t.Fatal("the live .tt/ beside .docket/ was not kept under backups")
	}
	if state, _ := inspectV24Legacy(root); state != v24LegacyTombstone {
		t.Fatal("the tombstone did not replace the live .tt/")
	}
}

// TestManagerV24RefusesALoneTombstone is a workspace another machine moved
// whose .docket/ has not arrived: nothing is created, and the refusal names it.
func TestManagerV24RefusesALoneTombstone(t *testing.T) {
	root := legacyV24Workspace(t)
	if err := NewManager(ConfigPathForDataDir(root)).RecoverWorkspace(); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(InternalDir(root)); err != nil {
		t.Fatal(err)
	}
	before := v24Tree(t, root)
	err := NewManager(ConfigPathForDataDir(root)).RecoverWorkspace()
	var moved *WorkspaceMovedElsewhereError
	if !errors.As(err, &moved) {
		t.Fatalf("lone tombstone error = %v, want WorkspaceMovedElsewhereError", err)
	}
	if _, _, schemaErr := WorkspaceCatalogSchema(root); !errors.As(schemaErr, &moved) {
		t.Fatalf("WorkspaceCatalogSchema error = %v, want WorkspaceMovedElsewhereError", schemaErr)
	}
	if _, loadErr := NewManager(ConfigPathForDataDir(root)).LoadDomains(); !errors.As(loadErr, &moved) {
		t.Fatalf("LoadDomains error = %v, want WorkspaceMovedElsewhereError", loadErr)
	}
	if after := v24Tree(t, root); len(after) != len(before) || dirExists(InternalDir(root)) {
		t.Fatal("refusing a lone tombstone wrote to the workspace")
	}
}

// TestManagerV24ReportsTheLegacySchemaBeforeMigrating keeps the read-only
// schema probe honest for a frontend that refuses to migrate.
func TestManagerV24ReportsTheLegacySchemaBeforeMigrating(t *testing.T) {
	root := legacyV24Workspace(t)
	schema, exists, err := WorkspaceCatalogSchema(root)
	if err != nil || !exists || schema != V23ConfigSchemaVersion {
		t.Fatalf("legacy schema = %d, %v, %v; want 23", schema, exists, err)
	}
	if dirExists(InternalDir(root)) {
		t.Fatal("probing the schema moved the workspace")
	}
}

// TestManagerV24AbortsOnWorkspaceMutationAfterSnapshot proves the activation
// guard refuses a candidate the rollback archive does not describe.
func TestManagerV24AbortsOnWorkspaceMutationAfterSnapshot(t *testing.T) {
	root := legacyV24Workspace(t)
	note := filepath.Join(LegacyInternalDir(root), "global-notes", "2026-10-25-review.md")
	err := NewManager(ConfigPathForDataDir(root)).activateV24Layout(&v24LayoutTestHooks{
		afterSnapshotPublished: func(string, string) {
			if writeErr := os.WriteFile(note, []byte("# Edited after the snapshot\n"), FilePermissions); writeErr != nil {
				t.Fatal(writeErr)
			}
		},
	})
	if err == nil || !strings.Contains(err.Error(), "changed after the rollback snapshot") {
		t.Fatalf("mutated activation error = %v", err)
	}
	if dirExists(InternalDir(root)) || dirExists(filepath.Join(root, v24StageDirName)) {
		t.Fatal("an aborted activation left .docket/ or its stage")
	}
}

// TestManagerV24ValidationFailurePublishesNothing refuses a symlink under .tt/
// before any archive is written.
func TestManagerV24ValidationFailurePublishesNothing(t *testing.T) {
	root := legacyV24Workspace(t)
	if err := os.Symlink(filepath.Join(root, "projects"), filepath.Join(LegacyInternalDir(root), "linked")); err != nil {
		t.Fatal(err)
	}
	before := v24Tree(t, LegacyInternalDir(root))
	err := NewManager(ConfigPathForDataDir(root)).ActivateV24Layout()
	if err == nil || !strings.Contains(err.Error(), "workspace was not changed") {
		t.Fatalf("symlink activation error = %v", err)
	}
	// The activation lock is coordination, not an artifact.
	after := v24Tree(t, LegacyInternalDir(root))
	delete(after, v24ActivationLockName)
	if len(after) != len(before) {
		t.Fatalf("validation failure published artifacts: %d files before, %d after", len(before), len(after))
	}
}

// TestManagerV24RefusesToMoveABlockedJournal refuses to move a journal recovery
// cannot empty: the manifests name .tt/ paths, so they cannot ride the move.
func TestManagerV24RefusesToMoveABlockedJournal(t *testing.T) {
	root := legacyV24Workspace(t)
	transactions := filepath.Join(LegacyInternalDir(root), "transactions")
	if err := os.MkdirAll(transactions, DirPermissions); err != nil {
		t.Fatal(err)
	}
	manifest := filepath.Join(transactions, "00000000-0000-4000-8000-000000000001.json")
	if err := os.WriteFile(manifest, []byte("{not a manifest"), FilePermissions); err != nil {
		t.Fatal(err)
	}
	err := NewManager(ConfigPathForDataDir(root)).ActivateV24Layout()
	if err == nil {
		t.Fatal("activation moved a journal holding an unreadable transaction")
	}
	if dirExists(InternalDir(root)) {
		t.Fatal("a refused activation created .docket/")
	}
}

// TestManagerV24ConcurrentOpenersMoveOnce races two openers over one pre-v24
// workspace: one moves it and the other waits, then settles what it finds.
func TestManagerV24ConcurrentOpenersMoveOnce(t *testing.T) {
	root := legacyV24Workspace(t)
	legacy := v24Tree(t, LegacyInternalDir(root))
	errs := make(chan error, 2)
	for range 2 {
		go func() { errs <- NewManager(ConfigPathForDataDir(root)).RecoverWorkspace() }()
	}
	for range 2 {
		if err := <-errs; err != nil {
			t.Fatalf("concurrent recover: %v", err)
		}
	}
	requireV24Activated(t, root, legacy)
}
