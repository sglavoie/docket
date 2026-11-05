package textfit

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
)

func TestLinkifyURLsLinksBareAddressesAndLeavesPunctuation(t *testing.T) {
	got := LinkifyURLs("See https://example.com/a. Or (https://x.dev/b) too; ftp://no")
	want := "See " + Hyperlink("https://example.com/a", "https://example.com/a") + ". Or (" +
		Hyperlink("https://x.dev/b", "https://x.dev/b") + ") too; ftp://no"
	if got != want {
		t.Fatalf("LinkifyURLs =\n%q\nwant\n%q", got, want)
	}
	if plain := "no address here"; LinkifyURLs(plain) != plain {
		t.Fatal("prose without an address must pass through unchanged")
	}
	if got := LinkifyURLs("https://en.wikipedia.org/wiki/Go_(language)"); !strings.Contains(got, "Go_(language)\x07") {
		t.Fatalf("a parenthesis the address opened must stay in it: %q", got)
	}
}

func TestWrapLinkedTextClosesEveryRowAndKeepsWidths(t *testing.T) {
	address := "https://example.com/" + strings.Repeat("segment/", 6)
	text := "read " + address + " first"
	rows := WrapLinkedText(text, 20)
	if len(rows) < 3 {
		t.Fatalf("rows = %d, want the address wrapped across several", len(rows))
	}
	plain := WrapText(text, 20)
	for i, row := range rows {
		if lipgloss.Width(row) != lipgloss.Width(plain[i]) {
			t.Fatalf("row %d width = %d, want %d", i, lipgloss.Width(row), lipgloss.Width(plain[i]))
		}
		opens := strings.Count(row, "\x1b]8;;http")
		closes := strings.Count(row, "\x1b]8;;\x07")
		if opens != closes {
			t.Fatalf("row %d is unbalanced (%d opens, %d closes): %q", i, opens, closes, row)
		}
		if strings.Contains(row, "segment") && !strings.Contains(row, "\x1b]8;;"+address+"\x07") {
			t.Fatalf("row %d carries part of the address but not the whole link: %q", i, row)
		}
	}
}
