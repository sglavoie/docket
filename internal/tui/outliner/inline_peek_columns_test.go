package outliner

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

func TestInlinePeekWideDockFlowsSectionsIntoColumns(t *testing.T) {
	task := pagedPeekTask()
	window := peekDockGeometry{bodyRows: 7}
	narrow := inlinePeekDocumentFor(inlinePeekOptions{Task: task, Indent: "  ", Width: 80, Window: window, ProjectSlug: "pulse"})
	wide := inlinePeekDocumentFor(inlinePeekOptions{Task: task, Indent: "  ", Width: 200, Window: window, ProjectSlug: "pulse"})

	if len(wide.Body) >= len(narrow.Body) {
		t.Fatalf("wide body = %d rows, want fewer than the narrow %d", len(wide.Body), len(narrow.Body))
	}
	if wide.Fingerprint != narrow.Fingerprint {
		t.Fatalf("fingerprint changed with the layout:\n%q\n%q", wide.Fingerprint, narrow.Fingerprint)
	}
	if got := ansi.Strip(wide.Body[0]); !strings.Contains(got, "Selected") {
		t.Fatalf("title row = %q, want the title spanning the top", got)
	}
	plain := ansi.Strip(strings.Join(wide.Body, "\n"))
	for _, want := range []string{"Status:", "Project:", "Context:", "Description:", "Attachments:", "Comments:", "Tags:"} {
		if !strings.Contains(plain, want) {
			t.Fatalf("wide body lost %q:\n%s", want, plain)
		}
	}
	sideBySide := false
	for _, line := range wide.Body {
		if lipgloss.Width(line) > 200 {
			t.Fatalf("row is %d cells, wider than the dock: %q", lipgloss.Width(line), ansi.Strip(line))
		}
		if strings.Count(ansi.Strip(line), ": ") >= 2 && strings.Contains(ansi.Strip(line), "   ") {
			sideBySide = true
		}
	}
	if !sideBySide {
		t.Fatalf("no row holds two columns:\n%s", plain)
	}
}

func TestInlinePeekWideDockKeepsOneColumnWhenTheBodyFits(t *testing.T) {
	task := pagedPeekTask()
	doc := inlinePeekDocumentFor(inlinePeekOptions{Task: task, Indent: "  ", Width: 200, Window: peekDockGeometry{bodyRows: 80}})
	for _, section := range doc.sections {
		if strings.HasPrefix(section.key, "columns:") {
			t.Fatal("a body that fits its window was split into columns")
		}
	}
}

func TestInlinePeekPartitionBalancesWholeSections(t *testing.T) {
	section := func(rows int) inlinePeekSection {
		return inlinePeekSection{key: "s", lines: make([]string, rows)}
	}
	runs := inlinePeekPartitionSections([]inlinePeekSection{section(1), section(1), section(4), section(2), section(2)}, 3)
	heights := make([]int, len(runs))
	for i, run := range runs {
		heights[i] = inlinePeekSectionRows(run)
	}
	if want := []int{2, 4, 4}; len(heights) != 3 || heights[0] != want[0] || heights[1] != want[1] || heights[2] != want[2] {
		t.Fatalf("heights = %v, want %v", heights, want)
	}
}
