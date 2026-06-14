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

func TestApplyScrollNonPositiveContentHeight(t *testing.T) {
	lines := []string{"one", "two"}

	for _, contentHeight := range []int{0, -1} {
		if got := applyScroll(lines, 1, contentHeight); got != nil {
			t.Fatalf("contentHeight %d: expected nil visible lines, got %#v", contentHeight, got)
		}
	}
}

func TestApplyPersistentScrollNonPositiveContentHeight(t *testing.T) {
	lines := []string{"one", "two"}

	for _, contentHeight := range []int{0, -1} {
		scrollOffset := 1
		if got := applyPersistentScroll(lines, 1, contentHeight, &scrollOffset); got != nil {
			t.Fatalf("contentHeight %d: expected nil visible lines, got %#v", contentHeight, got)
		}
		if scrollOffset != 1 {
			t.Fatalf("contentHeight %d: expected scroll offset to remain unchanged, got %d", contentHeight, scrollOffset)
		}
	}
}

func TestPersistentIndexWindowKeepsViewportStable(t *testing.T) {
	tests := []struct {
		name         string
		cursorIdx    int
		totalItems   int
		viewportRows int
		scrollOffset int
		wantStart    int
		wantEnd      int
		wantOffset   int
	}{
		{
			name:         "moves down past bottom",
			cursorIdx:    3,
			totalItems:   10,
			viewportRows: 3,
			scrollOffset: 0,
			wantStart:    1,
			wantEnd:      4,
			wantOffset:   1,
		},
		{
			name:         "moves up within window",
			cursorIdx:    4,
			totalItems:   10,
			viewportRows: 3,
			scrollOffset: 3,
			wantStart:    3,
			wantEnd:      6,
			wantOffset:   3,
		},
		{
			name:         "moves up past top",
			cursorIdx:    2,
			totalItems:   10,
			viewportRows: 3,
			scrollOffset: 3,
			wantStart:    2,
			wantEnd:      5,
			wantOffset:   2,
		},
		{
			name:         "jump down keeps cursor at bottom edge",
			cursorIdx:    7,
			totalItems:   10,
			viewportRows: 3,
			scrollOffset: 0,
			wantStart:    5,
			wantEnd:      8,
			wantOffset:   5,
		},
		{
			name:         "jump to top resets offset",
			cursorIdx:    0,
			totalItems:   10,
			viewportRows: 3,
			scrollOffset: 5,
			wantStart:    0,
			wantEnd:      3,
			wantOffset:   0,
		},
		{
			name:         "empty list resets offset",
			cursorIdx:    0,
			totalItems:   0,
			viewportRows: 3,
			scrollOffset: 5,
			wantStart:    0,
			wantEnd:      0,
			wantOffset:   0,
		},
		{
			name:         "shrinking list clamps offset",
			cursorIdx:    4,
			totalItems:   5,
			viewportRows: 3,
			scrollOffset: 8,
			wantStart:    2,
			wantEnd:      5,
			wantOffset:   2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scrollOffset := tt.scrollOffset
			start, end := persistentIndexWindow(tt.cursorIdx, tt.totalItems, tt.viewportRows, &scrollOffset)
			if start != tt.wantStart || end != tt.wantEnd {
				t.Fatalf("persistentIndexWindow() = (%d, %d), want (%d, %d)", start, end, tt.wantStart, tt.wantEnd)
			}
			if scrollOffset != tt.wantOffset {
				t.Fatalf("scrollOffset = %d, want %d", scrollOffset, tt.wantOffset)
			}
		})
	}
}

func TestPersistentIndexWindowInvalidViewportLeavesOffset(t *testing.T) {
	for _, viewportRows := range []int{0, -1} {
		scrollOffset := 2
		start, end := persistentIndexWindow(3, 10, viewportRows, &scrollOffset)
		if start != 0 || end != 0 {
			t.Fatalf("viewportRows %d: got (%d, %d), want (0, 0)", viewportRows, start, end)
		}
		if scrollOffset != 2 {
			t.Fatalf("viewportRows %d: scrollOffset = %d, want 2", viewportRows, scrollOffset)
		}
	}
}

