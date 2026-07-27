package config

import (
	"os"
	"path/filepath"
	"testing"
)

func BenchmarkReleaseCanonicalStartup(b *testing.B) {
	b.Setenv("HOME", b.TempDir())
	dataDir := filepath.Join(b.TempDir(), "workspace")
	manager := NewManager(ConfigPathForDataDir(dataDir))
	if _, err := manager.LoadOrCreateDomains(); err != nil {
		b.Fatalf("LoadOrCreateDomains: %v", err)
	}
	if _, err := manager.LoadRuntimeState(); err != nil {
		b.Fatalf("canonical warmup: %v", err)
	}

	var bytesPerStartup int64
	paths := []string{
		WorkspaceCatalogPath(dataDir),
		PlanningStatePath(dataDir),
		SettingsPath(dataDir),
		AutomationPath(dataDir),
	}
	domains, err := manager.LoadDomains()
	if err != nil {
		b.Fatalf("LoadDomains: %v", err)
	}
	machinePath, err := MachineStatePath(domains.Catalog.WorkspaceID)
	if err != nil {
		b.Fatalf("MachineStatePath: %v", err)
	}
	paths = append(paths, machinePath)
	for _, path := range paths {
		info, statErr := os.Stat(path)
		if statErr != nil {
			b.Fatalf("stat %s: %v", path, statErr)
		}
		bytesPerStartup += info.Size()
	}
	b.SetBytes(bytesPerStartup)
	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		if _, err := manager.LoadRuntimeState(); err != nil {
			b.Fatalf("LoadRuntimeState: %v", err)
		}
	}
}
