package help

import (
	"strings"
	"testing"
	"unicode/utf8"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/tui/operation"
	"github.com/sglavoie/tt/internal/tui/theme"
)

func TestHelpRenderKeepsSetStateChordOnOneLine(t *testing.T) {
	strategy := operation.SimplifiedInputStrategy()
	key := operation.DisplayKeysForOperation(strategy, operation.OperationTaskSetState)
	if !strings.HasPrefix(key, "w") {
		t.Fatalf("Set State key = %q, want the w chord prefix", key)
	}
	for _, binding := range strategy.Bindings {
		if binding.OperationID != operation.OperationTaskSetState || len(binding.Sequence) != 2 {
			continue
		}
		if letter := binding.Sequence[1]; !strings.Contains(key, letter) {
			t.Fatalf("Set State key %q omits the w%s terminal", key, letter)
		}
	}

	items := []operation.HelpItem{
		{Key: key, Description: "Set State", Category: "Status"},
		{Key: "!0-3", Description: "Set Priority", Category: "Priority"},
	}

	for _, width := range []int{46, 60, 80, 160} {
		m := NewHelpViewModel(items, "Outliner")
		m.SetSize(width, 40)
		lines := strings.Split(ansi.Strip(m.renderContent()), "\n")

		keyLines := make([]string, 0, 1)
		// A wrapped chord splits at one of the slashes between its terminals, so
		// counting the lines carrying a slash is what proves it stayed whole: the
		// only other item here spells itself !0-3, which has none. The retired
		// spelling could count its own unique `$`; `w` is an ordinary letter that
		// also appears in the prose beside the column.
		fragmentLines := 0
		for _, line := range lines {
			if strings.Contains(line, key) {
				keyLines = append(keyLines, line)
			}
			if strings.Contains(line, "/") {
				fragmentLines++
			}
		}
		if len(keyLines) != 1 {
			t.Fatalf("width %d: %d lines carry the whole %q chord, want 1:\n%s",
				width, len(keyLines), key, strings.Join(lines, "\n"))
		}
		if fragmentLines != 1 {
			t.Errorf("width %d: %d lines carry a chord fragment, want the chord unsplit on 1:\n%s",
				width, fragmentLines, strings.Join(lines, "\n"))
		}
		if !strings.Contains(keyLines[0], key+"  S") {
			t.Errorf("width %d: chord line %q does not open the description", width, keyLines[0])
		}
	}
}

func TestHelpRenderKeyColumnFitsEveryComposedSpelling(t *testing.T) {
	strategy := operation.SimplifiedInputStrategy()
	seen := make(map[operation.OperationID]bool, len(strategy.Bindings))
	for _, binding := range strategy.Bindings {
		if seen[binding.OperationID] {
			continue
		}
		seen[binding.OperationID] = true
		key := operation.DisplayKeysForOperation(strategy, binding.OperationID)
		if width := lipgloss.Width(key); width > helpKeyColumnWidth {
			t.Errorf("%s displays as %q (%d cells), which wraps the %d-cell help key column",
				binding.OperationID, key, width, helpKeyColumnWidth)
		}
	}
}

func TestHelpColumnsFollowAvailableDescriptionWidth(t *testing.T) {
	m := NewHelpViewModel([]operation.HelpItem{
		{Key: "e", Description: "Edit the task title", Category: "Tasks"},
		{Key: "a", Description: "Capture a new task", Category: "Tasks"},
	}, "Outliner")
	for _, width := range []int{60, 80, 91, 92, 100, 160, 60} {
		m.SetSize(width, 20)
		plain := ansi.Strip(m.renderContent())
		together := false
		for _, line := range strings.Split(plain, "\n") {
			if lipgloss.Width(line) > m.viewport.Width() {
				t.Fatalf("width %d overflows: %q", width, line)
			}
			if strings.Contains(line, "Edit the task title") && strings.Contains(line, "Capture a new task") {
				together = true
			}
		}
		if together != (width >= 92) {
			t.Fatalf("width %d: unexpected column layout:\n%s", width, plain)
		}
		for _, text := range []string{"Edit the task title", "Capture a new task"} {
			if !strings.Contains(plain, text) {
				t.Fatalf("width %d lost description %q", width, text)
			}
		}
	}
}

func TestHelpHighlightsMatchesAfterWrappingUnicodeDescription(t *testing.T) {
	// A visible marker makes the test independent of terminal color support.
	previous := theme.HighlightStyle
	theme.HighlightStyle = lipgloss.NewStyle().Underline(true)
	t.Cleanup(func() { theme.HighlightStyle = previous })
	m := NewHelpViewModel([]operation.HelpItem{{
		Key: "e", Category: "Tasks",
		Description: "Read 界界界 café details before choosing the zebra action",
	}}, "Outliner")
	m.searchQuery = "zebra"
	m.filterItems()
	for _, width := range []int{60, 100} {
		m.SetSize(width, 20)
		rendered := m.renderContent()
		if !utf8.ValidString(rendered) {
			t.Fatalf("width %d split a Unicode character", width)
		}
		found := false
		for i, line := range strings.Split(rendered, "\n") {
			if lipgloss.Width(line) > m.viewport.Width() {
				t.Fatalf("width %d overflows: %q", width, line)
			}
			if strings.Contains(ansi.Strip(line), "zebra") {
				if i <= 1 || !strings.Contains(line, theme.HighlightStyle.Render("z")) {
					t.Fatalf("wrapped match lost highlighting: %q", line)
				}
				found = true
			}
		}
		if !found {
			t.Fatalf("missing wrapped match:\n%s", rendered)
		}
	}
}

