package config

import (
	"fmt"
	"maps"

	"github.com/sglavoie/tt/internal/aiharness"
)

// GetAIHarness returns the remembered harness, defaulting unset or invalid
// in-memory state to Claude. Manager load paths validate persisted machine
// state before returning it for normal use.
func (m *MachineState) GetAIHarness() aiharness.Name {
	if m != nil {
		if name, err := aiharness.Parse(m.AIHarness); err == nil {
			return name
		}
	}
	return aiharness.Default
}

// SetAIHarness validates and stores one supported harness preference.
func (m *MachineState) SetAIHarness(name aiharness.Name) error {
	if m == nil {
		return fmt.Errorf("machine state is required")
	}
	if !name.Valid() {
		return fmt.Errorf("set AI harness: %w %q", aiharness.ErrUnknownHarness, name)
	}
	m.AIHarness = name.String()
	return nil
}

// GetAIConfigDirs returns a defensive copy of the explicit per-harness
// overrides. Mutating the returned map never changes machine state.
func (m *MachineState) GetAIConfigDirs() map[string]string {
	if m == nil {
		return nil
	}
	return maps.Clone(m.AIConfigDirs)
}

// GetAIConfigDir returns the explicit override for name, or an empty string
// when none is remembered.
func (m *MachineState) GetAIConfigDir(name aiharness.Name) string {
	if m == nil || !name.Valid() {
		return ""
	}
	return m.AIConfigDirs[name.String()]
}

// SetAIConfigDir normalizes and validates one explicit override. An empty path
// clears the override so only explicit non-empty values are persisted.
func (m *MachineState) SetAIConfigDir(name aiharness.Name, path string) error {
	if m == nil {
		return fmt.Errorf("machine state is required")
	}
	if !name.Valid() {
		return fmt.Errorf("set AI config directory: %w %q", aiharness.ErrUnknownHarness, name)
	}
	if path == "" {
		delete(m.AIConfigDirs, name.String())
		if len(m.AIConfigDirs) == 0 {
			m.AIConfigDirs = nil
		}
		return nil
	}
	normalized, err := aiharness.NormalizeExplicitConfigDir(path)
	if err != nil {
		return err
	}
	if m.AIConfigDirs == nil {
		m.AIConfigDirs = make(map[string]string)
	}
	m.AIConfigDirs[name.String()] = normalized
	return nil
}

// SetAIConfigDirs replaces all explicit overrides atomically after validating
// their harness keys and paths. The input map is never retained.
func (m *MachineState) SetAIConfigDirs(dirs map[string]string) error {
	if m == nil {
		return fmt.Errorf("machine state is required")
	}
	normalized := make(map[string]string, len(dirs))
	for rawName, path := range dirs {
		name, err := aiharness.Parse(rawName)
		if err != nil {
			return fmt.Errorf("set AI config directories: %w", err)
		}
		if path == "" {
			continue
		}
		normalizedPath, err := aiharness.NormalizeExplicitConfigDir(path)
		if err != nil {
			return fmt.Errorf("set %s AI config directory: %w", name, err)
		}
		normalized[name.String()] = normalizedPath
	}
	if len(normalized) == 0 {
		m.AIConfigDirs = nil
		return nil
	}
	m.AIConfigDirs = normalized
	return nil
}
