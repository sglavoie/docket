package tui

import (
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/operation"
	"github.com/sglavoie/tt/internal/tui/presentation"
)

func registeredSimplifiedOperationRegistry(t *testing.T) *operation.OperationRegistry {
	t.Helper()
	registry := operation.NewOperationRegistry()
	descriptors := append(operation.CoreInputOperationDescriptors(), simplifiedTaskActionDescriptors()...)
	for _, descriptor := range descriptors {
		if err := registry.Register(descriptor); err != nil {
			t.Fatalf("Register(%s): %v", descriptor.ID, err)
		}
	}
	if err := registry.RegisterInputStrategy(SimplifiedInputStrategy()); err != nil {
		t.Fatalf("RegisterInputStrategy: %v", err)
	}
	return registry
}

func guidanceHintsText(hints []chrome.GuidanceHint) string {
	parts := make([]string, 0, len(hints))
	for _, hint := range hints {
		parts = append(parts, chrome.GuidanceHintText(hint))
	}
	return strings.Join(parts, chrome.FooterHelpSeparator)
}

// simplifiedTaskActionDescriptors supplies the task-scoped and filter
// descriptors the simplified strategy binds directly, for tests that assemble a
// registry without a fully initialized AppModel.
func simplifiedTaskActionDescriptors() []operation.OperationDescriptor {
	descriptors := append((&AppModel{}).taskActionOperationDescriptors(), filterOperationDescriptors()...)
	descriptors = append(descriptors, projectFilterOperationDescriptors()...)
	descriptors = append(descriptors, sortAxisOperationDescriptors()...)
	descriptors = append(descriptors, checklistRowOperationDescriptors()...)
	descriptors = append(descriptors, (&AppModel{}).checklistStartOperationDescriptor())
	descriptors = append(descriptors, projectActionSheetOperationDescriptor())
	return append(descriptors, projectOperationDescriptors()...)
}

func simplifiedOperationContext() operation.CommandContext {
	ctx := PresentationCommandContext(presentation.ViewCrossProjectOutliner)
	ctx.Capabilities.Add(operation.CapabilitySimplifiedInput)
	return ctx
}

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
	// One help row per applicable operation, so a preset family collapses instead
	// of documenting the same operation once per spelling.
	help := registry.HelpItemsForInputStrategy(operation.InputStrategySimplified, ctx)
	if len(help) != len(documented) {
		t.Fatalf("help items = %d, applicable bound operations = %d", len(help), len(documented))
	}
	footer := guidanceHintsText(registry.GuidanceHintsForInputStrategy(
		operation.InputStrategySimplified, simplifiedOperationContext(),
	))
	for _, want := range []string{"a capture", "$ workflow", "/ search", "T theme", "? help"} {
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
		if item.Key == "Ctrl+I" && item.Description == "Jump to Newer Task Position" {
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
	app.crossOutliner.loadedProjects = []*models.Project{project}
	app.crossOutliner.rows = []outlinerRow{
		{task: first, ProjectID: project.ID},
		{task: selected, ProjectID: project.ID},
	}
	app.crossOutliner.cursor = 1

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
