package tui

import (
	"strconv"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/models"
)

func TestOperationalTitlePulseFittingOrder(t *testing.T) {
	pulse := WorkspacePulse{Inbox: 12, Today: 0, Overdue: 4}
	allMode, _ := OutlinerModeByID(ModeAll)
	spec := operationalTitleSpec{
		Surface: TitleStyle.Render("Outliner"),
		Scope:   TitleStyle.Render(activeProjectsScopeLabel),
		Mode:    operationalTitleModeSegment(allMode),
		Pulse:   &pulse,
	}
	tests := []struct {
		width       int
		wantInbox   bool
		wantToday   bool
		wantOverdue bool
	}{
		{width: 40},
		{width: 60, wantToday: true},
		{width: 72, wantToday: true, wantOverdue: true},
		{width: 100, wantInbox: true, wantToday: true, wantOverdue: true},
		{width: 160, wantInbox: true, wantToday: true, wantOverdue: true},
	}

	for _, tt := range tests {
		t.Run(strconv.Itoa(tt.width), func(t *testing.T) {
			rendered := renderOperationalTitle(spec, tt.width)
			plain := ansi.Strip(rendered)
			if strings.Contains(rendered, "\n") {
				t.Fatalf("title wrapped structurally at width %d: %q", tt.width, plain)
			}
			if got := lipgloss.Width(rendered); got > tt.width {
				t.Fatalf("title width = %d, exceeds %d: %q", got, tt.width, plain)
			}
			for token, want := range map[string]bool{
				"inbox 12":  tt.wantInbox,
				"today 0":   tt.wantToday,
				"overdue 4": tt.wantOverdue,
			} {
				if got := strings.Contains(plain, token); got != want {
					t.Errorf("width %d token %q present = %v, want %v: %q", tt.width, token, got, want, plain)
				}
			}
		})
	}
}

func TestOperationalTitlePreservesInteractionBeforePulse(t *testing.T) {
	pulse := WorkspacePulse{Inbox: 9, Today: 8, Overdue: 7}
	rendered := renderOperationalTitle(operationalTitleSpec{
		Surface: "Outliner",
		Scope:   activeProjectsScopeLabel,
		Mode:    "All",
		Context: []string{SelectionCountStyle.Render("MULTI-SELECT: 3 selected")},
		Pace:    &todayPaceReadout{openCount: 8, medianPerDay: 3},
		Pulse:   &pulse,
	}, 40)
	plain := ansi.Strip(rendered)

	if lipgloss.Width(rendered) > 40 || strings.Contains(rendered, "\n") {
		t.Fatalf("narrow interaction title did not fit one row: width=%d %q", lipgloss.Width(rendered), plain)
	}
	if !strings.Contains(plain, "MULTI-SELECT") {
		t.Fatalf("interaction context was removed before pulse: %q", plain)
	}
	if strings.Contains(plain, "workspace:") {
		t.Fatalf("passive pulse survived ahead of interaction context: %q", plain)
	}
	if strings.Contains(plain, "median") || strings.Contains(plain, "/day") {
		t.Fatalf("Today pace survived ahead of interaction context: %q", plain)
	}
}

func TestOperationalTitleTodayPaceFittingOrder(t *testing.T) {
	pulse := WorkspacePulse{Inbox: 9, Today: 8, Overdue: 7}
	pace := todayPaceReadout{openCount: 8, medianPerDay: 1.5}
	spec := operationalTitleSpec{Scope: activeProjectsScopeLabel, Mode: "Today", Pace: &pace, Pulse: &pulse}
	identity := renderOperationalIdentity(spec, 200)
	paceVariants := operationalPaceVariants(pace)
	fullBase := joinOperationalTitleRegions(identity, paceVariants[0])
	mediumBase := joinOperationalTitleRegions(identity, paceVariants[1])
	shortBase := joinOperationalTitleRegions(identity, paceVariants[2])
	fullPulse := operationalPulseVariants(pulse)[0]

	tests := []struct {
		name          string
		width         int
		want          string
		forbid        []string
		wantWorkspace bool
	}{
		{
			name:          "full pace and pulse",
			width:         lipgloss.Width(fullBase) + operationalTitlePulseGap + lipgloss.Width(fullPulse),
			want:          "today 8 · median 1.5/day (30d)",
			wantWorkspace: true,
		},
		{name: "full pace after pulse", width: lipgloss.Width(fullBase), want: "today 8 · median 1.5/day (30d)", forbid: []string{"workspace:"}},
		{name: "shorten window", width: lipgloss.Width(mediumBase), want: "today 8 · median 1.5/day", forbid: []string{"(30d)", "workspace:"}},
		{name: "shorten label", width: lipgloss.Width(shortBase), want: "today 8 · 1.5/day", forbid: []string{"median", "workspace:"}},
		{name: "omit pace", width: lipgloss.Width(identity), want: "Active Projects · Today", forbid: []string{"/day", "workspace:"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rendered := renderOperationalTitle(spec, tt.width)
			plain := ansi.Strip(rendered)
			if got := lipgloss.Width(rendered); got > tt.width {
				t.Fatalf("title width = %d, exceeds %d: %q", got, tt.width, plain)
			}
			if !strings.Contains(plain, tt.want) {
				t.Fatalf("title missing %q: %q", tt.want, plain)
			}
			if got := strings.Contains(plain, "workspace:"); got != tt.wantWorkspace {
				t.Fatalf("workspace pulse present = %v, want %v: %q", got, tt.wantWorkspace, plain)
			}
			for _, forbidden := range tt.forbid {
				if strings.Contains(plain, forbidden) {
					t.Fatalf("title contains forbidden %q: %q", forbidden, plain)
				}
			}
		})
	}
}

