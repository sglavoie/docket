package config

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/sglavoie/docket/internal/storage"
	"github.com/sglavoie/docket/internal/util"
)

type workspaceMoveTestHooks struct {
	beforeWorkspaceLocks     func()
	afterInventory           func()
	afterDestinationPrecheck func()
	afterCopy                func()
}

func (h *workspaceMoveTestHooks) beforeWorkspaceLocksAcquired() {
	if h != nil && h.beforeWorkspaceLocks != nil {
		h.beforeWorkspaceLocks()
	}
}

func (h *workspaceMoveTestHooks) didInventoryWorkspace() {
	if h != nil && h.afterInventory != nil {
		h.afterInventory()
	}
}

func (h *workspaceMoveTestHooks) didPrecheckDestination() {
	if h != nil && h.afterDestinationPrecheck != nil {
		h.afterDestinationPrecheck()
	}
}

func (h *workspaceMoveTestHooks) didCopyWorkspace() {
	if h != nil && h.afterCopy != nil {
		h.afterCopy()
	}
}

// readActivePointerDir returns the data dir recorded in the machine-local
// active-workspace pointer, failing the test if none is present.
func readActivePointerDir(t *testing.T) string {
	t.Helper()
	dir, ok := ReadActiveWorkspacePointer()
	if !ok {
		t.Fatal("expected an active-workspace pointer after a move")
	}
	return dir
}

