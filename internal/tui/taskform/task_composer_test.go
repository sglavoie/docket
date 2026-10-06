package taskform

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/grammar"
	"github.com/sglavoie/tt/internal/tui/theme"
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

// Esc closes the dropdown over the token it was showing, and the textarea's own
// traffic — cursor blinks and other non-editing messages — must not reopen it.
// Editing the draft or moving its caret brings suggestions back.
func TestTaskComposerEscDismissalSurvivesNonEditingMessages(t *testing.T) {
	project := &models.Project{ID: "project", Name: "Project", Slug: "project"}
	composer := NewTaskComposerModel(TaskComposerPreset{Kind: ComposerProject, Project: project}, nil)
	for _, r := range "Fix $tod" {
		composer.Update(tea.KeyPressMsg{Code: r, Text: string(r)})
	}
	if !composer.IsAutocompleteActive() {
		t.Fatal("destination autocomplete did not open for a partial $ token")
	}

	composer.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if composer.IsAutocompleteActive() {
		t.Fatal("Esc did not close the suggestions")
	}
	if composer.ConfirmDiscard {
		t.Fatal("Esc over open suggestions raised the discard question instead of closing them")
	}

	composer.Update(struct{}{})
	if composer.IsAutocompleteActive() {
		t.Fatal("a non-editing message reopened the suggestions Esc closed")
	}

	composer.Update(tea.KeyPressMsg{Code: tea.KeyLeft})
	if !composer.IsAutocompleteActive() {
		t.Fatal("moving the caret did not refresh suggestions")
	}
	composer.Update(tea.KeyPressMsg{Code: tea.KeyEnd})
	composer.Update(tea.KeyPressMsg{Code: 'a', Text: "a"})
	if !composer.IsAutocompleteActive() {
		t.Fatal("editing the token again did not bring the suggestions back")
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
	if got := lipgloss.Height(multiline); got != composerPanelMinRows+5 {
		t.Fatalf("three draft lines, summary, and preview took %d rows, want %d:\n%s",
			got, composerPanelMinRows+5, multiline)
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

// TestTaskComposerPastedBatchShowsEveryLine pins the growth seam a paste
// crosses: the batch arrives while the input is one row tall, so the cursor
// lands on the last line with the input scrolled to it, and the panel only then
// grows to fit the draft. The grown input must start at the first line rather
// than keep the offset it took while short and draw blank rows under the last.
func TestTaskComposerPastedBatchShowsEveryLine(t *testing.T) {
	project := &models.Project{ID: "project", Name: "Project"}
	composer := NewTaskComposerModel(TaskComposerPreset{Kind: ComposerProject, Project: project}, nil)
	composer.SetSize(100, 35)
	_ = composer.View()

	lines := make([]string, 8)
	for i := range lines {
		lines[i] = fmt.Sprintf("pasted task %d", i+1)
	}
	composer.Update(tea.PasteMsg{Content: strings.Join(lines, "\n")})

	rendered := ansi.Strip(composer.View().Content)
	for _, line := range lines {
		if !strings.Contains(rendered, line) {
			t.Fatalf("pasted batch hid %q behind the offset it took while short:\n%s", line, rendered)
		}
	}
	if composer.Input.Line() != len(lines)-1 {
		t.Fatalf("re-seating the scroll moved the cursor to line %d, want the last line", composer.Input.Line())
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
	if composer.Input.Line() != 2 || composer.Input.Column() != 0 {
		t.Fatalf("cursor = %d:%d, want start of source line 3", composer.Input.Line(), composer.Input.Column())
	}
	if got := len(env.reload(t).Tasks); got != 0 {
		t.Fatalf("atomic validation created %d tasks", got)
	}
}

func TestTaskComposerFailedCaptureFocusesOffscreenWrappedLine(t *testing.T) {
	project := &models.Project{ID: "project", Name: "Project"}
	composer := NewTaskComposerModel(TaskComposerPreset{Project: project}, nil)
	composer.SetSize(60, 18) // The shell reserves the other two terminal rows.
	lines := make([]string, 25)
	for i := range lines {
		lines[i] = fmt.Sprintf("Task %d %s", i, strings.Repeat("wrapped text ", 8))
	}
	lines[12] = "!1"
	draft := strings.Join(lines, "\n")
	composer.Update(tea.PasteMsg{Content: draft})
	if len(composer.Errors) != 0 {
		t.Fatal("line validation ran before submission")
	}
	composer.AC.Clear()
	composer.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if composer.Input.Line() != 12 || composer.Input.Column() != 0 {
		t.Fatalf("cursor = %d:%d, want start of line 13", composer.Input.Line(), composer.Input.Column())
	}
	if composer.Input.Value() != draft || composer.SubmissionInFlight {
		t.Fatal("refused submission changed the draft or started a commit")
	}
	if view := ansi.Strip(composer.Input.View()); !strings.Contains(view, "!1") {
		t.Fatalf("off-screen error was not revealed:\n%s", view)
	}
	composer.SetSize(100, 33)
	if composer.Input.Line() != 12 || !strings.Contains(ansi.Strip(composer.Input.View()), "!1") {
		t.Fatal("resize lost the error line")
	}
}

func TestTaskComposerRefreshesErrorsWithoutMovingCaretOrCommitting(t *testing.T) {
	project := &models.Project{ID: "project", Name: "Project"}
	composer := NewTaskComposerModel(TaskComposerPreset{Project: project}, nil)
	composer.Input.SetValue("!1\n\n!2")
	composer.CreateCompact(false)
	composer.Update(tea.PasteMsg{Content: "First "})
	if len(composer.Errors) != 1 || composer.Errors[0].Line != 3 {
		t.Fatalf("corrected line retained stale errors: %+v", composer.Errors)
	}
	if composer.Input.Line() != 0 || composer.Input.Column() != 6 {
		t.Fatal("editing jumped to the next error")
	}
	composer.Input.MoveToEnd()
	composer.Input.CursorStart()
	composer.Update(tea.PasteMsg{Content: "Second "})
	if len(composer.Errors) != 0 || composer.SubmissionInFlight {
		t.Fatalf("corrected draft has errors or submitted: %+v", composer.Errors)
	}
	// Live validation remains active after a correction, including the newline
	// branch that bypasses the textarea's usual UpdateInput path.
	composer.Input.MoveToEnd()
	composer.Update(tea.KeyPressMsg{Code: tea.KeyEnter, Mod: tea.ModShift})
	composer.Update(tea.PasteMsg{Content: "!3"})
	if len(composer.Errors) != 1 || composer.Errors[0].Line != 4 || composer.Input.Column() != 2 {
		t.Fatalf("new error did not follow the edited draft: %+v, col=%d", composer.Errors, composer.Input.Column())
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

func TestMultilineCapturePreviewFollowsCaretAndSurvivesResize(t *testing.T) {
	project := &models.Project{ID: "p", Name: "Project", Slug: "proj"}
	other := &models.Project{ID: "q", Name: "Other", Slug: "other"}
	composer := NewTaskComposerModel(TaskComposerPreset{
		Kind: ComposerGlobal, Project: project, AllProjects: []*models.Project{project, other}, AllowProjectSwitch: true,
	}, nil)
	composer.SetSize(100, 35)
	const draft = "First !1 @2030-01-02 %proj $today\n\nSecond !2 @2030-02-03 %other $next"
	composer.Update(tea.PasteMsg{Content: draft})
	composer.AC.Clear()
	check := func(line int, wants, absent []string) {
		t.Helper()
		preview := ansi.Strip(composer.renderComposerPanelPreview(12))
		for _, want := range append(wants, fmt.Sprintf("2 tasks · Line %d", line)) {
			if !strings.Contains(preview, want) {
				t.Fatalf("missing %q:\n%s", want, preview)
			}
		}
		for _, unwanted := range absent {
			if strings.Contains(preview, unwanted) {
				t.Fatalf("preview borrowed %q from another line:\n%s", unwanted, preview)
			}
		}
	}
	check(3, []string{"Second", "!2", "Feb 3", "Other", "$next"}, []string{"First", "!1", "Jan 2", "$today"})
	composer.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	check(2, nil, []string{"First", "Second", "!1", "!2"})
	composer.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	check(1, []string{"First", "!1", "Jan 2", "Project", "$today"}, []string{"Second", "!2", "Feb 3", "$next"})
	for _, size := range [][2]int{{60, 18}, {160, 48}, {100, 35}} {
		composer.SetSize(size[0], size[1])
		frame := composer.View().Content
		if lipgloss.Height(frame) > composerPanelMaxRows {
			t.Fatal("panel exceeded its ceiling")
		}
		for _, row := range strings.Split(frame, "\n") {
			if lipgloss.Width(row) > size[0] {
				t.Fatalf("row exceeds width: %q", row)
			}
		}
		check(1, []string{"First", "!1"}, []string{"Second", "!2"})
	}
	if composer.Input.Value() != draft || composer.Input.Line() != 0 {
		t.Fatal("preview or resize changed draft/caret")
	}
	reqs, ok := composer.ParseCompactLines()
	if !ok || len(reqs) != 2 || reqs[0].Priority != 1 || reqs[1].Priority != 2 || reqs[1].ProjectID != other.ID {
		t.Fatalf("preview changed capture semantics: %+v, %+v", reqs, composer.Errors)
	}
}

func TestTaskComposerLiveBatchDiagnosticsPreserveDraftAndCaret(t *testing.T) {
	project := &models.Project{ID: "project", Slug: "project", Name: "Project"}
	composer := NewTaskComposerModel(TaskComposerPreset{
		Project: project, AllowProjectSwitch: true,
		InitialText: "First task\n\n!1\nLast task",
	}, nil)
	composer.SetSize(60, 18)
	if got := ansi.Strip(composer.renderComposerPanelPreview(2)); !strings.Contains(got, "3 tasks · Line 4 · 1 line needs attention") {
		t.Fatalf("missing live diagnosis: %s", got)
	}
	if len(composer.Errors) != 0 || composer.validateOnEdit || composer.SubmissionInFlight {
		t.Fatal("live diagnosis published a submit refusal or submitted the draft")
	}
	before, line, column := composer.Input.Value(), composer.Input.Line(), composer.Input.Column()
	composer.View()
	if composer.Input.Value() != before || composer.Input.Line() != line || composer.Input.Column() != column {
		t.Fatal("rendering live diagnostics changed the draft or caret")
	}
	// Editing another line adds a project-resolution error before any submit.
	for _, r := range " %missing" {
		composer.Update(tea.KeyPressMsg{Code: r, Text: string(r)})
	}
	if composer.batchInvalidLines != 2 {
		t.Fatalf("live count = %d, want 2", composer.batchInvalidLines)
	}
	_, errs := composer.parseCompactLines()
	if len(errs) != 2 || errs[0].Line != 3 || errs[1].Line != 4 {
		t.Fatalf("submit diagnostics differ: %+v", errs)
	}
	for range len(" %missing") {
		composer.Update(tea.KeyPressMsg{Code: tea.KeyBackspace})
	}
	if composer.batchInvalidLines != 1 {
		t.Fatalf("correcting the draft left %d invalid lines", composer.batchInvalidLines)
	}
	composer.Input.MoveToBegin()
	composer.Input.CursorDown()
	composer.Input.CursorDown()
	composer.Input.CursorEnd()
	for _, r := range " Valid title" {
		composer.Update(tea.KeyPressMsg{Code: r, Text: string(r)})
	}
	if composer.batchInvalidLines != 0 || strings.Contains(ansi.Strip(composer.renderComposerPanelPreview(2)), "needs attention") {
		t.Fatal("fixed batch retained its warning")
	}
}

func TestTaskComposerNextProblemVisitsReasonsAndWraps(t *testing.T) {
	project := &models.Project{ID: "test", Slug: "test", Name: "Test"}
	m := NewTaskComposerModel(TaskComposerPreset{Kind: ComposerProject, Project: project}, nil)
	m.SetSize(60, 18)
	draft := "Valid\nBad !4\n\nAlso bad !5\nLast bad !4"
	m.Input.SetValue(draft)
	if cmd := m.CreateCompact(false); cmd != nil {
		t.Fatal("invalid batch committed")
	}
	for i, line := range []int{2, 4, 5, 2} {
		if i > 0 {
			m.Update(tea.KeyPressMsg{Code: 'n', Mod: tea.ModAlt})
		}
		if m.Input.Line()+1 != line {
			t.Fatalf("visit %d: line %d want %d", i, m.Input.Line()+1, line)
		}
		expected := fmt.Sprintf("Problem %d of 3 · Line %d", i%3+1, line)
		if !strings.Contains(ansi.Strip(m.View().Content), expected) {
			t.Fatalf("missing %q:\n%s", expected, ansi.Strip(m.View().Content))
		}
		if m.Input.Value() != draft || m.SubmissionInFlight {
			t.Fatal("navigation altered the draft or submitted")
		}
	}
	m.Input.SetValue("Valid\nFixed\n\nAlso bad !5\nLast bad !4")
	m.refreshCaptureErrorsAfterEdit(draft, project)
	m.focusNextCaptureError()
	if m.Input.Line() != 3 || len(m.Errors) != 2 {
		t.Fatal("next problem retained corrected line")
	}
}

func TestTaskComposerPreviousProblemPreservesDraftAndSkipsCorrectedLines(t *testing.T) {
	project := &models.Project{ID: "test", Slug: "test", Name: "Test"}
	m := NewTaskComposerModel(TaskComposerPreset{Kind: ComposerProject, Project: project}, nil)
	m.SetSize(60, 18)
	draft := "Valid\nBad !4\n\nAlso bad !5\nLast bad !4"
	m.Input.SetValue(draft)
	previous := tea.KeyPressMsg{Code: 'p', Mod: tea.ModAlt}
	line, column := m.Input.Line(), m.Input.Column()
	m.Update(previous)
	if m.Input.Line() != line || m.Input.Column() != column || m.Input.Value() != draft {
		t.Fatal("previous problem changed the draft before a refused submission")
	}
	if cmd := m.CreateCompact(false); cmd != nil {
		t.Fatal("invalid batch committed")
	}
	for i, line := range []int{5, 4, 2, 5} {
		if _, cmd := m.Update(previous); cmd != nil {
			t.Fatal("problem navigation emitted a command")
		}
		if m.Input.Line()+1 != line || m.Input.Column() != 0 {
			t.Fatalf("visit %d: line %d column %d, want line %d start", i, m.Input.Line()+1, m.Input.Column(), line)
		}
		problem := []int{3, 2, 1, 3}[i]
		want := fmt.Sprintf("Problem %d of 3 · Line %d", problem, line)
		if !strings.Contains(ansi.Strip(m.View().Content), want) {
			t.Fatalf("missing %q in narrow view: %s", want, ansi.Strip(m.View().Content))
		}
		if m.Input.Value() != draft || m.SubmissionInFlight {
			t.Fatal("navigation altered the draft or submitted")
		}
	}
	m.HelpExpanded = true
	m.Update(previous)
	if m.Input.Line() != 4 {
		t.Fatal("previous problem navigated underneath syntax help")
	}
	m.HelpExpanded = false
	fixed := "Valid\nBad !4\n\nFixed\nLast bad !4"
	m.Input.SetValue(fixed)
	m.refreshCaptureErrorsAfterEdit(draft, project)
	m.focusCaptureErrorLine(5)
	m.Update(previous)
	if m.Input.Line() != 1 || len(m.Errors) != 2 {
		t.Fatal("previous problem retained the corrected line")
	}
	m.Input.SetValue("Valid\nFixed\n\nFixed\nLast bad !4")
	m.refreshCaptureErrorsAfterEdit(fixed, project)
	m.Update(previous)
	m.Update(previous)
	if m.Input.Line() != 4 || len(m.Errors) != 1 {
		t.Fatal("previous problem did not wrap to the sole remaining error")
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
			if view := ansi.Strip(m.renderComposerPanelPreview(3)); strings.Contains(view, tc.want) {
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
