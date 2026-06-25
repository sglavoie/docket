package tui

import (
	"slices"
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

func TestFocusBoardCommandsShortcutsMatchCockpitGrammar(t *testing.T) {
	app := newTestAppModel()
	cmds := app.focusBoardCommands()

	cases := []struct {
		name     string
		shortcut string
	}{
		{"today open task", "enter"},
		{"today complete or dismiss", "x"},
		{"today set priority", "p"},
		{"today plan backlog", "p"},
		{"today triage inbox", "ctrl+i"},
		{"today quick add", "a"},
		{"today search", "/"},
		{"today unblock task", "u"},
		{"today next tab", "tab"},
		{"today new project", "p"},
		{"today pipeline hub", ";;"},
		{"today help", "?"},
		{"today close", "esc"},
	}

	for _, tc := range cases {
		cmd := findCommand(cmds, tc.name)
		if cmd == nil {
			t.Errorf("%q command missing", tc.name)
			continue
		}
		if cmd.Shortcut != tc.shortcut {
			t.Errorf("%q shortcut = %q, want %q", tc.name, cmd.Shortcut, tc.shortcut)
		}
		if !slices.Contains(cmd.Views, ViewFocusBoard) {
			t.Errorf("%q should be scoped to Today, got views %v", tc.name, cmd.Views)
		}
	}
}

func TestFocusBoardCommandsUseCockpitActionHandlers(t *testing.T) {
	app := newTestAppModel()
	project := cockpitProject(t, "Command Surface")
	task := cockpitTask(t, project, "Sharpen palette command", models.StatusTodo, models.SectionNext)
	dailyPlanIDs := map[string]bool{task.ID: true}
	focusBoard := cockpitModel(
		[]boardRow{cockpitBoardRow(task, project)},
		nil,
		nil,
		dailyPlanIDs,
		OpenLoops{TodayTotal: 1, HasTasks: true},
		boardTabToday,
	)
	app.views.Register(ViewFocusBoard, focusBoard)
	app.viewStack = []ViewType{ViewDashboard, ViewFocusBoard, ViewCommandPalette}

	cmd := findCommand(app.focusBoardCommands(), "today set priority")
	if cmd == nil {
		t.Fatal("today set priority command missing")
	}
	result, ok := cmd.Handler().(PaletteCommandResultMsg)
	if !ok {
		t.Fatalf("today set priority should return PaletteCommandResultMsg, got %T", cmd.Handler())
	}
	if result.Cmd == nil {
		t.Fatal("today set priority should carry a command")
	}
	if _, isPriorityMsg := result.Cmd().(ShowPriorityPickerMsg); !isPriorityMsg {
		t.Fatalf("today set priority should open priority picker, got %T", result.Cmd())
	}

	cmd = findCommand(app.focusBoardCommands(), "today plan backlog")
	if cmd == nil {
		t.Fatal("today plan backlog command missing")
	}
	result, ok = cmd.Handler().(PaletteCommandResultMsg)
	if !ok {
		t.Fatalf("today plan backlog should return PaletteCommandResultMsg, got %T", cmd.Handler())
	}
	if _, ok := result.Cmd().(ShowFocusViewMsg); !ok {
		t.Fatalf("today plan backlog should open Backlog, got %T", result.Cmd())
	}
}

func TestFocusBoardPaletteSuggestedItemsFollowCockpitRecommendation(t *testing.T) {
	t.Run("planned task", func(t *testing.T) {
		app := newTestAppModel()
		project := cockpitProject(t, "Command Surface")
		task := cockpitTask(t, project, "Use the suggested cockpit action", models.StatusTodo, models.SectionNext)
		dailyPlanIDs := map[string]bool{task.ID: true}
		focusBoard := cockpitModel(
			[]boardRow{cockpitBoardRow(task, project)},
			nil,
			nil,
			dailyPlanIDs,
			OpenLoops{TodayTotal: 1, HasTasks: true},
			boardTabToday,
		)
		palette := openFocusBoardPaletteForCommandTest(t, app, focusBoard)

		got := suggestedCommandNames(palette)
		want := []string{"today open task", "today complete or dismiss", "today set priority"}
		if !slices.Equal(got, want) {
			t.Fatalf("planned task suggested commands = %v, want %v", got, want)
		}
	})

	t.Run("inbox only", func(t *testing.T) {
		app := newTestAppModel()
		focusBoard := cockpitModel(
			nil,
			nil,
			nil,
			nil,
			OpenLoops{Inbox: 2, HasTasks: true},
			boardTabToday,
		)
		palette := openFocusBoardPaletteForCommandTest(t, app, focusBoard)

		got := suggestedCommandNames(palette)
		want := []string{"today triage inbox", "today quick add", "today next tab"}
		if !slices.Equal(got, want) {
			t.Fatalf("inbox-only suggested commands = %v, want %v", got, want)
		}
	})
}

func TestFocusBoardPaletteKeepsAdvancedCommandsSearchable(t *testing.T) {
	app := newTestAppModel()
	focusBoard := cockpitModel(nil, nil, nil, nil, OpenLoops{}, boardTabToday)
	palette := openFocusBoardPaletteForCommandTest(t, app, focusBoard)

	var suggestedAdvanced bool
	var searchableAdvanced bool
	for _, fi := range palette.filteredItems {
		if fi.Item.Command == nil || fi.Item.Command.Name != "quick note" {
			continue
		}
		searchableAdvanced = true
		suggestedAdvanced = fi.IsSuggested
	}
	if !searchableAdvanced {
		t.Fatal("advanced quick note command should remain searchable from Today")
	}
	if suggestedAdvanced {
		t.Fatal("advanced quick note command should not displace cockpit suggestions")
	}
}

func TestViewCommandsDailyReviewDoesNotClaimTodayAlias(t *testing.T) {
	app := newTestAppModel()
	cmd := findCommand(app.viewCommands(), "daily review")
	if cmd == nil {
		t.Fatal("daily review command missing")
	}
	if slices.Contains(cmd.Aliases, "today") {
		t.Fatalf("daily review aliases should not claim Today, got %v", cmd.Aliases)
	}
}

func TestViewCommandsTodayUsesCockpitLanguage(t *testing.T) {
	app := newTestAppModel()
	cmd := findCommand(app.viewCommands(), "today")
	if cmd == nil {
		t.Fatal("today command missing")
	}
	if cmd.Description != "Today cockpit for daily execution" {
		t.Fatalf("today command description = %q, want cockpit wording", cmd.Description)
	}
	if slices.Contains(cmd.Aliases, "focus board") {
		t.Fatalf("today command aliases should not advertise Focus Board, got %v", cmd.Aliases)
	}
	if !slices.Contains(cmd.Aliases, "today cockpit") {
		t.Fatalf("today command aliases should include today cockpit, got %v", cmd.Aliases)
	}
}

func openFocusBoardPaletteForCommandTest(t *testing.T, app *AppModel, focusBoard *FocusBoardModel) *CommandPaletteModel {
	t.Helper()
	registry := NewCommandRegistry()
	for i, group := range app.commandGroups() {
		if err := registry.RegisterBatch(group); err != nil {
			t.Fatalf("RegisterBatch[%d] failed: %v", i, err)
		}
	}
	app.commandRegistry = registry
	app.views.Register(ViewFocusBoard, focusBoard)
	app.viewStack = []ViewType{ViewDashboard, ViewFocusBoard}

	recentPaletteSelections = nil
	t.Cleanup(func() { recentPaletteSelections = nil })

	handled, _ := app.handleSearchMessages(ShowCommandPaletteMsg{})
	if !handled {
		t.Fatal("ShowCommandPaletteMsg should be handled")
	}
	palette, ok := app.views.Get(ViewCommandPalette).(*CommandPaletteModel)
	if !ok || palette == nil {
		t.Fatalf("expected command palette view, got %T", app.views.Get(ViewCommandPalette))
	}
	return palette
}

func suggestedCommandNames(palette *CommandPaletteModel) []string {
	var names []string
	for _, fi := range palette.filteredItems {
		if !fi.IsSuggested || fi.Item.Command == nil {
			continue
		}
		names = append(names, fi.Item.Command.Name)
	}
	return names
}