func TestMoveWorkspace_CopiesVerifiesAndSwitchesPointer(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	src := filepath.Join(t.TempDir(), "src")
	dst := filepath.Join(t.TempDir(), "dst")
	if _, err := NewManager(ConfigPathForDataDir(src)).LoadOrCreateDomains(); err != nil {
		t.Fatalf("bootstrap schema-6 source: %v", err)
	}
	writeFileTree(t, src, map[string]string{
		"projects/alpha/tasks.md": "# Alpha",
		"themes/custom.json":      `{"name":"custom"}`,
		// Runtime-only entries must not be moved.
		"logs/tt.log":          "log",
		"machine.json":         `{"kind":"machine"}`,
		"runtime/abc/pid.lock": "999",
	})

	report, moveErr := MoveWorkspace(src, dst)
	if moveErr != nil {
		t.Fatalf("MoveWorkspace: %v", moveErr)
	}

	// Durable data copied and byte-identical.
	if got, _ := os.ReadFile(filepath.Join(dst, "projects/alpha/tasks.md")); string(got) != "# Alpha" {
		t.Errorf("copied task content = %q, want %q", got, "# Alpha")
	}
	if _, err := os.Stat(filepath.Join(dst, "themes/custom.json")); err != nil {
		t.Errorf("expected themes copied: %v", err)
	}
	// Runtime state excluded.
	if _, err := os.Stat(filepath.Join(dst, "logs")); !os.IsNotExist(err) {
		t.Errorf("logs must not be moved, stat err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(dst, "runtime")); !os.IsNotExist(err) {
		t.Errorf("runtime must not be moved, stat err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(dst, "machine.json")); !os.IsNotExist(err) {
		t.Errorf("legacy machine state must not be moved, stat err=%v", err)
	}
	// Source retained by default.
	if !report.SourceRetained {
		t.Error("source should be retained by default")
	}
	if _, err := os.Stat(filepath.Join(src, "projects/alpha/tasks.md")); err != nil {
		t.Errorf("source must remain intact: %v", err)
	}
	// Pointer switched to the destination.
	if got := readActivePointerDir(t); got != filepath.Clean(dst) {
		t.Errorf("active pointer = %q, want %q", got, filepath.Clean(dst))
	}
	// Split domains are path-portable; runtime binds them to the destination root.
	runtime, err := NewManager(ConfigPathForDataDir(dst)).LoadRuntimeState()
	if err != nil {
		t.Fatalf("load moved config: %v", err)
	}
	if runtime.DataDir() != filepath.Clean(dst) {
		t.Errorf("moved runtime data dir = %q, want %q", runtime.DataDir(), filepath.Clean(dst))
	}
}

func TestMoveWorkspace_RetryIsIdempotent(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	src := filepath.Join(t.TempDir(), "src")
	dst := filepath.Join(t.TempDir(), "dst")
	writeFileTree(t, src, map[string]string{
		"projects/alpha/tasks.md": "# Alpha",
		"projects/beta/tasks.md":  "# Beta",
	})

	if _, err := MoveWorkspace(src, dst); err != nil {
		t.Fatalf("first MoveWorkspace: %v", err)
	}
	// Simulate an interrupted move by removing one destination file, then re-run.
	if err := os.Remove(filepath.Join(dst, "projects/beta/tasks.md")); err != nil {
		t.Fatalf("remove: %v", err)
	}

	report, err := MoveWorkspace(src, dst)
	if err != nil {
		t.Fatalf("retry MoveWorkspace: %v", err)
	}
	if report.FilesCopied != 1 {
		t.Errorf("retry FilesCopied = %d, want 1 (only the removed file)", report.FilesCopied)
	}
	if report.FilesSkipped != 1 {
		t.Errorf("retry FilesSkipped = %d, want 1 (identical file)", report.FilesSkipped)
	}
	if got, _ := os.ReadFile(filepath.Join(dst, "projects/beta/tasks.md")); string(got) != "# Beta" {
		t.Errorf("re-copied content = %q, want %q", got, "# Beta")
	}
}

func TestMoveWorkspace_RefusesDivergentDestinationFile(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	src := filepath.Join(t.TempDir(), "src")
	dst := filepath.Join(t.TempDir(), "dst")
	writeFileTree(t, src, map[string]string{
		"projects/alpha/tasks.md": "# Alpha src",
		"themes/custom.json":      `{"name":"custom"}`,
	})
	// A conflicting file already exists at the destination.
	writeFileTree(t, dst, map[string]string{
		"projects/alpha/tasks.md": "# Alpha existing",
	})

	if _, err := MoveWorkspace(src, dst); err == nil {
		t.Fatal("expected conflict error, got nil")
	}
	// All-or-nothing: the other file must not have been copied.
	if _, err := os.Stat(filepath.Join(dst, "themes/custom.json")); !os.IsNotExist(err) {
		t.Errorf("no partial copy expected, stat err=%v", err)
	}
	// The existing file is untouched.
	if got, _ := os.ReadFile(filepath.Join(dst, "projects/alpha/tasks.md")); string(got) != "# Alpha existing" {
		t.Errorf("existing file was modified: %q", got)
	}
	// No pointer switch on a failed move.
	if _, ok := ReadActiveWorkspacePointer(); ok {
		t.Error("no active pointer should be written on a failed move")
	}
}

func TestMoveWorkspace_RefusesNestedDestination(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	src := filepath.Join(t.TempDir(), "src")
	writeFileTree(t, src, map[string]string{"projects/alpha/tasks.md": "# Alpha"})

	nested := filepath.Join(src, "sub")
	if _, err := MoveWorkspace(src, nested); err == nil {
		t.Fatal("expected nested-destination error, got nil")
	}
	if _, err := MoveWorkspace(nested, src); err == nil {
		t.Fatal("expected nested-source error, got nil")
	}
}

func TestMoveWorkspace_RefusesSameDir(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	src := filepath.Join(t.TempDir(), "src")
	writeFileTree(t, src, map[string]string{"projects/alpha/tasks.md": "# Alpha"})

	if _, err := MoveWorkspace(src, src); err == nil {
		t.Fatal("expected same-directory error, got nil")
	}
}

func TestMoveWorkspace_RefusesSymlinkInSource(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink semantics differ on windows")
	}
	home := t.TempDir()
	t.Setenv("HOME", home)

	src := filepath.Join(t.TempDir(), "src")
	dst := filepath.Join(t.TempDir(), "dst")
	writeFileTree(t, src, map[string]string{"projects/alpha/tasks.md": "# Alpha"})

	// A symlink inside the workspace must abort the move rather than be followed.
	link := filepath.Join(src, "projects", "link")
	if err := os.Symlink(filepath.Join(src, "projects", "alpha"), link); err != nil {
		t.Fatalf("symlink: %v", err)
	}

	if _, err := MoveWorkspace(src, dst); err == nil {
		t.Fatal("expected symlink error, got nil")
	}
}

func TestMoveWorkspaceIncludesCommitCompletedBeforeWorkspaceLockAcquisition(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	src := filepath.Join(t.TempDir(), "src")
	dst := filepath.Join(t.TempDir(), "dst")
	tasksPath := filepath.Join(src, "projects", "alpha", "tasks.md")
	writeFileTree(t, src, map[string]string{"projects/alpha/tasks.md": "before"})

	moveReachedLocks := make(chan struct{})
	allowMove := make(chan struct{})
	moveDone := make(chan error, 1)
	go func() {
		_, err := moveWorkspace(src, dst, &workspaceMoveTestHooks{
			beforeWorkspaceLocks: func() {
				close(moveReachedLocks)
				<-allowMove
			},
		})
		moveDone <- err
	}()

	select {
	case <-moveReachedLocks:
	case <-time.After(5 * time.Second):
		t.Fatal("move did not reach workspace-lock boundary")
	}
	if _, err := storage.NewCommitService(TransactionsDir(src), []string{src}).CommitArtifacts([]storage.ArtifactChange{{
		Path: tasksPath,
		Data: []byte("committed-before-lock"),
		Mode: FilePermissions,
	}}); err != nil {
		t.Fatalf("commit before move lock: %v", err)
	}
	close(allowMove)

	select {
	case err := <-moveDone:
		if err != nil {
			t.Fatalf("move: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("move did not complete")
	}
	if got, err := os.ReadFile(filepath.Join(dst, "projects", "alpha", "tasks.md")); err != nil || string(got) != "committed-before-lock" {
		t.Fatalf("destination snapshot = %q, err=%v; want completed commit", got, err)
	}
}

func TestMoveWorkspaceBlocksCommitAttemptedDuringSnapshot(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	src := filepath.Join(t.TempDir(), "src")
	dst := filepath.Join(t.TempDir(), "dst")
	tasksPath := filepath.Join(src, "projects", "alpha", "tasks.md")
	writeFileTree(t, src, map[string]string{"projects/alpha/tasks.md": "snapshot"})

	inventoried := make(chan struct{})
	allowMove := make(chan struct{})
	moveDone := make(chan error, 1)
	go func() {
		_, err := moveWorkspace(src, dst, &workspaceMoveTestHooks{
			afterInventory: func() {
				close(inventoried)
				<-allowMove
			},
		})
		moveDone <- err
	}()

	select {
	case <-inventoried:
	case <-time.After(5 * time.Second):
		t.Fatal("move did not inventory under workspace locks")
	}
	commitStarted := make(chan struct{})
	commitDone := make(chan error, 1)
	go func() {
		close(commitStarted)
		_, err := storage.NewCommitService(TransactionsDir(src), []string{src}).CommitArtifacts([]storage.ArtifactChange{{
			Path: tasksPath,
			Data: []byte("after-move"),
			Mode: FilePermissions,
		}})
		commitDone <- err
	}()
	<-commitStarted
	select {
	case err := <-commitDone:
		t.Fatalf("commit completed while move held the workspace lock: %v", err)
	case <-time.After(50 * time.Millisecond):
	}

	close(allowMove)
	select {
	case err := <-moveDone:
		if err != nil {
			t.Fatalf("move: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("move did not complete")
	}
	select {
	case err := <-commitDone:
		if err != nil {
			t.Fatalf("blocked commit after move: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("blocked commit did not resume after move")
	}

	if got, err := os.ReadFile(filepath.Join(dst, "projects", "alpha", "tasks.md")); err != nil || string(got) != "snapshot" {
		t.Fatalf("destination snapshot = %q, err=%v; want pre-commit bytes", got, err)
	}
	if got, err := os.ReadFile(tasksPath); err != nil || string(got) != "after-move" {
		t.Fatalf("retained source = %q, err=%v; want resumed commit", got, err)
	}
}

func TestMoveWorkspaceRejectsActiveSourceOrDestinationTUI(t *testing.T) {
	for _, role := range []string{"source", "destination"} {
		t.Run(role, func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("HOME", home)
			src := filepath.Join(t.TempDir(), "src")
			dst := filepath.Join(t.TempDir(), "dst")
			writeFileTree(t, src, map[string]string{"projects/alpha/tasks.md": "snapshot"})

			lockedDataDir := src
			if role == "destination" {
				lockedDataDir = dst
			}
			lockPath := TUIProcessLockPath(lockedDataDir)
			if err := os.MkdirAll(filepath.Dir(lockPath), DirPermissions); err != nil {
				t.Fatalf("mkdir TUI runtime dir: %v", err)
			}
			lock, err := util.AcquirePIDLock(lockPath)
			if err != nil {
				t.Fatalf("hold %s TUI lock: %v", role, err)
			}
			defer lock.Release()

			_, moveErr := MoveWorkspace(src, dst)
			if moveErr == nil {
				t.Fatalf("expected active %s TUI rejection", role)
			}
			if message := moveErr.Error(); !strings.Contains(message, role+" TUI safety lock") || !strings.Contains(message, "close the active tt TUI") {
				t.Fatalf("active %s TUI error is not actionable: %v", role, moveErr)
			}
			if _, ok := ReadActiveWorkspacePointer(); ok {
				t.Fatal("active TUI rejection must not switch the workspace pointer")
			}
		})
	}
}

func TestMoveWorkspaceSourceDivergenceLeavesPointerUnchanged(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	src := filepath.Join(t.TempDir(), "src")
	dst := filepath.Join(t.TempDir(), "dst")
	tasksPath := filepath.Join(src, "projects", "alpha", "tasks.md")
	writeFileTree(t, src, map[string]string{"projects/alpha/tasks.md": "snapshot"})
	if err := WriteActiveWorkspacePointer(src, ""); err != nil {
		t.Fatalf("seed pointer: %v", err)
	}

	_, err := moveWorkspace(src, dst, &workspaceMoveTestHooks{afterCopy: func() {
		if writeErr := os.WriteFile(tasksPath, []byte("direct-source-race"), FilePermissions); writeErr != nil {
			t.Fatalf("write source race: %v", writeErr)
		}
	}})
	if err == nil || !strings.Contains(err.Error(), "source changed while copying") {
		t.Fatalf("source divergence error = %v", err)
	}
	if got := readActivePointerDir(t); got != filepath.Clean(src) {
		t.Fatalf("pointer after source divergence = %q, want %q", got, src)
	}
}

func TestMoveWorkspaceDestinationRaceIsNeverOverwritten(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	src := filepath.Join(t.TempDir(), "src")
	dst := filepath.Join(t.TempDir(), "dst")
	target := filepath.Join(dst, "projects", "alpha", "tasks.md")
	writeFileTree(t, src, map[string]string{"projects/alpha/tasks.md": "snapshot"})
	if err := WriteActiveWorkspacePointer(src, ""); err != nil {
		t.Fatalf("seed pointer: %v", err)
	}

	_, err := moveWorkspace(src, dst, &workspaceMoveTestHooks{afterDestinationPrecheck: func() {
		if mkdirErr := os.MkdirAll(filepath.Dir(target), DirPermissions); mkdirErr != nil {
			t.Fatalf("mkdir destination race: %v", mkdirErr)
		}
		if writeErr := os.WriteFile(target, []byte("destination-race"), FilePermissions); writeErr != nil {
			t.Fatalf("write destination race: %v", writeErr)
		}
	}})
	if err == nil || !strings.Contains(err.Error(), "destination file changed while copying") {
		t.Fatalf("destination race error = %v", err)
	}
	if got, readErr := os.ReadFile(target); readErr != nil || string(got) != "destination-race" {
		t.Fatalf("destination race was overwritten: bytes=%q err=%v", got, readErr)
	}
	if got := readActivePointerDir(t); got != filepath.Clean(src) {
		t.Fatalf("pointer after destination race = %q, want %q", got, src)
	}
}

func TestMoveWorkspaceFinalDestinationRecheckDetectsPostCopyMutation(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	src := filepath.Join(t.TempDir(), "src")
	dst := filepath.Join(t.TempDir(), "dst")
	target := filepath.Join(dst, "projects", "alpha", "tasks.md")
	writeFileTree(t, src, map[string]string{"projects/alpha/tasks.md": "snapshot"})
	if err := WriteActiveWorkspacePointer(src, ""); err != nil {
		t.Fatalf("seed pointer: %v", err)
	}

	_, err := moveWorkspace(src, dst, &workspaceMoveTestHooks{afterCopy: func() {
		if writeErr := os.WriteFile(target, []byte("post-copy-race"), FilePermissions); writeErr != nil {
			t.Fatalf("write post-copy race: %v", writeErr)
		}
	}})
	if err == nil || !strings.Contains(err.Error(), "destination file already exists with different contents") {
		t.Fatalf("post-copy destination race error = %v", err)
	}
	if got := readActivePointerDir(t); got != filepath.Clean(src) {
		t.Fatalf("pointer after post-copy race = %q, want %q", got, src)
	}
}

func TestStorageLocationLabel(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	makeICloudDrive(t, home)

	local := LocalDataRoot()
	if got := StorageLocationLabel(local); got != "local" {
		t.Errorf("StorageLocationLabel(local) = %q, want %q", got, "local")
	}

	icloudRoot := filepath.Join(home, iCloudDriveRelPath, iCloudDataDirName)
	if got := StorageLocationLabel(icloudRoot); got == "local" {
		t.Errorf("StorageLocationLabel(iCloud) = %q, want a synced label", got)
	}
}
