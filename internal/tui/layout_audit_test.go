package tui

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/tui/textfit"
)

func TestTruncateToWidthUsesVisualWidth(t *testing.T) {
	input := "修复界面布局  详情说明内容"
	truncated := textfit.TruncateToVisualWidth(input, 10)

	if lipgloss.Width(truncated) > 10 {
		t.Fatalf("TruncateToVisualWidth exceeded visual width: %q (width=%d)", truncated, lipgloss.Width(truncated))
	}
	if !strings.HasSuffix(truncated, "…") {
		t.Fatalf("expected ellipsis for truncated text, got %q", truncated)
	}
}
