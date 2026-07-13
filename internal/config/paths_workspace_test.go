package config

import (
	"path/filepath"
	"testing"
)

func TestResolveWorkspaceDataDir_FlagWins(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv(DataDirEnvVar, filepath.Join(home, "from-env"))
	if err := WriteActiveWorkspacePointer(filepath.Join(home, "from-pointer"), "ws"); err != nil {
		t.Fatalf("WriteActiveWorkspacePointer: %v", err)
	}

	flag := filepath.Join(home, "from-flag")
	dir, explicit := ResolveWorkspaceDataDir(flag)
	if !explicit {
		t.Fatal("flag selection should be explicit")
	}
	if dir != filepath.Clean(flag) {
		t.Fatalf("dir = %q, want %q", dir, flag)
	}
}

func TestResolveWorkspaceDataDir_EnvBeatsPointer(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	env := filepath.Join(home, "from-env")
	t.Setenv(DataDirEnvVar, env)
	if err := WriteActiveWorkspacePointer(filepath.Join(home, "from-pointer"), "ws"); err != nil {
		t.Fatalf("WriteActiveWorkspacePointer: %v", err)
	}

	dir, explicit := ResolveWorkspaceDataDir("")
	if !explicit {
		t.Fatal("env selection should be explicit")
	}
	if dir != filepath.Clean(env) {
		t.Fatalf("dir = %q, want env %q", dir, env)
	}
}

func TestResolveWorkspaceDataDir_PointerBeatsDefault(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv(DataDirEnvVar, "") // empty is treated as unset by the resolver
	pointer := filepath.Join(home, "from-pointer")
	if err := WriteActiveWorkspacePointer(pointer, "ws"); err != nil {
		t.Fatalf("WriteActiveWorkspacePointer: %v", err)
	}

	dir, explicit := ResolveWorkspaceDataDir("")
	if !explicit {
		t.Fatal("pointer selection should be explicit")
	}
	if dir != filepath.Clean(pointer) {
		t.Fatalf("dir = %q, want pointer %q", dir, pointer)
	}
}

func TestResolveWorkspaceDataDir_FallsBackToLocalDefault(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv(DataDirEnvVar, "") // empty is treated as unset by the resolver
	// No flag, no env, no pointer.

	dir, explicit := ResolveWorkspaceDataDir("")
	if explicit {
		t.Fatal("no selection should not be explicit")
	}
	if dir != LocalDataRoot() {
		t.Fatalf("dir = %q, want local default %q", dir, LocalDataRoot())
	}
}

func TestActiveWorkspacePointer_RoundTrip(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	if _, ok := ReadActiveWorkspacePointer(); ok {
		t.Fatal("no pointer should exist initially")
	}

	dataDir := filepath.Join(home, "ws-data")
	if err := WriteActiveWorkspacePointer(dataDir, "ws-id"); err != nil {
		t.Fatalf("WriteActiveWorkspacePointer: %v", err)
	}
	got, ok := ReadActiveWorkspacePointer()
	if !ok {
		t.Fatal("pointer should exist after write")
	}
	if got != filepath.Clean(dataDir) {
		t.Fatalf("pointer dir = %q, want %q", got, dataDir)
	}
}

func TestWorkspaceRootFromConfigPath(t *testing.T) {
	cfgPath := filepath.Join("/tmp", "isolated", ".tt.json")
	if got := WorkspaceRootFromConfigPath(cfgPath); got != filepath.Join("/tmp", "isolated") {
		t.Fatalf("WorkspaceRootFromConfigPath = %q, want %q", got, filepath.Join("/tmp", "isolated"))
	}
	// ConfigPathForDataDir is its inverse for the data dir.
	dataDir := filepath.Join("/tmp", "isolated")
	if got := ConfigPathForDataDir(dataDir); WorkspaceRootFromConfigPath(got) != dataDir {
		t.Fatalf("ConfigPathForDataDir/WorkspaceRootFromConfigPath not inverse: %q", got)
	}
}

func TestDomainPathsUnderInternalDir(t *testing.T) {
	dataDir := filepath.Join(t.TempDir(), "ws")
	internal := InternalDir(dataDir)
	for _, p := range []string{
		WorkspaceCatalogPath(dataDir),
		PlanningStatePath(dataDir),
		SettingsPath(dataDir),
		AutomationPath(dataDir),
	} {
		if filepath.Dir(p) != internal {
			t.Fatalf("synced artifact %q not under internal dir %q", p, internal)
		}
	}
}
