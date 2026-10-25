package taskform

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/docket/internal/app"
	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/tui/grammar"
	"github.com/sglavoie/docket/internal/tui/theme"
)

func TestTaskComposerHeaderShowsEffectiveRoute(t *testing.T) {
	inbox := &models.Project{ID: "inbox", Name: "Inbox", Slug: "inbox"}
	target := &models.Project{ID: "tt", Name: "tt", Slug: "tt"}
	composer := NewTaskComposerModel(TaskComposerPreset{
		Kind:               ComposerGlobal,
		Project:            inbox,
		AllProjects:        []*models.Project{inbox, target},
		AllowProjectSwitch: true,
		Shared:             TaskComposerSharedAttributes{Destination: app.CaptureDestinationInbox},
	}, nil)
	composer.SetSize(80, 30)
	composer.Input.SetValue("Write regression test %tt $doing")
	composer.UpdatePreview()

	project, plan, status := composer.effectiveRouteHeader().strings()
	if project != "tt" || plan != "" || status != "Doing" {
		t.Fatalf("effective header = %q / %q / %q, want tt / no plan / Doing", project, plan, status)
	}
	plain := ansi.Strip(composer.View().Content)
	if strings.Contains(plain, "Preset:") || !strings.Contains(plain, "Status: Doing") {
		t.Fatalf("composer retained launch preset metadata:\n%s", plain)
	}
	// A lifecycle destination has no membership to name, so the row spends no
	// room saying it has none.
	if strings.Contains(plain, "Plan:") {
		t.Fatalf("the route row names a plan for a task that is in none:\n%s", plain)
	}
}

// TestTaskComposerRouteRowSeparatesPlanFromStatus is the session's own contract:
// one planning sigil decides two axes, and the row states both. `$today` is the
// case that cannot be read off the token, since the status it lands in is a word
// the token never spells.
func TestTaskComposerRouteRowSeparatesPlanFromStatus(t *testing.T) {
	project := &models.Project{ID: "project", Name: "Project", Slug: "project"}
	composer := NewTaskComposerModel(TaskComposerPreset{
		Kind:        ComposerProject,
		Project:     project,
		AllProjects: []*models.Project{project},
		Shared:      TaskComposerSharedAttributes{Destination: app.CaptureDestinationInbox},
	}, nil)
	composer.SetSize(80, 30)
	composer.Input.SetValue("Plan this one $today")
	composer.UpdatePreview()

	if _, plan, status := composer.effectiveRouteHeader().strings(); plan != "Today" || status != "Backlog" {
		t.Fatalf("effective header plan/status = %q / %q, want Today / Backlog", plan, status)
	}
	plain := ansi.Strip(composer.View().Content)
	if !strings.Contains(plain, "Plan: Today · Status: Backlog") {
		t.Fatalf("the route row does not report the plan beside the status:\n%s", plain)
	}
}

func TestTaskComposerRetainsCtrlPProjectCycling(t *testing.T) {
	alpha := &models.Project{ID: "alpha", Name: "Alpha", Slug: "alpha"}
	beta := &models.Project{ID: "beta", Name: "Beta", Slug: "beta"}
	composer := NewTaskComposerModel(TaskComposerPreset{
		Kind:               ComposerGlobal,
		Project:            alpha,
		AllProjects:        []*models.Project{alpha, beta},
		AllowProjectSwitch: true,
	}, nil)

	composer.Update(syntheticCtrlKey('p'))
	if composer.Project == nil || composer.Project.ID != beta.ID {
		t.Fatalf("composer Ctrl+P project = %#v, want Beta", composer.Project)
	}
}

