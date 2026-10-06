package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestUITestKeyParsing(t *testing.T) {
	tests := []struct {
		name     string
		wantCode rune
		wantMod  tea.KeyMod
		wantText string
	}{
		{"a", 'a', 0, "a"},
		{"A", 'A', 0, "A"},
		{"1", '1', 0, "1"},
		{"!", '!', 0, "!"},
		{"enter", tea.KeyEnter, 0, ""},
		{"f10", tea.KeyF10, 0, ""},
		{"esc", tea.KeyEscape, 0, ""},
		{"escape", tea.KeyEscape, 0, ""},
		{"tab", tea.KeyTab, 0, ""},
		{"space", tea.KeySpace, 0, " "},
		{"backspace", tea.KeyBackspace, 0, ""},
		{"up", tea.KeyUp, 0, ""},
		{"down", tea.KeyDown, 0, ""},
		{"left", tea.KeyLeft, 0, ""},
		{"right", tea.KeyRight, 0, ""},
		{"pgup", tea.KeyPgUp, 0, ""},
		{"pageup", tea.KeyPgUp, 0, ""},
		{"pgdown", tea.KeyPgDown, 0, ""},
		{"pagedown", tea.KeyPgDown, 0, ""},
		{"ctrl+x", 'x', tea.ModCtrl, ""},
		{"alt+x", 'x', tea.ModAlt, ""},
		{"shift+tab", tea.KeyTab, tea.ModShift, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key, err := parseUITestKey(tt.name)
			if err != nil {
				t.Fatalf("parseUITestKey(%q) error: %v", tt.name, err)
			}
			if key.Code != tt.wantCode {
				t.Errorf("Code = %v, want %v", key.Code, tt.wantCode)
			}
			if key.Mod != tt.wantMod {
				t.Errorf("Mod = %v, want %v", key.Mod, tt.wantMod)
			}
			if key.Text != tt.wantText {
				t.Errorf("Text = %q, want %q", key.Text, tt.wantText)
			}
		})
	}
}

func TestUITestKeyErrors(t *testing.T) {
	for _, name := range []string{"", "ctrl+", "meta+a", "notakey"} {
		if _, err := parseUITestKey(name); err == nil {
			t.Errorf("parseUITestKey(%q) expected error, got nil", name)
		}
	}
}

func TestUITestKeyCombinedModifiers(t *testing.T) {
	key, err := parseUITestKey("ctrl+shift+a")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if key.Code != 'a' {
		t.Errorf("Code = %v, want 'a'", key.Code)
	}
	if key.Mod&tea.ModCtrl == 0 || key.Mod&tea.ModShift == 0 {
		t.Errorf("Mod = %v, want ctrl+shift", key.Mod)
	}
	if key.Text != "" {
		t.Errorf("Text = %q, want empty for modified key", key.Text)
	}
}
