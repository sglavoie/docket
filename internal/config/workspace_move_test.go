package config

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

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
	writeFileTree(t, src, map[string]string{
		configFileName:            `{"version":1,"data_dir":"` + jsonEscape(src) + `"}`,
		"projects/alpha/tasks.md": "# Alpha",
		"themes/custom.json":      `{"name":"custom"}`,
		// Runtime-only entries must not be moved.
		"logs/tt.log":          "log",
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
	// Destination config data_dir rewritten to the new root.
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

// jsonEscape escapes backslashes so a filesystem path can be embedded in a JSON
// string literal in test fixtures (Windows paths contain backslashes).
func jsonEscape(s string) string {
	out := make([]rune, 0, len(s))
	for _, r := range s {
		if r == '\\' {
			out = append(out, '\\', '\\')
			continue
		}
		out = append(out, r)
	}
	return string(out)
}
