package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const adoptionTestWorkspaceID = "11111111-2222-4333-8444-555555555555"

// writeAdoptionFile writes body at root/rel, creating its parents.
func writeAdoptionFile(t *testing.T, root, rel, body string) {
	t.Helper()
	path := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(path), DirPermissions); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), FilePermissions); err != nil {
		t.Fatal(err)
	}
}

// writeAdoptionPointer records dataDir in the pointer under a machine-local root.
func writeAdoptionPointer(t *testing.T, localRoot, dataDir string) {
	t.Helper()
	data, err := json.Marshal(activeWorkspacePointer{SchemaVersion: 7, WorkspaceID: adoptionTestWorkspaceID, DataDir: dataDir})
	if err != nil {
		t.Fatal(err)
	}
	writeAdoptionFile(t, localRoot, filepath.Join(runtimeDirName, activeWorkspacePointerName), string(data))
}

// readAdoptionPointer decodes the pointer under a machine-local root.
func readAdoptionPointer(t *testing.T, localRoot string) activeWorkspacePointer {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(localRoot, runtimeDirName, activeWorkspacePointerName))
	if err != nil {
		t.Fatal(err)
	}
	var ptr activeWorkspacePointer
	if err := json.Unmarshal(data, &ptr); err != nil {
		t.Fatal(err)
	}
	return ptr
}

func assertAbsent(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("%s should be absent (lstat err = %v)", path, err)
	}
}

func assertFileBody(t *testing.T, path, want string) {
	t.Helper()
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != want {
		t.Fatalf("%s = %q, want %q", path, got, want)
	}
}

// populateLegacyLocalRoot lays out a ~/.tt that is both the machine-local root
// and the default local workspace, with the pointer naming it.
func populateLegacyLocalRoot(t *testing.T, home string) string {
	t.Helper()
	old := filepath.Join(home, legacyLocalRootName)
	writeAdoptionFile(t, old, filepath.Join("projects", "inbox", "tasks.md"), "tasks")
	writeAdoptionFile(t, old, filepath.Join(internalDirName, "workspace.json"), "{}")
	writeAdoptionFile(t, old, filepath.Join(runtimeDirName, adoptionTestWorkspaceID, machineStateFileName), "{}")
	writeAdoptionFile(t, old, filepath.Join("logs", "tt-old.log"), "log")
	writeAdoptionPointer(t, old, old)
	return old
}

// adoptionICloudDrive creates the iCloud Drive container under home.
func adoptionICloudDrive(t *testing.T, home string) string {
	t.Helper()
	makeICloudDrive(t, home)
	return filepath.Join(home, iCloudDriveRelPath)
}

func TestAdoptLegacyRootsMovesPopulatedLocalRoot(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	old := populateLegacyLocalRoot(t, home)

	if err := adoptLegacyRootsOn(home); err != nil {
		t.Fatalf("adopt: %v", err)
	}

	newRoot := filepath.Join(home, localRootName)
	assertAbsent(t, old)
	assertFileBody(t, filepath.Join(newRoot, "projects", "inbox", "tasks.md"), "tasks")
	assertFileBody(t, filepath.Join(newRoot, internalDirName, "workspace.json"), "{}")
	assertFileBody(t, filepath.Join(newRoot, runtimeDirName, adoptionTestWorkspaceID, machineStateFileName), "{}")
	assertFileBody(t, filepath.Join(newRoot, "logs", "tt-old.log"), "log")
	ptr := readAdoptionPointer(t, newRoot)
	if ptr.DataDir != newRoot {
		t.Fatalf("pointer data_dir = %q, want %q", ptr.DataDir, newRoot)
	}
	if ptr.WorkspaceID != adoptionTestWorkspaceID || ptr.SchemaVersion != 7 {
		t.Fatalf("pointer lost its other fields: %+v", ptr)
	}
	if got := localDataRootOn(home); got != newRoot {
		t.Fatalf("localDataRootOn = %q, want %q", got, newRoot)
	}
}

func TestAdoptLegacyRootsOnEmptyHomeCreatesNothing(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	makeICloudDrive(t, home)

	if err := adoptLegacyRootsOn(home); err != nil {
		t.Fatalf("adopt: %v", err)
	}

	assertAbsent(t, filepath.Join(home, localRootName))
	assertAbsent(t, filepath.Join(home, iCloudDriveRelPath, iCloudDataDirName))
}

