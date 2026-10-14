package textfit

import (
	"slices"
	"testing"
	"unicode/utf8"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
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

func TestWrapSegmentsBreaksOnlyBetweenSegments(t *testing.T) {
	t.Parallel()

	segments := []string{"Active", "3 open · 5 tasks", "slug agent-mux", "pinned"}
	got := WrapSegments(segments, " · ", 30)
	want := []string{"Active · 3 open · 5 tasks", "slug agent-mux · pinned"}
	if !slices.Equal(got, want) {
		t.Fatalf("WrapSegments = %q, want %q", got, want)
	}
	if got := WrapSegments(segments, " · ", 80); len(got) != 1 {
		t.Fatalf("WrapSegments at 80 = %q, want one line", got)
	}
}

func TestWrapSegmentsFallsBackToWrapTextForAnOversizedSegment(t *testing.T) {
	t.Parallel()

	got := WrapSegments([]string{"a", "one two three four", "b"}, " · ", 10)
	want := []string{"a", "one two", "three four", "b"}
	if !slices.Equal(got, want) {
		t.Fatalf("WrapSegments = %q, want %q", got, want)
	}
	for _, line := range got {
		if w := ansi.StringWidth(line); w > 10 {
			t.Fatalf("line %q is %d wide, want <= 10", line, w)
		}
	}
}
