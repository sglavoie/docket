package config

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestReleaseMigrationRehearsalUsesEveryLegacyConfigFixture(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	dataDir := filepath.Join(t.TempDir(), "workspace")
	manager := NewManager(ConfigPathForDataDir(dataDir))
	domains, err := manager.LoadOrCreateDomains()
	if err != nil {
		t.Fatalf("LoadOrCreateDomains: %v", err)
	}
	workspaceID := domains.Catalog.WorkspaceID

	fixtures := []struct {
		name   string
		kind   domainKind
		before string
		after  string
	}{
		{name: "planning", kind: domainPlanning, before: "planning-before.json", after: "planning-after.json"},
		{name: "settings", kind: domainSettings, before: "settings-before.json", after: "settings-after.json"},
		{name: "machine", kind: domainMachine, before: "machine-before.json", after: "machine-after.json"},
	}

	legacyEnvelopes := make(map[string][]byte, len(fixtures))
	for _, fixture := range fixtures {
		payload := readTUIStateFixture(t, fixture.before)
		path := writeRawDomainPayload(t, dataDir, workspaceID, fixture.kind, json.RawMessage(payload))
		legacyEnvelopes[path], err = os.ReadFile(path)
		if err != nil {
			t.Fatalf("read copied %s fixture: %v", fixture.name, err)
		}
	}

	if _, err = manager.LoadRuntimeState(); err != nil {
		t.Fatalf("migrate copied fixtures: %v", err)
	}

	canonicalEnvelopes := make(map[string][]byte, len(fixtures))
	for _, fixture := range fixtures {
		path := domainFixturePath(t, dataDir, workspaceID, fixture.kind)
		raw, readErr := os.ReadFile(path)
		if readErr != nil {
			t.Fatalf("read migrated %s fixture: %v", fixture.name, readErr)
		}
		canonicalEnvelopes[path] = raw
		if bytes.Equal(raw, legacyEnvelopes[path]) {
			t.Fatalf("%s fixture was not reserialized", fixture.name)
		}

		var envelope domainEnvelope
		if decodeErr := json.Unmarshal(raw, &envelope); decodeErr != nil {
			t.Fatalf("decode migrated %s envelope: %v", fixture.name, decodeErr)
		}
		assertJSONEqual(
			t,
			envelope.Data,
			releaseFixtureWithProductionDefaults(t, fixture.kind, readTUIStateFixture(t, fixture.after)),
		)

		backup, backupErr := os.ReadFile(path + ".bak")
		if backupErr != nil {
			t.Fatalf("read %s rollback copy: %v", fixture.name, backupErr)
		}
		if !bytes.Equal(backup, legacyEnvelopes[path]) {
			t.Fatalf("%s rollback copy does not preserve the exact legacy fixture", fixture.name)
		}
	}

	if _, err = manager.LoadRuntimeState(); err != nil {
		t.Fatalf("idempotent migrated startup: %v", err)
	}
	for path, want := range canonicalEnvelopes {
		got, readErr := os.ReadFile(path)
		if readErr != nil {
			t.Fatalf("read idempotent fixture %s: %v", path, readErr)
		}
		if !bytes.Equal(got, want) {
			t.Fatalf("idempotent startup rewrote %s", path)
		}
	}
}

func releaseFixtureWithProductionDefaults(t *testing.T, kind domainKind, data []byte) []byte {
	t.Helper()
	var value any
	switch kind {
	case domainPlanning:
		value = NewPlanningState()
	case domainSettings:
		value = NewSettings()
	case domainMachine:
		value = NewMachineState()
	default:
		t.Fatalf("unsupported release fixture kind %s", kind.name())
	}
	if err := json.Unmarshal(data, value); err != nil {
		t.Fatalf("decode canonical %s fixture: %v", kind.name(), err)
	}
	out, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("encode canonical %s fixture: %v", kind.name(), err)
	}
	return out
}

func domainFixturePath(t *testing.T, dataDir, workspaceID string, kind domainKind) string {
	t.Helper()
	switch kind {
	case domainPlanning:
		return PlanningStatePath(dataDir)
	case domainSettings:
		return SettingsPath(dataDir)
	case domainMachine:
		path, err := MachineStatePath(workspaceID)
		if err != nil {
			t.Fatalf("resolve machine fixture path: %v", err)
		}
		return path
	default:
		t.Fatalf("unsupported release fixture kind %s", kind.name())
		return ""
	}
}
