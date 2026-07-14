package tui

import (
	"fmt"
	"slices"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/scoring"
)

// startAddPreset computes available presets and enters picker mode or adds directly.
func (m *ScoreProfileManagerModel) startAddPreset() tea.Cmd {
	existing := make(map[string]bool)
	for _, p := range m.profiles {
		existing[p.ID] = true
	}

	var available []string
	for _, name := range scoring.PresetNames() {
		if !existing[name] {
			available = append(available, name)
		}
	}

	if len(available) == 0 {
		m.setStatus("All preset profiles already added")
		return defaultStatusTimeout()
	}

	if len(available) == 1 {
		return m.addPreset(available[0])
	}

	m.presetOptions = available
	m.presetCursor = 0
	m.pickingPreset = true
	return nil
}

// addPreset adds a preset profile by name.
func (m *ScoreProfileManagerModel) addPreset(name string) tea.Cmd {
	m.pickingPreset = false
	presets := scoring.PresetProfiles()
	factory, ok := presets[name]
	if !ok {
		return nil
	}

	profile := factory()
	automation := cloneAutomationConfig(m.runtime.Automation())
	if err := automation.AddScoringProfile(profile); err != nil {
		m.setStatus(err.Error())
		return defaultStatusTimeout()
	}

	if err := m.coordinator.SaveNow(automation); err != nil {
		m.setStatus("Failed to save: " + err.Error())
		return defaultStatusTimeout()
	}

	m.profiles = slices.Clone(m.runtime.Automation().ScoringProfiles)

	for i, p := range m.profiles {
		if p.ID == profile.ID {
			m.cursor = i
			break
		}
	}

	m.setStatus(fmt.Sprintf("Added %s profile", profile.Name))
	return tea.Batch(
		defaultStatusTimeout(),
		func() tea.Msg { return ScoreProfileChangedMsg{} },
	)
}

// executeDelete removes the profile at cursor.
func (m *ScoreProfileManagerModel) executeDelete() tea.Cmd {
	if m.cursor >= len(m.profiles) {
		return nil
	}

	profile := m.profiles[m.cursor]

	// RemoveScoringProfile also clears any project assignments that referenced
	// the deleted profile, so no separate assignment sweep is needed.
	automation := cloneAutomationConfig(m.runtime.Automation())
	automation.RemoveScoringProfile(profile.ID)
	if err := m.coordinator.SaveNow(automation); err != nil {
		m.setStatus("Failed to save: " + err.Error())
		return defaultStatusTimeout()
	}

	m.profiles = slices.Clone(m.runtime.Automation().ScoringProfiles)

	if m.cursor >= len(m.profiles) && m.cursor > 0 {
		m.cursor--
	}

	m.setStatus(fmt.Sprintf("Deleted profile %q", profile.Name))
	return tea.Batch(
		defaultStatusTimeout(),
		func() tea.Msg { return ScoreProfileChangedMsg{} },
	)
}

// assignProfile assigns the selected profile to the current project.
func (m *ScoreProfileManagerModel) assignProfile() tea.Cmd {
	if len(m.profiles) == 0 || m.cursor >= len(m.profiles) || m.currentProjectID == "" {
		return nil
	}

	profile := m.profiles[m.cursor]
	automation := cloneAutomationConfig(m.runtime.Automation())
	automation.SetProjectScoringProfile(m.currentProjectID, profile.ID)
	if err := m.coordinator.SaveNow(automation); err != nil {
		m.setStatus("Failed to save: " + err.Error())
		return defaultStatusTimeout()
	}

	m.setStatus(fmt.Sprintf("Assigned %s to project", profile.Name))
	return tea.Batch(
		defaultStatusTimeout(),
		func() tea.Msg { return ScoreProfileChangedMsg{} },
	)
}

// unassignProfile removes the scoring profile from the current project.
func (m *ScoreProfileManagerModel) unassignProfile() tea.Cmd {
	if m.currentProjectID == "" {
		return nil
	}

	automation := cloneAutomationConfig(m.runtime.Automation())
	automation.UnsetProjectScoringProfile(m.currentProjectID)
	if err := m.coordinator.SaveNow(automation); err != nil {
		m.setStatus("Failed to save: " + err.Error())
		return defaultStatusTimeout()
	}

	m.setStatus("Unassigned scoring profile from project")
	return tea.Batch(
		defaultStatusTimeout(),
		func() tea.Msg { return ScoreProfileChangedMsg{} },
	)
}
