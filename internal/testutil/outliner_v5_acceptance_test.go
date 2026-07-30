package testutil

import (
	"slices"
	"strings"
	"testing"
)

func TestOutlinerV5TargetContractInventories(t *testing.T) {
	contract := OutlinerV5TargetContract()
	if contract.SchemaVersion != 5 {
		t.Fatalf("schema version = %d, want 5", contract.SchemaVersion)
	}
	if contract.PrimaryPresentation != "outliner" ||
		contract.InitialMode != "all" ||
		contract.InitialScope != "all-projects" {
		t.Fatalf(
			"initial presentation = %s / %s / %s, want outliner / all / all-projects",
			contract.PrimaryPresentation,
			contract.InitialMode,
			contract.InitialScope,
		)
	}

	wantModes := []string{"all", "inbox", "next", "backlog", "due"}
	if !slices.Equal(contract.Modes, wantModes) {
		t.Errorf("modes = %v, want %v", contract.Modes, wantModes)
	}
	wantStates := []string{
		"inbox",
		"ready",
		"next",
		"in_progress",
		"waiting",
		"paused",
		"someday",
		"done",
		"canceled",
	}
	if !slices.Equal(contract.States, wantStates) {
		t.Errorf("states = %v, want %v", contract.States, wantStates)
	}
	wantMenu := []string{
		"Recurrence",
		"Move Project",
		"Set Parent",
		"Dependencies",
		"Checklist",
		"Comments",
		"Decisions",
		"Attachments",
		"Delete",
	}
	if !slices.Equal(contract.TaskMenuActions, wantMenu) {
		t.Errorf("task menu = %v, want %v", contract.TaskMenuActions, wantMenu)
	}

	domains := make([]string, len(contract.PersistenceDomains))
	for i, domain := range contract.PersistenceDomains {
		domains[i] = domain.Name
		if domain.Location == "" || domain.Contents == "" {
			t.Errorf("persistence domain is incomplete: %+v", domain)
		}
	}
	if !slices.Equal(domains, []string{"workspace", "planning", "machine"}) {
		t.Errorf("persistence domains = %v, want workspace planning machine", domains)
	}

	wantCLI := []string{
		"tt",
		"tt doctor",
		"tt conflict",
		"tt workspace",
		"tt completion",
		"tt project export",
		"tt help",
		"tt --version",
	}
	if !slices.Equal(contract.CLICommands, wantCLI) {
		t.Errorf("CLI commands = %v, want %v", contract.CLICommands, wantCLI)
	}
	if !slices.Equal(contract.GlobalFlags, []string{"--data-dir", "--no-color"}) {
		t.Errorf("global flags = %v, want --data-dir and --no-color", contract.GlobalFlags)
	}
}

func TestOutlinerV5TargetGrammarIsCanonicalAndSingleOwner(t *testing.T) {
	contract := OutlinerV5TargetContract()
	seenKeys := make(map[string]string)
	directActions := make(map[string]bool)
	for _, binding := range append(contract.DirectGrammar, contract.FilterGrammar...) {
		if binding.Key == "" || binding.Action == "" || binding.Area == "" {
			t.Errorf("incomplete grammar binding: %+v", binding)
			continue
		}
		if previous, ok := seenKeys[binding.Key]; ok {
			t.Errorf("key %q is owned by both %q and %q", binding.Key, previous, binding.Action)
		}
		seenKeys[binding.Key] = binding.Action
		directActions[binding.Action] = true
	}

	for _, removed := range []string{"m", ":", "R", "p", "Ctrl+Esc"} {
		if action, ok := seenKeys[removed]; ok {
			t.Errorf("removed key %q still owns %q", removed, action)
		}
	}
	for _, required := range []string{
		"Tab",
		"Shift+Tab",
		"a",
		"A",
		"e",
		"E",
		"S",
		",i",
		"!0",
		"!1-!5",
		"Space",
		"v",
		"V",
		"Ctrl+Y",
		"Ctrl+R",
	} {
		if _, ok := seenKeys[required]; !ok {
			t.Errorf("direct grammar is missing %q", required)
		}
	}

	seenMenu := make(map[string]bool)
	for _, action := range contract.TaskMenuActions {
		if action == "" || seenMenu[action] {
			t.Errorf("blank or duplicate task-menu action %q", action)
		}
		seenMenu[action] = true
		if directActions[strings.ToLower(strings.ReplaceAll(action, " ", "-"))] {
			t.Errorf("task-menu action %q duplicates a direct action", action)
		}
	}
}

