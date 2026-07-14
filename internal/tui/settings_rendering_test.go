package tui

import (
	"strings"
	"testing"
)

func TestSettingsMinimumLayoutCustomEditorNoOverflow(t *testing.T) {
	cfg := newTestConfig()
	cfg.Machine().Editor = "nvim"

	m := NewSettingsModel(newSettingsDeps(cfg, nil))
	m.SetSize(72, 20)

	plain, issues := normalizeScreen(m.View().Content, 72, 20)
	if len(issues) > 0 {
		t.Fatalf("settings layout should fit 72x20 with a custom editor, got %+v\n\n%s", issues, plain)
	}
	if !strings.Contains(plain, "> nvim") {
		t.Fatalf("settings render should include the configured editor, got:\n%s", plain)
	}
}