func TestAdoptLegacyRootsRerunIsIdempotent(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	populateLegacyLocalRoot(t, home)
	if err := adoptLegacyRootsOn(home); err != nil {
		t.Fatalf("first adopt: %v", err)
	}
	newRoot := filepath.Join(home, localRootName)
	before := readAdoptionPointer(t, newRoot)

	// A ~/.tt recreated by an older binary is left alone once ~/.docket exists.
	stray := filepath.Join(home, legacyLocalRootName)
	writeAdoptionFile(t, stray, filepath.Join("logs", "late.log"), "late")
	if err := adoptLegacyRootsOn(home); err != nil {
		t.Fatalf("second adopt: %v", err)
	}

	if after := readAdoptionPointer(t, newRoot); after != before {
		t.Fatalf("rerun rewrote the pointer: %+v, then %+v", before, after)
	}
	assertFileBody(t, filepath.Join(stray, "logs", "late.log"), "late")
	assertAbsent(t, filepath.Join(newRoot, "logs", "late.log"))
}

func TestAdoptLegacyRootsMovesICloudWorkspaceAndRewritesPointer(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	drive := adoptionICloudDrive(t, home)
	oldCloud := filepath.Join(drive, legacyICloudDataDirName)
	writeAdoptionFile(t, oldCloud, filepath.Join(internalDirName, "workspace.json"), "{}")
	writeAdoptionFile(t, oldCloud, filepath.Join("projects", "inbox", "tasks.md"), "cloud")
	oldLocal := filepath.Join(home, legacyLocalRootName)
	writeAdoptionPointer(t, oldLocal, oldCloud)

	if err := adoptLegacyRootsOn(home); err != nil {
		t.Fatalf("adopt: %v", err)
	}

	newCloud := filepath.Join(drive, iCloudDataDirName)
	assertAbsent(t, oldCloud)
	assertFileBody(t, filepath.Join(newCloud, "projects", "inbox", "tasks.md"), "cloud")
	if ptr := readAdoptionPointer(t, filepath.Join(home, localRootName)); ptr.DataDir != newCloud {
		t.Fatalf("pointer data_dir = %q, want %q", ptr.DataDir, newCloud)
	}
	if got := activeDefaultDataDirOn(home); got != newCloud {
		t.Fatalf("default data dir = %q, want %q", got, newCloud)
	}
}

func TestAdoptLegacyRootsLeavesOtherICloudPointerUntouched(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	drive := adoptionICloudDrive(t, home)
	elsewhere := filepath.Join(drive, "Work", "tasks")
	writeAdoptionFile(t, elsewhere, filepath.Join(internalDirName, "workspace.json"), "{}")
	// A folder named tt that is not a workspace stays where it is.
	unrelated := filepath.Join(drive, legacyICloudDataDirName)
	writeAdoptionFile(t, unrelated, "notes.txt", "mine")
	oldLocal := filepath.Join(home, legacyLocalRootName)
	writeAdoptionPointer(t, oldLocal, elsewhere)

	if err := adoptLegacyRootsOn(home); err != nil {
		t.Fatalf("adopt: %v", err)
	}

	if ptr := readAdoptionPointer(t, filepath.Join(home, localRootName)); ptr.DataDir != elsewhere {
		t.Fatalf("pointer data_dir = %q, want it untouched at %q", ptr.DataDir, elsewhere)
	}
	assertFileBody(t, filepath.Join(elsewhere, internalDirName, "workspace.json"), "{}")
	assertFileBody(t, filepath.Join(unrelated, "notes.txt"), "mine")
	assertAbsent(t, filepath.Join(drive, iCloudDataDirName))
}

func TestAdoptLegacyRootsFollowsICloudMovedByAnotherMachine(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	drive := adoptionICloudDrive(t, home)
	newCloud := filepath.Join(drive, iCloudDataDirName)
	writeAdoptionFile(t, newCloud, filepath.Join(internalDirName, "workspace.json"), "{}")
	writeAdoptionPointer(t, filepath.Join(home, legacyLocalRootName), filepath.Join(drive, legacyICloudDataDirName))

	if err := adoptLegacyRootsOn(home); err != nil {
		t.Fatalf("adopt: %v", err)
	}

	if ptr := readAdoptionPointer(t, filepath.Join(home, localRootName)); ptr.DataDir != newCloud {
		t.Fatalf("pointer data_dir = %q, want %q", ptr.DataDir, newCloud)
	}
}