func TestTaskComposerDueTodayRoutesCaptureToToday(t *testing.T) {
	project := &models.Project{ID: "project", Name: "Project", Slug: "project"}
	composer := NewTaskComposerModel(TaskComposerPreset{
		Kind:        ComposerProject,
		Project:     project,
		AllProjects: []*models.Project{project},
		Shared: TaskComposerSharedAttributes{
			Destination: app.CaptureDestinationBacklog,
		},
	}, nil)
	composer.Input.SetValue("Due today @today")

	requests, ok := composer.ParseCompactLines()
	if !ok || len(requests) != 1 {
		t.Fatalf("parseCompactLines = %+v, %v", requests, ok)
	}
	if requests[0].Destination != app.CaptureDestinationToday {
		t.Fatalf("destination = %q, want Today", requests[0].Destination)
	}
	// The due date routes the capture, so the row reports the plan that routing
	// produces and the Backlog status it carries, not the `@today` that caused it.
	_, plan, status := composer.effectiveRouteHeader().strings()
	if plan != "Today" || status != "Backlog" {
		t.Fatalf("displayed plan/status = %q / %q, want Today / Backlog", plan, status)
	}
}

func TestTaskComposerHeaderAggregatesMultilineRoutes(t *testing.T) {
	inbox := &models.Project{ID: "inbox", Name: "Inbox", Slug: "inbox"}
	target := &models.Project{ID: "tt", Name: "tt", Slug: "tt"}
	composer := NewTaskComposerModel(TaskComposerPreset{
		Kind:               ComposerGlobal,
		Project:            inbox,
		AllProjects:        []*models.Project{inbox, target},
		AllowProjectSwitch: true,
		Shared:             TaskComposerSharedAttributes{Destination: app.CaptureDestinationInbox},
	}, nil)

	// Each axis answers for itself across the batch, so two lines can agree about
	// their status and disagree about their plan: `$today` and `$next` both land
	// in Backlog and belong to different collections.
	tests := []struct {
		name        string
		input       string
		wantProject string
		wantPlan    string
		wantStatus  string
	}{
		{"uniform", "One %tt $doing\nTwo %tt $doing", "tt", "", "Doing"},
		{"mixed project", "One %tt $doing\nTwo $doing", "Mixed", "", "Doing"},
		{"mixed status", "One %tt $doing\nTwo %tt $waiting", "tt", "", "Mixed"},
		{"both mixed", "One %tt $doing\nTwo $waiting", "Mixed", "", "Mixed"},
		{"uniform plan", "One %tt $today\nTwo %tt $today", "tt", "Today", "Backlog"},
		{"mixed plan alone", "One %tt $today\nTwo %tt $next", "tt", "Mixed", "Backlog"},
		{"plan absent on one line", "One %tt $today\nTwo %tt $backlog", "tt", "Mixed", "Backlog"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			composer.Input.SetValue(tt.input)
			project, plan, status := composer.effectiveRouteHeader().strings()
			if project != tt.wantProject || plan != tt.wantPlan || status != tt.wantStatus {
				t.Fatalf("effective header = %q / %q / %q, want %q / %q / %q",
					project, plan, status, tt.wantProject, tt.wantPlan, tt.wantStatus)
			}
		})
	}
}

func TestChildComposerDuplicateHintUsesImplicitParent(t *testing.T) {
	parentID := "parent"
	project := &models.Project{ID: "web", Slug: "web", Tasks: []*models.Task{
		{ID: parentID, ShortID: 1, Title: "Release", WorkflowState: models.StateDoing},
		{ID: "root", ShortID: 2, Title: "Fix login", WorkflowState: models.StateBacklog},
		{ID: "sibling", ShortID: 3, Title: "Fix login", WorkflowState: models.StateWaiting, ParentID: &parentID},
	}}
	composer := NewTaskComposerModel(TaskComposerPreset{
		Kind: ComposerChild, Project: project, InitialText: "Fix login",
		Shared: TaskComposerSharedAttributes{ParentID: &parentID},
	}, nil)
	composer.SetSize(100, 30)
	if view := ansi.Strip(composer.View().Content); !strings.Contains(view, "Matching title: web#3") {
		t.Fatalf("child capture did not prefer its sibling: %s", view)
	}
}

