package tui

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"sort"
	"strings"
	"testing"
)

func TestTUIScenarioBaseline(t *testing.T) {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller could not locate operation ledger test")
	}
	scenarioDir := filepath.Join(filepath.Dir(filename), "..", "..", "docs", "tui-testing", "scenarios")
	entries, err := os.ReadDir(scenarioDir)
	if err != nil {
		t.Fatalf("read committed TUI scenarios: %v", err)
	}
	count := 0
	for _, entry := range entries {
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ".json" {
			count++
		}
	}
	if count != TUIScenarioBaseline {
		t.Fatalf("committed TUI scenarios = %d, want baseline %d", count, TUIScenarioBaseline)
	}
}

func TestRegisteredOperationInventoryHasStableClassification(t *testing.T) {
	app, _ := newTestAppModelWithProject()
	registry := NewCommandRegistry()
	for groupIndex, group := range app.commandGroups() {
		if err := registry.RegisterBatch(group); err != nil {
			t.Fatalf("RegisterBatch[%d]: %v", groupIndex, err)
		}
	}

	inventory := registry.Inventory()
	if len(inventory) != 224 {
		t.Fatalf("registered command variants = %d, want audited baseline 224", len(inventory))
	}
	validViews := make(map[PresentationID]bool, len(allPresentations()))
	for _, view := range allPresentations() {
		validViews[view] = true
	}
	validReachability := map[CommandReachability]bool{
		CommandReachGlobalBinding: true,
		CommandReachViewMenu:      true,
		CommandReachHub:           true,
		CommandReachContextAction: true,
		CommandReachPaletteAlias:  true,
	}
	operationNames := make(map[OperationID]string)
	operationClassifications := make(map[OperationID]CommandInventoryEntry)
	observedOverrides := make(map[OperationID]bool)
	operationsByReachability := make(map[CommandReachability]map[OperationID]bool)
	for _, entry := range inventory {
		if entry.OperationID == "" {
			t.Errorf("command %q has no operation ID", entry.Name)
		}
		if want := operationIDForName(entry.Name); entry.OperationID != want {
			t.Errorf("command %q operation ID = %q, want %q", entry.Name, entry.OperationID, want)
		}
		if previous, ok := operationNames[entry.OperationID]; ok && previous != entry.Name {
			t.Errorf("operation ID collision %q: %q and %q", entry.OperationID, previous, entry.Name)
		}
		operationNames[entry.OperationID] = entry.Name
		operationClassifications[entry.OperationID] = entry
		if !validReachability[entry.Reachability] {
			t.Errorf("command %q has invalid reachability %q", entry.Name, entry.Reachability)
		}
		if entry.FinalTarget == "" {
			t.Errorf("command %q has no final reachability target", entry.Name)
		}
		if operationsByReachability[entry.Reachability] == nil {
			operationsByReachability[entry.Reachability] = make(map[OperationID]bool)
		}
		operationsByReachability[entry.Reachability][entry.OperationID] = true
		if _, ok := commandClassificationTable[entry.OperationID]; ok {
			observedOverrides[entry.OperationID] = true
		}
		seenViews := make(map[PresentationID]bool, len(entry.Views))
		for _, view := range entry.Views {
			if !validViews[view] {
				t.Errorf("command %q references undeclared PresentationID %d", entry.Name, view)
			}
			if seenViews[view] {
				t.Errorf("command %q repeats view scope %s", entry.Name, view)
			}
			seenViews[view] = true
		}
	}
	for operationID := range commandClassificationTable {
		if !observedOverrides[operationID] {
			t.Errorf("command classification table contains unregistered operation %q", operationID)
		}
	}
	if len(operationNames) != 221 {
		t.Fatalf("stable command operations = %d, want audited baseline 221", len(operationNames))
	}
	for name, variants := range registry.commands {
		for _, command := range variants {
			if command.Handler == nil {
				t.Errorf("registered command %q has no handler and is not currently executable", name)
			}
		}
	}

	counts := make(map[CommandReachability]int, len(operationsByReachability))
	for reachability, operations := range operationsByReachability {
		counts[reachability] = len(operations)
	}
	operationSignatures := make([]string, 0, len(operationNames))
	conditionallyUnavailable := make([]string, 0)
	for operationID, name := range operationNames {
		entry := operationClassifications[operationID]
		operationSignatures = append(operationSignatures,
			fmt.Sprintf("%s|%s|%s|%s", operationID, name, entry.Reachability, entry.FinalTarget))
		if _, ok := registry.GetByName(name); !ok {
			conditionallyUnavailable = append(conditionallyUnavailable, name)
		}
	}
	sort.Strings(operationSignatures)
	sort.Strings(conditionallyUnavailable)
	fingerprint := sha256.Sum256([]byte(strings.Join(operationSignatures, "\n")))
	const wantFingerprint = "ee6b42b46713b44d14b86fe2e812d3764cecddc9f59172d081919afe756104d8"
	if got := fmt.Sprintf("%x", fingerprint); got != wantFingerprint {
		t.Fatalf("command ledger fingerprint = %s, want audited baseline %s", got, wantFingerprint)
	}
	wantConditionallyUnavailable := []string{"enable sidebar for all projects"}
	if !slices.Equal(conditionallyUnavailable, wantConditionallyUnavailable) {
		t.Fatalf("state-dependent unavailable operations = %v, want documented baseline %v",
			conditionallyUnavailable, wantConditionallyUnavailable)
	}
	t.Logf("command ledger fingerprint: %x", fingerprint)
	t.Logf("command ledger: %d variants, %d stable operations, classifications=%v", len(inventory), len(operationNames), counts)
	t.Logf("state-dependent operations unavailable in the baseline fixture: %v", conditionallyUnavailable)
}

func TestCommandContextFiltersCapabilitiesSelectionAndEntityState(t *testing.T) {
	registry := NewCommandRegistry()
	mutableSelection := Command{
		Name:                 "edit selected task through context",
		Views:                []PresentationID{ViewOutliner},
		RequiredCapabilities: NewCapabilitySet(CapabilityProjectScope),
		ContextAvailable: func(ctx CommandContext) bool {
			return ctx.Selection.Count() == 1 && ctx.Entity.Mutable && ctx.Entity.State == "ready"
		},
	}
	if err := registry.Register(mutableSelection); err != nil {
		t.Fatalf("Register: %v", err)
	}

	projectContext := PresentationCommandContext(ViewOutliner)
	if got := registry.GetForContext(projectContext); len(got) != 0 {
		t.Fatalf("context-free query returned selection-dependent command: %v", commandProjection(got))
	}
	projectContext.Selection = SelectionContext{ProjectID: "p1", TaskIDs: []string{"t1"}}
	projectContext.Entity = EntityContext{ProjectID: "p1", TaskID: "t1", State: "ready", Mutable: true}
	got := registry.GetForContext(projectContext)
	if len(got) != 1 || got[0].Name != mutableSelection.Name {
		t.Fatalf("selection/entity context commands = %v, want %q", commandProjection(got), mutableSelection.Name)
	}

	appContext := PresentationCommandContext(ViewDashboard)
	appContext.Selection = projectContext.Selection
	appContext.Entity = projectContext.Entity
	if got := registry.GetForContext(appContext); len(got) != 0 {
		t.Fatalf("app-scope context leaked project command: %v", commandProjection(got))
	}
}

func commandProjection(commands []Command) []string {
	projection := make([]string, 0, len(commands))
	for _, command := range commands {
		projection = append(projection, fmt.Sprintf("%s|%s|%s", command.OperationID, command.Name, command.Shortcut))
	}
	return projection
}
