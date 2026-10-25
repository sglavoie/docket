package tui

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"

	"github.com/sglavoie/docket/internal/tui/textfit"
)

func TestTruncateToVisualWidthUsesDisplayWidth(t *testing.T) {
	truncated := textfit.TruncateToVisualWidth("修复界面布局问题", 8)

	if lipgloss.Width(truncated) > 8 {
		t.Fatalf("TruncateToVisualWidth exceeded width: %q (width=%d)", truncated, lipgloss.Width(truncated))
	}
	if !strings.HasSuffix(truncated, "…") {
		t.Fatalf("expected ellipsis for truncated text, got %q", truncated)
	}
	if lipgloss.Width("修复界面") != 8 {
		t.Fatalf("test assumption failed: expected sample text width 8")
	}
}
