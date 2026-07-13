package tui

import (
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
)

// triageInboxCommands rebuilds the registry from production groups and
// returns the commands available from the given inbox processor view.
func triageInboxCommands(t *testing.T, app *AppModel, view ViewType) []Command {
	t.Helper()
	registry := NewCommandRegistry()
	for i, group := range app.commandGroups() {
		if err := registry.RegisterBatch(group); err != nil {
			t.Fatalf("RegisterBatch[%d] failed: %v", i, err)
		}
	}
	return registry.GetForView(view)
}

// TestPaletteTriage_InboxProcessorShortcutsMatchKeymap pins the palette's
// command shortcuts for the inbox processor against the authoritative keymap
// vocabulary (mark done = D, manage tags = T, route = ctrl+s, send to waiting
// = ctrl+w, delete = tD, due date = dd, dependencies = bm, subtask = cs).
// Sessions 2/5/6 mandate this triage vocabulary across footer + palette.
func TestPaletteTriage_InboxProcessorShortcutsMatchKeymap(t *testing.T) {
	app, _ := newTestAppModelWithProject()

	cases := []struct {
		name     string
		shortcut string
	}{
		// Primary triage vocabulary (TriagePrimaryGroup).
		{"skip task", "n"},
		{"mark task done", "D"},
		{"edit inbox task", "e"},
		{"manage inbox task tags", "T"},
		{"undo last triage action", "u"},
		// Navigation.
		{"view task details", "enter"},
		// Route & schedule.
		{"route to section", "ctrl+s"},
		{"send to waiting", "ctrl+w"},
		{"set inbox task due date", "dd"},
		// Refine.
		{"create subtask from inbox", "cs"},
		{"manage inbox dependencies", "bm"},
		// Destructive.
		{"delete inbox task", "tD"},
	}

	for _, viewType := range []ViewType{ViewInboxProcessor, ViewGlobalInboxProcessor} {
		cmds := triageInboxCommands(t, app, viewType)
		for _, tc := range cases {
			cmd := findCommand(cmds, tc.name)
			if cmd == nil {
				t.Errorf("[%v] %q should be reachable from palette", viewType, tc.name)
				continue
			}
			if cmd.Shortcut != tc.shortcut {
				t.Errorf("[%v] %q shortcut = %q, want %q", viewType, tc.name, cmd.Shortcut, tc.shortcut)
			}
		}
	}
}

// TestPaletteTriage_InboxProcessorTriageAliases asserts the inbox processor
// commands carry triage-vocabulary aliases so palette search matches the
// language users learn from the footer and help screens ("triage", "refine",
// "resolve", "route", "schedule", "waiting").
func TestPaletteTriage_InboxProcessorTriageAliases(t *testing.T) {
	app, _ := newTestAppModelWithProject()
	cmds := triageInboxCommands(t, app, ViewInboxProcessor)

	cases := []struct {
		name    string
		aliases []string
	}{
		{"skip task", []string{"triage skip"}},
		{"mark task done", []string{"resolve task", "triage done"}},
		{"edit inbox task", []string{"refine task", "triage edit"}},
		{"manage inbox task tags", []string{"manage tags", "triage tags"}},
		{"undo last triage action", []string{"undo"}},
		{"route to section", []string{"assign section", "route inbox"}},
		{"send to waiting", []string{"waiting", "defer task", "triage wait"}},
		{"set inbox task due date", []string{"schedule task", "deadline"}},
		{"create subtask from inbox", []string{"refine subtask"}},
		{"manage inbox dependencies", []string{"manage blockers"}},
	}

	for _, tc := range cases {
		cmd := findCommand(cmds, tc.name)
		if cmd == nil {
			t.Errorf("%q command missing", tc.name)
			continue
		}
		for _, alias := range tc.aliases {
			if !slices.Contains(cmd.Aliases, alias) {
				t.Errorf("%q missing alias %q (got %v)", tc.name, alias, cmd.Aliases)
			}
		}
	}
}

