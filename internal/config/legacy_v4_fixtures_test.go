package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

const legacyV4FixtureRoot = "../storage/testdata/v4_migration"
const legacyV4WorkspaceID = "00000000-0000-4000-8000-000000000004"

func readLegacyV4Envelope(t *testing.T, rel string) domainEnvelope {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(legacyV4FixtureRoot, rel))
	if err != nil {
		t.Fatalf("read fixture %s: %v", rel, err)
	}
	var env domainEnvelope
	if err = json.Unmarshal(data, &env); err != nil {
		t.Fatalf("decode fixture envelope %s: %v", rel, err)
	}
	if env.SchemaVersion != CurrentDomainSchemaVersion {
		t.Fatalf("%s envelope schema = %d, want %d", rel, env.SchemaVersion, CurrentDomainSchemaVersion)
	}
	if env.WorkspaceID != legacyV4WorkspaceID {
		t.Fatalf("%s workspace ID = %q, want %q", rel, env.WorkspaceID, legacyV4WorkspaceID)
	}
	return env
}

func decodeLegacyV4Payload(t *testing.T, rel, kind string, out any) {
	t.Helper()
	env := readLegacyV4Envelope(t, rel)
	if env.Kind != kind {
		t.Fatalf("%s kind = %q, want %q", rel, env.Kind, kind)
	}
	if err := json.Unmarshal(env.Data, out); err != nil {
		t.Fatalf("decode %s payload: %v", rel, err)
	}
}

func TestLegacyV4SplitDomainFixtureDecodesEveryDomain(t *testing.T) {
	var catalog LegacyV4WorkspaceCatalog
	decodeLegacyV4Payload(t, "workspace/.tt/workspace.json", "workspace", &catalog)
	if catalog.SchemaVersion != legacyV4ConfigSchema {
		t.Fatalf("catalog schema = %d, want v%d", catalog.SchemaVersion, legacyV4ConfigSchema)
	}
	if catalog.SystemInboxProjectID != "project-inbox" || len(catalog.Projects) != 6 {
		t.Fatalf("catalog fixture lost project coverage: inbox=%q projects=%d", catalog.SystemInboxProjectID, len(catalog.Projects))
	}
	var sawArchived, sawSandbox, sawInternal bool
	for _, project := range catalog.Projects {
		sawArchived = sawArchived || project.Archived
		sawSandbox = sawSandbox || project.Sandbox
		sawInternal = sawInternal || project.Internal
	}
	if !sawArchived || !sawSandbox || !sawInternal {
		t.Fatalf("catalog fixture is missing project shapes: archived=%v sandbox=%v internal=%v", sawArchived, sawSandbox, sawInternal)
	}

	var planning LegacyV4PlanningState
	decodeLegacyV4Payload(t, "workspace/.tt/planning.json", "planning", &planning)
	if planning.TUIStateVersion != 4 ||
		len(planning.FocusQueue) != 2 ||
		len(planning.DailyPlan) != 2 {
		t.Fatalf("planning fixture does not cover legacy planning collections: %+v", planning)
	}
	planningEnvelope := readLegacyV4Envelope(t, "workspace/.tt/planning.json")
	for _, token := range [][]byte{[]byte(`"alert_dismissals"`), []byte(`"streak_count"`)} {
		if !bytes.Contains(planningEnvelope.Data, token) {
			t.Fatalf("planning fixture does not cover retired field %s: %s", token, planningEnvelope.Data)
		}
	}

	var settings LegacyV4Settings
	decodeLegacyV4Payload(t, "workspace/.tt/settings.json", "settings", &settings)
	if settings.TUIStateVersion != 4 ||
		settings.ActiveTheme != "synthetic-custom" {
		t.Fatalf("settings fixture does not cover v4 appearance/behavior fields: %+v", settings)
	}
	settingsEnvelope := readLegacyV4Envelope(t, "workspace/.tt/settings.json")
	for _, token := range [][]byte{[]byte(`"theme_dark_name"`), []byte(`"onboarding"`), []byte(`"state_group_limits"`)} {
		if !bytes.Contains(settingsEnvelope.Data, token) {
			t.Fatalf("settings fixture does not cover retired field %s: %s", token, settingsEnvelope.Data)
		}
	}

	var automation LegacyV4AutomationConfig
	decodeLegacyV4Payload(t, "workspace/.tt/automation.json", "automation", &automation)
	automationEnvelope := readLegacyV4Envelope(t, "workspace/.tt/automation.json")
	for _, token := range [][]byte{[]byte(`"workflow_rules"`), []byte(`"task_templates"`), []byte(`"scoring_profiles"`), []byte(`"workflow_eval_states"`), []byte(`"project_scoring_profile"`)} {
		if !bytes.Contains(automationEnvelope.Data, token) {
			t.Fatalf("automation fixture is missing %s: %s", token, automationEnvelope.Data)
		}
	}

	var machine LegacyV4MachineState
	decodeLegacyV4Payload(t, "machine/runtime/"+legacyV4WorkspaceID+"/machine.json", "machine", &machine)
	if machine.TUIStateVersion != 4 ||
		len(machine.CustomContexts) == 0 {
		t.Fatalf("machine fixture does not cover retained and retired fields: %+v", machine)
	}
	machineEnvelope := readLegacyV4Envelope(t, "machine/runtime/"+legacyV4WorkspaceID+"/machine.json")
	for _, token := range [][]byte{[]byte(`"editor"`), []byte(`"recent_task_visits"`), []byte(`"recent_palette_items"`), []byte(`"recent_destinations"`)} {
		if !bytes.Contains(machineEnvelope.Data, token) {
			t.Fatalf("machine fixture does not cover retired field %s: %s", token, machineEnvelope.Data)
		}
	}
}

