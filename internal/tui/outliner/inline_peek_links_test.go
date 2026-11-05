package outliner

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

// A bare web address in a description links to itself on every row it wraps
// across, and no row leaves a link open for the padding or border after it.
func TestInlinePeekDescriptionLinksBareAddressesRowByRow(t *testing.T) {
	address := "https://example.com/docs/" + strings.Repeat("deep/", 8) + "page"
	lines := inlinePeekDescriptionLines("Spec lives at "+address+". Read it first.", "", 40)
	if len(lines) < 3 {
		t.Fatalf("lines = %d, want the address wrapped across rows", len(lines))
	}
	linked := 0
	for i, line := range lines {
		opens := strings.Count(line, "\x1b]8;;http")
		if closes := strings.Count(line, "\x1b]8;;\x07"); opens != closes {
			t.Fatalf("row %d leaves a link open: %q", i, line)
		}
		if opens > 0 {
			linked++
			if !strings.Contains(line, "\x1b]8;;"+address+"\x07") {
				t.Fatalf("row %d links something other than the whole address: %q", i, line)
			}
		}
	}
	if linked < 2 {
		t.Fatalf("linked rows = %d, want every row the address spans", linked)
	}
	plain := ansi.Strip(strings.Join(lines, "\n"))
	if !strings.Contains(plain, "/deep/deep/page. Read it") || strings.Contains(plain, "\x1b") {
		t.Fatalf("the visible text changed: %q", plain)
	}
}
