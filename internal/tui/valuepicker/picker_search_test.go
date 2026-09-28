package valuepicker

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/tui/chrome"
)

func TestValuePickerSearchEditingAndGuidance(t *testing.T) {
	picker := newDatePickerForTest(t, "")
	press := func(msg tea.KeyPressMsg) {
		t.Helper()
		_, dismissed, result := picker.HandleKey(msg)
		if dismissed || result != nil {
			t.Fatalf("%s unexpectedly activated the picker: %v", msg.String(), result)
		}
	}
	guidance := func() string {
		spec, _ := picker.GuidanceSpec()
		return ansi.Strip(chrome.RenderGuidanceText(spec, 180))
	}
	if !strings.Contains(guidance(), "j/k or ↑/↓") {
		t.Fatalf("browse guidance = %q", guidance())
	}
	press(tea.KeyPressMsg{Code: '/', Text: "/"})
	if got := guidance(); strings.Contains(got, "j/k") || !strings.Contains(got, "↑/↓") {
		t.Fatalf("search guidance = %q", got)
	}
	press(tea.KeyPressMsg{Code: 't', Text: "tomorow"})
	press(tea.KeyPressMsg{Code: tea.KeyLeft})
	press(tea.KeyPressMsg{Code: tea.KeyLeft})
	press(tea.KeyPressMsg{Code: 'r', Text: "r"})
	if got := picker.surface.Query().Raw; got != "tomorrow" || len(picker.surface.Matches()) == 0 {
		t.Fatalf("edited query = %q, matches = %d", got, len(picker.surface.Matches()))
	}
	press(tea.KeyPressMsg{Code: tea.KeyEnter})
	if picker.searching || !strings.Contains(guidance(), "j/k or ↑/↓") {
		t.Fatal("Enter did not restore browse guidance")
	}
	press(tea.KeyPressMsg{Code: '/', Text: "/"})
	press(tea.KeyPressMsg{Code: tea.KeyEscape})
	press(tea.KeyPressMsg{Code: 'j', Text: "jk"})
	if picker.surface.Query().Raw != "jk" {
		t.Fatal("j/k did not enter search text")
	}
	press(tea.KeyPressMsg{Code: 'x', Text: strings.Repeat("界", 80)})
	if view := picker.RenderWithin(60, 20); lipgloss.Width(view) > 60 || lipgloss.Height(view) > 20 {
		t.Fatalf("long search overflowed the picker: %dx%d", lipgloss.Width(view), lipgloss.Height(view))
	}
	press(tea.KeyPressMsg{Code: tea.KeyEscape})
	if !picker.searching || picker.surface.Query().Raw != "" {
		t.Fatal("first Esc did not clear search")
	}
	press(tea.KeyPressMsg{Code: tea.KeyEscape})
	if picker.searching {
		t.Fatal("second Esc did not leave search")
	}
}
