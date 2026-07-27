package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/config"
)

func TestManageAutomationModulesUseOneTypedRoute(t *testing.T) {
	tests := []struct {
		name     string
		open     func(*AppModel) (bool, tea.Cmd)
		module   ManageModule
		mode     ManageRulesMode
		wantView any
	}{
		{
			name: "workflow rules",
			open: func(app *AppModel) (bool, tea.Cmd) {
				return app.handleViewSettingsMessages(ShowWorkflowRulesMsg{})
			},
			module: ManageRules, mode: ManageRulesWorkflow, wantView: (*WorkflowRulesModel)(nil),
		},
		{
			name: "scoring profiles",
			open: func(app *AppModel) (bool, tea.Cmd) {
				return app.handleScoringMessages(ShowScoreProfileManagerMsg{})
			},
			module: ManageRules, mode: ManageRulesScoring, wantView: (*ScoreProfileManagerModel)(nil),
		},
		{
			name: "templates",
			open: func(app *AppModel) (bool, tea.Cmd) {
				return app.handleViewSettingsMessages(ShowTaskTemplatesMsg{})
			},
			module: ManageTemplates, wantView: (*TaskTemplatesModel)(nil),
		},
		{
			name: "settings",
			open: func(app *AppModel) (bool, tea.Cmd) {
				return app.handleViewSettingsMessages(SettingsMsg{})
			},
			module: ManageSettings, wantView: (*SettingsModel)(nil),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			app := typedTestApp(t)
			handled, _ := tc.open(app)
			if !handled {
				t.Fatal("deep link was not handled")
			}
			route := app.CurrentRoute()
			params, ok := route.Params().(ManageParams)
			if route.ID() != SecondaryTool || !ok || params.Module != tc.module || params.Scope != AppEntityScope() {
				t.Fatalf("route = %s/%#v, want SecondaryTool/%s/app", route.ID(), route.Params(), tc.module)
			}
			if _, legacy := route.Presentation(); legacy {
				t.Fatal("Manage automation module retained a legacy route presentation")
			}
			if app.manageHub.RulesMode() != tc.mode && tc.module == ManageRules {
				t.Fatalf("Rules mode = %s, want %s", app.manageHub.RulesMode(), tc.mode)
			}
			active := app.manageHub.ActiveModuleView()
			switch tc.wantView.(type) {
			case *WorkflowRulesModel:
				if _, ok := active.(*WorkflowRulesModel); !ok {
					t.Fatalf("active view = %T, want WorkflowRulesModel", active)
				}
			case *ScoreProfileManagerModel:
				if _, ok := active.(*ScoreProfileManagerModel); !ok {
					t.Fatalf("active view = %T, want ScoreProfileManagerModel", active)
				}
			case *TaskTemplatesModel:
				if _, ok := active.(*TaskTemplatesModel); !ok {
					t.Fatalf("active view = %T, want TaskTemplatesModel", active)
				}
			case *SettingsModel:
				if _, ok := active.(*SettingsModel); !ok {
					t.Fatalf("active view = %T, want SettingsModel", active)
				}
			}
		})
	}
}

func TestManageRulesViewMenuSwitchesHubOwnedMode(t *testing.T) {
	app := typedTestApp(t)
	app.activateManageRules(ManageRulesWorkflow)
	before := app.CurrentRoute()

	_, cmd := app.manageHub.Update(syntheticKey('v'))
	if cmd == nil {
		t.Fatal("Rules v did not open the mode picker")
	}
	app.handleManageHubMessages(cmd())
	if app.navigation.Overlays.Depth() != 1 || app.manageHub.pickerTitle != "Rules mode" {
		t.Fatalf("Rules picker = depth %d title %q", app.navigation.Overlays.Depth(), app.manageHub.pickerTitle)
	}
	app.manageHub.pickerCursor = 1
	_, cmd = app.manageHub.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("Scoring selection emitted no module change")
	}
	app.handleManageHubMessages(cmd())

	if !app.CurrentRoute().Equal(before) {
		t.Fatalf("Rules submode changed primary route: before=%#v after=%#v", before, app.CurrentRoute())
	}
	if app.manageHub.RulesMode() != ManageRulesScoring {
		t.Fatalf("Rules mode = %s, want Scoring", app.manageHub.RulesMode())
	}
	if _, ok := app.manageHub.ActiveModuleView().(*ScoreProfileManagerModel); !ok {
		t.Fatalf("active Rules mode = %T, want ScoreProfileManagerModel", app.manageHub.ActiveModuleView())
	}
	if app.currentView() != ViewScoreProfileManager {
		t.Fatalf("compatibility observation = %s, want ScoreProfileManager", app.currentView())
	}
	if app.navigation.Overlays.Depth() != 0 {
		t.Fatalf("Rules picker remained on overlay stack: depth=%d", app.navigation.Overlays.Depth())
	}
}

