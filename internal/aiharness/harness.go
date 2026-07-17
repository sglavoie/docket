// Package aiharness defines the AI CLI providers supported by tt and the
// provider metadata shared by the CLI, TUI, and subprocess runner.
package aiharness

import (
	"errors"
	"fmt"
)

// Name is the stable, serialized identifier for an AI CLI harness.
type Name string

const (
	Claude  Name = "claude"
	Codex   Name = "codex"
	Copilot Name = "copilot"

	// Default is the compatibility harness used by installations that predate
	// configurable AI providers.
	Default = Claude
)

// ErrUnknownHarness is returned when a caller supplies a harness outside tt's
// fixed allowlist.
var ErrUnknownHarness = errors.New("unknown AI harness")

// Metadata describes the provider-specific values needed to present and invoke
// one supported harness.
type Metadata struct {
	Name            Name
	Label           string
	Executable      string
	ConfigEnvVar    string
	NativeConfigDir string
}

var orderedNames = [...]Name{Claude, Codex, Copilot}

var metadataByName = map[Name]Metadata{
	Claude: {
		Name:            Claude,
		Label:           "Claude",
		Executable:      "claude",
		ConfigEnvVar:    "CLAUDE_CONFIG_DIR",
		NativeConfigDir: "~/.claude",
	},
	Codex: {
		Name:            Codex,
		Label:           "Codex",
		Executable:      "codex",
		ConfigEnvVar:    "CODEX_HOME",
		NativeConfigDir: "~/.codex",
	},
	Copilot: {
		Name:            Copilot,
		Label:           "GitHub Copilot",
		Executable:      "copilot",
		ConfigEnvVar:    "COPILOT_HOME",
		NativeConfigDir: "~/.copilot",
	},
}

func (n Name) String() string { return string(n) }

// Valid reports whether n belongs to tt's fixed harness allowlist.
func (n Name) Valid() bool {
	_, ok := metadataByName[n]
	return ok
}

// Parse validates a serialized or command-line harness name. Names are
// intentionally exact and lower-case so persisted values stay canonical.
func Parse(value string) (Name, error) {
	name := Name(value)
	if !name.Valid() {
		return "", fmt.Errorf("%w %q (want claude, codex, or copilot)", ErrUnknownHarness, value)
	}
	return name, nil
}

// Names returns the supported harnesses in stable presentation order. The
// returned slice is independent and safe for callers to modify.
func Names() []Name {
	names := make([]Name, len(orderedNames))
	copy(names, orderedNames[:])
	return names
}

// AllowedValues returns the canonical string values accepted by Parse.
func AllowedValues() []string {
	values := make([]string, len(orderedNames))
	for i, name := range orderedNames {
		values[i] = name.String()
	}
	return values
}

// Info returns the immutable metadata for a supported harness.
func Info(name Name) (Metadata, error) {
	info, ok := metadataByName[name]
	if !ok {
		return Metadata{}, fmt.Errorf("%w %q (want claude, codex, or copilot)", ErrUnknownHarness, name)
	}
	return info, nil
}

// Label returns the human-facing provider label, or an empty string for an
// invalid Name.
func (n Name) Label() string {
	info, _ := Info(n)
	return info.Label
}

// Executable returns the provider CLI executable, or an empty string for an
// invalid Name.
func (n Name) Executable() string {
	info, _ := Info(n)
	return info.Executable
}

// ConfigEnvVar returns the provider's native configuration environment
// variable, or an empty string for an invalid Name.
func (n Name) ConfigEnvVar() string {
	info, _ := Info(n)
	return info.ConfigEnvVar
}

// NativeConfigDir returns the provider's documented native directory default,
// or an empty string for an invalid Name.
func (n Name) NativeConfigDir() string {
	info, _ := Info(n)
	return info.NativeConfigDir
}
