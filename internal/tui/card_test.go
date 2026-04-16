package tui

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

func TestRenderTitledTopBorder_ExactWidth(t *testing.T) {
	borderStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FF0000"))
	titleStyle := lipgloss.NewStyle().Bold(true)

	tests := []struct {
		name  string
		title string
		width int
	}{
		{"short title wide box", titleStyle.Render("Hi"), 40},
		{"medium title", titleStyle.Render("Confirm"), 30},
		{"long title tight fit", titleStyle.Render("A Longer Title Here"), 30},
		{"unstyled title", "Plain", 25},
		{"minimum dashes", titleStyle.Render("Confirm"), 13}, // 5 + 7 + 1 = 13
		{"wide box", titleStyle.Render("X"), 80},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := renderTitledTopBorder(tt.title, tt.width, borderStyle)
			got := lipgloss.Width(result)
			if got != tt.width {
				t.Errorf("width = %d, want %d", got, tt.width)
			}
			stripped := ansi.Strip(result)
			if !strings.HasSuffix(stripped, "╮") {
				t.Errorf("expected border to end with corner char, got %q", stripped)
			}
		})
	}
}

func TestRenderTitledBottomBorder_ExactWidth(t *testing.T) {
	borderStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FF0000"))

	tests := []struct {
		name   string
		footer string
		width  int
	}{
		{"short footer", lipgloss.NewStyle().Render("Esc"), 30},
		{"long footer tight fit", lipgloss.NewStyle().Render("y confirm · n/Esc cancel"), 30},
		{"wide box", lipgloss.NewStyle().Render("footer"), 80},
		{"minimum dashes", lipgloss.NewStyle().Render("ok"), 8}, // 5 + 2 + 1 = 8
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := renderTitledBottomBorder(tt.footer, tt.width, borderStyle)
			got := lipgloss.Width(result)
			if got != tt.width {
				t.Errorf("width = %d, want %d", got, tt.width)
			}
			stripped := ansi.Strip(result)
			if !strings.HasSuffix(stripped, "╯") {
				t.Errorf("expected border to end with corner char, got %q", stripped)
			}
		})
	}
}

func TestRenderTitledBorder_OverflowSafety(t *testing.T) {
	borderStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FF0000"))
	longTitle := lipgloss.NewStyle().Bold(true).Render(strings.Repeat("X", 50))

	// Width too small for the title — safety clamp should keep it bounded
	result := renderTitledTopBorder(longTitle, 20, borderStyle)
	got := lipgloss.Width(result)
	if got > 20 {
		t.Errorf("top border overflowed: width = %d, want <= 20", got)
	}

	longFooter := lipgloss.NewStyle().Render(strings.Repeat("Y", 50))
	result = renderTitledBottomBorder(longFooter, 20, borderStyle)
	got = lipgloss.Width(result)
	if got > 20 {
		t.Errorf("bottom border overflowed: width = %d, want <= 20", got)
	}
}