// TestPaletteTriage_InboxProcessorDualViewScoping confirms every inbox
// processor command is scoped to BOTH ViewInboxProcessor and
// ViewGlobalInboxProcessor — the parity contract from session 6 means the
// palette must surface the same commands from either inbox flavor.
func TestPaletteTriage_InboxProcessorDualViewScoping(t *testing.T) {
	app, _ := newTestAppModelWithProject()
	cmds := app.inboxProcessorCommands()
	if len(cmds) == 0 {
		t.Fatal("inboxProcessorCommands() returned no commands")
	}
	for _, cmd := range cmds {
		if !slices.Contains(cmd.Views, ViewInboxProcessor) {
			t.Errorf("%q missing ViewInboxProcessor in Views (got %v)", cmd.Name, cmd.Views)
		}
		if !slices.Contains(cmd.Views, ViewGlobalInboxProcessor) {
			t.Errorf("%q missing ViewGlobalInboxProcessor in Views (got %v)", cmd.Name, cmd.Views)
		}
	}
}

// TestPaletteTriage_ChordHandlersDispatchToActiveProcessor asserts that the
// chord-based commands (due date, dependencies, subtask, delete) route through
// the live InboxProcessorModel instead of attempting to synth single keys that
// would only put the dispatcher into prefix mode.
func TestPaletteTriage_ChordHandlersDispatchToActiveProcessor(t *testing.T) {
	app, project := newTestAppModelWithProject()
	task := &models.Task{ID: "task-1", ShortID: 42, Title: "Test task"}
	project.Tasks = []*models.Task{task}
	processor := NewInboxProcessorModel(project, app.config, app.configManager)
	processor.inboxTasks = []*models.Task{task}
	processor.currentIndex = 0
	app.inboxProcessor = processor
	app.pushView(ViewInboxProcessor)
	app.pushView(ViewCommandPalette)

	cmds := app.inboxProcessorCommands()

	dueDateCmd := findCommand(cmds, "set inbox task due date")
	if dueDateCmd == nil {
		t.Fatal("set inbox task due date command missing")
	}
	msg := dueDateCmd.Handler()
	if _, ok := msg.(SetDueDateMsg); !ok {
		t.Errorf("set inbox task due date handler should return SetDueDateMsg, got %T", msg)
	}

	depCmd := findCommand(cmds, "manage inbox dependencies")
	if depCmd == nil {
		t.Fatal("manage inbox dependencies command missing")
	}
	msg = depCmd.Handler()
	if _, ok := msg.(ManageDependenciesMsg); !ok {
		t.Errorf("manage inbox dependencies handler should return ManageDependenciesMsg, got %T", msg)
	}

	subtaskCmd := findCommand(cmds, "create subtask from inbox")
	if subtaskCmd == nil {
		t.Fatal("create subtask from inbox command missing")
	}
	msg = subtaskCmd.Handler()
	if _, ok := msg.(CreateTaskMsg); !ok {
		t.Errorf("create subtask handler should return CreateTaskMsg, got %T", msg)
	}

	delCmd := findCommand(cmds, "delete inbox task")
	if delCmd == nil {
		t.Fatal("delete inbox task command missing")
	}
	if msg := delCmd.Handler(); msg != nil {
		t.Errorf("delete inbox task handler should return nil (it mutates confirm), got %T", msg)
	}
	if !processor.confirm.IsActive() {
		t.Error("delete handler should activate the inbox processor's confirm dialog")
	}
}

// newTriageLocalProcessor builds a fresh local InboxProcessorModel below a
// CommandPalette view with one processable inbox task. Used by primary-key
// triage tests so each subtest starts from clean state and mutations from
// earlier handlers cannot bleed across cases.
func newTriageLocalProcessor(t *testing.T) (*AppModel, *models.Task, *InboxProcessorModel) {
	t.Helper()
	app, project := newTestAppModelWithProject()
	task := &models.Task{
		ID:            "task-1",
		ShortID:       42,
		Title:         "Test task",
		WorkflowState: models.StateInbox,
	}
	project.Tasks = []*models.Task{task}
	processor := NewInboxProcessorModel(project, app.config, app.configManager)
	processor.inboxTasks = []*models.Task{task}
	processor.currentIndex = 0
	app.inboxProcessor = processor
	app.pushView(ViewInboxProcessor)
	app.pushView(ViewCommandPalette)
	return app, task, processor
}

