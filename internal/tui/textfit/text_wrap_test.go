package textfit

import (
	"testing"
	"unicode/utf8"

	"charm.land/lipgloss/v2"
)

func TestWrapTextUnicodePreservesValidUTF8AndWidth(t *testing.T) {
	lines := WrapText("修复界面布局问题", 6)
	if len(lines) == 0 {
		t.Fatal("expected wrapped lines")
	}
	for _, line := range lines {
		if !utf8.ValidString(line) {
			t.Fatalf("WrapText produced invalid UTF-8 line: %q", line)
		}
		if width := lipgloss.Width(line); width > 6 {
			t.Fatalf("WrapText line width = %d, want <= 6 for line %q", width, line)
		}
	}
}
