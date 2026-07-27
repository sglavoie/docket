package tui

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

type baselineSize struct {
	label  string
	width  int
	height int
}

var baselineSizes = []baselineSize{
	{"80x24", 80, 24},
	{"100x30", 100, 30},
	{"140x40", 140, 40},
}

var narrowBaseline = baselineSize{"narrow 60x20", 60, 20}

func assertRenderedLinesWithinVisualWidth(t *testing.T, label, rendered string, width int) {
	t.Helper()
	for line := range strings.SplitSeq(strings.TrimRight(rendered, "\n"), "\n") {
		if visualWidth := lipgloss.Width(ansi.Strip(line)); visualWidth > width {
			t.Fatalf("%s line exceeds width %d: %q (width=%d)", label, width, ansi.Strip(line), visualWidth)
		}
	}
}