func TestAdoptLegacyRootsRefusesWhenICloudDocketExists(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	drive := adoptionICloudDrive(t, home)
	oldCloud := filepath.Join(drive, legacyICloudDataDirName)
	newCloud := filepath.Join(drive, iCloudDataDirName)
	writeAdoptionFile(t, oldCloud, filepath.Join(internalDirName, "workspace.json"), "old")
	writeAdoptionFile(t, newCloud, "keep.txt", "new")
	old := populateLegacyLocalRoot(t, home)

	err := adoptLegacyRootsOn(home)
	if err == nil {
		t.Fatal("adopt succeeded beside an existing iCloud Drive/Docket")
	}
	for _, name := range []string{oldCloud, newCloud} {
		if !strings.Contains(err.Error(), DisplayPath(name)) {
			t.Fatalf("refusal %q does not name %s", err, name)
		}
	}
	assertFileBody(t, filepath.Join(oldCloud, internalDirName, "workspace.json"), "old")
	assertFileBody(t, filepath.Join(newCloud, "keep.txt"), "new")
	assertFileBody(t, filepath.Join(old, "projects", "inbox", "tasks.md"), "tasks")
	assertAbsent(t, filepath.Join(home, localRootName))
}

func TestAdoptLegacyRootsFailedMoveLeavesLegacyRootsIntact(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	drive := adoptionICloudDrive(t, home)
	oldCloud := filepath.Join(drive, legacyICloudDataDirName)
	writeAdoptionFile(t, oldCloud, filepath.Join(internalDirName, "workspace.json"), "{}")
	old := populateLegacyLocalRoot(t, home)
	// A read-only home refuses the local rename after the iCloud one succeeded.
	if err := os.Chmod(home, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(home, 0o700) })

	err := adoptLegacyRootsOn(home)
	if err == nil {
		t.Fatal("adopt succeeded in a read-only home")
	}
	if !strings.Contains(err.Error(), "left untouched") {
		t.Fatalf("error %q does not report the untouched root", err)
	}
	assertFileBody(t, filepath.Join(old, "projects", "inbox", "tasks.md"), "tasks")
	assertFileBody(t, filepath.Join(oldCloud, internalDirName, "workspace.json"), "{}")
	assertAbsent(t, filepath.Join(drive, iCloudDataDirName))
	assertAbsent(t, filepath.Join(home, localRootName))
}

// TestAdoptLegacyRootsFollowsHomeSwitch drives the exported entry point through
// $HOME: each home is a machine and adopts its own roots.
func TestAdoptLegacyRootsFollowsHomeSwitch(t *testing.T) {
	first := t.TempDir()
	second := t.TempDir()
	populateLegacyLocalRoot(t, first)
	populateLegacyLocalRoot(t, second)

	t.Setenv("HOME", first)
	if err := AdoptLegacyRoots(); err != nil {
		t.Fatalf("adopt first home: %v", err)
	}
	assertAbsent(t, filepath.Join(first, legacyLocalRootName))
	if _, err := os.Stat(filepath.Join(second, legacyLocalRootName)); err != nil {
		t.Fatalf("adopting the first home touched the second: %v", err)
	}

	t.Setenv("HOME", second)
	if err := AdoptLegacyRoots(); err != nil {
		t.Fatalf("adopt second home: %v", err)
	}
	for _, home := range []string{first, second} {
		newRoot := filepath.Join(home, localRootName)
		if ptr := readAdoptionPointer(t, newRoot); ptr.DataDir != newRoot {
			t.Fatalf("pointer under %s = %q, want %q", home, ptr.DataDir, newRoot)
		}
	}
	got, ok := ReadActiveWorkspacePointer()
	if want := filepath.Join(second, localRootName); !ok || got != want {
		t.Fatalf("ReadActiveWorkspacePointer = %q, %v, want %q", got, ok, want)
	}
}
