package textfit

import (
	"testing"

	"charm.land/lipgloss/v2"
)

func TestHyperlinkIsZeroWidthAndSelfClosing(t *testing.T) {
	got := Hyperlink("https://example.com/a", "example")
	want := "\x1b]8;;https://example.com/a\x07example\x1b]8;;\x07"
	if got != want {
		t.Fatalf("Hyperlink = %q, want %q", got, want)
	}
	if w := lipgloss.Width(got); w != len("example") {
		t.Fatalf("width = %d, want %d", w, len("example"))
	}
	if Hyperlink("", "plain") != "plain" || Hyperlink("https://x", "") != "" {
		t.Fatal("an empty uri or text must pass the text through unlinked")
	}
}
