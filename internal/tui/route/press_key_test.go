package route

import (
	"testing"

	tea "charm.land/bubbletea/v2"
)

// TestPressKeySpellsTheKeyItNames holds PressKey to the key strings every
// surface's key handler switches on, since a press that spelled Enter
// differently would reach a different arm, or none.
func TestPressKeySpellsTheKeyItNames(t *testing.T) {
	for _, key := range []string{"enter", "space", "up", "down", "ctrl+f", "ctrl+b"} {
		press, ok := PressKey(key)().(tea.KeyPressMsg)
		if !ok {
			t.Fatalf("PressKey(%q) delivered no key press", key)
		}
		if got := press.String(); got != key {
			t.Fatalf("PressKey(%q) delivered %q", key, got)
		}
	}
	if PressKey("") != nil || PressKey("x") != nil {
		t.Fatal("PressKey pressed a key it does not spell")
	}
}
