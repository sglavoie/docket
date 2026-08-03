package tui

import (
	"path/filepath"
	"testing"

	"github.com/sglavoie/tt/internal/config"
)

// TestNewAppModelWiresShellDeps proves the composition root assembles the
// domain-native dependency bundle alongside the legacy config.Config facade: the
// bundle carries the same RuntimeState snapshot, application service, config
// manager, and workspace ledger the shell holds, so a feature group can migrate
// onto the bundle without re-plumbing its construction.
func TestNewAppModelWiresShellDeps(t *testing.T) {
	cfg := newTestConfig()
	configPath := filepath.Join(t.TempDir(), "tt.json")
	manager := config.NewManager(configPath)

	app, err := NewAppModel(testRuntimeAt(cfg.DataDir(), cfg), manager, WithPersister(NewMockPersister()))
	if err != nil {
		t.Fatalf("NewAppModel: %v", err)
	}

	if app.deps == nil {
		t.Fatal("NewAppModel must wire the shell dependency bundle")
	}
	if app.deps.Ledger == nil {
		t.Error("bundle must carry the shared workspace ledger")
	}
}
