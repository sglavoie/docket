package outliner

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/tui/chrome"
	"github.com/sglavoie/docket/internal/tui/pulse"
	"github.com/sglavoie/docket/internal/tui/query"
	"github.com/sglavoie/docket/internal/tui/taskref"
	"github.com/sglavoie/docket/internal/tui/theme"
)

// operationalTitleSeparator and operationalTitleContextGap mirror the row
// spelling internal/tui/pulse owns. Both are duplicated here rather than
// exported from that package because no production code outside it names
// either, and the two assertions below are the only readers: a change to the
// real spelling fails them by name, which is the notice this mirror exists to
// give.
const (
	operationalTitleSeparator  = " · "
	operationalTitleContextGap = "  "
)

func TestOutlinerOperationalTitleScenariosFitSupportedWidths(t *testing.T) {
	task := &models.Task{ID: "launch", ShortID: 1, Title: "Launch", WorkflowState: models.StateBacklog}
	project := scopeTestProject("alpha", "Alpha", task.ID)
	project.Tasks[0].Title = task.Title
	project.BuildTaskTree()

	newOutliner := func() *OutlinerModel {
		outliner := scopeTestOutliner(project)
		outliner.SetWorkspacePulse(pulse.WorkspacePulse{Inbox: 12, Today: 3, Overdue: 4})
		return outliner
	}

	tests := []struct {
		name       string
		configure  func(*OutlinerModel)
		wantNarrow []string
		wantWide   []string
	}{
		{name: "Active Projects", configure: func(*OutlinerModel) {}, wantNarrow: []string{activeProjectsScopeLabel}, wantWide: []string{activeProjectsScopeLabel, "All"}},
		{name: "focused project", configure: func(m *OutlinerModel) { m.Scope.ProjectID = project.ID }, wantNarrow: []string{project.Name}, wantWide: []string{project.Name, "All"}},
		{name: "focus", configure: func(m *OutlinerModel) {
			m.Focus.CrossStack = []CrossProjectFocusEntry{{ProjectID: project.ID, TaskID: task.ID}}
		}, wantNarrow: []string{"Focus:"}, wantWide: []string{"Focus: [Alpha] › " + task.Title}},
		{name: "search", configure: func(m *OutlinerModel) { m.SetSearchQuery("needle") }, wantNarrow: []string{"Search:"}, wantWide: []string{"Search: \"needle\""}},
		{name: "filters", configure: func(m *OutlinerModel) {
			m.Filter = query.NewFilterState()
			m.Filter.Priority = 1
			adoptFilterCarriers(m)
		}, wantNarrow: []string{"Filters:"}, wantWide: []string{"Filters:", "!1+"}},
		{name: "multiselect", configure: func(m *OutlinerModel) {
			m.MultiSelectMode = true
			m.Selected = map[taskref.TaskIdentity]bool{{ProjectID: project.ID, TaskID: task.ID}: true}
		}, wantNarrow: []string{"Multi-select"}, wantWide: []string{"Multi-select: 1 selected"}},
		{name: "combined interaction context", configure: func(m *OutlinerModel) {
			m.MultiSelectMode = true
			m.Selected = map[taskref.TaskIdentity]bool{{ProjectID: project.ID, TaskID: task.ID}: true}
			m.SetSearchQuery("needle")
			m.Filter = query.NewFilterState()
			m.Filter.Priority = 1
			adoptFilterCarriers(m)
			m.Focus.CrossStack = []CrossProjectFocusEntry{{ProjectID: project.ID, TaskID: task.ID}}
		}, wantNarrow: []string{"Multi-select"}, wantWide: []string{"Multi-select: 1 selected", "Search: \"needle\"", "Filters:", "Focus: [Alpha] › Launch"}},
	}

	for _, mode := range chrome.OutlinerModeCatalog() {
		mode := mode
		tests = append(tests, struct {
			name       string
			configure  func(*OutlinerModel)
			wantNarrow []string
			wantWide   []string
		}{
			name: "mode " + mode.Label,
			configure: func(m *OutlinerModel) {
				m.PrimaryMode = mode.ID
			},
			wantNarrow: []string{mode.Label},
			wantWide:   []string{mode.Label},
		})
	}

	widths := []int{40, 60, 72, 100, 160}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, width := range widths {
				m := newOutliner()
				m.SetSize(width, 30)
				tt.configure(m)
				title := m.BuildOutlinerTitleLine(width)
				context := m.BuildOutlinerContextRow(width)
				titlePlain := ansi.Strip(title)
				// The header is at most two rows, and each of them is exactly one row:
				// what the fitters give up is content, never a wrap.
				for label, row := range map[string]string{"title": title, "context": context} {
					plain := ansi.Strip(row)
					if strings.Contains(row, "\n") {
						t.Fatalf("width %d %s row produced multiple rows: %q", width, label, plain)
					}
					if got := lipgloss.Width(row); got > width {
						t.Fatalf("width %d %s row rendered %d cells: %q", width, label, got, plain)
					}
				}
				if strings.Contains(titlePlain, "Outliner") {
					t.Fatalf("width %d kept the retired surface label: %q", width, titlePlain)
				}
				modeLabel := m.ActiveOutlinerModeEntry().Label
				if strings.Count(titlePlain, modeLabel) != 1 {
					t.Fatalf("width %d mode identity count != 1: %q", width, titlePlain)
				}
				if strings.Contains(titlePlain, operationalTitleSeparator+" "+modeLabel) {
					t.Fatalf("width %d mode identity has incidental leading chip padding: %q", width, titlePlain)
				}
				if strings.HasSuffix(titlePlain, modeLabel+operationalTitleContextGap+" ") {
					t.Fatalf("width %d mode identity has incidental trailing chip padding: %q", width, titlePlain)
				}
				header := titlePlain + "\n" + ansi.Strip(context)
				if width == 40 {
					for _, want := range tt.wantNarrow {
						if !strings.Contains(header, want) {
							t.Errorf("narrow header missing %q:\n%s", want, header)
						}
					}
				}
				if width == 160 {
					for _, want := range tt.wantWide {
						if !strings.Contains(header, want) {
							t.Errorf("wide header missing %q:\n%s", want, header)
						}
					}
				}
			}
		})
	}
}

