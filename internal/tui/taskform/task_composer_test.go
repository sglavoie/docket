package taskform

import (
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

	project, destination := composer.effectiveRouteHeader()
	if project != "tt" || destination != models.StateDoing.Icon()+" $doing" {
		t.Fatalf("effective header = %q / %q, want tt / doing", project, destination)
	}
	plain := ansi.Strip(composer.View().Content)
	if strings.Contains(plain, "Preset:") || !strings.Contains(plain, "Destination:") {
		t.Fatalf("composer retained launch preset metadata:\n%s", plain)
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
	_, destination := composer.effectiveRouteHeader()
	if destination != renderComposerDestination(app.CaptureDestinationToday) {
		t.Fatalf("displayed destination = %q, want Today", destination)
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

	tests := []struct {
		name            string
		input           string
		wantProject     string
		wantDestination string
	}{
		{"uniform", "One %tt $doing\nTwo %tt $doing", "tt", models.StateDoing.Icon() + " $doing"},
		{"mixed project", "One %tt $doing\nTwo $doing", "Mixed", models.StateDoing.Icon() + " $doing"},
		{"mixed destination", "One %tt $doing\nTwo %tt $waiting", "tt", "Mixed"},
		{"both mixed", "One %tt $doing\nTwo $waiting", "Mixed", "Mixed"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			composer.Input.SetValue(tt.input)
			project, destination := composer.effectiveRouteHeader()
			if project != tt.wantProject || destination != tt.wantDestination {
				t.Fatalf("effective header = %q / %q, want %q / %q", project, destination, tt.wantProject, tt.wantDestination)
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

func TestTaskComposerAutocompleteUsesAvailableHeight(t *testing.T) {
	project := &models.Project{ID: "project", Name: "Project"}
	composer := NewTaskComposerModel(TaskComposerPreset{Kind: ComposerProject, Project: project}, nil)
	composer.SetSize(80, 30)
	composer.Input.SetValue("Task $")
	composer.UpdatePreview()
	composer.RefreshAutocomplete()

	tall := ansi.Strip(composer.View().Content)
	if strings.Contains(tall, "more") || !strings.Contains(tall, "$canceled") {
		t.Fatalf("tall composer did not show every destination:\n%s", tall)
	}

	composer.SetSize(80, 18)
	short := ansi.Strip(composer.View().Content)
	if !strings.Contains(short, "more") {
		t.Fatalf("short composer did not show an overflow hint:\n%s", short)
	}
	if got := lipgloss.Height(short); got > 18 {
		t.Fatalf("short composer height = %d, want at most 18", got)
	}
}

func TestTaskComposerInputDividerSpacing(t *testing.T) {
	project := &models.Project{ID: "project", Name: "Project"}
	composer := NewTaskComposerModel(TaskComposerPreset{Kind: ComposerProject, Project: project}, nil)
	composer.SetSize(80, 30)
	composer.Input.SetValue("Draft task")
	composer.UpdatePreview()

	lines := strings.Split(ansi.Strip(composer.View().Content), "\n")
	inputLine := -1
	for i, line := range lines {
		if strings.Contains(line, "Draft task") {
			inputLine = i
			break
		}
	}
	if inputLine < 0 || inputLine+5 >= len(lines) {
		t.Fatalf("could not locate input and lower pane:\n%s", strings.Join(lines, "\n"))
	}
	if strings.TrimSpace(lines[inputLine+1]) != "" || strings.TrimSpace(lines[inputLine+2]) != "" {
		t.Fatalf("expected two blank rows above divider:\n%s", strings.Join(lines[inputLine:inputLine+6], "\n"))
	}
	divider := strings.TrimSpace(lines[inputLine+3])
	if divider != strings.Repeat("─", 80) {
		t.Fatalf("divider = %q, want 80-column rule", divider)
	}
	if strings.TrimSpace(lines[inputLine+4]) != "" || !strings.HasPrefix(lines[inputLine+5], "Task") {
		t.Fatalf("expected one blank row below divider before preview:\n%s", strings.Join(lines[inputLine:inputLine+6], "\n"))
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