func TestTaskComposerPresetModes(t *testing.T) {
	runtime, project := newTestRuntimeWithProject()
	task := project.Tasks[0]
	parentID := task.ID
	tests := []struct {
		name       string
		preset     TaskComposerPreset
		wantMode   TaskComposerMode
		wantKind   TaskComposerPresetKind
		wantParent bool
	}{
		{"global", TaskComposerPreset{Kind: ComposerGlobal, Project: project}, ComposerCompact, ComposerGlobal, false},
		{"project", TaskComposerPreset{Kind: ComposerProject, Project: project}, ComposerCompact, ComposerProject, false},
		{"child", TaskComposerPreset{Kind: ComposerChild, Project: project, Shared: TaskComposerSharedAttributes{ParentID: &parentID}}, ComposerCompact, ComposerChild, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			composer := NewTaskComposerModel(tt.preset, runtime)
			if composer.Mode != tt.wantMode || composer.Preset.Kind != tt.wantKind {
				t.Fatalf("mode/kind = %v/%v, want %v/%v", composer.Mode, composer.Preset.Kind, tt.wantMode, tt.wantKind)
			}
			if tt.wantParent && (composer.Preset.Shared.ParentID == nil || *composer.Preset.Shared.ParentID != task.ID) {
				t.Fatal("child preset lost its typed parent")
			}
		})
	}
}

// TestTaskComposerRouteRowColorsEachAxis holds the row to the authorities it
// borrows from: the project wears its own slot color, the plan and the status
// their mode-badge accents, and the stripped text is what it always was.
func TestTaskComposerRouteRowColorsEachAxis(t *testing.T) {
	home := &models.Project{ID: "home", Name: "Home", Slug: "home", ColorSlot: 1}
	proj := &models.Project{ID: "proj", Name: "Proj", Slug: "proj", ColorSlot: 3}
	composer := NewTaskComposerModel(TaskComposerPreset{
		Kind:               ComposerGlobal,
		Project:            home,
		AllProjects:        []*models.Project{home, proj},
		AllowProjectSwitch: true,
		Shared:             TaskComposerSharedAttributes{Destination: app.CaptureDestinationInbox},
	}, nil)
	composer.SetSize(80, 30)
	composer.Input.SetValue("Colored %proj $next #x")
	composer.UpdatePreview()

	row := composer.renderComposerRouteRow(80)
	for name, want := range map[string]string{
		"project": theme.ProjectColorStyle(proj).Render("Proj"),
		"plan":    grammar.CapturePlanAccentStyle(app.CaptureDestinationNext).Render("Next"),
		"status":  grammar.CaptureStatusAccentStyle(models.StateBacklog).Render("Backlog"),
	} {
		if want == ansi.Strip(want) {
			t.Fatalf("the %s style emits no color; the test proves nothing", name)
		}
		if !strings.Contains(row, want) {
			t.Errorf("route row does not render the %s as %q:\n%q", name, want, row)
		}
	}
	if got := ansi.Strip(row); got != "Project: Proj · Plan: Next · Status: Backlog" {
		t.Fatalf("stripped route row = %q", got)
	}

	composer.Input.SetValue("One %proj $doing\nTwo $waiting")
	row = composer.renderComposerRouteRow(80)
	if !strings.Contains(row, theme.MutedStyle.Render("Mixed")) {
		t.Fatalf("a mixed axis is not muted:\n%q", row)
	}
	if got := ansi.Strip(row); got != "Project: Mixed · Status: Mixed" {
		t.Fatalf("stripped mixed route row = %q", got)
	}
}

