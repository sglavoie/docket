package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/docket/internal/tui/help"
	"github.com/sglavoie/docket/internal/tui/listsurface"
	"github.com/sglavoie/docket/internal/tui/operation"
	"github.com/sglavoie/docket/internal/tui/operationhost"
	"github.com/sglavoie/docket/internal/tui/outliner"
	"github.com/sglavoie/docket/internal/tui/presentation"
	"github.com/sglavoie/docket/internal/tui/projecthub"
	"github.com/sglavoie/docket/internal/tui/query"
	"github.com/sglavoie/docket/internal/tui/route"
)

func TestProjectHubHelpNamesTheTaskFilterOnce(t *testing.T) {
	hub := openProjectHub(t, projectHubApp(t))
	found := 0
	for _, item := range hub.GetHelpItems() {
		if item.Key != "Ctrl+T" {
			continue
		}
		found++
		if item.Description != "Cycle every project, only those holding open tasks, only those holding none open" ||
			item.Category != "Discovery" {
			t.Fatalf("Ctrl+T help item = %+v", item)
		}
	}
	if found != 1 {
		t.Fatalf("Ctrl+T help entries = %d, want exactly 1", found)
	}
}

// TestProjectHubHelpSpellsEachKeyOnce holds the merged `?` rows to the
// spelling every other help row uses: a key pair is named whole and capitalized,
// and the split's footer-only pair does not repeat the registry's two
// directional rows.
func TestProjectHubHelpSpellsEachKeyOnce(t *testing.T) {
	hub := openProjectHub(t, projectHubApp(t))
	hub.SetSize(120, 40)
	documented := map[string]string{}
	for _, item := range hub.GetHelpItems() {
		if previous, dup := documented[item.Key]; dup {
			t.Fatalf("help lists %q twice: %q and %q", item.Key, previous, item.Description)
		}
		documented[item.Key] = item.Description
	}
	for key, want := range map[string]string{
		"j/k":      "Move selection",
		"Ctrl+D/U": "Move half page",
		"<":        "Move Pane Split Left",
		">":        "Move Pane Split Right",
		"Ctrl+R":   "Redo workspace change",
	} {
		if got := documented[key]; got != want {
			t.Errorf("help row %q = %q, want %q (rows: %v)", key, got, want, documented)
		}
	}
	for _, stale := range []string{"j", "ctrl+d", "ctrl+r", "</>"} {
		if description, listed := documented[stale]; listed {
			t.Errorf("help still spells %q (%q)", stale, description)
		}
	}
}

// TestProjectHubHelpKeyRaisesTheHubGrammar holds `?` to the surface it was
// pressed on. The core help descriptor hardcodes the Outliner, so dispatching it
// through the executor would answer a Hub press with the Outliner's grammar; the
// Hub raises its own instead, and the press stays a literal character inside the
// query where a search is being typed.
func TestProjectHubHelpKeyRaisesTheHubGrammar(t *testing.T) {
	app := projectHubApp(t)
	hub := openProjectHub(t, app)

	_, cmd := hub.Update(syntheticKey('?'))
	if cmd == nil {
		t.Fatal("? raised no help on the Project Hub")
	}
	raised, ok := cmd().(help.ShowHelpMsg)
	if !ok {
		t.Fatalf("? produced %T, want help.ShowHelpMsg", cmd())
	}
	if raised.ViewName != "Projects" {
		t.Fatalf("help header = %q, want \"Projects\"", raised.ViewName)
	}

	documented := make(map[string]string, len(raised.Items))
	for _, item := range raised.Items {
		documented[item.Key] = item.Description
	}
	if documented["Enter"] != "Open Project" {
		t.Fatalf("Hub help describes Enter as %q, want the Hub's own grammar: %+v", documented["Enter"], raised.Items)
	}
	// `?` documents itself only because the keymap names it: the core descriptor
	// is inapplicable off the Outliner, so the registry half of the merge drops it.
	if documented["?"] != "Help" {
		t.Fatalf("Hub help omits ? itself: %+v", raised.Items)
	}
	for _, outlinerOnly := range []string{"tab", "T", "ctrl+p"} {
		if description, listed := documented[outlinerOnly]; listed {
			t.Fatalf("Hub help leaked the Outliner key %q (%q)", outlinerOnly, description)
		}
	}

	searching := openProjectHub(t, projectHubApp(t))
	searching.Update(syntheticKey('/'))
	// The input's own cursor blink can ride along, so the message decides rather
	// than the presence of a command.
	if _, cmd := searching.Update(syntheticKey('?')); cmd != nil {
		if raised, isHelp := cmd().(help.ShowHelpMsg); isHelp {
			t.Fatalf("? raised %q help from inside the query instead of typing a character", raised.ViewName)
		}
	}
	if got := searching.SearchQuery(); got != "?" {
		t.Fatalf("query after ? = %q, want a literal %q", got, "?")
	}
	for _, item := range searching.GetHelpItems() {
		if item.Key == "?" {
			t.Fatalf("search-editing help still offers ?: %+v", item)
		}
	}
}

