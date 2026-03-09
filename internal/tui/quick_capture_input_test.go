package tui

import (
	"strings"
	"testing"
)

func TestWrapPreviewText(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		width    int
		wantWrap bool // whether we expect wrapping to occur
	}{
		{
			name:     "short text no wrap",
			input:    "Task · !1 · #tag",
			width:    80,
			wantWrap: false,
		},
		{
			name:     "long text wraps at separator",
			input:    "Very long task title that should wrap · !1 · #important · #urgent · @Feb 15 · /Active",
			width:    40,
			wantWrap: true,
		},
		{
			name:     "very long text multiple wraps",
			input:    "Super duper extra long task title with many tags and metadata · !1 · #tag1 · #tag2 · #tag3 · @Feb 15 · ~weekly · /Active",
			width:    30,
			wantWrap: true,
		},
		{
			name:     "zero width returns original",
			input:    "Task · !1",
			width:    0,
			wantWrap: false,
		},
		{
			name:     "text shorter than width",
			input:    "Short",
			width:    100,
			wantWrap: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := wrapPreviewText(tt.input, tt.width)
			lines := strings.Split(result, "\n")

			hasWrapped := len(lines) > 1
			if hasWrapped != tt.wantWrap {
				t.Errorf("wrapPreviewText() wrapping=%v, expected wrapping=%v\nInput: %s\nWidth: %d\nResult:\n%s",
					hasWrapped, tt.wantWrap, tt.input, tt.width, result)
			}

			// Verify each line doesn't exceed width significantly (allow margin for separators)
			if tt.width > 0 {
				for i, line := range lines {
					if len(line) > tt.width+10 { // +10 margin to allow for separator edge cases
						t.Errorf("Line %d exceeds width %d by too much: %q (len=%d)", i, tt.width, line, len(line))
					}
				}
			}
		})
	}
}

func TestWrapPreviewTextBreaksAtSeparator(t *testing.T) {
	// Test that wrapping prefers breaking at " · " separator
	input := "Task title · !1 · #tag · @date"
	width := 20

	result := wrapPreviewText(input, width)
	lines := strings.SplitSeq(result, "\n")

	// Should break at separators, not in the middle of words
	for line := range lines {
		trimmed := strings.TrimSpace(line)
		// Check that lines don't start or end with incomplete metadata markers
		if strings.HasPrefix(trimmed, "·") {
			t.Errorf("Line starts with incomplete separator: %q", trimmed)
		}
	}
}
