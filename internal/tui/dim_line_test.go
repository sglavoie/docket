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

func TestDimSelectedLineStripsAndRestylesContent(t *testing.T) {
	styled := InProgressStyle.Render("▶") + " " + NormalStyle.Render("My task")
	dimSelected := DimSelectedLine(styled, 80)

	// The output must differ from the styled input.
	if dimSelected == styled {
		t.Fatalf("DimSelectedLine produced identical output:\ninput:  %q\noutput: %q", styled, dimSelected)
	}

	// The stripped text should be preserved (no extra prefix added).
	expected := ansi.Strip(styled)
	stripped := ansi.Strip(dimSelected)
	// Width padding may add trailing spaces; trim for text comparison.
	if len(stripped) >= len(expected) {
		stripped = stripped[:len(expected)]
	}
	if stripped != expected {
		t.Fatalf("DimSelectedLine changed the visible text:\nexpected: %q\ngot:      %q", expected, stripped)
	}

	// The output should contain ANSI styling.
	if dimSelected == ansi.Strip(dimSelected) {
		t.Fatalf("DimSelectedLine output has no ANSI styling applied: %q", dimSelected)
	}
}

func TestDimLineStripsAndRestylesContent(t *testing.T) {
	// Render a styled string with known colors.
	styled := InProgressStyle.Render("▶") + " " + NormalStyle.Render("My task")
	dimmed := DimLine(styled)

	// The dimmed output must differ from the styled input.
	if dimmed == styled {
		t.Fatalf("DimLine produced identical output:\ninput:  %q\noutput: %q", styled, dimmed)
	}

	// The stripped text should be preserved (no extra prefix added).
	expected := ansi.Strip(styled)
	if ansi.Strip(dimmed) != expected {
		t.Fatalf("DimLine changed the visible text:\nexpected: %q\ngot:      %q", expected, ansi.Strip(dimmed))
	}

	// The dimmed output should contain ANSI styling (from lipgloss).
	if dimmed == expected {
		t.Fatalf("DimLine output has no ANSI styling applied: %q", dimmed)
	}
}
