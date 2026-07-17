package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/aiharness"
)

func TestMachineAIHarnessDefaultsAndValidation(t *testing.T) {
	machine := &MachineState{}
	if got := machine.GetAIHarness(); got != aiharness.Claude {
		t.Fatalf("legacy machine harness = %q, want claude", got)
	}
	if err := machine.SetAIHarness(aiharness.Codex); err != nil {
		t.Fatalf("SetAIHarness: %v", err)
	}
	if machine.AIHarness != "codex" || machine.GetAIHarness() != aiharness.Codex {
		t.Fatalf("stored harness = %q / %q", machine.AIHarness, machine.GetAIHarness())
	}
	if err := machine.SetAIHarness(aiharness.Name("gemini")); !errors.Is(err, aiharness.ErrUnknownHarness) {
		t.Fatalf("SetAIHarness(unknown) error = %v", err)
	}
	if err := (&MachineState{AIHarness: "gemini"}).Validate(); err == nil {
		t.Fatal("unknown persisted AI harness should fail validation")
	}
	if got := (&MachineState{AIHarness: "gemini"}).GetAIHarness(); got != aiharness.Claude {
		t.Fatalf("invalid in-memory harness fallback = %q, want claude", got)
	}
}

func TestMachineAIConfigDirAccessorsNormalizeCopyAndClear(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "codex")
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	machine := &MachineState{}
	if err := machine.SetAIConfigDir(aiharness.Codex, dir+string(os.PathSeparator)+"."); err != nil {
		t.Fatalf("SetAIConfigDir: %v", err)
	}
	if got := machine.GetAIConfigDir(aiharness.Codex); got != dir {
		t.Fatalf("GetAIConfigDir = %q, want %q", got, dir)
	}
	copyOfDirs := machine.GetAIConfigDirs()
	copyOfDirs["codex"] = "changed"
	if got := machine.GetAIConfigDir(aiharness.Codex); got != dir {
		t.Fatalf("returned map mutation leaked into machine state: %q", got)
	}
	if err := machine.SetAIConfigDir(aiharness.Codex, ""); err != nil {
		t.Fatalf("clear SetAIConfigDir: %v", err)
	}
	if machine.AIConfigDirs != nil {
		t.Fatalf("cleared config dirs = %#v, want nil", machine.AIConfigDirs)
	}
	if err := machine.SetAIConfigDir(aiharness.Name("gemini"), dir); !errors.Is(err, aiharness.ErrUnknownHarness) {
		t.Fatalf("SetAIConfigDir(unknown) error = %v", err)
	}
}

func TestMachineSetAIConfigDirsIsAtomicAndCopiesInput(t *testing.T) {
	claudeDir := filepath.Join(t.TempDir(), "claude")
	if err := os.Mkdir(claudeDir, 0o700); err != nil {
		t.Fatal(err)
	}
	input := map[string]string{"claude": claudeDir, "codex": ""}
	machine := &MachineState{}
	if err := machine.SetAIConfigDirs(input); err != nil {
		t.Fatalf("SetAIConfigDirs: %v", err)
	}
	input["claude"] = "changed"
	if got := machine.GetAIConfigDir(aiharness.Claude); got != claudeDir {
		t.Fatalf("input map mutation leaked: %q", got)
	}
	if _, exists := machine.AIConfigDirs["codex"]; exists {
		t.Fatal("empty path was persisted as an override")
	}

	before := machine.GetAIConfigDirs()
	if err := machine.SetAIConfigDirs(map[string]string{"gemini": claudeDir}); !errors.Is(err, aiharness.ErrUnknownHarness) {
		t.Fatalf("unknown bulk harness error = %v", err)
	}
	if got := machine.GetAIConfigDirs(); got["claude"] != before["claude"] || len(got) != len(before) {
		t.Fatalf("failed replacement mutated state: before=%v after=%v", before, got)
	}
}

func TestMachineValidationKeepsUnavailableConfigDirsLoadable(t *testing.T) {
	stale := filepath.Join(t.TempDir(), "removed")
	machine := &MachineState{AIConfigDirs: map[string]string{"codex": stale}}
	if err := machine.Validate(); err != nil {
		t.Fatalf("stale known config directory should remain loadable: %v", err)
	}
	if _, err := aiharness.ResolveConfigDir(aiharness.Codex, machine.GetAIConfigDir(aiharness.Codex)); err == nil {
		t.Fatal("using a stale explicit override should fail")
	}
	if err := (&MachineState{AIConfigDirs: map[string]string{"gemini": stale}}).Validate(); err == nil {
		t.Fatal("unknown config-directory harness should fail validation")
	}
	if err := (&MachineState{AIConfigDirs: map[string]string{"codex": ""}}).Validate(); err == nil {
		t.Fatal("empty persisted override should fail validation")
	}
	if err := (&MachineState{AIConfigDirs: map[string]string{"codex": "configs/codex"}}).Validate(); err == nil {
		t.Fatal("relative persisted override should fail validation")
	}
}

func TestManagerLoadDomainsRejectsInvalidPersistedAIPreferences(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*Domains)
		want   string
	}{
		{
			name: "unknown harness",
			mutate: func(domains *Domains) {
				domains.Machine.AIHarness = "gemini"
			},
			want: "invalid machine domain",
		},
		{
			name: "relative config directory",
			mutate: func(domains *Domains) {
				domains.Machine.AIConfigDirs = map[string]string{"codex": "configs/codex"}
			},
			want: "must contain an absolute path",
		},
		{
			name: "unknown consent provider",
			mutate: func(domains *Domains) {
				domains.Settings.AIConsent.Providers = map[string]AIProviderConsent{"gemini": {Granted: true}}
			},
			want: "invalid settings domain",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			domains := NewDomains()
			tt.mutate(domains)
			manager, _ := seedWorkspace(t, domains)

			loaded, err := manager.LoadDomains()
			if loaded != nil {
				t.Fatalf("LoadDomains returned invalid persisted domains: %+v", loaded)
			}
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("LoadDomains error = %v, want containing %q", err, tt.want)
			}
		})
	}
}

