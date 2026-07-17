package aiharness

import (
	"errors"
	"reflect"
	"testing"
)

func TestHarnessMetadataAndAllowlist(t *testing.T) {
	wantNames := []Name{Claude, Codex, Copilot}
	if got := Names(); !reflect.DeepEqual(got, wantNames) {
		t.Fatalf("Names() = %v, want %v", got, wantNames)
	}
	if got := AllowedValues(); !reflect.DeepEqual(got, []string{"claude", "codex", "copilot"}) {
		t.Fatalf("AllowedValues() = %v", got)
	}

	want := map[Name]Metadata{
		Claude:  {Name: Claude, Label: "Claude", Executable: "claude", ConfigEnvVar: "CLAUDE_CONFIG_DIR", NativeConfigDir: "~/.claude"},
		Codex:   {Name: Codex, Label: "Codex", Executable: "codex", ConfigEnvVar: "CODEX_HOME", NativeConfigDir: "~/.codex"},
		Copilot: {Name: Copilot, Label: "GitHub Copilot", Executable: "copilot", ConfigEnvVar: "COPILOT_HOME", NativeConfigDir: "~/.copilot"},
	}
	for name, wantInfo := range want {
		gotName, err := Parse(name.String())
		if err != nil {
			t.Fatalf("Parse(%q): %v", name, err)
		}
		if gotName != name {
			t.Errorf("Parse(%q) = %q, want %q", name, gotName, name)
		}
		gotInfo, err := Info(name)
		if err != nil {
			t.Fatalf("Info(%q): %v", name, err)
		}
		if gotInfo != wantInfo {
			t.Errorf("Info(%q) = %+v, want %+v", name, gotInfo, wantInfo)
		}
		if name.Label() != wantInfo.Label || name.Executable() != wantInfo.Executable ||
			name.ConfigEnvVar() != wantInfo.ConfigEnvVar || name.NativeConfigDir() != wantInfo.NativeConfigDir {
			t.Errorf("metadata methods for %q do not match %+v", name, wantInfo)
		}
	}

	copyOfNames := Names()
	copyOfNames[0] = Copilot
	if Names()[0] != Claude {
		t.Fatal("Names returned mutable package state")
	}
}

func TestParseRejectsUnknownHarnesses(t *testing.T) {
	for _, raw := range []string{"", "Claude", " claude", "gemini"} {
		if _, err := Parse(raw); !errors.Is(err, ErrUnknownHarness) {
			t.Errorf("Parse(%q) error = %v, want ErrUnknownHarness", raw, err)
		}
	}
	if _, err := Info(Name("gemini")); !errors.Is(err, ErrUnknownHarness) {
		t.Fatalf("Info(unknown) error = %v, want ErrUnknownHarness", err)
	}
	unknown := Name("gemini")
	if unknown.Valid() || unknown.Label() != "" || unknown.Executable() != "" ||
		unknown.ConfigEnvVar() != "" || unknown.NativeConfigDir() != "" {
		t.Fatalf("unknown harness unexpectedly exposed metadata")
	}
}