// TestPaletteTriage_PrimaryKeysDispatchDirectActions ensures the direct-key
// triage commands no longer fall back to synthetic key replay. Modal/detail
// handlers (edit, manage tags, view details, route to section) produce the
// underlying message type directly so the palette executor can keep or close
// the palette per the modal switch; mutating handlers (skip, mark done, undo,
// send to waiting) wrap their tea.Cmd in PaletteCommandResultMsg so the
// runtime executes save/advance after the palette closes.
func TestPaletteTriage_PrimaryKeysDispatchDirectActions(t *testing.T) {
	directCases := []struct {
		name   string
		verify func(t *testing.T, msg tea.Msg)
	}{
		{
			name: "edit inbox task",
			verify: func(t *testing.T, msg tea.Msg) {
				if _, ok := msg.(EditTaskMsg); !ok {
					t.Errorf("expected EditTaskMsg, got %T", msg)
				}
			},
		},
		{
			name: "manage inbox task tags",
			verify: func(t *testing.T, msg tea.Msg) {
				if _, ok := msg.(ShowTagSelectorMsg); !ok {
					t.Errorf("expected ShowTagSelectorMsg, got %T", msg)
				}
			},
		},
		{
			name: "view task details",
			verify: func(t *testing.T, msg tea.Msg) {
				if _, ok := msg.(ShowTaskDetailMsg); !ok {
					t.Errorf("expected ShowTaskDetailMsg, got %T", msg)
				}
			},
		},
		{
			name: "route to section",
			verify: func(t *testing.T, msg tea.Msg) {
				if _, ok := msg.(ShowSectionPickerMsg); !ok {
					t.Errorf("expected ShowSectionPickerMsg, got %T", msg)
				}
			},
		},
	}
	for _, tc := range directCases {
		t.Run(tc.name, func(t *testing.T) {
			app, _, _ := newTriageLocalProcessor(t)
			cmd := findCommand(app.inboxProcessorCommands(), tc.name)
			if cmd == nil {
				t.Fatalf("%q command missing", tc.name)
			}
			tc.verify(t, cmd.Handler())
		})
	}

	resultCases := []string{
		"skip task",
		"mark task done",
		"send to waiting",
	}
	for _, name := range resultCases {
		t.Run(name, func(t *testing.T) {
			app, _, _ := newTriageLocalProcessor(t)
			cmd := findCommand(app.inboxProcessorCommands(), name)
			if cmd == nil {
				t.Fatalf("%q command missing", name)
			}
			msg := cmd.Handler()
			result, ok := msg.(PaletteCommandResultMsg)
			if !ok {
				t.Fatalf("expected PaletteCommandResultMsg, got %T", msg)
			}
			if result.Cmd == nil {
				t.Errorf("PaletteCommandResultMsg.Cmd should be non-nil for %q", name)
			}
		})
	}

	t.Run("undo last triage action with empty history", func(t *testing.T) {
		app, _, _ := newTriageLocalProcessor(t)
		cmd := findCommand(app.inboxProcessorCommands(), "undo last triage action")
		if cmd == nil {
			t.Fatal("undo last triage action command missing")
		}
		// With no recorded actions, undoLastAction returns a nil tea.Cmd, so
		// the palette handler short-circuits to a nil Msg (no-op) instead of
		// wrapping a nil cmd.
		if msg := cmd.Handler(); msg != nil {
			t.Errorf("undo with empty history should return nil, got %T", msg)
		}
	})
}