// The context row exists only while some axis is live, so a default view spends
// no second header row on saying that nothing is narrowing it — and the row it
// does spend is charged to the list exactly once.
func TestOutlinerContextRowIsAbsentFromADefaultView(t *testing.T) {
	project := scopeTestProject("alpha", "Alpha", "launch")
	project.Tasks[0].Title = "Launch"
	project.BuildTaskTree()
	m := scopeTestOutliner(project)
	m.SetSize(100, 20)
	m.Population.Invalidate()
	m.CrossProjectRebuildRows()

	if row := m.BuildOutlinerContextRow(100); row != "" {
		t.Fatalf("default view drew a context row: %q", ansi.Strip(row))
	}
	baseline := m.ContentHeight()

	m.SetSearchQuery("needle")
	if row := m.BuildOutlinerContextRow(100); row == "" {
		t.Fatalf("a live query drew no context row")
	}
	if got := m.ContentHeight(); got != baseline-1 {
		t.Fatalf("ContentHeight with a context row = %d, want %d", got, baseline-1)
	}
	if got := lipgloss.Height(m.RenderOutlinerTreeContent(100)); got > m.Height() {
		t.Fatalf("header row was not charged: frame height %d exceeds %d", got, m.Height())
	}
}

func TestOperationalTitleModeSegmentSpelling(t *testing.T) {
	const context = "Search: \"needle\""
	for _, mode := range chrome.OutlinerModeCatalog() {
		t.Run(mode.Label, func(t *testing.T) {
			if chip := ansi.Strip(chrome.OperationalTitleModeSegment(mode)); chip != mode.Label {
				t.Fatalf("mode chip = %q, want %q", chip, mode.Label)
			}
			plain := ansi.Strip(pulse.RenderOperationalTitle(pulse.OperationalTitleSpec{
				Surface: theme.TitleStyle.Render("Outliner"),
				Scope:   theme.TitleStyle.Render(activeProjectsScopeLabel),
				Mode:    chrome.OperationalTitleModeSegment(mode),
				Context: []string{theme.MutedStyle.Render(context)},
			}, 160))
			want := operationalTitleSeparator + mode.Label + operationalTitleContextGap + context
			if !strings.Contains(plain, want) {
				t.Fatalf("title = %q, want mode boundary %q", plain, want)
			}
		})
	}
}

func TestOutlinerFocusContextStaysOnTheContextRow(t *testing.T) {
	project := scopeTestProject("alpha", "Alpha", "launch")
	project.Tasks[0].Title = "Launch"
	project.BuildTaskTree()
	m := scopeTestOutliner(project)
	m.SetSize(100, 20)
	m.Focus.CrossStack = []CrossProjectFocusEntry{{ProjectID: project.ID, TaskID: "launch"}}
	m.Population.Invalidate()
	m.CrossProjectRebuildRows()

	rendered := m.RenderOutlinerTreeContent(100)
	plain := ansi.Strip(rendered)
	lines := strings.Split(plain, "\n")
	// Row 0 is identity; the context row is row 1, immediately under it and above
	// the list, which is the one place any axis is named.
	if len(lines) < 2 || !strings.Contains(lines[1], "Focus: [Alpha] › Launch") {
		t.Fatalf("Focus context is not on the context row:\n%s", plain)
	}
	if strings.Contains(lines[0], "Focus:") {
		t.Fatalf("Focus context stayed on the identity row:\n%s", plain)
	}
	if strings.Count(plain, "Focus: [Alpha] › Launch") != 1 {
		t.Fatalf("Focus context was duplicated into a breadcrumb row:\n%s", plain)
	}
}
