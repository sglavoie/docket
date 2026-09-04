package tui

import "fmt"

// The shell half of the advisory opt-out. advisories.go owns what an advisory
// is and the two helpers every ambient line routes through; this file owns the
// session value those helpers read, which is where the machine-local setting and
// the process-level `--no-advisories` flag meet.
//
// The suppression is session state on the shell rather than a read of the
// machine file, because two inputs decide it: the machine-local flag the menu
// toggle writes, and the process-level `--no-advisories` flag, which forces
// advisories hidden for one run and writes nothing. The toggle commits, so
// toggling back after a flagged launch persists "shown" — the state a user can
// see is the state that was saved.

// The session value itself lives on the feedback cluster beside the conflict
// count, because both are ambient status-strip inputs and both travel into an
// Outliner as the same kind of injected accessor; see feedback_controller.go.

// WithHiddenAdvisories starts the session with advisories suppressed. RunApp
// translates the `--no-advisories` flag into this option; the machine-local
// setting is read in NewAppModel, and either one hiding is enough.
func WithHiddenAdvisories() AppOption {
	return func(m *AppModel) { m.feedback.hideAdvisories = true }
}

// coreToggleAdvisories flips the machine-local setting and the session value
// together. It mirrors coreToggleMenuBar: chrome follows the terminal you are
// sitting at, so the durable home is machine.json rather than the workspace.
func (m *AppModel) coreToggleAdvisories() error {
	if m.runtime == nil || m.runtime.Machine() == nil {
		return fmt.Errorf("machine chrome state is unavailable")
	}
	machine := *m.runtime.Machine()
	machine.AdvisoriesHidden = !m.feedback.hideAdvisories
	if err := m.runtime.CommitMachine(&machine); err != nil {
		return fmt.Errorf("save advisory visibility: %w", err)
	}
	m.feedback.hideAdvisories = machine.AdvisoriesHidden
	if m.feedback.hideAdvisories {
		m.setInfoStatus("Advisories hidden")
	} else {
		m.setInfoStatus("Advisories shown")
	}
	return nil
}
