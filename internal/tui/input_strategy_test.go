package tui

import (
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
)

func registeredSimplifiedOperationRegistry(t *testing.T) *OperationRegistry {
	t.Helper()
	registry := NewOperationRegistry()
	descriptors := append(coreInputOperationDescriptors(), simplifiedTaskActionDescriptors()...)
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

// simplifiedTaskActionDescriptors supplies the task-scoped and filter
// descriptors the simplified strategy binds directly, for tests that assemble a
// registry without a fully initialized AppModel.
func simplifiedTaskActionDescriptors() []OperationDescriptor {
	return append((&AppModel{}).taskActionOperationDescriptors(), filterOperationDescriptors()...)
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

	ctx := simplifiedOperationContext()
	seen := make(map[string]OperationID)
	pathKeys := make(map[string]bool)
	documented := make(map[OperationID]bool)
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
			if !isDirectInputKey(key) {
				t.Errorf("%s key %q is not one direct event", binding.OperationID, key)
			}
			if owner, exists := seen[key]; exists {
				t.Errorf("key %q is shared by %s and %s", key, owner, binding.OperationID)
			}
			seen[key] = binding.OperationID
		}
		for _, path := range binding.paths() {
			for _, key := range path {
				pathKeys[key] = true
			}
		}
	}

	handled := registry.HandledKeysForInputStrategy(InputStrategySimplified)
	if len(handled) != len(pathKeys) {
		t.Fatalf("handled keys = %d, keys on declared paths = %d", len(handled), len(pathKeys))
	}
	// One help row per applicable operation, so a preset family collapses instead
	// of documenting the same operation once per spelling.
	help := registry.HelpItemsForInputStrategy(InputStrategySimplified, ctx)
	if len(help) != len(documented) {
		t.Fatalf("help items = %d, applicable bound operations = %d", len(help), len(documented))
	}
	footer := registry.FooterHintsForInputStrategy(
		InputStrategySimplified,
		simplifiedOperationContext(),
		0,
	)
	for _, want := range []string{"a capture", "s state/plan", "/ search", "T theme", "? help"} {
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
		want OperationID
	}{
		{key: "/", want: operationCoreScopeSearch},
		{key: "h", want: operationCoreCollapseParent},
		{key: "left", want: operationCoreCollapseParent},
		{key: "l", want: operationCoreExpand},
		{key: "right", want: operationCoreExpand},
		{key: "S", want: operationCoreToggleSections},
		{key: "ctrl+e", want: operationCoreExternalEditor},
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
	helpHasToggleSections := false
	for _, item := range registry.HelpItemsForInputStrategy(InputStrategySimplified, ctx) {
		if item.Key == "S" && item.Description == "Toggle All Sections" {
			helpHasToggleSections = true
			break
		}
	}
	if !helpHasToggleSections {
		t.Error("generated help omits S Toggle All Sections")
	}
	if _, _, ok := registry.InputBindingForKey(InputStrategySimplified, ":", ctx); ok {
		t.Error("removed Finder key still resolves")
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
		want    OperationID
		display string
	}{
		{keys: []string{"g", "home"}, want: operationCoreCursorTop, display: "g/Home"},
		{keys: []string{"G", "end"}, want: operationCoreCursorBottom, display: "G/End"},
		{keys: []string{"ctrl+u", "pgup"}, want: operationCoreCursorPageUp, display: "Ctrl+U/PgUp"},
		{keys: []string{"ctrl+d", "pgdown"}, want: operationCoreCursorPageDown, display: "Ctrl+D/PgDn"},
	}

	handled := registry.HandledKeysForInputStrategy(InputStrategySimplified)
	helpKeys := make(map[string]bool)
	for _, item := range registry.HelpItemsForInputStrategy(InputStrategySimplified, ctx) {
		helpKeys[item.Key] = true
	}

	for _, alias := range aliases {
		for _, key := range alias.keys {
			binding, _, ok := registry.InputBindingForKey(InputStrategySimplified, key, ctx)
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

	footer := registry.FooterHintsForInputStrategy(InputStrategySimplified, ctx, 0)
	for _, alias := range aliases {
		if strings.Contains(footer, alias.display) {
			t.Errorf("compact footer spends its hint budget on %q", alias.display)
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

const chordTestSetPriority OperationID = "test.task.set-priority"

// chordTestRegistry adds one value-taking operation to the core descriptors so
// preset-valued chord bindings can be exercised without the production catalog.
func chordTestRegistry(t *testing.T) *OperationRegistry {
	t.Helper()
	registry := NewOperationRegistry()
	for _, descriptor := range coreInputOperationDescriptors() {
		if err := registry.Register(descriptor); err != nil {
			t.Fatalf("Register(%s): %v", descriptor.ID, err)
		}
	}
	if err := registry.Register(OperationDescriptor{
		ID:                   chordTestSetPriority,
		Label:                "Set Priority",
		Category:             OperationCategoryTask,
		TargetSource:         OperationTargetCurrentTask,
		TargetCardinality:    TargetCardinalityOne,
		RequiredCapabilities: NewCapabilitySet(CapabilitySimplifiedInput),
		Applicability:        coreOutlinerApplicability,
		Risk:                 OperationRiskMutation,
		ValueProviderID:      ValueProviderPriority,
	}); err != nil {
		t.Fatalf("Register(%s): %v", chordTestSetPriority, err)
	}
	return registry
}

func chordTestStrategy() InputStrategy {
	return InputStrategy{
		ID: "chords",
		Bindings: []OperationKeyBinding{
			{OperationID: operationCoreCapture, Keys: []string{"a"}, FooterRank: 1, FooterLabel: "capture"},
			{
				OperationID:    chordTestSetPriority,
				Sequence:       []string{"p", "1"},
				DisplayKey:     "p1-2",
				HintLabel:      "critical",
				PresetValue:    1,
				HasPresetValue: true,
			},
			{
				OperationID:    chordTestSetPriority,
				Sequence:       []string{"p", "2"},
				DisplayKey:     "p1-2",
				HintLabel:      "high",
				PresetValue:    2,
				HasPresetValue: true,
			},
			{OperationID: operationCoreHelp, Sequence: []string{",", "h"}, HintLabel: "help"},
		},
		ChordGroups: []OperationChordGroup{
			{Sequence: []string{"p"}, HintLabel: "priority", FooterRank: 2, FooterLabel: "priority"},
			{Sequence: []string{","}, HintLabel: "state"},
		},
	}
}

func TestChordSequencesResolveAndProjectThroughOneStrategy(t *testing.T) {
	registry := chordTestRegistry(t)
	if err := registry.RegisterInputStrategy(chordTestStrategy()); err != nil {
		t.Fatalf("RegisterInputStrategy: %v", err)
	}
	ctx := simplifiedOperationContext()

	if _, _, resolution := registry.InputBindingForSequence("chords", []string{"p"}, ctx); resolution != ChordPending {
		t.Fatalf("prefix p resolution = %v, want pending", resolution)
	}
	binding, descriptor, resolution := registry.InputBindingForSequence("chords", []string{"p", "2"}, ctx)
	if resolution != ChordTerminal {
		t.Fatalf("chord p2 resolution = %v, want terminal", resolution)
	}
	if descriptor.ID != chordTestSetPriority {
		t.Errorf("chord p2 resolved %s, want %s", descriptor.ID, chordTestSetPriority)
	}
	if !binding.HasPresetValue || binding.PresetValue != 2 {
		t.Errorf("chord p2 preset = (%v, %v), want (2, true)", binding.PresetValue, binding.HasPresetValue)
	}
	if _, _, resolution := registry.InputBindingForSequence("chords", []string{"p", "9"}, ctx); resolution != ChordNoMatch {
		t.Errorf("unbound chord p9 resolution = %v, want no match", resolution)
	}
	if _, _, ok := registry.InputBindingForKey("chords", "p", ctx); ok {
		t.Error("a chord prefix resolved as a direct key")
	}
	if _, _, ok := registry.InputBindingForKey("chords", "a", ctx); !ok {
		t.Error("direct key a did not resolve as a length-1 sequence")
	}
	// The subtree is unreachable outside the Outliner, so its prefix must fail
	// closed instead of trapping input in a chord that can never complete.
	if _, _, resolution := registry.InputBindingForSequence(
		"chords",
		[]string{"p"},
		PresentationCommandContext(ViewContextHub),
	); resolution != ChordNoMatch {
		t.Error("an inapplicable chord subtree still accepted its prefix")
	}

	handled := registry.HandledKeysForInputStrategy("chords")
	for _, key := range []string{"a", "p", "1", "2", ",", "h"} {
		if !slices.Contains(handled, key) {
			t.Errorf("handled keys %v omit chord key %q", handled, key)
		}
	}

	if got := registry.InputShortcutForOperation("chords", chordTestSetPriority); got != "p1-2" {
		t.Errorf("shortcut = %q, want the shared preset spelling %q", got, "p1-2")
	}
	help := registry.HelpItemsForInputStrategy("chords", ctx)
	if len(help) != 3 {
		t.Fatalf("help items = %d, want one per operation (3)", len(help))
	}
	if help[1].Key != "p1-2" {
		t.Errorf("preset family help key = %q, want %q", help[1].Key, "p1-2")
	}
	if help[2].Key != ",h" {
		t.Errorf("chord help key = %q, want the composed sequence %q", help[2].Key, ",h")
	}

	hints := registry.ChordHintsForPrefix("chords", []string{"p"}, ctx)
	want := []ChordHint{{Key: "1", Label: "critical"}, {Key: "2", Label: "high"}}
	if !slices.Equal(hints, want) {
		t.Errorf("which-key hints = %v, want %v", hints, want)
	}
	row := registry.ChordHintRowForPrefix("chords", []string{"p"}, ctx)
	for _, fragment := range []string{"p…", "1 critical", "2 high"} {
		if !strings.Contains(row, fragment) {
			t.Errorf("which-key row %q omits %q", row, fragment)
		}
	}
	if registry.ChordHintRowForPrefix("chords", []string{"p", "1"}, ctx) != "" {
		t.Error("a terminal chord still produced a which-key row")
	}

	footer := registry.FooterHintsForInputStrategy("chords", ctx, 0)
	for _, fragment := range []string{"a capture", "p priority"} {
		if !strings.Contains(footer, fragment) {
			t.Errorf("footer %q omits %q", footer, fragment)
		}
	}
}

func TestChordStrategyRegistrationRejectsAmbiguousAndUnlabeledGrammars(t *testing.T) {
	priorityChord := func(sequence []string, preset any) OperationKeyBinding {
		return OperationKeyBinding{
			OperationID:    chordTestSetPriority,
			Sequence:       sequence,
			PresetValue:    preset,
			HasPresetValue: true,
		}
	}
	priorityGroup := OperationChordGroup{Sequence: []string{"p"}, HintLabel: "priority"}

	tests := []struct {
		name     string
		strategy InputStrategy
		want     string
	}{
		{
			name: "direct key shadows a chord prefix",
			strategy: InputStrategy{
				Bindings: []OperationKeyBinding{
					{OperationID: operationCoreCapture, Keys: []string{"p"}},
					priorityChord([]string{"p", "1"}, 1),
				},
				ChordGroups: []OperationChordGroup{priorityGroup},
			},
			want: "prefix collision",
		},
		{
			name: "chord extends a direct key",
			strategy: InputStrategy{
				Bindings: []OperationKeyBinding{
					priorityChord([]string{"p", "1"}, 1),
					{OperationID: operationCoreCapture, Keys: []string{"p"}},
				},
				ChordGroups: []OperationChordGroup{priorityGroup},
			},
			want: "prefix collision",
		},
		{
			name: "duplicate chord path",
			strategy: InputStrategy{
				Bindings: []OperationKeyBinding{
					priorityChord([]string{"p", "1"}, 1),
					priorityChord([]string{"p", "1"}, 2),
				},
				ChordGroups: []OperationChordGroup{priorityGroup},
			},
			want: "key collision",
		},
		{
			name: "one-key sequence",
			strategy: InputStrategy{
				Bindings: []OperationKeyBinding{priorityChord([]string{"p"}, 1)},
			},
			want: "one-key chord sequence",
		},
		{
			name: "direct keys and a sequence together",
			strategy: InputStrategy{
				Bindings: []OperationKeyBinding{{
					OperationID: operationCoreCapture,
					Keys:        []string{"a"},
					Sequence:    []string{",", "a"},
				}},
			},
			want: "declares both direct keys and a chord sequence",
		},
		{
			name: "repeated operation without presets",
			strategy: InputStrategy{
				Bindings: []OperationKeyBinding{
					{OperationID: operationCoreCapture, Keys: []string{"a"}},
					{OperationID: operationCoreCapture, Sequence: []string{",", "a"}},
				},
				ChordGroups: []OperationChordGroup{{Sequence: []string{","}, HintLabel: "state"}},
			},
			want: "more than once",
		},
		{
			name: "preset on an operation that takes no value",
			strategy: InputStrategy{
				Bindings: []OperationKeyBinding{{
					OperationID:    operationCoreCapture,
					Sequence:       []string{",", "a"},
					PresetValue:    1,
					HasPresetValue: true,
				}},
				ChordGroups: []OperationChordGroup{{Sequence: []string{","}, HintLabel: "state"}},
			},
			want: "presets a value but takes none",
		},
		{
			name: "preset type the provider cannot accept",
			strategy: InputStrategy{
				Bindings:    []OperationKeyBinding{priorityChord([]string{"p", "1"}, "high")},
				ChordGroups: []OperationChordGroup{priorityGroup},
			},
			want: "does not accept",
		},
		{
			name: "chord prefix without a group label",
			strategy: InputStrategy{
				Bindings: []OperationKeyBinding{priorityChord([]string{"p", "1"}, 1)},
			},
			want: "has no group label",
		},
		{
			name: "group label for a prefix that does not exist",
			strategy: InputStrategy{
				Bindings: []OperationKeyBinding{{OperationID: operationCoreCapture, Keys: []string{"a"}}},
				ChordGroups: []OperationChordGroup{
					{Sequence: []string{"f"}, HintLabel: "filter"},
				},
			},
			want: "does not label a chord prefix",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			registry := chordTestRegistry(t)
			strategy := test.strategy
			strategy.ID = "candidate"
			err := registry.RegisterInputStrategy(strategy)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want one containing %q", err, test.want)
			}
		})
	}
}

func TestPendingChordStoresHintsExecutesAndClears(t *testing.T) {
	newChordApp := func(t *testing.T) *AppModel {
		t.Helper()
		app := newUITestApp()
		app.operationRegistry = NewOperationRegistry()
		app.operationExecutor = NewOperationExecutor(app.operationRegistry)
		if err := app.registerCoreInputOperations(); err != nil {
			t.Fatalf("registerCoreInputOperations: %v", err)
		}
		if err := app.operationRegistry.RegisterInputStrategy(InputStrategy{
			ID: InputStrategySimplified,
			Bindings: []OperationKeyBinding{
				{OperationID: operationCoreCapture, Keys: []string{"a"}},
				{OperationID: operationCoreHelp, Sequence: []string{",", "h"}, HintLabel: "help"},
			},
			ChordGroups: []OperationChordGroup{{Sequence: []string{","}, HintLabel: "state"}},
		}); err != nil {
			t.Fatalf("RegisterInputStrategy: %v", err)
		}
		app.wireOutlinerOperationExecutor(app.crossOutliner)
		return app
	}
	comma := tea.KeyPressMsg{Code: ',', Text: ","}

	t.Run("prefix waits and shows its which-key row", func(t *testing.T) {
		app := newChordApp(t)
		handled, cmd := app.handleSimplifiedInputKey(comma)
		if !handled || cmd != nil {
			t.Fatalf("chord prefix = handled %v, cmd %v; want consumed with no command", handled, cmd)
		}
		if !slices.Equal(app.pendingChord, []string{","}) {
			t.Fatalf("pending chord = %v, want [,]", app.pendingChord)
		}
		row := pendingChordHintRowForTest(app)
		if !strings.Contains(row, "h help") {
			t.Errorf("which-key row %q omits the chord's hint", row)
		}
		if !strings.Contains(app.renderStatusBar(), "h help") {
			t.Error("the status bar footer does not show the pending chord")
		}
	})

	t.Run("terminal key executes and clears", func(t *testing.T) {
		app := newChordApp(t)
		app.handleSimplifiedInputKey(comma)
		handled, cmd := app.handleSimplifiedInputKey(tea.KeyPressMsg{Code: 'h', Text: "h"})
		if !handled || cmd == nil {
			t.Fatalf("chord completion = handled %v, cmd %v; want the operation's command", handled, cmd)
		}
		if len(app.pendingChord) != 0 {
			t.Errorf("pending chord = %v, want cleared", app.pendingChord)
		}
	})

	t.Run("esc unknown key and reset clear the prefix", func(t *testing.T) {
		for name, abandon := range map[string]func(app *AppModel){
			"esc":     func(app *AppModel) { app.handleSimplifiedInputKey(tea.KeyPressMsg{Code: tea.KeyEscape}) },
			"unknown": func(app *AppModel) { app.handleSimplifiedInputKey(tea.KeyPressMsg{Code: 'z', Text: "z"}) },
			"reset":   func(app *AppModel) { jumpToOutlinerRootForTest(app) },
			"inline control": func(app *AppModel) {
				app.crossOutliner.ActivateSearch()
				app.handleSimplifiedInputKey(tea.KeyPressMsg{Code: 'h', Text: "h"})
			},
		} {
			app := newChordApp(t)
			app.handleSimplifiedInputKey(comma)
			abandon(app)
			if len(app.pendingChord) != 0 {
				t.Errorf("%s left pending chord %v, want cleared", name, app.pendingChord)
			}
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
	if err := app.registerTaskActionOperations(); err != nil {
		t.Fatalf("registerTaskActionOperations: %v", err)
	}
	if err := app.registerFilterOperations(); err != nil {
		t.Fatalf("registerFilterOperations: %v", err)
	}
	if err := app.operationRegistry.RegisterInputStrategy(SimplifiedInputStrategy()); err != nil {
		t.Fatalf("RegisterInputStrategy: %v", err)
	}
	app.wireOutlinerOperationExecutor(app.crossOutliner)

	for _, retired := range []string{
		"F", "q", "w", "i", "x", "|", "ctrl+n", "ctrl+x",
		":", "R", "p", "ctrl+esc", "m", "b", "d", "n", "t", ",",
	} {
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
	for _, retired := range []string{
		"F", "q", "w", "i", "x", "|", "Ctrl+N", "Ctrl+X",
		":", "R", "p", "Ctrl+Esc", "m",
	} {
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
	if !handled || cmd != nil {
		t.Fatalf("removed Finder key = handled %v cmd %v; want consumed no-op", handled, cmd)
	}

	binding, _, ok := app.operationRegistry.InputBindingForKey(
		InputStrategySimplified,
		"g",
		app.operationCommandContext(),
	)
	if !ok || binding.OperationID != operationCoreCursorTop {
		t.Fatalf("g resolved to %s (found %v), want %s", binding.OperationID, ok, operationCoreCursorTop)
	}

	handled, cmd = app.handleGlobalKeyPress(tea.KeyPressMsg{Code: 'F', Text: "F"})
	if !handled || cmd != nil {
		t.Fatalf("retired legacy key F = handled %v cmd %v; want consumed no-op", handled, cmd)
	}
}

func TestRemovedFinderKeyDoesNotEscapeChildRoutes(t *testing.T) {
	runtime := newTestConfig()
	app, err := NewAppModel(runtime, nil, WithPersister(NewMockPersister()))
	if err != nil {
		t.Fatalf("NewAppModel: %v", err)
	}
	app.pushView(ViewContextHub)

	handled, cmd := app.handleGlobalKeyPress(tea.KeyPressMsg{Code: ':', Text: ":"})
	if !handled || cmd != nil {
		t.Fatalf("child-route Finder key = handled %v cmd %v; want consumed no-op", handled, cmd)
	}
}
