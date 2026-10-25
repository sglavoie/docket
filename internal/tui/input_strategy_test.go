package tui

import (
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/tui/operation"
	"github.com/sglavoie/docket/internal/tui/outliner"
)

func TestSimplifiedInputStrategyIsCollisionAndPrefixFree(t *testing.T) {
	registry := registeredSimplifiedOperationRegistry(t)
	strategy, ok := registry.InputStrategy(operation.InputStrategySimplified)
	if !ok {
		t.Fatal("simplified strategy was not registered")
	}

	ctx := simplifiedOperationContext()
	seen := make(map[string]operation.OperationID)
	sharable := make(map[string]bool)
	pathKeys := make(map[string]bool)
	documented := make(map[operation.OperationID]bool)
	for _, binding := range strategy.Bindings {
		if binding.DisplayKey == "" {
			t.Fatalf("%s has no documented display key", binding.OperationID)
		}
		descriptor, exists := registry.Descriptor(binding.OperationID)
		if !exists {
			t.Fatalf("%s has no descriptor", binding.OperationID)
		}
		if descriptor.Applicable(ctx) {
			documented[binding.OperationID] = true
		}
		for _, key := range binding.Keys {
			if !operation.IsDirectInputKey(key) {
				t.Errorf("%s key %q is not one direct event", binding.OperationID, key)
			}
			// A repeated key is a grammar defect unless every binding on it opted
			// into sharing, which is what keeps an accidental duplicate loud.
			shared := binding.SharedKey && sharable[key]
			if owner, exists := seen[key]; exists && !shared {
				t.Errorf("key %q is shared by %s and %s", key, owner, binding.OperationID)
			}
			seen[key] = binding.OperationID
			sharable[key] = binding.SharedKey
		}
		for _, path := range binding.Paths() {
			for _, key := range path {
				pathKeys[key] = true
			}
		}
	}

	handled := registry.HandledKeysForInputStrategy(operation.InputStrategySimplified)
	if len(handled) != len(pathKeys) {
		t.Fatalf("handled keys = %d, keys on declared paths = %d", len(handled), len(pathKeys))
	}
	// One help row per applicable operation, except a chord whose one display key
	// compresses several labeled presets (`wi/g/w/p/m/d/c`): `?` names each of
	// those letters on its own row instead of leaving initials to decode.
	presetRows := make(map[operation.OperationID]int)
	for _, binding := range strategy.Bindings {
		if documented[binding.OperationID] && binding.HasPresetValue && binding.HintLabel != "" &&
			len(binding.Sequence) > 0 && strings.Contains(binding.DisplayKey, "/") {
			presetRows[binding.OperationID]++
		}
	}
	want := len(documented)
	for _, rows := range presetRows {
		want += rows - 1
	}
	help := registry.HelpItemsForInputStrategy(operation.InputStrategySimplified, ctx)
	if len(help) != want {
		t.Fatalf("help items = %d, want %d (applicable bound operations %d plus spelled-out presets)", len(help), want, len(documented))
	}
	footer := guidanceHintsText(registry.GuidanceHintsForInputStrategy(
		operation.InputStrategySimplified, simplifiedOperationContext(),
	))
	for _, want := range []string{"a capture", "w workflow", "/ search", "T theme", "? help"} {
		if !strings.Contains(footer, want) {
			t.Errorf("footer %q does not contain %q", footer, want)
		}
	}
	for _, retired := range []string{": finder", "Ctrl+Esc root"} {
		if strings.Contains(footer, retired) {
			t.Errorf("footer %q retains removed hint %q", footer, retired)
		}
	}
}

