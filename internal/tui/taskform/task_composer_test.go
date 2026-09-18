package taskform

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/grammar"
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

	project, plan, status := composer.effectiveRouteHeader()
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

	if _, plan, status := composer.effectiveRouteHeader(); plan != "Today" || status != "Backlog" {
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
	_, plan, status := composer.effectiveRouteHeader()
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
			project, plan, status := composer.effectiveRouteHeader()
			if project != tt.wantProject || plan != tt.wantPlan || status != tt.wantStatus {
				t.Fatalf("effective header = %q / %q / %q, want %q / %q / %q",
					project, plan, status, tt.wantProject, tt.wantPlan, tt.wantStatus)
			}
		})
	}
}

func TestTaskComposerAutocompleteSuppressesUsedSingletonsPerLine(t *testing.T) {
	parent := &models.Task{ID: "parent", ShortID: 1, Title: "Parent"}
	project := &models.Project{ID: "project", Name: "Project", Slug: "project", Tasks: []*models.Task{parent}}
	target := &models.Project{ID: "target", Name: "Target", Slug: "target"}
	composer := NewTaskComposerModel(TaskComposerPreset{
		Kind:               ComposerGlobal,
		Project:            project,
		AllProjects:        []*models.Project{project, target},
		AllowProjectSwitch: true,
	}, nil)

	for _, input := range []string{
		"Task !1 !",
		"Task @tomorrow @",
		"Task $doing $",
		"Task %target %",
		"Task >1 >",
	} {
		composer.Input.SetValue(input)
		composer.RefreshAutocomplete()
		if composer.AC.IsActive() {
			t.Errorf("autocomplete remained active for used singleton in %q: %+v", input, composer.AC)
		}
	}

	composer.Input.SetValue("First $doing\nSecond $")
	composer.RefreshAutocomplete()
	if !composer.AC.IsActive() || composer.AC.Mode != grammar.ACModeDestination {
		t.Fatalf("singleton on another task line suppressed destination autocomplete: %+v", composer.AC)
	}
}

func TestTaskComposerAutocompleteKeepsUnusedTagsRepeatable(t *testing.T) {
	project := &models.Project{ID: "project", Name: "Project"}
	composer := NewTaskComposerModel(TaskComposerPreset{Kind: ComposerProject, Project: project}, nil)
	composer.TagCache = []string{"backend", "frontend", "urgent"}
	composer.Input.SetValue("Task #backend #")
	composer.RefreshAutocomplete()

	if !composer.AC.IsActive() || composer.AC.Mode != grammar.ACModeTag {
		t.Fatalf("repeatable tag autocomplete is inactive: %+v", composer.AC)
	}
	values := make([]string, len(composer.AC.Items))
	for i, item := range composer.AC.Items {
		values[i] = item.Value
	}
	if slices.Contains(values, "backend") || !slices.Contains(values, "frontend") || !slices.Contains(values, "urgent") {
		t.Fatalf("tag suggestions = %v, want unused tags only", values)
	}
}

// countDropdownRows counts the autocomplete rows the panel actually painted.
// Every one of them names the destination it would insert, so the marker column
// is the only thing that varies between the selected row and its neighbors.
func countDropdownRows(rendered string) int {
	count := 0
	for _, line := range strings.Split(rendered, "\n") {
		if strings.HasPrefix(strings.TrimLeft(strings.TrimSpace(line), "> "), "$") {
			count++
		}
	}
	return count
}

