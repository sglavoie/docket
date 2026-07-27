package config

import (
	"bytes"
	"os"
	"testing"

	"github.com/sglavoie/tt/internal/workflow"
)

// TestRuntimeState_LoadCapturesDomains confirms that a startup-loaded snapshot
// exposes every domain through its narrow accessor and is rooted at the workspace
// data dir.
func TestRuntimeState_LoadCapturesDomains(t *testing.T) {
	manager, _ := seededManager(t)

	rs, err := manager.LoadRuntimeState()
	if err != nil {
		t.Fatalf("LoadRuntimeState: %v", err)
	}
	if rs.Catalog() == nil || rs.Planning() == nil || rs.Settings() == nil ||
		rs.Automation() == nil || rs.Machine() == nil {
		t.Fatalf("every domain accessor must return a non-nil snapshot")
	}
	if rs.DataDir() == "" {
		t.Fatalf("DataDir must be populated from the workspace root")
	}
}

func TestRuntimeState_ReloadSanitizesRetiredSetActiveActions(t *testing.T) {
	manager, _ := seededManager(t)
	seed, err := manager.LoadRuntimeState()
	if err != nil {
		t.Fatalf("LoadRuntimeState seed: %v", err)
	}
	planning := seed.Planning()
	planning.AddToFocusQueue("already-queued", "project-1")
	if commitErr := seed.CommitPlanning(planning); commitErr != nil {
		t.Fatalf("CommitPlanning: %v", commitErr)
	}

	automation := NewAutomationConfig()
	automation.WorkflowRules = []workflow.WorkflowRule{
		{
			ID:       "mixed",
			Name:     "Keep remaining action",
			Enabled:  true,
			Triggers: []workflow.RuleTrigger{{Field: workflow.FieldState, Operation: workflow.OpEquals, Value: "ready"}}, Actions: []workflow.RuleAction{
				{ActionType: workflow.ActionType("set_active"), Value: "true"},
				{ActionType: workflow.ActionAddTag, Value: "retained"},
			},
		},
		{
			ID:       "retired-only",
			Name:     "Remove actionless rule",
			Enabled:  true,
			Triggers: []workflow.RuleTrigger{{Field: workflow.FieldState, Operation: workflow.OpEquals, Value: "ready"}}, Actions: []workflow.RuleAction{{ActionType: workflow.ActionType("set_active"), Value: "false"}}},
		{
			ID:       "unchanged",
			Name:     "Keep untouched rule",
			Enabled:  true,
			Triggers: []workflow.RuleTrigger{{Field: workflow.FieldState, Operation: workflow.OpEquals, Value: "ready"}}, Actions: []workflow.RuleAction{{ActionType: workflow.ActionSetPinned, Value: "true"}}},
	}
	if commitErr := manager.CommitAutomation(automation); commitErr != nil {
		t.Fatalf("CommitAutomation: %v", commitErr)
	}
	catalog := seed.Catalog()
	catalog.SchemaVersion = previousConfigVersion
	if commitErr := manager.CommitCatalog(catalog); commitErr != nil {
		t.Fatalf("CommitCatalog legacy schema: %v", commitErr)
	}

	rs, err := manager.LoadRuntimeState()
	if err != nil {
		t.Fatalf("LoadRuntimeState migration: %v", err)
	}
	rules := rs.Automation().WorkflowRules
	if len(rules) != 2 || rules[0].ID != "mixed" || rules[1].ID != "unchanged" {
		t.Fatalf("sanitized rules = %+v, want mixed and unchanged", rules)
	}
	if got := rules[0].Actions; len(got) != 1 || got[0].ActionType != workflow.ActionAddTag {
		t.Fatalf("mixed actions = %+v, want retained add_tag", got)
	}
	if queue := rs.Planning().GetFocusQueue(); len(queue) != 1 || queue[0].TaskID != "already-queued" {
		t.Fatalf("sanitizer changed focus queue: %+v", queue)
	}

	automationPath := AutomationPath(rs.DataDir())
	first, err := os.ReadFile(automationPath)
	if err != nil {
		t.Fatalf("read sanitized automation: %v", err)
	}
	if reloadErr := rs.Reload(); reloadErr != nil {
		t.Fatalf("idempotent Reload: %v", reloadErr)
	}
	second, err := os.ReadFile(automationPath)
	if err != nil {
		t.Fatalf("read reloaded automation: %v", err)
	}
	if !bytes.Equal(first, second) {
		t.Fatal("idempotent sanitizer rewrote automation on a second reload")
	}
}

