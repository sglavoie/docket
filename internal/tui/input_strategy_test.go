package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func registeredSimplifiedOperationRegistry(t *testing.T) *OperationRegistry {
	t.Helper()
	registry := NewOperationRegistry()
	for _, descriptor := range coreInputOperationDescriptors() {
		if err := registry.Register(descriptor); err != nil {
			t.Fatalf("Register(%s): %v", descriptor.ID, err)
		}
	}
	if err := registry.RegisterInputStrategy(SimplifiedInputStrategy()); err != nil {
		t.Fatalf("RegisterInputStrategy: %v", err)
	}
	return registry
}

func simplifiedOperationContext() CommandContext {
	ctx := PresentationCommandContext(ViewCrossProjectOutliner)
	ctx.Capabilities.Add(CapabilitySimplifiedInput)
	return ctx
}

func TestSimplifiedInputStrategyIsCollisionAndPrefixFree(t *testing.T) {
	registry := registeredSimplifiedOperationRegistry(t)
	strategy, ok := registry.InputStrategy(InputStrategySimplified)
	if !ok {
		t.Fatal("simplified strategy was not registered")
	}

	seen := make(map[string]OperationID)
	for _, binding := range strategy.Bindings {
		if binding.DisplayKey == "" {
			t.Fatalf("%s has no documented display key", binding.OperationID)
		}
		for _, key := range binding.Keys {
			if !isDirectInputKey(key) {
				t.Errorf("%s key %q is not one direct event", binding.OperationID, key)
			}
			if owner, exists := seen[key]; exists {
				t.Errorf("key %q is shared by %s and %s", key, owner, binding.OperationID)
			}
			seen[key] = binding.OperationID
		}
	}

	handled := registry.HandledKeysForInputStrategy(InputStrategySimplified)
	if len(handled) != len(seen) {
		t.Fatalf("handled keys = %d, physical bindings = %d", len(handled), len(seen))
	}
	help := registry.HelpItemsForInputStrategy(InputStrategySimplified, simplifiedOperationContext())
	if len(help) != len(strategy.Bindings) {
		t.Fatalf("help items = %d, operation bindings = %d", len(help), len(strategy.Bindings))
	}
	footer := registry.FooterHintsForInputStrategy(
		InputStrategySimplified,
		simplifiedOperationContext(),
		0,
	)
	for _, want := range []string{"a capture", "/ search", ": finder", "Ctrl+Esc root", "? help"} {
		if !strings.Contains(footer, want) {
			t.Errorf("footer %q does not contain %q", footer, want)
		}
	}
}

func TestSimplifiedInputStrategySeparatesVisibleSearchAndFinder(t *testing.T) {
	registry := registeredSimplifiedOperationRegistry(t)
	ctx := simplifiedOperationContext()
	tests := []struct {
		key  string
		want OperationID
	}{
		{key: "/", want: operationCoreVisibleSearch},
		{key: ":", want: operationCoreFinder},
		{key: "h", want: operationCoreCollapseParent},
		{key: "left", want: operationCoreCollapseParent},
		{key: "l", want: operationCoreExpand},
		{key: "right", want: operationCoreExpand},
	}
	for _, test := range tests {
		binding, _, ok := registry.InputBindingForKey(InputStrategySimplified, test.key, ctx)
		if !ok {
			t.Fatalf("key %q did not resolve", test.key)
		}
		if binding.OperationID != test.want {
			t.Errorf("key %q resolved to %s, want %s", test.key, binding.OperationID, test.want)
		}
	}
}

func TestInputStrategyRegistrationRejectsCollisionsAndPrefixes(t *testing.T) {
	newRegistry := func(t *testing.T) *OperationRegistry {
		t.Helper()
		registry := NewOperationRegistry()
		for _, descriptor := range coreInputOperationDescriptors()[:2] {
			if err := registry.Register(descriptor); err != nil {
				t.Fatal(err)
			}
		}
		return registry
	}

	t.Run("collision", func(t *testing.T) {
		registry := newRegistry(t)
		err := registry.RegisterInputStrategy(InputStrategy{
			ID: "collision",
			Bindings: []OperationKeyBinding{
				{OperationID: operationCoreCapture, Keys: []string{"a"}},
				{OperationID: operationCoreEdit, Keys: []string{"a"}},
			},
		})
		if err == nil || !strings.Contains(err.Error(), "collision") {
			t.Fatalf("error = %v, want collision rejection", err)
		}
	})

	t.Run("prefix", func(t *testing.T) {
		registry := newRegistry(t)
		err := registry.RegisterInputStrategy(InputStrategy{
			ID: "prefix",
			Bindings: []OperationKeyBinding{
				{OperationID: operationCoreCapture, Keys: []string{"gg"}},
			},
		})
		if err == nil || !strings.Contains(err.Error(), "direct key event") {
			t.Fatalf("error = %v, want prefix rejection", err)
		}
	})
}