func TestTaskComposerAutocompleteUsesAvailableHeight(t *testing.T) {
	project := &models.Project{ID: "project", Name: "Project"}
	composer := NewTaskComposerModel(TaskComposerPreset{Kind: ComposerProject, Project: project}, nil)
	composer.SetSize(80, 30)
	composer.Input.SetValue("Task $")
	composer.UpdatePreview()
	composer.RefreshAutocomplete()

	// An open dropdown is content the panel grows for, so a frame with rows to
	// spare lists every destination and spends no row on an overflow hint.
	tall := ansi.Strip(composer.View().Content)
	tallRows := countDropdownRows(tall)
	if tallRows != len(composer.AC.Items) || strings.Contains(tall, "more") {
		t.Fatalf("panel bounded a destination list the frame had room for (%d of %d):\n%s",
			tallRows, len(composer.AC.Items), tall)
	}
	if got, want := lipgloss.Height(tall), composer.compactPanelRows(); got != want {
		t.Fatalf("panel height = %d, want the %d rows it claimed", got, want)
	}

	// The panel's bound is still the bound: once the frame cannot leave both the
	// host its rows and the list its own, the dropdown is windowed rather than
	// the panel taking what the outline needs.
	composer.SetSize(80, 12)
	short := ansi.Strip(composer.View().Content)
	shortRows := countDropdownRows(short)
	if shortRows == 0 || shortRows >= tallRows {
		t.Fatalf("dropdown did not grow with the frame: short=%d tall=%d\n%s", shortRows, tallRows, short)
	}
	if !strings.Contains(short, "more") {
		t.Fatalf("short panel did not show an overflow hint:\n%s", short)
	}
	if got, want := lipgloss.Height(short), composer.compactPanelRows(); got != want {
		t.Fatalf("panel height = %d, want the %d rows it claimed", got, want)
	}
}

// TestTaskComposerPanelIsSizedByItsContent is the session-7 shape: the panel
// opens at the three rows a capture cannot be without and grows only for what
// it actually has to draw. The host's budget is the counterpart of every one of
// those measurements, so each is asserted against what the layout left behind
// rather than against a constant.
func TestTaskComposerPanelIsSizedByItsContent(t *testing.T) {
	project := &models.Project{ID: "project", Name: "Project"}
	composer := NewTaskComposerModel(TaskComposerPreset{Kind: ComposerProject, Project: project}, nil)
	composer.SetSize(80, 30)

	blank := ansi.Strip(composer.View().Content)
	if got := lipgloss.Height(blank); got != composerPanelMinRows {
		t.Fatalf("a blank capture took %d rows, want the %d it needs:\n%s", got, composerPanelMinRows, blank)
	}
	if got, want := composer.HostedPanelLayout(80, 30).HostRows, 30-composerPanelMinRows; got != want {
		t.Fatalf("blank capture left the host %d rows, want %d", got, want)
	}

	// A draft with a preview under it earns the rows that preview needs, and the
	// host pays for exactly those.
	composer.Input.SetValue("Draft task !1 #tag")
	composer.UpdatePreview()
	drafted := ansi.Strip(composer.View().Content)
	draftedRows := lipgloss.Height(drafted)
	if draftedRows <= composerPanelMinRows {
		t.Fatalf("a draft with a preview did not grow the panel (%d rows):\n%s", draftedRows, drafted)
	}
	if got, want := composer.HostedPanelLayout(80, 30).HostRows, 30-draftedRows; got != want {
		t.Fatalf("host kept %d rows beside a %d-row panel, want %d", got, draftedRows, want)
	}

	// Multiline growth is the input's own: three lines are three rows, and the
	// text is all still there.
	composer.Input.SetValue("one\ntwo\nthree")
	composer.UpdatePreview()
	multiline := ansi.Strip(composer.View().Content)
	for _, line := range []string{"one", "two", "three"} {
		if !strings.Contains(multiline, line) {
			t.Fatalf("growth lost draft line %q:\n%s", line, multiline)
		}
	}
	if got := lipgloss.Height(multiline); got != composerPanelMinRows+3 {
		t.Fatalf("three draft lines and their summary took %d rows, want %d:\n%s",
			got, composerPanelMinRows+3, multiline)
	}

	// The ceiling holds whatever the draft says, and the host keeps its floor.
	tallDraft := make([]string, 40)
	for i := range tallDraft {
		tallDraft[i] = fmt.Sprintf("line %d", i)
	}
	composer.Input.SetValue(strings.Join(tallDraft, "\n"))
	composer.UpdatePreview()
	if got := lipgloss.Height(ansi.Strip(composer.View().Content)); got != composerPanelMaxRows {
		t.Fatalf("a forty-line draft took %d rows, want the %d-row ceiling", got, composerPanelMaxRows)
	}
	if got := composer.HostedPanelLayout(80, 30).HostRows; got != 30-composerPanelMaxRows {
		t.Fatalf("host kept %d rows under a capped panel, want %d", got, 30-composerPanelMaxRows)
	}
}