// TestPaletteTriage_GlobalInboxMarkDoneExecution covers the end-to-end palette
// "mark task done" flow against a live GlobalInboxProcessorModel: the task is
// completed synchronously, the processor records it as processed, the palette
// pops to the underlying processor view, and the wrapped tea.Cmd propagates to
// the runtime so the async save runs.
func TestPaletteTriage_GlobalInboxMarkDoneExecution(t *testing.T) {
	app, project := newTestAppModelWithProject()
	task := &models.Task{
		ID:            "task-1",
		ShortID:       42,
		Title:         "Global test",
		WorkflowState: models.StateInbox,
	}
	project.Tasks = []*models.Task{task}

	processor := &GlobalInboxProcessorModel{
		projectCache: map[string]*models.Project{project.ID: project},
	}
	processor.initCore(processor, processor, app.config, app.configManager)
	processor.inboxTasks = []GlobalInboxTask{{Task: task, Project: project}}
	processor.currentIndex = 0
	app.globalInboxProcessor = processor

	app.pushView(ViewGlobalInboxProcessor)
	app.pushView(ViewCommandPalette)

	cmd := findCommand(app.inboxProcessorCommands(), "mark task done")
	if cmd == nil {
		t.Fatal("mark task done command missing")
	}

	msg := cmd.Handler()
	result, ok := msg.(PaletteCommandResultMsg)
	if !ok {
		t.Fatalf("expected PaletteCommandResultMsg, got %T", msg)
	}
	if result.Cmd == nil {
		t.Fatal("expected non-nil tea.Cmd from mark task done")
	}

	if task.State() != models.StateDone {
		t.Errorf("task.State() = %v, want StateDone", task.State())
	}
	if !processor.processedIDs[task.ID] {
		t.Error("global processor should have task in processedIDs after mark done")
	}

	handled, executeCmd := app.handleSearchMessages(CommandPaletteExecuteMsg{ResultMsg: result})
	if !handled {
		t.Fatal("palette execute should be handled")
	}
	if app.currentView() != ViewGlobalInboxProcessor {
		t.Errorf("palette should pop to ViewGlobalInboxProcessor, got %v", app.currentView())
	}
	if executeCmd == nil {
		t.Fatal("expected save cmd to propagate through palette execute, got nil")
	}
}

// TestPaletteTriage_DashboardTriageInboxCommand pins the new dashboard-scoped
// "triage inbox" entry that mirrors the dashboard's `O` key (session 7's
// prominent triage entry point).
func TestPaletteTriage_DashboardTriageInboxCommand(t *testing.T) {
	app, project := newTestAppModelWithProject()
	app.dashboard = NewDashboardModel(app.config, app.configManager)

	cmds := triageInboxCommands(t, app, ViewDashboard)
	cmd := findCommand(cmds, "triage inbox")
	if cmd == nil {
		t.Fatal("'triage inbox' command missing for ViewDashboard")
	}
	if cmd.Shortcut != "O" {
		t.Errorf("'triage inbox' shortcut = %q, want %q", cmd.Shortcut, "O")
	}
	if !slices.Contains(cmd.Views, ViewDashboard) {
		t.Errorf("'triage inbox' Views should include ViewDashboard, got %v", cmd.Views)
	}

	msg := cmd.Handler()
	showMsg, ok := msg.(ShowInboxProcessorMsg)
	if !ok {
		t.Fatalf("triage inbox handler should return ShowInboxProcessorMsg, got %T", msg)
	}
	if showMsg.Project == nil || showMsg.Project.ID != project.ID {
		t.Errorf("triage inbox handler should target the selected dashboard project, got %v", showMsg.Project)
	}
}

// TestPaletteTriage_InboxProcessorCommandsRetainTriageAliases pins triage
// aliases on the cross-view "inbox processor" and "global inbox processor"
// view-level commands so palette search for "triage" surfaces them.
func TestPaletteTriage_InboxProcessorCommandsRetainTriageAliases(t *testing.T) {
	app, _ := newTestAppModelWithProject()

	viewCmds := app.viewCommands()
	cases := []struct {
		name    string
		aliases []string
	}{
		{"inbox processor", []string{"triage inbox", "inbox triage"}},
		{"global inbox processor", []string{"triage all inboxes", "triage global inbox"}},
	}
	for _, tc := range cases {
		cmd := findCommand(viewCmds, tc.name)
		if cmd == nil {
			t.Fatalf("%q command missing", tc.name)
		}
		for _, alias := range tc.aliases {
			if !slices.Contains(cmd.Aliases, alias) {
				t.Errorf("%q missing alias %q (got %v)", tc.name, alias, cmd.Aliases)
			}
		}
	}
}

// TestPaletteTriage_FooterAdvertisesFullReference asserts the palette's
// empty-query footer mentions `ctrl+\: full reference` so users learn the same
// escape-hatch vocabulary across help and palette (session 8 contract).
func TestPaletteTriage_FooterAdvertisesFullReference(t *testing.T) {
	app, _ := newTestAppModelWithProject()
	palette := NewCommandPaletteModel(app.config, app.configManager, NewCommandRegistry(), ViewInboxProcessor, nil)
	footer := palette.buildFooter()
	if !strings.Contains(footer, `ctrl+\: full reference`) {
		t.Errorf("palette empty-query footer should advertise 'ctrl+\\: full reference', got %q", footer)
	}
}
