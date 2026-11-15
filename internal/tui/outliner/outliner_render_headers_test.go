package outliner

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/tui/chrome"
	"github.com/sglavoie/docket/internal/tui/theme"
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

func TestLaneHeadersShareOneNeutralStyle(t *testing.T) {
	t.Cleanup(func() { theme.ApplyTheme(theme.ThemeConfig{}) })
	m := NewOutlinerModel(nil, nil, nil)
	for _, name := range []string{"dark", "light"} {
		theme.ApplyTheme(theme.ThemeConfig{Name: name})
		for _, group := range []models.WorkflowStateGroup{
			models.StateGroupInbox, models.StateGroupDoing, models.StateGroupWaiting,
			models.StateGroupBacklog, models.StateGroupToday, models.StateGroupNext,
			models.StateGroupPaused, "",
		} {
			row := OutlinerRow{IsSectionHeader: true, StateGroup: group, SectionName: group.Label(), SectionCount: 3}
			if group == "" {
				row.SectionName = "Pinned"
			}
			for _, promoted := range []string{"", reviewLaneKey} {
				row.PromotedTag = promoted
				if promoted != "" {
					row.StateGroup = ""
					row.SectionName = "Review"
				}
				want := theme.LaneHeaderStyle(row.StateGroup == models.StateGroupToday).Render(row.SectionName)
				for _, width := range []int{60, 120} {
					line := m.RenderOutlinerSectionHeaderRow(row, false, width, 0)
					if !strings.Contains(line, want) || !strings.Contains(line, theme.MutedStyle.Render("(3)")) {
						t.Fatalf("%s %s: name/count styles disagree: %q", name, row.SectionName, line)
					}
					if lipgloss.Width(line) != width {
						t.Fatalf("header width = %d, want %d", lipgloss.Width(line), width)
					}
				}
			}
		}
		for _, mode := range chrome.OutlinerTabModes() {
			got := chrome.OperationalTitleModeSegmentArriving(mode, chrome.ModeArrivalNone)
			want := theme.LaneHeaderStyle(mode.ID == chrome.ModeToday).Padding(0).Render(mode.Label)
			if got != want {
				t.Errorf("%s %s tab = %q, want %q", name, mode.Label, got, want)
			}
		}
	}
}

func TestTreeConnectorsAreMuted(t *testing.T) {
	t.Cleanup(func() { theme.ApplyTheme(theme.ThemeConfig{}) })
	m := NewOutlinerModel(nil, nil, nil)
	m.Population.Projects = []*models.Project{{ID: "colored", ColorSlot: 1}}
	for _, name := range []string{"dark", "light"} {
		theme.ApplyTheme(theme.ThemeConfig{Name: name})
		for _, projectID := range []string{"", "missing", "colored"} {
			row := OutlinerRow{ProjectID: projectID}
			style := m.treeConnectorStyle(row)
			if style.GetForeground() != theme.ColorMuted || style.GetBackground() != (lipgloss.NoColor{}) {
				t.Fatalf("%s %s: connector must be muted without its own fill", name, projectID)
			}
			for _, connector := range []string{"├──", "└──", "│"} {
				got, _ := renderTaskPrefix(connector, "", nil, style)
				if got != theme.MutedStyle.Render(connector) {
					t.Fatalf("%s: connector lost muted paint", name)
				}
			}
		}
	}
}

func TestLiftedLabelReadsAsMore(t *testing.T) {
	for _, tc := range []struct {
		counts LiftedCounts
		want   string
	}{
		{LiftedCounts{}, ""},
		{LiftedCounts{Today: 1}, "1 more in Today"},
		{LiftedCounts{Pinned: 2, Today: 1, Next: 3, Review: 4}, "2 more in Pinned · 1 more in Today · 3 more in Next · 4 more in Review"},
	} {
		if got := liftedLabel(tc.counts); got != tc.want {
			t.Errorf("label = %q, want %q", got, tc.want)
		}
	}
	line := ansi.Strip(strings.Join(chrome.RenderNowCard(chrome.NowCardSpec{Title: "Current task", More: 2}, 60), "\n"))
	if !strings.Contains(line, "2 more in Doing") || strings.Contains(line, "+2") {
		t.Fatalf("counterpart disagrees: %s", line)
	}
}
