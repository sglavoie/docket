package tui

import (
	"fmt"

	"charm.land/bubbles/v2/textarea"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/aiharness"
)

func (m *AskViewModel) syncConfigDraft() {
	if m.configDrafts == nil {
		m.configDrafts = make(map[aiharness.Name]string, len(aiharness.Names()))
	}
	m.configDir = m.configInput.Value()
	m.configDrafts[m.harness] = m.configDir
}

func (m *AskViewModel) focusAskField(index int) tea.Cmd {
	m.textarea.Blur()
	m.configInput.Blur()
	index = (index%askFocusCount + askFocusCount) % askFocusCount
	m.focusIndex = index
	switch index {
	case askFocusQuery:
		m.textarea.Focus()
		return textarea.Blink
	case askFocusConfigDir:
		m.configInput.Focus()
		return textinput.Blink
	default:
		return nil
	}
}

func (m *AskViewModel) cycleAskFocus(delta int) tea.Cmd {
	m.syncConfigDraft()
	return m.focusAskField(m.focusIndex + delta)
}

func (m *AskViewModel) changeHarness(delta int) {
	m.syncConfigDraft()
	names := aiharness.Names()
	selected := 0
	for i, name := range names {
		if name == m.harness {
			selected = i
			break
		}
	}
	selected = (selected%len(names) + delta + len(names)) % len(names)
	m.harness = names[selected]
	m.configDir = m.configDrafts[m.harness]
	m.configInput.SetValue(m.configDir)
}

// beginAskSubmission validates the selected directory before scheduling the
// machine-preference write. Provider consent and invocation resume only from a
// successful AskPreferencesSavedMsg.
func (m *AskViewModel) beginAskSubmission(query string) tea.Cmd {
	m.syncConfigDraft()
	draft := m.configDrafts[m.harness]
	if draft != "" {
		normalized, err := aiharness.NormalizeExplicitConfigDir(draft)
		if err != nil {
			m.inputError = renderAskError("Invalid config directory", err)
			m.configDir = draft
			return m.focusAskField(askFocusConfigDir)
		}
		m.configDrafts[m.harness] = normalized
		m.configDir = normalized
		m.configInput.SetValue(normalized)
	}

	m.pendingQuery = query
	m.inputError = ""
	m.state = askStateSavingPreferences
	m.textarea.Blur()
	m.configInput.Blur()

	operationID := nextAskOperationID()
	m.preferenceSaveID = operationID
	harness := m.harness
	drafts := cloneAskConfigDrafts(m.configDrafts)
	runtime := m.runtime
	return func() tea.Msg {
		if runtime == nil {
			return AskPreferencesSavedMsg{OperationID: operationID, Err: fmt.Errorf("runtime state is unavailable")}
		}
		domains, err := runtime.CloneDomains()
		if err != nil {
			return AskPreferencesSavedMsg{OperationID: operationID, Err: err}
		}
		if err := domains.Machine.SetAIHarness(harness); err != nil {
			return AskPreferencesSavedMsg{OperationID: operationID, Err: err}
		}

		normalizedDrafts := cloneAskConfigDrafts(drafts)
		for _, name := range aiharness.Names() {
			draft := drafts[name]
			if err := domains.Machine.SetAIConfigDir(name, draft); err != nil {
				// Only the selected draft is required to be valid before this
				// request. Keep an invalid draft for another harness unsaved so
				// the user can return to it without clobbering its last valid
				// machine override.
				if name == harness {
					return AskPreferencesSavedMsg{OperationID: operationID, Err: err}
				}
				continue
			}
			normalizedDrafts[name] = domains.Machine.GetAIConfigDir(name)
		}

		if runtime.DataDir() == "" {
			runtime.SetMachineInMemory(domains.Machine)
		} else if err := runtime.CommitMachine(domains.Machine); err != nil {
			return AskPreferencesSavedMsg{OperationID: operationID, Err: err}
		}
		return AskPreferencesSavedMsg{
			OperationID: operationID,
			Harness:     harness,
			ConfigDir:   domains.Machine.GetAIConfigDir(harness),
			Drafts:      normalizedDrafts,
		}
	}
}

func cloneAskConfigDrafts(source map[aiharness.Name]string) map[aiharness.Name]string {
	cloned := make(map[aiharness.Name]string, len(aiharness.Names()))
	for _, name := range aiharness.Names() {
		cloned[name] = source[name]
	}
	return cloned
}
