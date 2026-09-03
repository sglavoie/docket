package tui

import (
	"slices"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/tui/keymap"
	"github.com/sglavoie/tt/internal/tui/operation"
	"github.com/sglavoie/tt/internal/tui/presentation"
	"github.com/sglavoie/tt/internal/tui/status"
	"github.com/sglavoie/tt/internal/tui/theme"
)

// mutableInputStrategy detaches the binding slice the one-path checks below
// rewrite. A strategy factory already returns a fresh value, so the package's
// own defensive clone is not what this needs: only Bindings is mutated here,
// and appending to a shared backing array is the single way these subtests
// could reach each other.
func mutableInputStrategy(strategy operation.InputStrategy) operation.InputStrategy {
	strategy.Bindings = append([]operation.OperationKeyBinding(nil), strategy.Bindings...)
	return strategy
}

func TestTaskOperationsCannotAppearInBothDirectGrammarAndMenu(t *testing.T) {
	registry := registeredSimplifiedOperationRegistry(t)
	strategy := SimplifiedInputStrategy()
	if err := validateOutlinerOperationOnePath(strategy); err != nil {
		t.Fatalf("production grammar: %v", err)
	}

	bound := make(map[operation.OperationID]bool)
	for _, binding := range strategy.Bindings {
		bound[binding.OperationID] = true
	}
	menu := make(map[operation.OperationID]bool)
	for _, id := range taskOperationManifestIDs() {
		menu[id] = true
	}
	for _, entry := range taskOperationDescriptorManifest() {
		if _, ok := registry.Descriptor(entry.ID); !ok {
			t.Errorf("manifested task operation %s is not registered", entry.ID)
		}
		if bound[entry.ID] == menu[entry.ID] {
			t.Errorf(
				"registered task operation %s has %d invocation paths; want exactly one",
				entry.ID,
				invocationPathCount(bound[entry.ID])+invocationPathCount(menu[entry.ID]),
			)
		}
	}

	t.Run("duplicate placement", func(t *testing.T) {
		candidate := mutableInputStrategy(strategy)
		candidate.Bindings = append(candidate.Bindings, operation.OperationKeyBinding{
			OperationID: operationTaskDelete,
			Keys:        []string{"Z"},
		})
		err := validateOutlinerOperationOnePath(candidate)
		if err == nil || !strings.Contains(err.Error(), "bound directly and listed") {
			t.Fatalf("error = %v, want duplicate-placement rejection", err)
		}
	})

	t.Run("unreachable direct-only operation", func(t *testing.T) {
		candidate := mutableInputStrategy(strategy)
		candidate.Bindings = slices.DeleteFunc(candidate.Bindings, func(binding operation.OperationKeyBinding) bool {
			return binding.OperationID == operationTaskSetDueDate
		})
		err := validateOutlinerOperationOnePath(candidate)
		if err == nil || !strings.Contains(err.Error(), "direct-only but has no direct binding") {
			t.Fatalf("error = %v, want unreachable-direct rejection", err)
		}
	})
}

func invocationPathCount(value bool) int {
	if value {
		return 1
	}
	return 0
}

// TestEverySurfaceManifestSatisfiesTheOnePathRule runs the rule the Outliner has
// always been held to over all five entity surfaces at once, so a surface cannot
// pass startup validation because nothing thought to check it. The table is
// compared against surfaceOperationGates so a sixth surface fails here rather
// than joining the app unexamined.
func TestEverySurfaceManifestSatisfiesTheOnePathRule(t *testing.T) {
	surfaces := []struct {
		name     string
		strategy operation.InputStrategy
		manifest []operation.OperationManifestEntry
	}{
		{"outliner", SimplifiedInputStrategy(), outlinerOperationDescriptorManifest()},
		{"project hub", ProjectHubInputStrategy(), projectOperationManifest()},
		{"notes", NotesInputStrategy(), notesOperationManifest()},
		{"attachments", AttachmentsInputStrategy(), attachmentOperationManifest()},
		{"comments", CommentsInputStrategy(), commentOperationManifest()},
	}

	gated := make([]string, 0, len(surfaceOperationGates()))
	for _, gate := range surfaceOperationGates() {
		gated = append(gated, string(gate.strategy.ID))
		if err := gate.validate(gate.strategy); err != nil {
			t.Errorf("production grammar for %s: %v", gate.strategy.ID, err)
		}
	}
	tabled := make([]string, 0, len(surfaces))
	for _, surface := range surfaces {
		tabled = append(tabled, string(surface.strategy.ID))
	}
	slices.Sort(gated)
	slices.Sort(tabled)
	if !slices.Equal(gated, tabled) {
		t.Fatalf("gated surfaces = %v, tabled = %v", gated, tabled)
	}

	for _, surface := range surfaces {
		t.Run(surface.name, func(t *testing.T) {
			var firstMenu, firstDirect operation.OperationID
			for _, entry := range surface.manifest {
				if entry.Risk == operation.OperationRiskDestructive && entry.DirectOnly {
					t.Errorf("destructive operation %s is direct; it belongs behind the menu", entry.ID)
				}
				switch {
				case entry.DirectOnly && firstDirect == "":
					firstDirect = entry.ID
				case !entry.DirectOnly && firstMenu == "":
					firstMenu = entry.ID
				}
			}
			if firstMenu == "" || firstDirect == "" {
				t.Fatalf("manifest has no menu entry (%q) or no direct entry (%q)", firstMenu, firstDirect)
			}

			duplicate := mutableInputStrategy(surface.strategy)
			duplicate.Bindings = append(duplicate.Bindings, operation.OperationKeyBinding{
				OperationID: firstMenu,
				Keys:        []string{"Z"},
			})
			err := operation.ValidateOperationOnePath(surface.manifest, duplicate)
			if err == nil || !strings.Contains(err.Error(), "bound directly and listed") {
				t.Errorf("duplicate placement error = %v, want rejection", err)
			} else if !strings.Contains(err.Error(), string(surface.strategy.ID)) {
				t.Errorf("error %q does not name the surface it came from", err)
			}

			unreachable := mutableInputStrategy(surface.strategy)
			unreachable.Bindings = slices.DeleteFunc(unreachable.Bindings, func(binding operation.OperationKeyBinding) bool {
				return binding.OperationID == firstDirect
			})
			err = operation.ValidateOperationOnePath(surface.manifest, unreachable)
			if err == nil || !strings.Contains(err.Error(), "direct-only but has no direct binding") {
				t.Errorf("unreachable direct-only error = %v, want rejection", err)
			}
		})
	}
}

