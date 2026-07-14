package config

import (
	"os"
	"path/filepath"
	"testing"
)

// makeICloudDrive creates the iCloud Drive container directory under home so the
// resolver detects iCloud as available.
func makeICloudDrive(t *testing.T, home string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(home, iCloudDriveRelPath), 0o755); err != nil {
		t.Fatalf("failed to create iCloud drive dir: %v", err)
	}
}

func TestActiveDefaultDataDir_ICloudPresentButNoWorkspaceStaysLocal(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	makeICloudDrive(t, home)

	// iCloud Drive is available but holds no tt workspace: local-first wins and
	// no automatic iCloud selection occurs.
	want := filepath.Join(home, dataDirName)
	if got := ActiveDefaultDataDir(); got != want {
		t.Fatalf("ActiveDefaultDataDir() = %q, want local root %q", got, want)
	}
}

func TestActiveDefaultDataDir_ExistingICloudWorkspacePreserved(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	makeICloudDrive(t, home)

	icloudRoot := filepath.Join(home, iCloudDriveRelPath, iCloudDataDirName)
	if err := os.MkdirAll(icloudRoot, 0o755); err != nil {
		t.Fatalf("mkdir iCloud root: %v", err)
	}
	if err := os.WriteFile(filepath.Join(icloudRoot, configFileName), []byte(`{"version":1}`), 0o600); err != nil {
		t.Fatalf("write iCloud config: %v", err)
	}

	// An existing iCloud workspace (its config already present) stays selected.
	if got := ActiveDefaultDataDir(); got != icloudRoot {
		t.Fatalf("ActiveDefaultDataDir() = %q, want existing iCloud root %q", got, icloudRoot)
	}
	if got := DefaultConfigPath(); got != filepath.Join(icloudRoot, configFileName) {
		t.Fatalf("DefaultConfigPath() = %q, want %q", got, filepath.Join(icloudRoot, configFileName))
	}
}

func TestActiveDefaultDataDir_ICloudAbsent(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	// No iCloud drive dir created.

	want := filepath.Join(home, dataDirName)
	if got := ActiveDefaultDataDir(); got != want {
		t.Fatalf("ActiveDefaultDataDir() = %q, want local root %q", got, want)
	}
	if got := LegacyConfigPath(); got != filepath.Join(want, configFileName) {
		t.Fatalf("LegacyConfigPath() = %q, want %q", got, filepath.Join(want, configFileName))
	}
}

func TestActiveDefaultDataDir_HomeFailureFallback(t *testing.T) {
	t.Setenv("HOME", "")

	if got := LocalDataRoot(); got != dataDirName {
		t.Fatalf("LocalDataRoot() with no home = %q, want %q", got, dataDirName)
	}
	if got := ActiveDefaultDataDir(); got != dataDirName {
		t.Fatalf("ActiveDefaultDataDir() with no home = %q, want %q", got, dataDirName)
	}
	if _, ok := iCloudDataRoot(); ok {
		t.Fatal("iCloudDataRoot() should report unavailable when home is unknown")
	}
}

func TestNewManager_CustomPathUnchanged(t *testing.T) {
	custom := filepath.Join(t.TempDir(), "custom", "config.json")
	m := NewManager(custom)

	if m.GetConfigPath() != custom {
		t.Fatalf("GetConfigPath() = %q, want %q", m.GetConfigPath(), custom)
	}
	if m.CreatedFromDefaultPath() {
		t.Fatal("custom-path manager should not be flagged as created from default path")
	}
}

func TestNewManager_DefaultPathFlagged(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	m := NewManager("")
	if !m.CreatedFromDefaultPath() {
		t.Fatal("default-path manager should be flagged as created from default path")
	}
	if got := m.GetConfigPath(); got != DefaultConfigPath() {
		t.Fatalf("GetConfigPath() = %q, want default %q", got, DefaultConfigPath())
	}
}

func TestLocalRuntimeDirForDataDir(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	local := LocalDataRoot()
	// The local default data dir maps to the local root itself so the historical
	// ~/.tt/logs layout is preserved.
	if got := LocalRuntimeDirForDataDir(local); got != local {
		t.Fatalf("LocalRuntimeDirForDataDir(local) = %q, want %q", got, local)
	}

	// A distinct (e.g. iCloud) data dir is keyed under the local runtime dir and
	// stays local rather than following the synced data dir.
	icloud := filepath.Join(home, iCloudDriveRelPath, iCloudDataDirName)
	rt := LocalRuntimeDirForDataDir(icloud)
	if rt == icloud {
		t.Fatal("runtime dir for a non-local data dir must not equal that data dir")
	}
	if filepath.Dir(filepath.Dir(rt)) != local {
		t.Fatalf("runtime dir %q should live under local root %q", rt, local)
	}
	// Deterministic and collision-isolated per data dir.
	if rt != LocalRuntimeDirForDataDir(icloud) {
		t.Fatal("LocalRuntimeDirForDataDir must be deterministic for the same input")
	}
	if rt == LocalRuntimeDirForDataDir(icloud+"-other") {
		t.Fatal("distinct data dirs must produce distinct runtime dirs")
	}
}

func TestMachineStatePathLocalAndExternal(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	local := filepath.Join(home, dataDirName)
	localWant := filepath.Join(local, machineStateFileName)
	if got := MachineStatePath(local); got != localWant {
		t.Fatalf("MachineStatePath(local) = %q, want %q", got, localWant)
	}

	external := filepath.Join(string(os.PathSeparator), "external", "tt")
	externalWant := filepath.Join(local, runtimeDirName, "c216d857b581", machineStateFileName)
	if got := MachineStatePath(external); got != externalWant {
		t.Fatalf("MachineStatePath(external) = %q, want %q", got, externalWant)
	}
}

func TestDefaultDataDirDisplay_AbbreviatesHome(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	// iCloud absent → local ~/.tt, abbreviated with ~.
	want := "~" + string(os.PathSeparator) + dataDirName
	if got := DefaultDataDirDisplay(); got != want {
		t.Fatalf("DefaultDataDirDisplay() = %q, want %q", got, want)
	}
}