func TestAIConsentLegacyAndProviderIsolation(t *testing.T) {
	legacyTime := time.Date(2025, 1, 2, 3, 4, 5, 0, time.UTC)
	legacy := AIConsentSettings{Granted: true, GrantedAt: &legacyTime}
	if !legacy.GrantedFor(aiharness.Claude) {
		t.Fatal("legacy consent should authorize Claude")
	}
	if legacy.GrantedFor(aiharness.Codex) || legacy.GrantedFor(aiharness.Copilot) {
		t.Fatal("legacy consent must not authorize Codex or Copilot")
	}

	localTime := time.Date(2026, 7, 16, 12, 0, 0, 0, time.FixedZone("test", -6*60*60))
	var codex AIConsentSettings
	if err := codex.GrantFor(aiharness.Codex, localTime); err != nil {
		t.Fatalf("GrantFor(Codex): %v", err)
	}
	if !codex.GrantedFor(aiharness.Codex) || codex.GrantedFor(aiharness.Claude) || codex.Granted {
		t.Fatalf("Codex grant leaked across providers: %+v", codex)
	}
	if got := codex.Providers["codex"].GrantedAt; got == nil || !got.Equal(localTime.UTC()) || got.Location() != time.UTC {
		t.Fatalf("Codex grant time = %v, want %v UTC", got, localTime.UTC())
	}

	var claude AIConsentSettings
	if err := claude.GrantFor(aiharness.Claude, localTime); err != nil {
		t.Fatalf("GrantFor(Claude): %v", err)
	}
	if !claude.Granted || claude.GrantedAt == nil || !claude.GrantedFor(aiharness.Claude) || !claude.Providers["claude"].Granted {
		t.Fatalf("Claude grant did not update both formats: %+v", claude)
	}
	if claude.GrantedFor(aiharness.Copilot) {
		t.Fatal("Claude grant authorized Copilot")
	}
	if err := claude.GrantFor(aiharness.Name("gemini"), localTime); !errors.Is(err, aiharness.ErrUnknownHarness) {
		t.Fatalf("GrantFor(unknown) error = %v", err)
	}
}

func TestAIHarnessPreferencesSerializeAndLegacyJSONDefaults(t *testing.T) {
	var legacyMachine MachineState
	if err := json.Unmarshal([]byte(`{"editor":"hx"}`), &legacyMachine); err != nil {
		t.Fatalf("unmarshal legacy machine: %v", err)
	}
	if legacyMachine.GetAIHarness() != aiharness.Claude || legacyMachine.AIConfigDirs != nil {
		t.Fatalf("legacy machine defaults = %+v", legacyMachine)
	}
	var legacySettings Settings
	if err := json.Unmarshal([]byte(`{"ai_consent":{"granted":true}}`), &legacySettings); err != nil {
		t.Fatalf("unmarshal legacy settings: %v", err)
	}
	if !legacySettings.AIConsent.GrantedFor(aiharness.Claude) || legacySettings.AIConsent.GrantedFor(aiharness.Codex) {
		t.Fatalf("legacy settings consent = %+v", legacySettings.AIConsent)
	}

	now := time.Date(2026, 7, 16, 18, 0, 0, 0, time.UTC)
	original := struct {
		Machine  MachineState `json:"machine"`
		Settings Settings     `json:"settings"`
	}{
		Machine: MachineState{AIHarness: "copilot", AIConfigDirs: map[string]string{"copilot": "/tmp/copilot"}},
	}
	if err := original.Settings.AIConsent.GrantFor(aiharness.Copilot, now); err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal preferences: %v", err)
	}
	var decoded struct {
		Machine  MachineState `json:"machine"`
		Settings Settings     `json:"settings"`
	}
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal preferences: %v", err)
	}
	if decoded.Machine.GetAIHarness() != aiharness.Copilot || decoded.Machine.GetAIConfigDir(aiharness.Copilot) != "/tmp/copilot" {
		t.Fatalf("machine round trip = %+v", decoded.Machine)
	}
	if !decoded.Settings.AIConsent.GrantedFor(aiharness.Copilot) || decoded.Settings.AIConsent.Granted {
		t.Fatalf("settings round trip = %+v", decoded.Settings.AIConsent)
	}
}

func TestAIProviderMapsValidateAndCloneIndependently(t *testing.T) {
	settings := NewSettings()
	settings.AIConsent.Providers = map[string]AIProviderConsent{"gemini": {Granted: true}}
	if err := settings.Validate(); err == nil {
		t.Fatal("unknown consent provider should fail validation")
	}

	runtime := EmptyRuntimeState(t.TempDir())
	runtime.Machine().AIConfigDirs = map[string]string{"codex": "/tmp/codex"}
	runtime.Settings().AIConsent.Providers = map[string]AIProviderConsent{"codex": {Granted: true}}
	cloned, err := runtime.CloneDomains()
	if err != nil {
		t.Fatalf("CloneDomains: %v", err)
	}
	cloned.Machine.AIConfigDirs["codex"] = "/changed"
	cloned.Settings.AIConsent.Providers["codex"] = AIProviderConsent{}
	if runtime.Machine().AIConfigDirs["codex"] != "/tmp/codex" {
		t.Fatal("cloned machine config map shares storage with runtime")
	}
	if !runtime.Settings().AIConsent.Providers["codex"].Granted {
		t.Fatal("cloned provider consent map shares storage with runtime")
	}
}
