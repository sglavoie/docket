package tui

import (
	"slices"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
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
	composer.input.SetValue("Write regression test %tt $doing")
	composer.updatePreview()

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
	if composer.project == nil || composer.project.ID != beta.ID {
		t.Fatalf("composer Ctrl+P project = %#v, want Beta", composer.project)
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
	composer.input.SetValue("Due today @today")

	requests, ok := composer.parseCompactLines()
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
			composer.input.SetValue(tt.input)
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
		composer.input.SetValue(input)
		composer.refreshAutocomplete()
		if composer.ac.IsActive() {
			t.Errorf("autocomplete remained active for used singleton in %q: %+v", input, composer.ac)
		}
	}

	composer.input.SetValue("First $doing\nSecond $")
	composer.refreshAutocomplete()
	if !composer.ac.IsActive() || composer.ac.Mode != acModeDestination {
		t.Fatalf("singleton on another task line suppressed destination autocomplete: %+v", composer.ac)
	}
}

func TestTaskComposerAutocompleteKeepsUnusedTagsRepeatable(t *testing.T) {
	project := &models.Project{ID: "project", Name: "Project"}
	composer := NewTaskComposerModel(TaskComposerPreset{Kind: ComposerProject, Project: project}, nil)
	composer.tagCache = []string{"backend", "frontend", "urgent"}
	composer.input.SetValue("Task #backend #")
	composer.refreshAutocomplete()

	if !composer.ac.IsActive() || composer.ac.Mode != acModeTag {
		t.Fatalf("repeatable tag autocomplete is inactive: %+v", composer.ac)
	}
	values := make([]string, len(composer.ac.Items))
	for i, item := range composer.ac.Items {
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
	composer.input.SetValue("Task $")
	composer.updatePreview()
	composer.refreshAutocomplete()

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
	composer.input.SetValue("Draft task")
	composer.updatePreview()

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
	appModel, project := newTestAppModelWithProject()
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
			composer := NewTaskComposerModel(tt.preset, appModel.runtime)
			if composer.mode != tt.wantMode || composer.preset.Kind != tt.wantKind {
				t.Fatalf("mode/kind = %v/%v, want %v/%v", composer.mode, composer.preset.Kind, tt.wantMode, tt.wantKind)
			}
			if tt.wantParent && (composer.preset.Shared.ParentID == nil || *composer.preset.Shared.ParentID != task.ID) {
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
	composer.input.SetValue(input)

	if cmd := composer.createCompact(false); cmd != nil {
		t.Fatalf("invalid batch returned commit command %T", cmd())
	}
	if len(composer.errors) != 1 || composer.errors[0].line != 3 {
		t.Fatalf("errors = %+v, want source line 3", composer.errors)
	}
	if composer.input.Value() != input {
		t.Fatalf("failed batch changed input: %q", composer.input.Value())
	}
	if got := len(env.reload(t).Tasks); got != 0 {
		t.Fatalf("atomic validation created %d tasks", got)
	}
}
