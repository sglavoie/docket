package tui

import (
	"strings"
	"testing"

	"github.com/sglavoie/tt/internal/config"
)

// TestSettingsGeneralTabShowsWorkspaceLocation verifies the General settings tab
// surfaces the workspace data directory and the CLI relocation hint.
func TestSettingsGeneralTabShowsWorkspaceLocation(t *testing.T) {
	cfg := config.NewConfig()
	cfg.DataDir = "/tmp/tt-workspace-test"

	m := NewSettingsModel(cfg, nil)
	m.SetSize(120, 40)

	out := m.renderGeneralTab()
	if !strings.Contains(out, "Workspace") {
		t.Fatalf("General tab should include a Workspace section, got:\n%s", out)
	}
	if !strings.Contains(out, config.DisplayPath(cfg.DataDir)) {
		t.Fatalf("General tab should show the workspace location %q, got:\n%s", config.DisplayPath(cfg.DataDir), out)
	}
	if !strings.Contains(out, "tt workspace move") {
		t.Fatalf("General tab should mention the relocation command, got:\n%s", out)
	}
}