func TestLegacyV4ThemeVariantsCoverBothV5Mappings(t *testing.T) {
	var light LegacyV4Settings
	decodeLegacyV4Payload(t, "variants/settings-light.json", "settings", &light)
	if light.ActiveTheme != "light" {
		t.Fatalf("light fixture active theme = %q, want light", light.ActiveTheme)
	}

	var custom LegacyV4Settings
	decodeLegacyV4Payload(t, "workspace/.tt/settings.json", "settings", &custom)
	if custom.ActiveTheme == "dark" || custom.ActiveTheme == "light" {
		t.Fatalf("custom fixture active theme = %q, want a custom variant that maps to dark", custom.ActiveTheme)
	}
}

func TestLegacyV4UnsupportedConfigFixturesHaveExpectedFailureShape(t *testing.T) {
	corrupt, err := os.ReadFile(filepath.Join(legacyV4FixtureRoot, "unsupported/corrupt-workspace.json"))
	if err != nil {
		t.Fatalf("read corrupt envelope: %v", err)
	}
	var env domainEnvelope
	if err = json.Unmarshal(corrupt, &env); err == nil {
		t.Fatal("corrupt workspace envelope unexpectedly decoded")
	}

	future, err := os.ReadFile(filepath.Join(legacyV4FixtureRoot, "unsupported/future-workspace.json"))
	if err != nil {
		t.Fatalf("read future envelope: %v", err)
	}
	if err = json.Unmarshal(future, &env); err != nil {
		t.Fatalf("decode future envelope: %v", err)
	}
	if env.SchemaVersion <= CurrentDomainSchemaVersion {
		t.Fatalf("future envelope schema = %d, want > %d", env.SchemaVersion, CurrentDomainSchemaVersion)
	}

	partialRoot := t.TempDir()
	partialSrc := filepath.Join(legacyV4FixtureRoot, "unsupported/partial/workspace/.tt/workspace.json")
	partialBytes, err := os.ReadFile(partialSrc)
	if err != nil {
		t.Fatalf("read partial fixture: %v", err)
	}
	partialDest := filepath.Join(partialRoot, ".tt", "workspace.json")
	if err = os.MkdirAll(filepath.Dir(partialDest), DirPermissions); err != nil {
		t.Fatalf("create partial fixture directory: %v", err)
	}
	if err = os.WriteFile(partialDest, partialBytes, FilePermissions); err != nil {
		t.Fatalf("copy partial fixture: %v", err)
	}
	manager := NewManager(filepath.Join(partialRoot, ".tt.json"))
	if _, err = manager.LoadDomains(); !errors.Is(err, ErrPartialDomainState) {
		t.Fatalf("partial fixture error = %v, want ErrPartialDomainState", err)
	}
}