func TestSimplifiedInputStrategyUsesVisibleSearchAndFinalNavigation(t *testing.T) {
	registry := registeredSimplifiedOperationRegistry(t)
	ctx := simplifiedOperationContext()
	tests := []struct {
		key  string
		want operation.OperationID
	}{
		{key: "/", want: operation.OperationCoreScopeSearch},
		{key: "h", want: operation.OperationCoreCollapseParent},
		{key: "left", want: operation.OperationCoreCollapseParent},
		{key: "l", want: operation.OperationCoreExpand},
		{key: "right", want: operation.OperationCoreExpand},
		{key: "s", want: operation.OperationCoreToggleSections},
		{key: "ctrl+o", want: operation.OperationCoreJumpBackward},
		{key: "ctrl+i", want: operation.OperationCoreJumpForward},
		{key: "ctrl+e", want: operation.OperationCoreExternalEditor},
	}
	for _, test := range tests {
		binding, _, ok := registry.InputBindingForKey(operation.InputStrategySimplified, test.key, ctx)
		if !ok {
			t.Fatalf("key %q did not resolve", test.key)
		}
		if binding.OperationID != test.want {
			t.Errorf("key %q resolved to %s, want %s", test.key, binding.OperationID, test.want)
		}
	}
	helpHasToggleSections := false
	helpHasJumpBackward := false
	helpHasJumpForward := false
	for _, item := range registry.HelpItemsForInputStrategy(operation.InputStrategySimplified, ctx) {
		if item.Key == "s" && item.Description == "Toggle All Sections" {
			helpHasToggleSections = true
		}
		if item.Key == "Ctrl+O" && item.Description == "Jump to Older Task Position" {
			helpHasJumpBackward = true
		}
		if item.Key == "Alt+→" && item.Description == "Jump to Newer Task Position" {
			helpHasJumpForward = true
		}
	}
	if !helpHasToggleSections {
		t.Error("generated help omits s Toggle All Sections")
	}
	if !helpHasJumpBackward || !helpHasJumpForward {
		t.Errorf("generated help jump rows = backward %t forward %t, want both", helpHasJumpBackward, helpHasJumpForward)
	}
	if _, _, ok := registry.InputBindingForKey(operation.InputStrategySimplified, ":", ctx); ok {
		t.Error("removed Finder key still resolves")
	}
}

