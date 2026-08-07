package tui

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
)

func TestTruncateToWidthUsesVisualWidth(t *testing.T) {
	input := "修复界面布局  详情说明内容"
	truncated := truncateToVisualWidth(input, 10)

	if lipgloss.Width(truncated) > 10 {
		t.Fatalf("truncateToVisualWidth exceeded visual width: %q (width=%d)", truncated, lipgloss.Width(truncated))
	}
	if !strings.HasSuffix(truncated, "…") {
		t.Fatalf("expected ellipsis for truncated text, got %q", truncated)
	}
}