func TestOutlinerV5CaptureGrammarContainsCanonicalSigilsOnly(t *testing.T) {
	grammar := OutlinerV5TargetContract().CaptureGrammar
	forms := make([]string, len(grammar))
	fields := make([]string, len(grammar))
	for i, sigil := range grammar {
		forms[i] = sigil.Form
		fields[i] = sigil.Field
		if sigil.ValuePolicy == "" {
			t.Errorf("capture sigil is missing its value policy: %+v", sigil)
		}
		if strings.Contains(sigil.Form, ":") || strings.HasPrefix(sigil.Form, "^") {
			t.Errorf("capture grammar retained an alias form %q", sigil.Form)
		}
	}
	if !slices.Equal(forms, []string{"!1-5", "#tag", "@date", "~interval", "$state", "%project", ">parent"}) {
		t.Errorf("capture forms = %v", forms)
	}
	if !slices.Equal(fields, []string{"priority", "tag", "due", "recurrence", "state", "project", "parent"}) {
		t.Errorf("capture fields = %v", fields)
	}
}

func TestOutlinerV5CurrentBaselineRecordsPreMigrationSurface(t *testing.T) {
	baseline := OutlinerV5CurrentBaseline()
	if baseline.RecordedAtCommit != "549400c3" {
		t.Fatalf("baseline commit = %q, want 549400c3", baseline.RecordedAtCommit)
	}
	if !slices.Contains(baseline.States, "review") {
		t.Error("baseline must record the pre-v5 Review state")
	}
	for _, mode := range []string{"priority", "blocked", "parked", "daily-review", "activity"} {
		if !slices.Contains(baseline.Modes, mode) {
			t.Errorf("baseline modes are missing pre-v5 mode %q", mode)
		}
	}
	if !slices.Equal(baseline.PersistenceDomains, []string{"workspace", "planning", "settings", "automation", "machine"}) {
		t.Errorf("baseline persistence domains = %v", baseline.PersistenceDomains)
	}
	for name, entries := range map[string][]string{
		"clean install": baseline.CleanInstall,
		"daily loop":    baseline.DailyLoop,
		"navigation":    baseline.Navigation,
		"storage":       baseline.Storage,
		"CLI":           baseline.CLICommands,
	} {
		requireUniqueNonEmptyStrings(t, name, entries)
	}
}

func TestOutlinerV5AcceptanceFunctionsReturnFreshData(t *testing.T) {
	firstContract := OutlinerV5TargetContract()
	firstContract.Modes[0] = "mutated"
	firstContract.DirectGrammar[0].Key = "mutated"
	firstContract.PersistenceDomains[0].Contents = "mutated"
	secondContract := OutlinerV5TargetContract()
	if secondContract.Modes[0] != "all" ||
		secondContract.DirectGrammar[0].Key != "Tab" ||
		secondContract.PersistenceDomains[0].Contents == "mutated" {
		t.Fatal("target contract returned aliased data")
	}

	firstBaseline := OutlinerV5CurrentBaseline()
	firstBaseline.CLICommands[0] = "mutated"
	if OutlinerV5CurrentBaseline().CLICommands[0] != "add" {
		t.Fatal("current baseline returned aliased data")
	}
}

func requireUniqueNonEmptyStrings(t *testing.T, name string, values []string) {
	t.Helper()
	if len(values) == 0 {
		t.Errorf("%s inventory is empty", name)
		return
	}
	seen := make(map[string]bool, len(values))
	for _, value := range values {
		if strings.TrimSpace(value) == "" {
			t.Errorf("%s inventory contains a blank value", name)
		}
		if seen[value] {
			t.Errorf("%s inventory contains duplicate %q", name, value)
		}
		seen[value] = true
	}
}