func TestManageAdministrativeFormsRemainTransient(t *testing.T) {
	app := typedTestApp(t)
	app.activateManageRules(ManageRulesWorkflow)
	primary := app.CurrentRoute()

	checks := []struct {
		name string
		open func() (bool, tea.Cmd)
		kind OverlayKind
	}{
		{name: "rule creator", open: func() (bool, tea.Cmd) {
			return app.handleViewSettingsMessages(ShowWorkflowRuleCreatorMsg{})
		}, kind: OverlayForm},
		{name: "workflow wizard", open: func() (bool, tea.Cmd) {
			return app.handleViewSettingsMessages(ShowWorkflowWizardMsg{})
		}, kind: OverlayForm},
		{name: "workflow dry run", open: func() (bool, tea.Cmd) {
			return app.handleViewSettingsMessages(ShowWorkflowDryRunMsg{})
		}, kind: OverlayPicker},
	}

	for _, tc := range checks {
		t.Run(tc.name, func(t *testing.T) {
			app.navigation.Overlays.CancelAll()
			handled, _ := tc.open()
			if !handled {
				t.Fatal("open message was not handled")
			}
			if !app.CurrentRoute().Equal(primary) {
				t.Fatalf("primary route changed to %#v", app.CurrentRoute())
			}
			overlay, ok := app.navigation.Overlays.Current()
			if !ok || overlay.Kind() != tc.kind {
				t.Fatalf("overlay = %#v/%v, want %s", overlay, ok, tc.kind)
			}
		})
	}
}

func TestSettingsExposeAndPersistOnlySupportedFields(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	dataDir := t.TempDir()
	cfg := newTestConfigAt(dataDir)
	runtime := testRuntimeAt(dataDir, cfg)
	m := NewSettingsModel(&shellDeps{Runtime: runtime, Manager: config.NewManager(dataDir + "/tt.json")})

	for _, query := range []string{"popup timeout", "task form collapsed", "today mode", "dashboard split", "outliner sidebar", "kanban detail"} {
		if got := searchSettingsFields(query); len(got) != 0 {
			t.Fatalf("retired query %q returned %#v", query, got)
		}
	}
	visual := strings.Join(settingsVisualTabNames, " ")
	if strings.Contains(visual, "Views") || len(settingsVisualTabNames) != 4 {
		t.Fatalf("Settings visual tabs = %v, want durable four-tab surface", settingsVisualTabNames)
	}

	m.editorInput.SetValue("sh")
	cmd := m.saveSettings()
	if cmd == nil {
		t.Fatal("saveSettings returned nil")
	}
	if msg := cmd(); msg == nil {
		t.Fatal("saveSettings returned nil message")
	}
	if m.baseMachine.Editor != "sh" {
		t.Fatalf("supported editor was not saved: %q", m.baseMachine.Editor)
	}
}

func TestSettingsThemeResetUsesExplicitTransientMenu(t *testing.T) {
	m, _ := makePresetTestModel(t)
	m.tabs.Active = tabPalette
	m.focusIndex = tabFieldStart[tabPalette]

	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil || !m.themeActionMenu {
		t.Fatal("Enter on the palette action row did not open Theme Actions")
	}
	if rendered := ansi.Strip(m.View().Content); !strings.Contains(rendered, "Theme Actions") {
		t.Fatalf("Theme Actions menu was not rendered:\n%s", rendered)
	}
	m.themeActionCursor = 3
	m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if m.themeActionMenu || m.themePresetMode != presetModeResetConfirm {
		t.Fatalf("reset selection = menu %v mode %d, want explicit confirmation", m.themeActionMenu, m.themePresetMode)
	}

	m.themePresetMode = presetModeNone
	m.handleGlobalKey("ctrl+r")
	if m.themePresetMode != presetModeNone {
		t.Fatal("retired Ctrl+R shortcut still opened palette reset")
	}
}