func TestSimplifiedDispatchConsumesLegacyKeysAndDefersInlineSearch(t *testing.T) {
	app := newUITestApp()
	app.operationRegistry = NewOperationRegistry()
	app.operationExecutor = NewOperationExecutor(app.operationRegistry)
	if err := app.registerCoreInputOperations(); err != nil {
		t.Fatalf("registerCoreInputOperations: %v", err)
	}
	if err := app.operationRegistry.RegisterInputStrategy(SimplifiedInputStrategy()); err != nil {
		t.Fatalf("RegisterInputStrategy: %v", err)
	}
	app.wireOutlinerOperationExecutor(app.crossOutliner)

	for _, retired := range []string{"g", "F", "P", "C", "q", "w", "i", "x", "#", "|", "ctrl+n", "ctrl+x"} {
		key, err := parseUITestKey(retired)
		if err != nil {
			t.Fatalf("parse retired key %q: %v", retired, err)
		}
		handled, cmd := app.handleGlobalKeyPress(key)
		if !handled || cmd != nil {
			t.Errorf("retired key %q = handled %v, cmd %v; want consumed no-op", retired, handled, cmd)
		}
	}

	app.crossOutliner.ActivateSearch()
	handled, cmd := app.handleGlobalKeyPress(tea.KeyPressMsg{Code: 'a', Text: "a"})
	if handled || cmd != nil {
		t.Fatalf("search input a = handled %v, cmd %v; want delegated to inline input", handled, cmd)
	}
}

func TestSimplifiedHelpContainsNoRetiredShortcutAliases(t *testing.T) {
	registry := registeredSimplifiedOperationRegistry(t)
	items := registry.HelpItemsForInputStrategy(InputStrategySimplified, simplifiedOperationContext())
	keys := make(map[string]bool, len(items))
	for _, item := range items {
		keys[item.Key] = true
	}
	for _, retired := range []string{"g", "F", "P", "C", "q", "w", "i", "x", "#", "|", "Ctrl+N", "Ctrl+X"} {
		if keys[retired] {
			t.Errorf("simplified help retains retired shortcut alias %q", retired)
		}
	}
}

func TestProductionInputStrategyDefaultsToSimplified(t *testing.T) {
	runtime := newTestConfig()
	app, err := NewAppModel(runtime, nil, WithPersister(NewMockPersister()))
	if err != nil {
		t.Fatalf("NewAppModel: %v", err)
	}
	if app.operationRegistry == nil || app.operationExecutor == nil {
		t.Fatal("default operation system is not initialized")
	}
	if app.crossOutliner == nil || app.crossOutliner.operationRegistry != app.operationRegistry {
		t.Fatal("root Outliner is not wired to the operation registry")
	}

	handled, cmd := app.handleGlobalKeyPress(tea.KeyPressMsg{Code: ':', Text: ":"})
	if !handled || cmd == nil {
		t.Fatal("canonical Finder key was not dispatched")
	}
	if _, ok := cmd().(ShowFinderMsg); !ok {
		t.Fatalf("Finder key produced %T, want ShowFinderMsg", cmd())
	}

	handled, cmd = app.handleGlobalKeyPress(tea.KeyPressMsg{Code: 'g', Text: "g"})
	if !handled || cmd != nil {
		t.Fatalf("retired legacy prefix g = handled %v cmd %v; want consumed no-op", handled, cmd)
	}
}

func TestProductionFinderKeyIsGlobalFromChildRoutes(t *testing.T) {
	runtime := newTestConfig()
	app, err := NewAppModel(runtime, nil, WithPersister(NewMockPersister()))
	if err != nil {
		t.Fatalf("NewAppModel: %v", err)
	}
	app.pushView(ViewSettings)

	handled, cmd := app.handleGlobalKeyPress(tea.KeyPressMsg{Code: ':', Text: ":"})
	if !handled || cmd == nil {
		t.Fatal("canonical Finder key was not dispatched from a child route")
	}
	if _, ok := cmd().(ShowFinderMsg); !ok {
		t.Fatalf("child-route Finder key produced %T, want ShowFinderMsg", cmd())
	}
}