// TestTaskComposerPanelKeepsRefusalsOverDraftRows states the one priority
// inside the panel's body: a refusal names work the typist has to do, while the
// line it refuses is still scrollable inside the input.
func TestTaskComposerPanelKeepsRefusalsOverDraftRows(t *testing.T) {
	project := &models.Project{ID: "project", Name: "Project"}
	composer := NewTaskComposerModel(TaskComposerPreset{Kind: ComposerProject, Project: project}, nil)
	composer.SetSize(80, 12)

	lines := make([]string, 20)
	for i := range lines {
		lines[i] = fmt.Sprintf("line %d", i)
	}
	composer.Input.SetValue(strings.Join(lines, "\n"))
	composer.UpdatePreview()
	composer.Errors = []grammar.LineError{
		{Line: 3, Message: "unknown destination"},
		{Line: 7, Message: "priority out of range"},
	}

	rendered := ansi.Strip(composer.View().Content)
	for _, want := range []string{"Line 3: unknown destination", "Line 7: priority out of range"} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("panel dropped the refusal %q for draft rows:\n%s", want, rendered)
		}
	}
	if got, want := lipgloss.Height(rendered), composer.compactPanelRows(); got != want {
		t.Fatalf("panel height = %d, want the %d rows it claimed", got, want)
	}
}

// TestTaskComposerSyntaxGuideLeavesTheHostBudget keeps session 6's fact true of
// content sizing: the guide is read over the host rather than beside it, so it
// is absent from the measurement and closing it costs the host no scroll window.
func TestTaskComposerSyntaxGuideLeavesTheHostBudget(t *testing.T) {
	project := &models.Project{ID: "project", Name: "Project"}
	composer := NewTaskComposerModel(TaskComposerPreset{Kind: ComposerProject, Project: project}, nil)
	composer.SetSize(80, 30)
	composer.Input.SetValue("Draft task")
	composer.UpdatePreview()

	before := composer.HostedPanelLayout(80, 30)
	composer.HelpExpanded = true
	during := composer.HostedPanelLayout(80, 30)
	if during.HostDrawn || during.HostRows != before.HostRows {
		t.Fatalf("expanded guide moved the host budget: %+v want undrawn %+v", during, before)
	}
	if got, want := during.PanelRows(30), 30; got != want {
		t.Fatalf("undrawn host left the panel %d rows, want %d", got, want)
	}
	composer.HelpExpanded = false
	if after := composer.HostedPanelLayout(80, 30); after != before {
		t.Fatalf("closing the guide returned %+v, want the %+v the open borrowed from", after, before)
	}
}