func TestHelpAliasExplanationSurvivesWrapping(t *testing.T) {
	m := NewHelpViewModel([]operation.HelpItem{{
		Key: "g", Category: "Navigation", Description: "Move the cursor to the beginning of the outline",
		Aliases: []string{"jump to start"},
	}}, "Outliner")
	m.searchQuery = "jump"
	m.filterItems()
	for _, width := range []int{60, 100, 160} {
		m.SetSize(width, 20)
		content := ansi.Strip(m.renderContent())
		if !strings.Contains(strings.Join(strings.Fields(content), " "), "· jump to start") {
			t.Fatalf("width %d lost alias explanation:\n%s", width, content)
		}
		for _, line := range strings.Split(content, "\n") {
			if lipgloss.Width(line) > m.viewport.Width() {
				t.Fatalf("width %d overflows: %q", width, line)
			}
		}
	}
}

// outlinerHelpSections are the sections the Outliner's own `?` draws: the
// categories its registry emits, then the grammar.
var outlinerHelpSections = []string{
	"Tasks", "Sorting", "Discovery", "Modes", "Navigation", "Projects", "Notes", "System",
}

func outlinerShapedHelpItems() []operation.HelpItem {
	items := make([]operation.HelpItem, 0, len(outlinerHelpSections))
	for _, section := range outlinerHelpSections {
		items = append(items, operation.HelpItem{Key: "x", Description: section + " item", Category: section})
	}
	return append(items, GrammarHelpItems()...)
}

// helpTabBarRows renders the header and returns the tab bar's lines: the
// title is the header's first line and the bar is the rest.
func helpTabBarRows(t *testing.T, width int) []string {
	t.Helper()
	m := NewHelpViewModel(outlinerShapedHelpItems(), "Outliner")
	m.SetSize(width, 30)
	header := strings.TrimSuffix(ansi.Strip(m.renderHelpHeader(m.renderHelpTabBar())), "\n")
	return strings.Split(header, "\n")[1:]
}

func TestHelpTabBarFitsOneRowAt80Columns(t *testing.T) {
	rows := helpTabBarRows(t, 80)
	if len(rows) != 2 {
		t.Fatalf("80 columns: tab bar is %d lines, want one row of label and underline:\n%s", len(rows), strings.Join(rows, "\n"))
	}
	labels := strings.Fields(rows[0])
	want := append(append([]string{"All"}, outlinerHelpSections...), helpGrammarCategory)
	if strings.Join(labels, " ") != strings.Join(want, " ") {
		t.Errorf("80 columns: tab labels = %v, want %v", labels, want)
	}
	if !strings.Contains(rows[1], "━") {
		t.Errorf("80 columns: the active tab carries no thick underline: %q", rows[1])
	}
}

func TestHelpWrappedTabRowsShareTheFirstRowsPad(t *testing.T) {
	rows := helpTabBarRows(t, 60)
	if len(rows) < 4 {
		t.Fatalf("60 columns: tab bar did not wrap:\n%s", strings.Join(rows, "\n"))
	}
	for _, row := range rows {
		if !strings.HasPrefix(row, " ") || strings.HasPrefix(row, "  ") {
			t.Errorf("60 columns: tab bar line %q is not padded by exactly one column", row)
		}
	}
}

func TestHelpFirstSectionHasNoBlankLineAbove(t *testing.T) {
	for _, width := range []int{60, 80} {
		m := NewHelpViewModel(outlinerShapedHelpItems(), "Outliner")
		m.SetSize(width, 30)
		content := strings.Split(ansi.Strip(m.renderContent()), "\n")
		if strings.TrimSpace(content[0]) != outlinerHelpSections[0] {
			t.Errorf("width %d: first content line = %q, want the %q heading", width, content[0], outlinerHelpSections[0])
		}
		blankBeforeSecond := 0
		for i, line := range content {
			if strings.TrimSpace(line) == outlinerHelpSections[1] {
				for j := i - 1; j >= 0 && strings.TrimSpace(content[j]) == ""; j-- {
					blankBeforeSecond++
				}
				break
			}
		}
		if blankBeforeSecond == 0 {
			t.Errorf("width %d: later sections lost the margin that separates them", width)
		}
	}
}

func TestHelpTaskGroupsKeepDailyWorkAheadOfDetails(t *testing.T) {
	items := []operation.HelpItem{
		{Key: "<", Description: "Adopt Children", Category: "Tasks", Group: "Hierarchy", Order: 500},
		{Key: "!", Description: "Priority", Category: "Tasks"},
		{Key: "u", Description: "Undo", Category: "Tasks", Group: "Recovery", Order: 300},
		{Key: "wd", Description: "Done", Category: "Tasks", Group: "Status", Order: 201},
		{Key: "wt", Description: "Today", Category: "Tasks", Group: "Plan", Order: 100},
		{Key: "a", Description: "Capture", Category: "Tasks", Group: "Everyday", Order: 10},
	}
	for _, width := range []int{60, 120} {
		m := NewHelpViewModel(items, "Outliner")
		m.SetSize(width, 40)
		content := ansi.Strip(m.renderContent())
		last := -1
		for _, name := range []string{"Everyday", "Capture", "Plan", "Today", "Status", "Done", "Recovery", "Undo", "Details", "Priority", "Hierarchy", "Adopt Children"} {
			index := strings.Index(content, name)
			if index <= last {
				t.Fatalf("%d: %s out of order:\n%s", width, name, content)
			}
			last = index
		}
		m.searchQuery = "Plan"
		m.filterItems()
		if len(m.filteredItems) != 1 || m.filteredItems[0].Key != "wt" {
			t.Fatalf("group search: %+v", m.filteredItems)
		}
	}
}
