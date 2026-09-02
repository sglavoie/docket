package tui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"
)

// An advisory is ambient workspace state the user did not ask for: a line that
// appears because of what the workspace is, not because of a key that was just
// pressed. The outstanding-conflict count is the archetype — it stands until the
// CLI resolves the last bundle — and the launch-time "N projects failed to load"
// notice is the other one. Feedback from the user's own operations is not an
// advisory, and neither is the query readout, which only exists because `Q` was
// pressed.
//
// Advisories are opt-out, and the opt-out has one rule: every ambient line
// routes through advisoryLine (a strip row) or advisoryStatus (a status
// message). A later ambient row inherits `--no-advisories` and the View menu's
// toggle by using those two helpers, without anyone editing this file.
//
// The suppression is session state on the shell rather than a read of the
// machine file, because two inputs decide it: the machine-local flag the menu
// toggle writes, and the process-level `--no-advisories` flag, which forces
// advisories hidden for one run and writes nothing. The toggle commits, so
// toggling back after a flagged launch persists "shown" — the state a user can
// see is the state that was saved.

// advisoriesHidden reports the shell's live setting. A nil model reads as
// showing, so nothing here can turn a missing shell into a silent one.
func (m *AppModel) advisoriesHidden() bool {
	if m == nil {
		return false
	}
	return m.hideAdvisories
}

// setAdvisoriesHidden injects the shell's setting, mirroring
// setOutstandingConflicts: a nil accessor is inert and reads as showing, which
// is what a struct-literal test model and the Hub's preview painter get.
func (m *OutlinerModel) setAdvisoriesHidden(accessor func() bool) {
	m.advisoriesHiddenAccessor = accessor
}

// advisoriesSuppressed answers the two helpers below.
func (m *OutlinerModel) advisoriesSuppressed() bool {
	if m == nil || m.advisoriesHiddenAccessor == nil {
		return false
	}
	return m.advisoriesHiddenAccessor()
}

// advisoryLine gates one status-strip row. Returning the empty string is what
// keeps statusStripHeight and the strip itself measuring the same body, so a
// suppressed advisory costs no reserved row either.
func (m *OutlinerModel) advisoryLine(text string) string {
	if m.advisoriesSuppressed() {
		return ""
	}
	return text
}

// advisoryStatus gates one ambient status message. A nil Cmd is the whole
// suppression: the message is never sent, so it never arms the footer flash.
func (m *OutlinerModel) advisoryStatus(text string) tea.Cmd {
	if text == "" || m.advisoriesSuppressed() {
		return nil
	}
	return func() tea.Msg { return InfoStatusMsg(text) }
}

// loadFailureAdvisory is the notice a partly-loaded population reports: the
// projects that did not read are workspace state, not an answer to a keystroke,
// so both the cross-project load and the lens load route their wording through
// this one helper and inherit the gate with it.
func (m *OutlinerModel) loadFailureAdvisory(failures []projectLoadFailure) tea.Cmd {
	if len(failures) == 0 {
		return nil
	}
	noun := "project"
	if len(failures) > 1 {
		noun = "projects"
	}
	return m.advisoryStatus(fmt.Sprintf("%d %s failed to load: %s", len(failures), noun, failures[0].Detail))
}

// WithHiddenAdvisories starts the session with advisories suppressed. RunApp
// translates the `--no-advisories` flag into this option; the machine-local
// setting is read in NewAppModel, and either one hiding is enough.
func WithHiddenAdvisories() AppOption {
	return func(m *AppModel) { m.hideAdvisories = true }
}

// coreToggleAdvisories flips the machine-local setting and the session value
// together. It mirrors coreToggleMenuBar: chrome follows the terminal you are
// sitting at, so the durable home is machine.json rather than the workspace.
func (m *AppModel) coreToggleAdvisories() error {
	if m.runtime == nil || m.runtime.Machine() == nil {
		return fmt.Errorf("machine chrome state is unavailable")
	}
	machine := *m.runtime.Machine()
	machine.AdvisoriesHidden = !m.hideAdvisories
	if err := m.runtime.CommitMachine(&machine); err != nil {
		return fmt.Errorf("save advisory visibility: %w", err)
	}
	m.hideAdvisories = machine.AdvisoriesHidden
	if m.hideAdvisories {
		m.setInfoStatus("Advisories hidden")
	} else {
		m.setInfoStatus("Advisories shown")
	}
	return nil
}
