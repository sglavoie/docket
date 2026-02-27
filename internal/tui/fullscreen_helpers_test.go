package tui

import (
	"strings"
	"testing"
)

func TestRenderScrollbar_ContentFits(t *testing.T) {
	// When totalItems <= viewportHeight, no scrollbar needed
	if got := RenderScrollbar(5, 10, 0); got != "" {
		t.Errorf("expected empty string for content that fits, got %q", got)
	}
	if got := RenderScrollbar(10, 10, 0); got != "" {
		t.Errorf("expected empty string when exactly equal, got %q", got)
	}
}

func TestRenderScrollbar_ZeroAndEdgeCases(t *testing.T) {
	if got := RenderScrollbar(0, 10, 0); got != "" {
		t.Errorf("expected empty for 0 items, got %q", got)
	}
	if got := RenderScrollbar(10, 0, 0); got != "" {
		t.Errorf("expected empty for 0 viewport, got %q", got)
	}
	if got := RenderScrollbar(1, 1, 0); got != "" {
		t.Errorf("expected empty for 1 item in 1 viewport, got %q", got)
	}
}

func TestRenderScrollbar_LineCount(t *testing.T) {
	result := RenderScrollbar(20, 10, 0)
	lines := strings.Split(result, "\n")
	if len(lines) != 10 {
		t.Errorf("expected 10 lines, got %d", len(lines))
	}
}

func TestRenderScrollbar_MinThumbSize(t *testing.T) {
	// With very large totalItems, thumb should still be at least 1 line
	result := RenderScrollbar(1000, 5, 0)
	lines := strings.Split(result, "\n")

	thumbCount := 0
	for _, line := range lines {
		if strings.Contains(line, "█") {
			thumbCount++
		}
	}
	if thumbCount < 1 {
		t.Errorf("expected at least 1 thumb line, got %d", thumbCount)
	}
}

func TestRenderScrollbar_ThumbAtTop(t *testing.T) {
	result := RenderScrollbar(20, 10, 0)
	lines := strings.Split(result, "\n")
	// First line should be thumb (at top position)
	if !strings.Contains(lines[0], "█") {
		t.Errorf("expected thumb at top when scrollOffset=0, first line: %q", lines[0])
	}
}

func TestRenderScrollbar_ThumbAtBottom(t *testing.T) {
	result := RenderScrollbar(20, 10, 10) // scrollOffset = totalItems - viewportHeight
	lines := strings.Split(result, "\n")
	// Last line should be thumb
	if !strings.Contains(lines[len(lines)-1], "█") {
		t.Errorf("expected thumb at bottom when at max scroll, last line: %q", lines[len(lines)-1])
	}
}

func TestRenderScrollbar_ThumbInMiddle(t *testing.T) {
	result := RenderScrollbar(30, 10, 10) // middle scroll position
	lines := strings.Split(result, "\n")

	// Find thumb lines
	firstThumb := -1
	lastThumb := -1
	for i, line := range lines {
		if strings.Contains(line, "█") {
			if firstThumb == -1 {
				firstThumb = i
			}
			lastThumb = i
		}
	}

	if firstThumb <= 0 || lastThumb >= len(lines)-1 {
		t.Errorf("expected thumb in middle region, got firstThumb=%d lastThumb=%d (total lines=%d)",
			firstThumb, lastThumb, len(lines))
	}
}

func TestRenderScrollbar_ViewportOne(t *testing.T) {
	result := RenderScrollbar(5, 1, 0)
	lines := strings.Split(result, "\n")
	if len(lines) != 1 {
		t.Errorf("expected 1 line for viewport=1, got %d", len(lines))
	}
	if !strings.Contains(lines[0], "█") {
		t.Errorf("expected thumb on the single line")
	}
}

func TestAttachScrollbar_EmptyScrollbar(t *testing.T) {
	content := "line1\nline2\nline3"
	result := AttachScrollbar(content, "")
	if result != content {
		t.Errorf("expected content unchanged when scrollbar empty, got %q", result)
	}
}

func TestAttachScrollbar_ZipsLines(t *testing.T) {
	content := "aaa\nbbb\nccc"
	scrollbar := "X\nY\nZ"
	result := AttachScrollbar(content, scrollbar)
	lines := strings.Split(result, "\n")
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines, got %d", len(lines))
	}
	expected := []string{"aaaX", "bbbY", "cccZ"}
	for i, exp := range expected {
		if lines[i] != exp {
			t.Errorf("line %d: expected %q, got %q", i, exp, lines[i])
		}
	}
}
