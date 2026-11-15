package outliner

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// Lane names start beside the fold arrow at every width.
func TestLaneHeaderStartsAtLeftMargin(t *testing.T) {
	m := NewOutlinerModel(nil, nil, nil)
	row := OutlinerRow{IsSectionHeader: true, SectionName: "Doing", SectionCount: 3}

	for _, width := range []int{60, 100, 160} {
		line := ansi.Strip(m.RenderOutlinerSectionHeaderRow(row, false, width, 0))
		if got := lipgloss.Width(line); got != width {
			t.Fatalf("width %d: header is %d cells wide: %q", width, got, line)
		}
		if !strings.HasPrefix(line, " ▾ Doing") {
			t.Fatalf("width %d: the name should start right after the fold arrow: %q", width, line)
		}
		name := strings.Index(line, "Doing  (3)")
		if name < 0 {
			t.Fatalf("width %d: header lost its name or count: %q", width, line)
		}
		if col := lipgloss.Width(line[:name]); col != 3 {
			t.Fatalf("width %d: name starts at column %d, want %d: %q", width, col, 3, line)
		}
	}

	m.Rows = []OutlinerRow{row, {}}
	pinned := ansi.Strip(m.renderPinnedContextRow(outlinerViewport{pinnedHeader: 0, startIdx: 1, rowWidth: 60}))
	if !strings.HasPrefix(pinned, " ▾ Doing") || lipgloss.Width(pinned) != 60 {
		t.Fatalf("pinned header must keep the same left margin and row width: %q", pinned)
	}

	narrow := ansi.Strip(m.RenderOutlinerSectionHeaderRow(row, false, 16, 0))
	if !strings.HasPrefix(narrow, " ▾ Doing  (3)") {
		t.Fatalf("a row with no room for the lead keeps the name at the edge: %q", narrow)
	}
}