func TestBatchCaptureDestinationSummaryFollowsDraftAndProject(t *testing.T) {
	home := &models.Project{ID: "home", Slug: "unfiled", Name: "Unfiled"}
	web := &models.Project{ID: "web", Slug: "web", Name: "Web"}
	m := NewTaskComposerModel(TaskComposerPreset{Project: home, AllProjects: []*models.Project{home, web}, AllowProjectSwitch: true}, nil)
	m.SetSize(60, 18)
	m.Update(tea.PasteMsg{Content: "First %web\n\nSecond\nThird %web"})
	m.AC.Clear()
	preview := ansi.Strip(m.renderComposerPanelPreview(4))
	if !strings.Contains(preview, "To: web 2 · unfiled 1") {
		t.Fatalf("wrong destinations: %s", preview)
	}
	for _, line := range strings.Split(preview, "\n") {
		if lipgloss.Width(line) > m.Width {
			t.Fatalf("overflow: %q", line)
		}
	}
	m.Update(syntheticCtrlKey('p'))
	if got := m.batchDestinationSummary(); got != "To: web 3" {
		t.Fatalf("project switch left stale summary: %q", got)
	}
	// One project is what the route row already names, so the counts give
	// their row back, in the measurement and in the render alike.
	m.AC.Clear()
	if m.batchSpansProjects() {
		t.Fatalf("a single-project batch still claims a destination row: %v", m.batchDestinations)
	}
	single := ansi.Strip(m.renderComposerPanelPreview(m.composerPanelPaneContentRows()))
	if strings.Contains(single, "To: ") {
		t.Fatalf("a single-project batch repeats the route row: %s", single)
	}
	if got, want := lipgloss.Height(single), m.composerPanelPaneContentRows(); got != want {
		t.Fatalf("pane measured %d rows and rendered %d: %s", want, got, single)
	}
	m.Update(tea.PasteMsg{Content: "\nInvalid %missing"})
	m.AC.Clear()
	if len(m.batchDestinations) != 0 || m.batchInvalidLines != 1 {
		t.Fatalf("invalid batch claimed complete destinations: %v", m.batchDestinations)
	}
	if requests, ok := m.ParseCompactLines(); ok || requests != nil {
		t.Fatal("invalid batch became partially submittable")
	}
}

func TestBatchDestinationSummaryNamesOverflow(t *testing.T) {
	m := NewTaskComposerModel(TaskComposerPreset{}, nil)
	m.SetSize(60, 18)
	m.batchDestinations = []string{strings.Repeat("x", 35) + " 1", "another-project 1", "third 2"}
	text := m.batchDestinationSummary()
	if !strings.Contains(text, "+2 projects") || lipgloss.Width(text) > m.Width {
		t.Fatalf("unbounded or silent overflow: %q", text)
	}
}

func TestBatchPlanSummaryUsesResolvedCaptureRequests(t *testing.T) {
	project := &models.Project{ID: "home", Slug: "unfiled", Name: "Unfiled"}
	for _, tc := range []struct {
		name, draft, want string
	}{
		{"mixed", "One $today\nTwo $next\nThree $backlog\nFour $today", "Plan: Today 2 · Next 1 · Unplanned 1"},
		{"due and defer", "One @today\nTwo $next @@tomorrow", "Plan: Today 1 · Unplanned 1"},
		{"default", "One\nTwo $next", "Plan: Today 1 · Next 1"},
		{"same plan", "One $today\nTwo $today", ""},
		{"only states", "One $doing\nTwo $waiting", ""},
		{"invalid", "One $today\nTwo !4", ""},
		{"single", "One $today", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := NewTaskComposerModel(TaskComposerPreset{Project: project, Shared: TaskComposerSharedAttributes{Destination: app.CaptureDestinationToday}}, nil)
			m.SetSize(60, 24)
			m.Update(tea.PasteMsg{Content: tc.draft})
			m.AC.Clear()
			if m.batchPlanSummary != tc.want {
				t.Fatalf("summary = %q, want %q", m.batchPlanSummary, tc.want)
			}
			if tc.want == "" {
				return
			}
			if view := ansi.Strip(m.renderComposerPanelPreview(5)); !strings.Contains(view, tc.want) {
				t.Fatalf("roomy preview omitted plans: %s", view)
			}
			if view := ansi.Strip(m.renderComposerPanelPreview(2)); strings.Contains(view, tc.want) {
				t.Fatalf("plans displaced the task preview: %s", view)
			}
			m.Width = 20
			if view := ansi.Strip(m.renderComposerPanelPreview(5)); strings.Contains(view, "Plan: ") {
				t.Fatalf("narrow preview squeezed in plans: %s", view)
			}
			m.Input.SetValue("One $next\nTwo $next")
			m.refreshBatchDiagnostics()
			if m.batchPlanSummary != "" {
				t.Fatal("editing retained mixed plan counts")
			}
		})
	}
}