func TestShowProjectHubMsgOpensTheHubInBrowse(t *testing.T) {
	app := projectHubApp(t)
	handled, cmd := app.handleProjectViewMessages(projecthub.ShowProjectHubMsg{})
	if !handled {
		t.Fatal("ShowProjectHubMsg was not handled")
	}
	if cmd != nil {
		cmd()
	}
	hub, ok := app.manageHub.ActiveModuleView().(*projecthub.ProjectHubModel)
	if app.CurrentRoute().ID() != route.SecondaryTool || app.currentView() != presentation.ViewProjectHub || !ok {
		t.Fatalf("Project Hub landed on %s/%s (%T)", app.CurrentRoute().ID(), app.currentView(), app.manageHub.ActiveModuleView())
	}
	if hub.HasActiveSearch() {
		t.Fatal("the Project Hub opened into search instead of browse")
	}
	// A live cursor is what makes the first Enter mean something, so the message
	// it emits is the assertion rather than the Hub's own cursor field.
	hub.SetSize(120, 30)
	_, cmd = hub.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("the Project Hub opened without a live cursor")
	}
	if selected, ok := cmd().(outliner.ApplyProjectScopeMsg); !ok || selected.ProjectID != "alpha" {
		t.Fatalf("Enter on a freshly opened Hub emitted %#v, want the first project", cmd())
	}
}

func TestOutlinerProjectHubShortcutsChooseTheirExplicitModes(t *testing.T) {
	app := projectHubApp(t)
	ctx := app.operationCommandContext()
	ctx.View = presentation.ViewCrossProjectOutliner

	for _, test := range []struct {
		name string
		path []string
		mode route.ProjectHubMode
	}{
		{name: "Ctrl+P pinned", path: []string{"ctrl+p"}, mode: route.ProjectHubModePinned},
		{name: "uppercase P all", path: []string{"P"}, mode: route.ProjectHubModeAll},
	} {
		t.Run(test.name, func(t *testing.T) {
			result := executeDirectPath(t, app, ctx, test.path)
			message, ok := result.Message.(projecthub.ShowProjectHubMsg)
			if !ok || message.Mode != test.mode {
				t.Fatalf("path %v emitted %#v, want ShowProjectHubMsg mode %v", test.path, result.Message, test.mode)
			}
			handled, cmd := app.handleProjectViewMessages(message)
			if !handled {
				t.Fatalf("path %v message was not handled", test.path)
			}
			if cmd != nil {
				cmd()
			}
			params := app.CurrentRoute().Params().(route.ManageParams)
			hub, mounted := app.manageHub.ActiveModuleView().(*projecthub.ProjectHubModel)
			mountedMode := route.ProjectHubMode(255)
			if mounted {
				mountedMode = hub.Mode()
			}
			if params.ProjectHubMode != test.mode || !mounted || mountedMode != test.mode {
				t.Fatalf("path %v route/model mode = %v/%v (%T)", test.path, params.ProjectHubMode, mountedMode, app.manageHub.ActiveModuleView())
			}
			app.viewNav.navigation.ResetRoot()
		})
	}
}

