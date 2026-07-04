package tui

import (
	"strings"
	"testing"

	"github.com/sglavoie/tt/internal/config"
)

func TestSettingsMinimumLayoutCustomEditorNoOverflow(t *testing.T) {
	cfg := config.NewConfig()
	cfg.Editor = "nvim"

	m := NewSettingsModel(cfg, nil)
	m.SetSize(72, 20)

	plain, issues := normalizeScreen(m.View().Content, 72, 20)
	if len(issues) > 0 {
		t.Fatalf("settings layout should fit 72x20 with a custom editor, got %+v\n\n%s", issues, plain)
	}
	if !strings.Contains(plain, "> nvim") {
		t.Fatalf("settings render should include the configured editor, got:\n%s", plain)
	}
}
