package tui

import (
	"path/filepath"
	"testing"

	"github.com/sglavoie/tt/internal/config"
)

// TestNewAppModelWiresShellDeps proves the composition root assembles the
// domain-native dependency bundle alongside the legacy config.Config facade: the
// bundle carries the same RuntimeState snapshot, application service, and config
// manager the shell holds, so a feature group can migrate onto the bundle without
// re-plumbing its construction.
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
	if app.deps.Runtime == nil {
		t.Error("bundle must carry the RuntimeState snapshot")
	}
	if app.deps.Runtime != app.runtime {
		t.Error("bundle Runtime must be the same snapshot the shell holds")
	}
	if app.deps.Service == nil {
		t.Error("bundle must carry the application service")
	}
	if app.deps.Service != app.appService {
		t.Error("bundle Service must be the same application service the shell holds")
	}
	if app.deps.Manager != manager {
		t.Error("bundle Manager must be the injected config manager")
	}
}

// TestShellDepsRuntimeReadsDomainsWithoutFacade confirms the bundle's RuntimeState
// exposes every configuration domain through its narrow accessors, so a migrated
// feature group reads domain data from the bundle without the config.Config
// facade.
func TestShellDepsRuntimeReadsDomainsWithoutFacade(t *testing.T) {
	cfg := newTestConfig()
	configPath := filepath.Join(t.TempDir(), "tt.json")
	manager := config.NewManager(configPath)

	app, err := NewAppModel(testRuntimeAt(cfg.DataDir(), cfg), manager, WithPersister(NewMockPersister()))
	if err != nil {
		t.Fatalf("NewAppModel: %v", err)
	}

	rs := app.deps.Runtime
	if rs.Catalog() == nil || rs.Planning() == nil || rs.Settings() == nil ||
		rs.Automation() == nil || rs.Machine() == nil {
		t.Fatal("bundle RuntimeState must expose every domain accessor")
	}
}