// TestRuntimeState_CommitSettingsRefreshesWithoutRebuildingConfig proves the
// domain-native settings commit persists only the settings + machine domains,
// leaves the domains it does not own untouched, and advances the in-memory
// snapshot so the accessors observe the mutation with no extra disk load.
func TestRuntimeState_CommitSettingsRefreshesWithoutRebuildingConfig(t *testing.T) {
	manager, _ := seededManager(t)

	rs, err := manager.LoadRuntimeState()
	if err != nil {
		t.Fatalf("LoadRuntimeState: %v", err)
	}
	// Seed a value in a domain the settings commit must NOT touch.
	planning := rs.Planning()
	planning.StreakCount = 7
	if commitErr := rs.CommitPlanning(planning); commitErr != nil {
		t.Fatalf("CommitPlanning seed: %v", commitErr)
	}

	settings := rs.Settings()
	settings.ActiveTheme = "light"
	machine := rs.Machine()
	machine.Editor = "hx"
	if commitErr := rs.CommitSettings(settings, machine); commitErr != nil {
		t.Fatalf("CommitSettings: %v", commitErr)
	}

	// The snapshot advanced in-lock-step: accessors return the committed values.
	if got := rs.Settings().ActiveTheme; got != "light" {
		t.Errorf("snapshot ActiveTheme = %q, want light", got)
	}
	if got := rs.Machine().Editor; got != "hx" {
		t.Errorf("snapshot machine editor = %q, want hx", got)
	}
	// The planning domain the settings commit did not own survived untouched.
	if got := rs.Planning().StreakCount; got != 7 {
		t.Errorf("planning domain clobbered by settings commit: StreakCount = %d, want 7", got)
	}

	// A fresh reload from disk agrees, proving the commit persisted rather than
	// only mutating the in-memory snapshot.
	reloaded, err := manager.LoadRuntimeState()
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if got := reloaded.Settings().ActiveTheme; got != "light" {
		t.Errorf("reloaded ActiveTheme = %q, want light", got)
	}
	if got := reloaded.Machine().Editor; got != "hx" {
		t.Errorf("reloaded machine editor = %q, want hx", got)
	}
	if got := reloaded.Planning().StreakCount; got != 7 {
		t.Errorf("reloaded StreakCount = %d, want 7", got)
	}
}

// TestRuntimeState_CommitNilArgsGuarded confirms the domain-native commit seams
// reject nil payloads before touching disk.
func TestRuntimeState_CommitNilArgsGuarded(t *testing.T) {
	manager, _ := seededManager(t)
	rs, err := manager.LoadRuntimeState()
	if err != nil {
		t.Fatalf("LoadRuntimeState: %v", err)
	}
	if err := rs.CommitSettings(nil, rs.Machine()); err == nil {
		t.Error("expected error for nil settings")
	}
	if err := rs.CommitSettings(rs.Settings(), nil); err == nil {
		t.Error("expected error for nil machine state")
	}
	if err := rs.CommitCatalog(nil); err == nil {
		t.Error("expected error for nil catalog")
	}
	if err := rs.CommitPlanning(nil); err == nil {
		t.Error("expected error for nil planning")
	}
	if err := rs.CommitAutomation(nil); err == nil {
		t.Error("expected error for nil automation")
	}
	if err := rs.CommitMachine(nil); err == nil {
		t.Error("expected error for nil machine")
	}
}

// TestRuntimeState_RefreshAfterCommitObservesWrite confirms the snapshot advances
// to a domain change written after load only through an explicit refresh, so
// render/update helpers reading the accessors never touch disk yet still observe
// committed mutations once the runtime refreshes.
func TestRuntimeState_RefreshAfterCommitObservesWrite(t *testing.T) {
	manager, _ := seededManager(t)

	rs, err := manager.LoadRuntimeState()
	if err != nil {
		t.Fatalf("LoadRuntimeState: %v", err)
	}
	if got := rs.Settings().DefaultContext; got != "personal" {
		t.Fatalf("seeded DefaultContext = %q, want personal", got)
	}

	// Mutate settings out-of-band and persist through the manager.
	cfg, err := manager.LoadDomains()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	cfg.Settings.DefaultContext = "work"
	if err := saveDomainBundle(manager.domainStore(), cfg); err != nil {
		t.Fatalf("Save: %v", err)
	}

	// The cached snapshot is unchanged until it refreshes.
	if got := rs.Settings().DefaultContext; got != "personal" {
		t.Fatalf("snapshot changed before refresh: %q", got)
	}
	if err := rs.RefreshAfterCommit(); err != nil {
		t.Fatalf("RefreshAfterCommit: %v", err)
	}
	if got := rs.Settings().DefaultContext; got != "work" {
		t.Fatalf("refreshed DefaultContext = %q, want work", got)
	}
}

