package tui

import (
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func TestSelectedLineStripsAndRestylesContent(t *testing.T) {
	ApplyTheme(ThemeConfig{})
	t.Cleanup(func() { ApplyTheme(ThemeConfig{}) })

	styled := MutedStyle.Render("prefix") + " " + TitleStyle.Render("My task")
	selected := SelectedLine(styled, 80)

	// The output must differ from the styled input.
	if selected == styled {
		t.Fatalf("SelectedLine produced identical output:\ninput:  %q\noutput: %q", styled, selected)
	}

	// The stripped text should be preserved (no extra prefix added).
	expected := ansi.Strip(styled)
	stripped := ansi.Strip(selected)
	// Width padding may add trailing spaces; trim for text comparison.
	if len(stripped) >= len(expected) {
		stripped = stripped[:len(expected)]
	}
	if stripped != expected {
		t.Fatalf("SelectedLine changed the visible text:\nexpected: %q\ngot:      %q", expected, stripped)
	}

	// The output should contain ANSI styling.
	if selected == ansi.Strip(selected) {
		t.Fatalf("SelectedLine output has no ANSI styling applied: %q", selected)
	}
}
