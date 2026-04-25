package tui

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

func TestTruncateToVisualWidthUsesDisplayWidth(t *testing.T) {
	truncated := truncateToVisualWidth("修复界面布局问题", 8)

	if lipgloss.Width(truncated) > 8 {
		t.Fatalf("truncateToVisualWidth exceeded width: %q (width=%d)", truncated, lipgloss.Width(truncated))
	}
	if !strings.HasSuffix(truncated, "…") {
		t.Fatalf("expected ellipsis for truncated text, got %q", truncated)
	}
	if lipgloss.Width("修复界面") != 8 {
		t.Fatalf("test assumption failed: expected sample text width 8")
	}
}

func TestRenderTextAttrPreviewUsesVisualWidth(t *testing.T) {
	rendered := renderTextAttrPreview("修复属性预览在窄宽度下的宽字符截断问题", false)
	if lipgloss.Width(ansi.Strip(rendered)) > 40 {
		t.Fatalf("text attr preview exceeds width 40: %q (width=%d)", ansi.Strip(rendered), lipgloss.Width(ansi.Strip(rendered)))
	}
}

func prefixWidthBeforeToken(t *testing.T, lines []string, contains string, token string) int {
	t.Helper()

	for _, line := range lines {
		if !strings.Contains(line, contains) {
			continue
		}
		idx := strings.Index(line, token)
		if idx == -1 {
			t.Fatalf("line %q missing token %q", line, token)
		}
		return lipgloss.Width(line[:idx])
	}

	t.Fatalf("no line found containing %q", contains)
	return 0
}