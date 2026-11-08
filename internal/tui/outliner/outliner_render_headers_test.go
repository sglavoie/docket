package outliner

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// A lane header sets its name a quarter of the row in, behind a rule that
// starts beside the fold arrow, and gives that lead back whole on a row too
// narrow to afford it rather than squeezing the name or its count.
func TestSectionHeaderNameSitsAQuarterInBehindALeadRule(t *testing.T) {
	m := NewOutlinerModel(nil, nil, nil)
	row := OutlinerRow{IsSectionHeader: true, SectionName: "Doing", SectionCount: 3}

	for _, width := range []int{60, 100, 160} {
		line := ansi.Strip(m.RenderOutlinerSectionHeaderRow(row, false, width, 0))
		if got := lipgloss.Width(line); got != width {
			t.Fatalf("width %d: header is %d cells wide: %q", width, got, line)
		}
		if !strings.HasPrefix(line, " ▾ ─") {
			t.Fatalf("width %d: the rule should start right after the fold arrow: %q", width, line)
		}
		name := strings.Index(line, "Doing  (3)")
		if name < 0 {
			t.Fatalf("width %d: header lost its name or count: %q", width, line)
		}
		if col := lipgloss.Width(line[:name]); col != width/laneHeaderLeadDivisor+1 {
			t.Fatalf("width %d: name starts at column %d, want %d: %q", width, col, width/laneHeaderLeadDivisor+1, line)
		}
	}

	narrow := ansi.Strip(m.RenderOutlinerSectionHeaderRow(row, false, 16, 0))
	if !strings.HasPrefix(narrow, " ▾ Doing  (3)") {
		t.Fatalf("a row with no room for the lead keeps the name at the edge: %q", narrow)
	}
}