func TestTaskComposerCompactPanelShape(t *testing.T) {
	project := &models.Project{ID: "project", Name: "Project"}
	composer := NewTaskComposerModel(TaskComposerPreset{Kind: ComposerProject, Project: project}, nil)
	composer.SetSize(80, 30)
	composer.Input.SetValue("Draft task")
	composer.UpdatePreview()

	rendered := ansi.Strip(composer.View().Content)
	lines := strings.Split(rendered, "\n")
	if len(lines) < 4 {
		t.Fatalf("panel is too short to read:\n%s", rendered)
	}
	// The panel leads with a divider naming the capture rather than a fullscreen
	// title row: the rows a title would spend belong to the host underneath.
	if !strings.HasPrefix(lines[0], "── Project task ") {
		t.Fatalf("panel does not lead with its labeled divider:\n%s", rendered)
	}
	if !strings.HasPrefix(lines[1], "Project: Project · Status: ") {
		t.Fatalf("panel does not state its effective route on one row:\n%s", rendered)
	}
	if !strings.Contains(lines[2], "Draft task") {
		t.Fatalf("input does not follow the route row:\n%s", rendered)
	}
	if !strings.Contains(rendered, "Task     Draft task") {
		t.Fatalf("panel does not preview what Enter would create:\n%s", rendered)
	}
	// A bounded panel spends no row separating its own blocks, so the leading
	// divider is the only rule it draws.
	for i, line := range lines[1:] {
		if strings.TrimSpace(line) == strings.Repeat("─", 80) {
			t.Fatalf("panel row %d is a second full-width rule:\n%s", i+1, rendered)
		}
	}

	height := lipgloss.Height(rendered)
	if want := composer.compactPanelRows(); height != want {
		t.Fatalf("panel height = %d, want the %d rows it claimed", height, want)
	}
	layout := composer.HostedPanelLayout(80, 30)
	if !layout.Active() || layout.HostRows+height > 30 {
		t.Fatalf("panel took %d rows beside a host of %d in a 30-row area", height, layout.HostRows)
	}
}

func TestTaskComposerHostedPanelLayout(t *testing.T) {
	project := &models.Project{ID: "project", Name: "Project"}
	composer := NewTaskComposerModel(TaskComposerPreset{Kind: ComposerProject, Project: project}, nil)
	composer.SetSize(80, 18)

	layout := composer.HostedPanelLayout(80, 18)
	if !layout.Active() || !layout.HostDrawn {
		t.Fatalf("compact capture is not a panel over a drawn host: %+v", layout)
	}
	if layout.HostRows < composerHostMinRows {
		t.Fatalf("host kept %d rows, want at least %d", layout.HostRows, composerHostMinRows)
	}

	// The syntax guide is read instead of the outline, not beside it, so it takes
	// the frame without moving the budget the host will get back.
	composer.HelpExpanded = true
	help := composer.HostedPanelLayout(80, 18)
	if help.HostDrawn || help.HostRows != layout.HostRows {
		t.Fatalf("expanded syntax guide changed the host budget: %+v want undrawn %+v", help, layout)
	}
	if got, want := help.PanelRows(18), 18; got != want {
		t.Fatalf("undrawn host left the panel %d rows, want %d", got, want)
	}
	composer.HelpExpanded = false

	// The expanded form is the composer's other presentation and owns the whole
	// content area, which it declares with the zero layout.
	composer.Mode = ComposerExpanded
	if expanded := composer.HostedPanelLayout(80, 18); expanded.Active() {
		t.Fatalf("expanded composer still claims a host: %+v", expanded)
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

func TestTaskComposerMultilineValidationIsAtomicAndUsesSourceLines(t *testing.T) {
	env := newScenarioEnv(t, "composer-atomic")
	composer := NewTaskComposerModel(TaskComposerPreset{
		Kind:    ComposerProject,
		Project: env.project,
		Shared:  TaskComposerSharedAttributes{Destination: app.CaptureDestinationInbox},
	}, nil)
	composer.SetCaptureCoordinator(env.features.Capture)
	input := "First task\n\n!1\nThird task"
	composer.Input.SetValue(input)

	if cmd := composer.CreateCompact(false); cmd != nil {
		t.Fatalf("invalid batch returned commit command %T", cmd())
	}
	if len(composer.Errors) != 1 || composer.Errors[0].Line != 3 {
		t.Fatalf("errors = %+v, want source line 3", composer.Errors)
	}
	if composer.Input.Value() != input {
		t.Fatalf("failed batch changed Input: %q", composer.Input.Value())
	}
	if got := len(env.reload(t).Tasks); got != 0 {
		t.Fatalf("atomic validation created %d tasks", got)
	}
}
