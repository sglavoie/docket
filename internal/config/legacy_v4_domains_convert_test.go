package config

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

func legacyV4DomainArtifactsForTest(t *testing.T, settingsRel string) LegacyV4DomainArtifacts {
	t.Helper()
	read := func(rel string) []byte {
		t.Helper()
		data, err := os.ReadFile(filepath.Join(legacyV4FixtureRoot, rel))
		if err != nil {
			t.Fatalf("read fixture %s: %v", rel, err)
		}
		return data
	}
	return LegacyV4DomainArtifacts{
		Workspace:  read("workspace/.docket/workspace.json"),
		Planning:   read("workspace/.docket/planning.json"),
		Settings:   read(settingsRel),
		Automation: read("workspace/.docket/automation.json"),
		Machine:    read("machine/runtime/" + legacyV4WorkspaceID + "/machine.json"),
	}
}

//nolint:gocyclo // This end-to-end fixture asserts the complete conversion contract.
func TestConvertLegacyV4DomainsMixedFixture(t *testing.T) {
	artifacts := legacyV4DomainArtifactsForTest(t, "workspace/.docket/settings.json")
	sourceCopy := cloneLegacyV4DomainArtifactBytes(artifacts)
	legacy, err := DecodeLegacyV4Domains(artifacts)
	if err != nil {
		t.Fatalf("DecodeLegacyV4Domains: %v", err)
	}
	first, err := ConvertLegacyV4Domains(legacy)
	if err != nil {
		t.Fatalf("ConvertLegacyV4Domains: %v", err)
	}
	second, err := ConvertLegacyV4Domains(legacy)
	if err != nil {
		t.Fatalf("second ConvertLegacyV4Domains: %v", err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatal("repeated conversion produced different candidates")
	}
	if !reflect.DeepEqual(artifacts, sourceCopy) {
		t.Fatal("conversion mutated source artifact bytes")
	}

	catalog := first.Domains.Catalog
	if catalog.SchemaVersion != V5ConfigSchemaVersion ||
		catalog.WorkspaceID != legacyV4WorkspaceID ||
		catalog.SystemInboxProjectID != "project-inbox" {
		t.Fatalf("catalog identity = %+v", catalog)
	}
	if len(catalog.Projects) != 4 {
		t.Fatalf("retained projects = %d, want 4", len(catalog.Projects))
	}
	if got, want := first.RemovedProjectIDs, []string{"project-sandbox", "project-internal"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("removed project IDs = %v, want %v", got, want)
	}
	for _, project := range catalog.Projects {
		if project.ID == "project-sandbox" || project.ID == "project-internal" {
			t.Fatalf("removed project leaked into v5 catalog: %+v", project)
		}
		if project.ColorSlot < 1 || project.ColorSlot > ProjectColorSlotCount {
			t.Fatalf("project %q color slot = %d", project.ID, project.ColorSlot)
		}
	}
	rawCatalog, err := json.Marshal(catalog)
	if err != nil {
		t.Fatalf("marshal v5 catalog: %v", err)
	}
	for _, retired := range []string{"status_icon", "status_icon_source", "sandbox", "internal"} {
		if bytes.Contains(rawCatalog, []byte(`"`+retired+`"`)) {
			t.Fatalf("v5 catalog retained %q: %s", retired, rawCatalog)
		}
	}

	if got := first.Domains.Planning.NextOrder; len(got) != 2 ||
		got[0].TaskID != "synthetic-alpha-weekday" ||
		got[1].TaskID != "synthetic-beta-root" {
		t.Fatalf("Next order = %+v", got)
	}
	if got := first.Domains.Planning.Today; len(got) != 2 ||
		got[0].TaskID != "synthetic-alpha-rich" ||
		got[1].TaskID != "synthetic-beta-root" {
		t.Fatalf("Today = %+v", got)
	}
	rawPlanning, err := json.Marshal(first.Domains.Planning)
	if err != nil {
		t.Fatalf("marshal v5 planning: %v", err)
	}
	for _, retired := range []string{"planned_date", "alert", "review", "streak", "dismiss"} {
		if bytes.Contains(rawPlanning, []byte(retired)) {
			t.Fatalf("v5 planning retained %q: %s", retired, rawPlanning)
		}
	}

	machine := first.Domains.Machine
	if machine.Theme != "dark" {
		t.Fatalf("custom v4 theme mapped to %q, want dark", machine.Theme)
	}
	if got := machine.CustomContexts["synthetic-focus"]; !reflect.DeepEqual(got, []string{"project-alpha", "project-beta"}) {
		t.Fatalf("custom context members = %v", got)
	}
	rawMachine, err := json.Marshal(machine)
	if err != nil {
		t.Fatalf("marshal v5 machine: %v", err)
	}
	for _, retired := range []string{"editor", "route", "recent", "harness", "palette"} {
		if bytes.Contains(rawMachine, []byte(retired)) {
			t.Fatalf("v5 machine retained %q: %s", retired, rawMachine)
		}
	}
	for _, domain := range []string{"settings", "automation"} {
		if !hasV5DomainAudit(first.Audit, domain, "snapshot-only", "domain") {
			t.Fatalf("missing snapshot-only audit for %s: %+v", domain, first.Audit)
		}
	}
}

func TestConvertLegacyV4DomainsThemeAndContextRepairs(t *testing.T) {
	lightArtifacts := legacyV4DomainArtifactsForTest(t, "variants/settings-light.json")
	legacy, err := DecodeLegacyV4Domains(lightArtifacts)
	if err != nil {
		t.Fatalf("DecodeLegacyV4Domains: %v", err)
	}
	legacy.Machine.CustomContexts = map[string][]string{
		"mixed":        {"project-alpha", "project-alpha", "project-sandbox", "missing", "project-beta"},
		"removed-only": {"project-internal"},
	}
	legacy.Planning.FocusQueue = append(
		legacy.Planning.FocusQueue,
		legacy.Planning.FocusQueue[0],
		LegacyV4FocusQueueItem{TaskID: "sandbox-next", ProjectID: "project-sandbox"},
	)
	inputCopy := append([]string(nil), legacy.Machine.CustomContexts["mixed"]...)
	result, err := ConvertLegacyV4Domains(legacy)
	if err != nil {
		t.Fatalf("ConvertLegacyV4Domains: %v", err)
	}
	if result.Domains.Machine.Theme != "light" {
		t.Fatalf("light theme mapped to %q", result.Domains.Machine.Theme)
	}
	if got, want := result.Domains.Machine.CustomContexts["mixed"], []string{"project-alpha", "project-beta"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("repaired context = %v, want %v", got, want)
	}
	if _, exists := result.Domains.Machine.CustomContexts["removed-only"]; exists {
		t.Fatal("empty repaired custom context should be removed")
	}
	if !reflect.DeepEqual(legacy.Machine.CustomContexts["mixed"], inputCopy) {
		t.Fatal("context conversion mutated its input")
	}
	if !hasV5DomainAudit(result.Audit, "machine", "repaired", "custom_context_member") {
		t.Fatalf("missing duplicate-membership repair audit: %+v", result.Audit)
	}
	if len(result.Domains.Planning.NextOrder) != 2 ||
		!hasV5DomainAudit(result.Audit, "planning", "repaired", "next_order") ||
		!hasV5DomainAudit(result.Audit, "planning", "snapshot-only", "next_order") {
		t.Fatalf("planning reference repairs = %+v audit=%+v", result.Domains.Planning.NextOrder, result.Audit)
	}
}

func TestDecodeLegacyV4DomainsRejectsUnsupportedAndMismatchedInputs(t *testing.T) {
	artifacts := legacyV4DomainArtifactsForTest(t, "workspace/.docket/settings.json")
	future, err := os.ReadFile(filepath.Join(legacyV4FixtureRoot, "unsupported/future-workspace.json"))
	if err != nil {
		t.Fatalf("read future fixture: %v", err)
	}
	artifacts.Workspace = future
	if _, decodeErr := DecodeLegacyV4Domains(artifacts); decodeErr == nil || !strings.Contains(decodeErr.Error(), "envelope schema") {
		t.Fatalf("future workspace error = %v", decodeErr)
	}

	artifacts = legacyV4DomainArtifactsForTest(t, "workspace/.docket/settings.json")
	var env map[string]any
	if decodeErr := json.Unmarshal(artifacts.Machine, &env); decodeErr != nil {
		t.Fatalf("decode machine fixture: %v", decodeErr)
	}
	env["workspace_id"] = "00000000-0000-4000-8000-000000000099"
	artifacts.Machine, err = json.Marshal(env)
	if err != nil {
		t.Fatalf("encode mismatched machine fixture: %v", err)
	}
	if _, decodeErr := DecodeLegacyV4Domains(artifacts); decodeErr == nil || !strings.Contains(decodeErr.Error(), "does not match") {
		t.Fatalf("workspace mismatch error = %v", decodeErr)
	}
}

func TestDryRunV5WorkspaceConversionIsReadOnlyAndRepeatable(t *testing.T) {
	workspaceRoot, err := filepath.Abs(filepath.Join(legacyV4FixtureRoot, "workspace"))
	if err != nil {
		t.Fatalf("resolve workspace fixture: %v", err)
	}
	machinePath, err := filepath.Abs(filepath.Join(
		legacyV4FixtureRoot,
		"machine/runtime",
		legacyV4WorkspaceID,
		"machine.json",
	))
	if err != nil {
		t.Fatalf("resolve machine fixture: %v", err)
	}
	before := inventoryV5DryRunSources(t, workspaceRoot, machinePath)

	options := V5WorkspaceDryRunOptions{DataDir: workspaceRoot, MachineStatePath: machinePath}
	first, err := DryRunV5WorkspaceConversion(options)
	if err != nil {
		t.Fatalf("DryRunV5WorkspaceConversion: %v", err)
	}
	second, err := DryRunV5WorkspaceConversion(options)
	if err != nil {
		t.Fatalf("second DryRunV5WorkspaceConversion: %v", err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatal("unchanged workspace produced a different second dry-run plan")
	}
	after := inventoryV5DryRunSources(t, workspaceRoot, machinePath)
	if !reflect.DeepEqual(before, after) {
		t.Fatalf("dry run changed live source inventory\nbefore=%v\nafter=%v", before, after)
	}

	if len(first.Artifacts) != 6 {
		t.Fatalf("target artifacts = %d, want three domains and three notes", len(first.Artifacts))
	}
	if got, want := first.RetiredDomainPaths, []string{
		SettingsPath(workspaceRoot),
		AutomationPath(workspaceRoot),
	}; !reflect.DeepEqual(got, want) {
		t.Fatalf("retired domain paths = %v, want %v", got, want)
	}
	if len(first.SnapshotOnlyProjectDirs) != 2 {
		t.Fatalf("snapshot-only project dirs = %v", first.SnapshotOnlyProjectDirs)
	}
	noteTargets := 0
	for _, artifact := range first.Artifacts {
		if strings.Contains(artifact.Kind, "note") {
			noteTargets++
			for _, retired := range []string{"tags:", "linked_tasks:", "pinned:"} {
				if bytes.Contains(artifact.Data, []byte(retired)) {
					t.Fatalf("note target %s retained %q:\n%s", artifact.Path, retired, artifact.Data)
				}
			}
		}
	}
	if noteTargets != 3 {
		t.Fatalf("note targets = %d, want 3", noteTargets)
	}
}

func cloneLegacyV4DomainArtifactBytes(input LegacyV4DomainArtifacts) LegacyV4DomainArtifacts {
	return LegacyV4DomainArtifacts{
		Workspace:  append([]byte(nil), input.Workspace...),
		Planning:   append([]byte(nil), input.Planning...),
		Settings:   append([]byte(nil), input.Settings...),
		Automation: append([]byte(nil), input.Automation...),
		Machine:    append([]byte(nil), input.Machine...),
	}
}

func hasV5DomainAudit(entries []V5DomainMigrationAuditEntry, domain, action, field string) bool {
	for _, entry := range entries {
		if entry.Domain == domain && entry.Action == action && entry.Field == field {
			return true
		}
	}
	return false
}

func inventoryV5DryRunSources(t *testing.T, root string, extraPaths ...string) map[string][32]byte {
	t.Helper()
	result := make(map[string][32]byte)
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		result[path] = sha256.Sum256(data)
		return nil
	})
	if err != nil {
		t.Fatalf("inventory workspace fixture: %v", err)
	}
	sort.Strings(extraPaths)
	for _, path := range extraPaths {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read extra source %s: %v", path, err)
		}
		result[path] = sha256.Sum256(data)
	}
	return result
}