func TestPersistentLineWindowUsesPersistentScroll(t *testing.T) {
	lines := []string{"zero", "one", "two", "three", "four"}
	scrollOffset := 0

	visible, start, end := persistentLineWindow(lines, 3, 3, &scrollOffset)
	if start != 1 || end != 4 || scrollOffset != 1 {
		t.Fatalf("got start=%d end=%d offset=%d, want start=1 end=4 offset=1", start, end, scrollOffset)
	}
	if strings.Join(visible, ",") != "one,two,three" {
		t.Fatalf("visible lines = %#v", visible)
	}

	visible, start, end = persistentLineWindow(lines, 2, 3, &scrollOffset)
	if start != 1 || end != 4 || scrollOffset != 1 {
		t.Fatalf("move within window got start=%d end=%d offset=%d, want start=1 end=4 offset=1", start, end, scrollOffset)
	}
	if strings.Join(visible, ",") != "one,two,three" {
		t.Fatalf("visible lines after move within window = %#v", visible)
	}
}

func TestApplyPersistentScrollClampsNegativeOffset(t *testing.T) {
	lines := []string{"zero", "one", "two"}
	scrollOffset := -5

	visible := applyPersistentScroll(lines, 0, 2, &scrollOffset)
	if scrollOffset != 0 {
		t.Fatalf("scrollOffset = %d, want 0", scrollOffset)
	}
	if strings.Join(visible, ",") != "zero,one" {
		t.Fatalf("visible lines = %#v", visible)
	}
}

func TestPersistentProjectTreeWindowByLinesAccountsForSeparators(t *testing.T) {
	visible := []*TreeNode{
		{TopGroupIndex: 0},
		{TopGroupIndex: 0},
		{TopGroupIndex: 1},
		{TopGroupIndex: 1},
		{TopGroupIndex: 2},
	}

	scrollOffset := 0
	start, end := persistentProjectTreeWindowByLines(visible, 3, 3, &scrollOffset)
	if start != 2 || end != 4 || scrollOffset != 2 {
		t.Fatalf("moving down got start=%d end=%d offset=%d, want start=2 end=4 offset=2", start, end, scrollOffset)
	}

	start, end = persistentProjectTreeWindowByLines(visible, 2, 3, &scrollOffset)
	if start != 2 || end != 4 || scrollOffset != 2 {
		t.Fatalf("moving within window got start=%d end=%d offset=%d, want start=2 end=4 offset=2", start, end, scrollOffset)
	}

	start, end = persistentProjectTreeWindowByLines(visible, 1, 3, &scrollOffset)
	if start != 1 || end != 3 || scrollOffset != 1 {
		t.Fatalf("moving above top got start=%d end=%d offset=%d, want start=1 end=3 offset=1", start, end, scrollOffset)
	}
}

func TestPersistentProjectTreeWindowByLinesEmptyShrinkAndInvalidViewport(t *testing.T) {
	t.Run("empty list resets offset", func(t *testing.T) {
		scrollOffset := 4
		start, end := persistentProjectTreeWindowByLines(nil, 0, 3, &scrollOffset)
		if start != 0 || end != 0 || scrollOffset != 0 {
			t.Fatalf("got start=%d end=%d offset=%d, want zeros", start, end, scrollOffset)
		}
	})

	t.Run("shrinking list clamps offset", func(t *testing.T) {
		visible := []*TreeNode{
			{TopGroupIndex: 0},
			{TopGroupIndex: 0},
		}
		scrollOffset := 5
		start, end := persistentProjectTreeWindowByLines(visible, 1, 3, &scrollOffset)
		if start != 0 || end != 2 || scrollOffset != 0 {
			t.Fatalf("got start=%d end=%d offset=%d, want start=0 end=2 offset=0", start, end, scrollOffset)
		}
	})

	t.Run("invalid viewport leaves offset", func(t *testing.T) {
		visible := []*TreeNode{{TopGroupIndex: 0}}
		scrollOffset := 2
		start, end := persistentProjectTreeWindowByLines(visible, 0, 0, &scrollOffset)
		if start != 0 || end != 0 || scrollOffset != 2 {
			t.Fatalf("got start=%d end=%d offset=%d, want start=0 end=0 offset=2", start, end, scrollOffset)
		}
	})
}