func TestOutlinerOperationalTitleScenariosFitSupportedWidths(t *testing.T) {
	task := &models.Task{ID: "launch", ShortID: 1, Title: "Launch", WorkflowState: models.StateBacklog}
	project := scopeTestProject("alpha", "Alpha", task.ID)
	project.Tasks[0].Title = task.Title
	project.BuildTaskTree()

	newOutliner := func() *OutlinerModel {
		outliner := scopeTestOutliner(project)
		outliner.workspacePulse = WorkspacePulse{Inbox: 12, Today: 3, Overdue: 4}
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
			m.crossFocusStack = []CrossProjectFocusEntry{{ProjectID: project.ID, TaskID: task.ID}}
		}, wantNarrow: []string{"Focus:"}, wantWide: []string{"Focus: [Alpha] › " + task.Title}},
		{name: "search", configure: func(m *OutlinerModel) { m.SetSearchQuery("needle") }, wantNarrow: []string{"Search:"}, wantWide: []string{"Search: \"needle\""}},
		{name: "filters", configure: func(m *OutlinerModel) {
			m.filter = NewFilterState()
			m.filter.Priority = 1
		}, wantNarrow: []string{"Filters:"}, wantWide: []string{"Filters:", "priority:High+"}},
		{name: "multiselect", configure: func(m *OutlinerModel) {
			m.multiSelectMode = true
			m.selectedTaskIdentities = map[TaskIdentity]bool{{ProjectID: project.ID, TaskID: task.ID}: true}
		}, wantNarrow: []string{"MULTI-SELECT"}, wantWide: []string{"MULTI-SELECT: 1 selected"}},
		{name: "combined interaction context", configure: func(m *OutlinerModel) {
			m.multiSelectMode = true
			m.selectedTaskIdentities = map[TaskIdentity]bool{{ProjectID: project.ID, TaskID: task.ID}: true}
			m.SetSearchQuery("needle")
			m.filter = NewFilterState()
			m.filter.Priority = 1
			m.crossFocusStack = []CrossProjectFocusEntry{{ProjectID: project.ID, TaskID: task.ID}}
		}, wantNarrow: []string{"MULTI-SELECT"}, wantWide: []string{"MULTI-SELECT: 1 selected", "Search: \"needle\"", "Filters:", "Focus: [Alpha] › Launch"}},
	}

	for _, mode := range OutlinerModeCatalog() {
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
	for _, mode := range OutlinerModeCatalog() {
		t.Run(mode.Label, func(t *testing.T) {
			if chip := ansi.Strip(operationalTitleModeSegment(mode)); chip != mode.Label {
				t.Fatalf("mode chip = %q, want %q", chip, mode.Label)
			}
			plain := ansi.Strip(renderOperationalTitle(operationalTitleSpec{
				Surface: TitleStyle.Render("Outliner"),
				Scope:   TitleStyle.Render(activeProjectsScopeLabel),
				Mode:    operationalTitleModeSegment(mode),
				Context: []string{MutedStyle.Render(context)},
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
	m.crossFocusStack = []CrossProjectFocusEntry{{ProjectID: project.ID, TaskID: "launch"}}
	m.crossForceRebuild = true
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
