package tui

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

func TestRenderHorizontalDividerRightIndicatorUsesVisualWidth(t *testing.T) {
	result := renderHorizontalDivider(12, WithRightIndicator(" 修复 "))

	if got := lipgloss.Width(result); got != 12 {
		t.Fatalf("divider width = %d, want 12 for %q", got, ansi.Strip(result))
	}
	if !strings.HasSuffix(ansi.Strip(result), " 修复 ") {
		t.Fatalf("expected right indicator at end, got %q", ansi.Strip(result))
	}
}

func TestRenderHorizontalDividerRightIndicatorAllowsStyledText(t *testing.T) {
	indicator := HelpStyle.Render(" [1/2] ")
	result := renderHorizontalDivider(12, WithRightIndicator(indicator))

	if got := lipgloss.Width(result); got != 12 {
		t.Fatalf("divider width = %d, want 12 for %q", got, ansi.Strip(result))
	}
	if !strings.HasSuffix(ansi.Strip(result), " [1/2] ") {
		t.Fatalf("expected styled right indicator at end, got %q", ansi.Strip(result))
	}
}
