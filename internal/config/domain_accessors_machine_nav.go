package config

import (
	"slices"
	"strings"
)

func (m *MachineState) findCustomContextKey(name string) (string, bool) {
	if m == nil || m.CustomContexts == nil {
		return "", false
	}
	for key := range m.CustomContexts {
		if strings.EqualFold(key, name) {
			return key, true
		}
	}
	return "", false
}

func (m *MachineState) GetCustomContextNames() []string {
	if m == nil || m.CustomContexts == nil {
		return []string{}
	}
	names := make([]string, 0, len(m.CustomContexts))
	for name := range m.CustomContexts {
		names = append(names, name)
	}
	slices.Sort(names)
	return names
}

func (m *MachineState) GetCustomContext(name string) []string {
	key, ok := m.findCustomContextKey(name)
	if !ok {
		return nil
	}
	return m.CustomContexts[key]
}

func (m *MachineState) SetCustomContext(name string, projectIDs []string) {
	if m.CustomContexts == nil {
		m.CustomContexts = make(map[string][]string)
	}
	if key, ok := m.findCustomContextKey(name); ok {
		delete(m.CustomContexts, key)
	}
	m.CustomContexts[name] = projectIDs
}

func (m *MachineState) DeleteCustomContext(name string) bool {
	key, ok := m.findCustomContextKey(name)
	if !ok {
		return false
	}
	delete(m.CustomContexts, key)
	return true
}

func (m *MachineState) CustomContextExists(name string) bool {
	_, ok := m.findCustomContextKey(name)
	return ok
}

func (m *MachineState) IsProjectInCustomContext(name, projectID string) bool {
	return slices.Contains(m.GetCustomContext(name), projectID)
}
