package tui

import "testing"

func TestGlobalCommands_Help(t *testing.T) {
	app := newTestAppModel()
	cmds := app.globalCommands()
	cmd := findCommand(cmds, "help")
	if cmd == nil {
		t.Fatal("expected 'help' command")
	}
	msg := cmd.Handler()
	if _, ok := msg.(ShowHelpMsg); !ok {
		t.Errorf("expected ShowHelpMsg, got %T", msg)
	}
}

func TestGlobalCommands_Quit(t *testing.T) {
	app := newTestAppModel()
	cmds := app.globalCommands()
	cmd := findCommand(cmds, "quit")
	if cmd == nil {
		t.Fatal("expected 'quit' command")
	}
	msg := cmd.Handler()
	if _, ok := msg.(QuitMsg); !ok {
		t.Errorf("expected QuitMsg, got %T", msg)
	}
}

func TestGlobalCommands_GlobalSearch(t *testing.T) {
	app := newTestAppModel()
	cmds := app.globalCommands()
	cmd := findCommand(cmds, "global search")
	if cmd == nil {
		t.Fatal("expected 'global search' command")
	}
	msg := cmd.Handler()
	if _, ok := msg.(ShowGlobalSearchMsg); !ok {
		t.Errorf("expected ShowGlobalSearchMsg, got %T", msg)
	}
}

func TestGlobalCommands_Settings(t *testing.T) {
	app := newTestAppModel()
	cmds := app.globalCommands()
	cmd := findCommand(cmds, "settings")
	if cmd == nil {
		t.Fatal("expected 'settings' command")
	}
	msg := cmd.Handler()
	if _, ok := msg.(SettingsMsg); !ok {
		t.Errorf("expected SettingsMsg, got %T", msg)
	}
}

func TestGlobalCommands_Notifications(t *testing.T) {
	app := newTestAppModel()
	cmds := app.globalCommands()
	cmd := findCommand(cmds, "notifications")
	if cmd == nil {
		t.Fatal("expected 'notifications' command")
	}
	msg := cmd.Handler()
	if _, ok := msg.(ShowNotificationCenterMsg); !ok {
		t.Errorf("expected ShowNotificationCenterMsg, got %T", msg)
	}
}

func TestGlobalCommands_Tutor(t *testing.T) {
	app := newTestAppModel()
	cmds := app.globalCommands()
	cmd := findCommand(cmds, "tutor")
	if cmd == nil {
		t.Fatal("expected 'tutor' command")
	}
	msg := cmd.Handler()
	if _, ok := msg.(ShowTutorMsg); !ok {
		t.Errorf("expected ShowTutorMsg, got %T", msg)
	}
}

func TestGlobalCommands_AskAI(t *testing.T) {
	app := newTestAppModel()
	cmds := app.globalCommands()
	cmd := findCommand(cmds, "ask ai")
	if cmd == nil {
		t.Fatal("expected 'ask ai' command")
	}
	msg := cmd.Handler()
	if _, ok := msg.(ShowAskMsg); !ok {
		t.Errorf("expected ShowAskMsg, got %T", msg)
	}
}

func TestGlobalCommands_StagesSettings(t *testing.T) {
	app := newTestAppModel()
	cmds := app.globalCommands()
	cmd := findCommand(cmds, "stages settings")
	if cmd == nil {
		t.Fatal("expected 'stages settings' command")
	}
	msg := cmd.Handler()
	if _, ok := msg.(SettingsStagesMsg); !ok {
		t.Errorf("expected SettingsStagesMsg, got %T", msg)
	}
}

func TestGlobalCommands_Back(t *testing.T) {
	app := newTestAppModel()
	cmds := app.globalCommands()
	cmd := findCommand(cmds, "back")
	if cmd == nil {
		t.Fatal("expected 'back' command")
	}
	msg := cmd.Handler()
	if _, ok := msg.(BackMsg); !ok {
		t.Errorf("expected BackMsg, got %T", msg)
	}
}

func TestGlobalCommands_SwitchProject(t *testing.T) {
	app := newTestAppModel()
	cmds := app.globalCommands()
	cmd := findCommand(cmds, "switch project")
	if cmd == nil {
		t.Fatal("expected 'switch project' command")
	}
	msg := cmd.Handler()
	if _, ok := msg.(ShowProjectPickerMsg); !ok {
		t.Errorf("expected ShowProjectPickerMsg, got %T", msg)
	}
}

func TestGlobalCommands_WorkflowRules(t *testing.T) {
	app := newTestAppModel()
	cmds := app.globalCommands()
	cmd := findCommand(cmds, "workflow rules")
	if cmd == nil {
		t.Fatal("expected 'workflow rules' command")
	}
	msg := cmd.Handler()
	if _, ok := msg.(ShowWorkflowRulesMsg); !ok {
		t.Errorf("expected ShowWorkflowRulesMsg, got %T", msg)
	}
}

func TestGlobalCommands_NewWorkflowRule(t *testing.T) {
	app := newTestAppModel()
	cmds := app.globalCommands()
	cmd := findCommand(cmds, "new workflow rule")
	if cmd == nil {
		t.Fatal("expected 'new workflow rule' command")
	}
	msg := cmd.Handler()
	if _, ok := msg.(ShowWorkflowRuleCreatorMsg); !ok {
		t.Errorf("expected ShowWorkflowRuleCreatorMsg, got %T", msg)
	}
}

func TestGlobalCommands_TaskTemplates(t *testing.T) {
	app := newTestAppModel()
	cmds := app.globalCommands()
	cmd := findCommand(cmds, "task templates")
	if cmd == nil {
		t.Fatal("expected 'task templates' command")
	}
	msg := cmd.Handler()
	if _, ok := msg.(ShowTaskTemplatesMsg); !ok {
		t.Errorf("expected ShowTaskTemplatesMsg, got %T", msg)
	}
}

func TestGlobalCommands_CreateTaskTemplate(t *testing.T) {
	app := newTestAppModel()
	cmds := app.globalCommands()
	cmd := findCommand(cmds, "create task template")
	if cmd == nil {
		t.Fatal("expected 'create task template' command")
	}
	msg := cmd.Handler()
	if _, ok := msg.(ShowTaskTemplateCreatorMsg); !ok {
		t.Errorf("expected ShowTaskTemplateCreatorMsg, got %T", msg)
	}
}
