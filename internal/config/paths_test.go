package config

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
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

func TestActiveDefaultDataDirOn_ICloudPresentButNoWorkspaceStaysLocal(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	makeICloudDrive(t, home)

	// iCloud Drive is available but holds no Docket workspace: local-first wins and
	// no automatic iCloud selection occurs.
	want := filepath.Join(home, localRootName)
	if got := activeDefaultDataDirOn(""); got != want {
		t.Fatalf("activeDefaultDataDirOn(\"\") = %q, want local root %q", got, want)
	}
}

func TestActiveDefaultDataDirOn_ExistingICloudWorkspacePreserved(t *testing.T) {
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
	if got := activeDefaultDataDirOn(""); got != icloudRoot {
		t.Fatalf("activeDefaultDataDirOn(\"\") = %q, want existing iCloud root %q", got, icloudRoot)
	}
	if got := defaultConfigPathOn(""); got != filepath.Join(icloudRoot, configFileName) {
		t.Fatalf("defaultConfigPathOn(\"\") = %q, want %q", got, filepath.Join(icloudRoot, configFileName))
	}
}

func TestActiveDefaultDataDirOn_ExistingICloudSplitWorkspacePreserved(t *testing.T) {
	for _, tc := range []struct {
		name    string
		markers []func(string) string
	}{
		{
			name: "complete",
			markers: []func(string) string{
				WorkspaceCatalogPath,
				PlanningStatePath,
				SettingsPath,
				AutomationPath,
			},
		},
		{
			name:    "partial",
			markers: []func(string) string{PlanningStatePath},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("HOME", home)
			makeICloudDrive(t, home)

			// A stale local aggregate must not hide the authoritative iCloud
			// split workspace, including when that split state is partial.
			if err := os.MkdirAll(LocalDataRoot(), DirPermissions); err != nil {
				t.Fatalf("mkdir local root: %v", err)
			}
			if err := os.WriteFile(ConfigPathForDataDir(LocalDataRoot()), []byte(`{"version":1}`), FilePermissions); err != nil {
				t.Fatalf("write local legacy config: %v", err)
			}

			icloudRoot := filepath.Join(home, iCloudDriveRelPath, iCloudDataDirName)
			for _, marker := range tc.markers {
				path := marker(icloudRoot)
				if err := os.MkdirAll(filepath.Dir(path), DirPermissions); err != nil {
					t.Fatalf("mkdir marker parent: %v", err)
				}
				if err := os.WriteFile(path, []byte("{}"), FilePermissions); err != nil {
					t.Fatalf("write split marker: %v", err)
				}
			}
			if _, err := os.Stat(ConfigPathForDataDir(icloudRoot)); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("iCloud aggregate config should be absent, stat error = %v", err)
			}
			if got := activeDefaultDataDirOn(""); got != icloudRoot {
				t.Fatalf("activeDefaultDataDirOn(\"\") = %q, want split iCloud root %q", got, icloudRoot)
			}
		})
	}
}

func TestActiveDefaultDataDirOn_ICloudAbsent(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	// No iCloud drive dir created.

	want := filepath.Join(home, localRootName)
	if got := activeDefaultDataDirOn(""); got != want {
		t.Fatalf("activeDefaultDataDirOn(\"\") = %q, want local root %q", got, want)
	}
}

func TestActiveDefaultDataDirOn_HomeFailureFallback(t *testing.T) {
	t.Setenv("HOME", "")

	if got := LocalDataRoot(); got != localRootName {
		t.Fatalf("LocalDataRoot() with no home = %q, want %q", got, localRootName)
	}
	if got := activeDefaultDataDirOn(""); got != localRootName {
		t.Fatalf("activeDefaultDataDirOn(\"\") with no home = %q, want %q", got, localRootName)
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
	if got := m.GetConfigPath(); got != defaultConfigPathOn("") {
		t.Fatalf("GetConfigPath() = %q, want default %q", got, defaultConfigPathOn(""))
	}
}

func TestLocalRuntimeDirForDataDir(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	local := LocalDataRoot()
	// The local default data dir maps to the local root itself so the historical
	// ~/.docket/logs layout is preserved.
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

func TestMachineStatePathUsesValidatedWorkspaceIDForLocalAndExternal(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	want := filepath.Join(home, localRootName, runtimeDirName, testWorkspaceIDA, machineStateFileName)
	localPath, err := MachineStatePath(testWorkspaceIDA)
	if err != nil {
		t.Fatalf("MachineStatePath(local workspace): %v", err)
	}
	if localPath != want {
		t.Fatalf("MachineStatePath(local workspace) = %q, want %q", localPath, want)
	}

	// A move from the default local data directory to an external directory does
	// not change machine-state addressing: the stable workspace ID is the only
	// path input.
	externalPath, err := MachineStatePath(testWorkspaceIDA)
	if err != nil {
		t.Fatalf("MachineStatePath(external workspace): %v", err)
	}
	if externalPath != localPath {
		t.Fatalf("same workspace ID at external path = %q, want local path %q", externalPath, localPath)
	}

	otherPath, err := MachineStatePath(testWorkspaceIDB)
	if err != nil {
		t.Fatalf("MachineStatePath(other workspace): %v", err)
	}
	if otherPath == localPath {
		t.Fatal("distinct workspace IDs must produce distinct machine-state paths")
	}
}

func TestMachineStatePathRejectsInvalidWorkspaceID(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	for _, workspaceID := range []string{
		"",
		"not-a-uuid",
		"../" + testWorkspaceIDA,
		strings.ToUpper("aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"),
	} {
		if path, err := MachineStatePath(workspaceID); !errors.Is(err, ErrInvalidWorkspaceID) || path != "" {
			t.Errorf("MachineStatePath(%q) = (%q, %v), want empty path and ErrInvalidWorkspaceID", workspaceID, path, err)
		}
	}
}
