package tui

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/pulse"
	"github.com/sglavoie/tt/internal/tui/query"
	"github.com/sglavoie/tt/internal/tui/taskref"
	"github.com/sglavoie/tt/internal/tui/theme"
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
		{name: "focused project", configure: func(m *OutlinerModel) { m.scopedProjectID = project.ID }, wantNarrow: []string{project.Name}, wantWide: []string{project.Name, "All"}},
		{name: "focus", configure: func(m *OutlinerModel) {
			m.focus.crossStack = []CrossProjectFocusEntry{{ProjectID: project.ID, TaskID: task.ID}}
		}, wantNarrow: []string{"Focus:"}, wantWide: []string{"Focus: [Alpha] › " + task.Title}},
		{name: "search", configure: func(m *OutlinerModel) { m.SetSearchQuery("needle") }, wantNarrow: []string{"Search:"}, wantWide: []string{"Search: \"needle\""}},
		{name: "filters", configure: func(m *OutlinerModel) {
			m.filter = query.NewFilterState()
			m.filter.Priority = 1
			m.adoptFilterCarriers()
		}, wantNarrow: []string{"Filters:"}, wantWide: []string{"Filters:", "!1+"}},
		{name: "multiselect", configure: func(m *OutlinerModel) {
			m.MultiSelectMode = true
			m.Selected = map[taskref.TaskIdentity]bool{{ProjectID: project.ID, TaskID: task.ID}: true}
		}, wantNarrow: []string{"MULTI-SELECT"}, wantWide: []string{"MULTI-SELECT: 1 selected"}},
		{name: "combined interaction context", configure: func(m *OutlinerModel) {
			m.MultiSelectMode = true
			m.Selected = map[taskref.TaskIdentity]bool{{ProjectID: project.ID, TaskID: task.ID}: true}
			m.SetSearchQuery("needle")
			m.filter = query.NewFilterState()
			m.filter.Priority = 1
			m.adoptFilterCarriers()
			m.focus.crossStack = []CrossProjectFocusEntry{{ProjectID: project.ID, TaskID: task.ID}}
		}, wantNarrow: []string{"MULTI-SELECT"}, wantWide: []string{"MULTI-SELECT: 1 selected", "Search: \"needle\"", "Filters:", "Focus: [Alpha] › Launch"}},
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
				m.primaryMode = mode.ID
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
				tt.configure(m)
				rendered := m.buildOutlinerTitleLine(width)
				plain := ansi.Strip(rendered)
				if strings.Contains(rendered, "\n") {
					t.Fatalf("width %d produced multiple rows: %q", width, plain)
				}
				if got := lipgloss.Width(rendered); got > width {
					t.Fatalf("width %d rendered %d cells: %q", width, got, plain)
				}
				if strings.Contains(plain, "Outliner") {
					t.Fatalf("width %d kept the retired surface label: %q", width, plain)
				}
				if strings.Count(plain, m.activeOutlinerModeEntry().Label) != 1 {
					t.Fatalf("width %d mode identity count != 1: %q", width, plain)
				}
				modeLabel := m.activeOutlinerModeEntry().Label
				if strings.Contains(plain, operationalTitleSeparator+" "+modeLabel) {
					t.Fatalf("width %d mode identity has incidental leading chip padding: %q", width, plain)
				}
				if len(m.buildOutlinerInteractionContext(width)) > 0 &&
					strings.Contains(plain, modeLabel+operationalTitleContextGap+" ") {
					t.Fatalf("width %d mode identity has incidental trailing chip padding: %q", width, plain)
				}
				if width == 40 {
					for _, want := range tt.wantNarrow {
						if !strings.Contains(plain, want) {
							t.Errorf("narrow title missing %q: %q", want, plain)
						}
					}
				}
				if width == 160 {
					for _, want := range tt.wantWide {
						if !strings.Contains(plain, want) {
							t.Errorf("wide title missing %q: %q", want, plain)
						}
					}
				}
			}
		})
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

func TestOutlinerFocusContextStaysOnHeaderRow(t *testing.T) {
	project := scopeTestProject("alpha", "Alpha", "launch")
	project.Tasks[0].Title = "Launch"
	project.BuildTaskTree()
	m := scopeTestOutliner(project)
	m.SetSize(100, 20)
	m.focus.crossStack = []CrossProjectFocusEntry{{ProjectID: project.ID, TaskID: "launch"}}
	m.population.invalidate()
	m.crossProjectRebuildRows()

	rendered := m.renderOutlinerTreeContent(100)
	plain := ansi.Strip(rendered)
	lines := strings.Split(plain, "\n")
	if len(lines) == 0 || !strings.Contains(lines[0], "Focus: [Alpha] › Launch") {
		t.Fatalf("Focus context is not on the operational title row:\n%s", plain)
	}
	if strings.Count(plain, "Focus: [Alpha] › Launch") != 1 {
		t.Fatalf("Focus context was duplicated into a breadcrumb row:\n%s", plain)
	}
}