// TestPanelFootersOnlyAdvertiseKeysTheStrategyBinds covers the three surfaces
// whose footers come from the gated keymap rather than from the registry. Their
// keymaps are the only footer source, so nothing else notices when one promises
// a key the strategy stopped binding. Modal keys are the exception and are named
// here rather than waved through, because a composer's save and newline keys
// exist only while its textarea has focus.
func TestPanelFootersOnlyAdvertiseKeysTheStrategyBinds(t *testing.T) {
	for _, surface := range []struct {
		name     string
		strategy operation.InputStrategy
		keys     *keymap.KeyMap
		modal    []string
	}{
		{"notes", NotesInputStrategy(), (&NotesViewModel{}).buildKeyMap(), nil},
		{"attachments", AttachmentsInputStrategy(), (&TaskAttachmentsModel{}).buildKeyMap(), nil},
		{
			name:     "comments",
			strategy: CommentsInputStrategy(),
			keys:     (&TaskCommentsModel{}).buildKeyMap(),
			modal:    []string{"ctrl+s", "ctrl+e", "shift+enter"},
		},
	} {
		t.Run(surface.name, func(t *testing.T) {
			for _, binding := range surface.strategy.Bindings {
				if binding.FooterRank != 0 || binding.FooterLabel != "" {
					t.Errorf(
						"%s carries footer metadata nothing reads; the keymap is the footer source",
						binding.OperationID,
					)
				}
			}

			bound := make(map[string]bool)
			for _, binding := range surface.strategy.Bindings {
				for _, key := range binding.Keys {
					bound[key] = true
				}
			}
			advertised := 0
			for _, entry := range surface.keys.Entries {
				if entry.Binding == nil || entry.Binding.FooterRank <= 0 {
					continue
				}
				advertised++
				key := entry.Binding.Key
				if !bound[key] && !slices.Contains(surface.modal, key) {
					t.Errorf("footer advertises %q, which the strategy does not bind", key)
				}
			}
			if advertised == 0 {
				t.Fatal("keymap advertises no footer hints")
			}
		})
	}
}

func TestDirectThemeAndQuitOperationsUseTheirDescriptors(t *testing.T) {
	driver, workspace, err := NewFixtureDriver(FixtureFresh, 100, 30)
	if err != nil {
		t.Fatalf("NewFixtureDriver: %v", err)
	}
	t.Cleanup(func() { _ = workspace.Cleanup() })
	app := driver.App()
	beforeTheme := app.runtime.Machine().Theme
	t.Cleanup(func() {
		theme.ApplyTheme(theme.NewThemeConfig(app.runtime.Machine()))
	})

	themeResult := executeDirectPath(
		t,
		app,
		app.operationCommandContext(),
		[]string{"T"},
	)
	if themeResult.Err != nil {
		t.Fatalf("toggle theme: %v", themeResult.Err)
	}
	timeout, ok := themeResult.Message.(OperationCommandResultMsg)
	if !ok || timeout.Cmd == nil {
		t.Fatalf("toggle theme message = %T, want a status timeout command", themeResult.Message)
	}
	if app.statusMessage != "Theme: "+app.runtime.Machine().Theme {
		t.Fatalf("theme status = %q, want the active theme", app.statusMessage)
	}
	if handled, _ := app.handleViewMessages(status.StatusClearMsg{ScheduledAt: app.statusUpdatedAt.Add(time.Nanosecond)}); !handled {
		t.Fatal("theme status clear message was not handled")
	}
	if app.statusMessage != "" {
		t.Fatalf("theme status = %q after timeout, want empty", app.statusMessage)
	}
	afterTheme := app.runtime.Machine().Theme
	if afterTheme == beforeTheme || (afterTheme != "dark" && afterTheme != "light") {
		t.Fatalf("theme changed from %q to %q, want the opposite maintained variant", beforeTheme, afterTheme)
	}
	reloaded, err := config.LoadRuntimeState(workspace.DataDir)
	if err != nil {
		t.Fatalf("reload machine theme: %v", err)
	}
	if reloaded.Machine().Theme != afterTheme {
		t.Fatalf("persisted machine theme = %q, want %q", reloaded.Machine().Theme, afterTheme)
	}
	app.pushView(presentation.ViewProjectHub)
	handled, cmd := app.handleGlobalKeyPress(tea.KeyPressMsg{
		Code: 'c',
		Mod:  tea.ModCtrl,
	})
	if !handled || cmd == nil {
		t.Fatalf("Ctrl+C over a subordinate route = handled %v cmd %v, want descriptor dispatch", handled, cmd)
	}
	if _, ok := cmd().(status.QuitMsg); !ok {
		t.Fatalf("Ctrl+C command emitted %T, want QuitMsg", cmd())
	}
}
