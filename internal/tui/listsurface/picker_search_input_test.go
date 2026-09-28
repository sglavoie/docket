package listsurface

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

func TestPickerSearchEditsAtCursor(t *testing.T) {
	var input PickerSearchInput
	raw := "café界"
	for _, step := range []struct {
		key  tea.KeyPressMsg
		want string
	}{
		{tea.KeyPressMsg{Code: tea.KeyLeft}, "café界"},
		{tea.KeyPressMsg{Code: tea.KeyBackspace}, "caf界"},
		{tea.KeyPressMsg{Code: 'é', Text: "é"}, "café界"},
		{tea.KeyPressMsg{Code: tea.KeyDelete}, "café"},
		{tea.KeyPressMsg{Code: tea.KeyHome}, "café"},
		{tea.KeyPressMsg{Code: 'j', Text: "jk "}, "jk café"},
		{tea.KeyPressMsg{Code: tea.KeyEnd}, "jk café"},
		{tea.KeyPressMsg{Code: 'w', Mod: tea.ModCtrl}, "jk "},
		{tea.KeyPressMsg{Code: 'u', Mod: tea.ModCtrl}, ""},
	} {
		raw = input.HandleKey(raw, step.key)
		if raw != step.want {
			t.Fatalf("after %s: query = %q, want %q", step.key.String(), raw, step.want)
		}
		// Rendering between keystrokes must not move the editing position.
		input.View(raw, true, 30)
	}
}

func TestPickerSearchViewportFollowsCaretAndResize(t *testing.T) {
	for _, raw := range []string{
		strings.Repeat("long query ", 15) + "TAIL",
		strings.Repeat("界é", 50) + "TAIL",
	} {
		var input PickerSearchInput
		for _, width := range []int{70, 20, 40, 18} {
			view := input.View(raw, true, width)
			if lipgloss.Width(view) > width || strings.Contains(view, "\n") {
				t.Fatalf("width %d overflowed: %q", width, view)
			}
			if !strings.Contains(ansi.Strip(view), "TAIL") {
				t.Fatalf("width %d hid the text beside the caret: %q", width, view)
			}
		}
		raw = input.HandleKey(raw, tea.KeyPressMsg{Code: tea.KeyHome})
		if view := ansi.Strip(input.View(raw, true, 20)); !strings.HasPrefix(view, "Search: "+string([]rune(raw)[:3])) {
			t.Fatalf("Home did not reveal the beginning: %q", view)
		}
		if view := input.View(raw, false, 20); lipgloss.Width(view) > 20 {
			t.Fatalf("accepted query overflowed: %q", view)
		}
		// An external clear (Esc) must also reset the editor before the next key.
		if got := input.HandleKey("", tea.KeyPressMsg{Code: 'x', Text: "x"}); got != "x" {
			t.Fatalf("query after clear = %q", got)
		}
	}
}