// TestSimplifiedNamedLensesIsTaughtByHelpAndNotByTheFooter pins the one axis the
// binding chooses deliberately: `\` is a discoverable entry point, so `?` names
// it while the compact footer stays exactly as quiet as it was.
func TestSimplifiedNamedLensesIsTaughtByHelpAndNotByTheFooter(t *testing.T) {
	registry := registeredSimplifiedOperationRegistry(t)
	ctx := simplifiedOperationContext()

	binding, _, ok := registry.InputBindingForKey(operation.InputStrategySimplified, `\`, ctx)
	if !ok {
		t.Fatal(`\ did not resolve on the Outliner`)
	}
	if binding.OperationID != operation.OperationCoreNamedLenses {
		t.Fatalf(`\ resolved to %s, want %s`, binding.OperationID, operation.OperationCoreNamedLenses)
	}
	if binding.FooterRank != 0 || binding.FooterLabel != "" {
		t.Errorf("named lenses declares footer rank %d label %q, want neither", binding.FooterRank, binding.FooterLabel)
	}

	taught := false
	for _, item := range registry.HelpItemsForInputStrategy(operation.InputStrategySimplified, ctx) {
		if item.Key == `\` && item.Description == "Named Lenses" {
			taught = true
		}
	}
	if !taught {
		t.Error(`generated help omits \ Named Lenses`)
	}

	footer := guidanceHintsText(registry.GuidanceHintsForInputStrategy(
		operation.InputStrategySimplified, ctx,
	))
	if strings.Contains(footer, `\`) {
		t.Errorf("footer %q advertises the named-lens key", footer)
	}
}

func TestSimplifiedCtrlEOpensCursorTaskInExternalEditor(t *testing.T) {
	app := newUITestApp()
	first := &models.Task{
		ID: "first", ShortID: 1, Title: "First",
		WorkflowState: models.StateBacklog,
	}
	selected := &models.Task{
		ID: "selected", ShortID: 2, Title: "Selected",
		WorkflowState: models.StateBacklog,
	}
	project := &models.Project{
		ID: "alpha", Name: "Alpha", Slug: "alpha",
		Tasks: []*models.Task{first, selected},
	}
	project.BuildTaskTree()
	setTestAppProjects(app, project)
	app.crossOutliner.Population.Projects = []*models.Project{project}
	app.crossOutliner.Rows = []outliner.OutlinerRow{
		{Task: first, ProjectID: project.ID},
		{Task: selected, ProjectID: project.ID},
	}
	app.crossOutliner.Cursor = 1

	handled, cmd := app.handleGlobalKeyPress(tea.KeyPressMsg{
		Code: 'e',
		Mod:  tea.ModCtrl,
	})
	if !handled || cmd == nil {
		t.Fatalf("Ctrl+E = handled %v cmd %v, want external-editor dispatch", handled, cmd)
	}
	msg, ok := cmd().(EditTaskContentMsg)
	if !ok {
		t.Fatalf("Ctrl+E emitted %T, want EditTaskContentMsg", cmd())
	}
	if msg.Task != selected || msg.Project != project {
		t.Fatalf(
			"Ctrl+E target = task %v project %v, want selected task %v project %v",
			msg.Task,
			msg.Project,
			selected,
			project,
		)
	}
}

func TestSimplifiedNavigationAliasesShareOneOperationAndDisplayKey(t *testing.T) {
	registry := registeredSimplifiedOperationRegistry(t)
	ctx := simplifiedOperationContext()
	aliases := []struct {
		keys    []string
		want    operation.OperationID
		display string
	}{
		{keys: []string{"home"}, want: operation.OperationCoreCursorTop, display: "gg/Home"},
		{keys: []string{"G", "end"}, want: operation.OperationCoreCursorBottom, display: "G/End"},
		{keys: []string{"ctrl+u", "pgup"}, want: operation.OperationCoreCursorPageUp, display: "Ctrl+U/PgUp"},
		{keys: []string{"ctrl+d", "pgdown"}, want: operation.OperationCoreCursorPageDown, display: "Ctrl+D/PgDn"},
	}

	handled := registry.HandledKeysForInputStrategy(operation.InputStrategySimplified)
	helpKeys := make(map[string]bool)
	for _, item := range registry.HelpItemsForInputStrategy(operation.InputStrategySimplified, ctx) {
		helpKeys[item.Key] = true
	}

	for _, alias := range aliases {
		for _, key := range alias.keys {
			binding, _, ok := registry.InputBindingForKey(operation.InputStrategySimplified, key, ctx)
			if !ok {
				t.Fatalf("key %q did not resolve", key)
			}
			if binding.OperationID != alias.want {
				t.Errorf("key %q resolved to %s, want %s", key, binding.OperationID, alias.want)
			}
			if binding.DisplayKey != alias.display {
				t.Errorf("key %q display = %q, want %q", key, binding.DisplayKey, alias.display)
			}
			if !slices.Contains(handled, key) {
				t.Errorf("handled keys omit %q", key)
			}
		}
		if !helpKeys[alias.display] {
			t.Errorf("help omits %q", alias.display)
		}
	}

	footer := guidanceHintsText(registry.GuidanceHintsForInputStrategy(operation.InputStrategySimplified, ctx))
	for _, alias := range aliases {
		if strings.Contains(footer, alias.display) {
			t.Errorf("compact footer spends its hint budget on %q", alias.display)
		}
	}
}

func TestPortableKeysRetainEnhancedAliases(t *testing.T) {
	registry := registeredSimplifiedOperationRegistry(t)
	ctx := simplifiedOperationContext()
	ctx.Entity = operation.EntityContext{ProjectID: "project", TaskID: "parent"}
	ctx.CurrentEntity = operation.TaskEntity("project", "parent")
	for _, pair := range [][2]string{{"alt+right", "ctrl+i"}, {"alt+up", "shift+enter"}, {"f10", "ctrl+m"}} {
		first, _, ok := registry.InputBindingForKey(operation.InputStrategySimplified, pair[0], ctx)
		second, _, also := registry.InputBindingForKey(operation.InputStrategySimplified, pair[1], ctx)
		if !ok || !also || first.OperationID != second.OperationID {
			t.Fatalf("portable alias %v differs", pair)
		}
	}
}