// TestEmptyRuntimeState_AccessorsSafe confirms the fallback snapshot never returns
// nil domains so a caller that could not load real files stays usable.
func TestEmptyRuntimeState_AccessorsSafe(t *testing.T) {
	rs := EmptyRuntimeState(t.TempDir())
	if rs.Catalog() == nil || rs.Planning() == nil || rs.Settings() == nil ||
		rs.Automation() == nil || rs.Machine() == nil {
		t.Fatalf("empty runtime state must default every domain")
	}
	if rs.ActiveProjects() != nil {
		t.Fatalf("empty catalog should yield no active projects")
	}
}

// TestRuntimeState_CloneDomainsIsIndependent proves lifecycle planning can
// mutate a defensive copy without changing the command's loaded snapshot before
// the transaction succeeds.
func TestRuntimeState_CloneDomainsIsIndependent(t *testing.T) {
	manager, _ := seededManager(t)
	rs, err := manager.LoadRuntimeState()
	if err != nil {
		t.Fatalf("LoadRuntimeState: %v", err)
	}
	cloned, err := rs.CloneDomains()
	if err != nil {
		t.Fatalf("CloneDomains: %v", err)
	}
	cloned.Settings.DefaultContext = "cloned"
	cloned.Planning.StreakCount = 99
	cloned.Machine.Editor = "cloned-editor"

	if got := rs.Settings().DefaultContext; got == "cloned" {
		t.Fatal("settings mutation leaked into runtime snapshot")
	}
	if got := rs.Planning().StreakCount; got == 99 {
		t.Fatal("planning mutation leaked into runtime snapshot")
	}
	if got := rs.Machine().Editor; got == "cloned-editor" {
		t.Fatal("machine mutation leaked into runtime snapshot")
	}
}

// TestRuntimeState_CheckedArtifactsKeepStartupRevision proves artifact encoding
// does not adopt a concurrent writer's newer revision for stale in-memory intent.
func TestRuntimeState_CheckedArtifactsKeepStartupRevision(t *testing.T) {
	manager, _ := seededManager(t)
	rs, err := manager.LoadRuntimeState()
	if err != nil {
		t.Fatalf("LoadRuntimeState: %v", err)
	}
	settingsPath := SettingsPath(rs.DataDir())
	loaded, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("read loaded settings: %v", err)
	}
	concurrent := []byte(`{"concurrent":true}`)
	if writeErr := os.WriteFile(settingsPath, concurrent, FilePermissions); writeErr != nil {
		t.Fatalf("write concurrent settings: %v", writeErr)
	}

	domains, err := rs.CloneDomains()
	if err != nil {
		t.Fatalf("CloneDomains: %v", err)
	}
	artifacts, err := rs.CheckedSyncedArtifacts(domains)
	if err != nil {
		t.Fatalf("CheckedSyncedArtifacts: %v", err)
	}
	for _, artifact := range artifacts {
		if artifact.Path != settingsPath {
			continue
		}
		if artifact.ExpectedRevision == nil {
			t.Fatal("settings artifact has no expected revision")
		}
		if got, want := *artifact.ExpectedRevision, revisionOf(loaded); got != want {
			t.Fatalf("expected revision = %q, want startup revision %q", got, want)
		}
		if *artifact.ExpectedRevision == revisionOf(concurrent) {
			t.Fatal("artifact incorrectly adopted concurrent revision")
		}
		return
	}
	t.Fatal("settings artifact not emitted")
}

func TestRuntimeState_CheckedAutomationArtifactKeepsStartupRevision(t *testing.T) {
	manager, _ := seededManager(t)
	rs, err := manager.LoadRuntimeState()
	if err != nil {
		t.Fatalf("LoadRuntimeState: %v", err)
	}
	automationPath := AutomationPath(rs.DataDir())
	loaded, err := os.ReadFile(automationPath)
	if err != nil {
		t.Fatalf("read loaded automation: %v", err)
	}
	concurrent := []byte(`{"concurrent":true}`)
	if writeErr := os.WriteFile(automationPath, concurrent, FilePermissions); writeErr != nil {
		t.Fatalf("write concurrent automation: %v", writeErr)
	}

	artifact, err := rs.CheckedAutomationArtifact(rs.Automation())
	if err != nil {
		t.Fatalf("CheckedAutomationArtifact: %v", err)
	}
	if artifact.Path != automationPath || artifact.ExpectedRevision == nil {
		t.Fatalf("artifact = %+v, want checked automation path", artifact)
	}
	if got, want := *artifact.ExpectedRevision, revisionOf(loaded); got != want {
		t.Fatalf("expected revision = %q, want startup revision %q", got, want)
	}
	if *artifact.ExpectedRevision == revisionOf(concurrent) {
		t.Fatal("automation artifact incorrectly adopted concurrent revision")
	}
}