func TestCtrlPRemainsOwnedByInlineOutlinerControls(t *testing.T) {
	app := projectHubApp(t)
	out := app.crossOutliner
	ctrlP := syntheticCtrlKey('p')

	out.ActivateSearch()
	if handled, cmd := app.handleSimplifiedInputKey(ctrlP); handled || cmd != nil {
		t.Fatalf("active search let Ctrl+P reach the Outliner grammar: handled=%v cmd=%v", handled, cmd)
	}
	if app.CurrentRoute().ID() != route.TaskWorkspace {
		t.Fatal("Ctrl+P opened the Project Hub while search owned input")
	}
	out.ClearSearch()

	out.Confirm.Show("test", "Confirm?")
	if handled, cmd := app.handleSimplifiedInputKey(ctrlP); handled || cmd != nil {
		t.Fatalf("active confirmation let Ctrl+P reach the Outliner grammar: handled=%v cmd=%v", handled, cmd)
	}
	if app.CurrentRoute().ID() != route.TaskWorkspace {
		t.Fatal("Ctrl+P opened the Project Hub while confirmation owned input")
	}
	out.Confirm.Cancel()

	app.operations.TextPrompt = operationhost.NewOperationTextPromptModel(
		operation.OperationExecutionRequest{}, "Name", "", "draft", "Name",
		operationhost.NameTextPromptOptions(),
	)
	if handled, _ := app.handleGlobalKeyPress(ctrlP); !handled || app.operations.TextPrompt == nil {
		t.Fatalf("text prompt did not retain Ctrl+P: handled=%v prompt=%v", handled, app.operations.TextPrompt != nil)
	}
	app.operations.TextPrompt = nil

	app.operations.ValuePicker = newSemanticTagPickerForTest(t, []query.TagInfo{{Name: "urgent", TaskCount: 1}}, true)
	if handled, _ := app.handleGlobalKeyPress(ctrlP); !handled || app.operations.ValuePicker == nil {
		t.Fatalf("value picker did not retain Ctrl+P: handled=%v picker=%v", handled, app.operations.ValuePicker != nil)
	}
	app.operations.ValuePicker = nil

	descriptor := contextActionTestDescriptor("test.ctrl-p-owner", "Owner")
	registry, executor := contextActionTestEngine(t, []operation.OperationDescriptor{descriptor}, nil)
	sheet, err := listsurface.NewContextActionSheetModel(
		registry,
		executor,
		contextActionTestContext(),
		[]operation.OperationID{descriptor.ID},
		listsurface.ContextActionSheetConfig{Host: listsurface.ContextActionHostOutliner},
	)
	if err != nil {
		t.Fatalf("NewContextActionSheetModel: %v", err)
	}
	app.operations.ActionSheet = sheet
	if handled, _ := app.handleGlobalKeyPress(ctrlP); !handled || app.operations.ActionSheet == nil {
		t.Fatalf("action sheet did not retain Ctrl+P: handled=%v sheet=%v", handled, app.operations.ActionSheet != nil)
	}
	app.operations.ActionSheet = nil

	app.operations.ArmConfirm(operation.OperationExecutionRequest{ID: "test.confirm"}, "Confirm?")
	if handled, _ := app.handleGlobalKeyPress(ctrlP); !handled || !app.operations.Confirm.IsActive() {
		t.Fatalf("operation confirmation did not retain Ctrl+P: handled=%v active=%v", handled, app.operations.Confirm.IsActive())
	}
	if app.CurrentRoute().ID() != route.TaskWorkspace {
		t.Fatal("Ctrl+P opened the Project Hub while an overlay owned input")
	}
}
