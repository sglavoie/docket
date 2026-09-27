package outliner

import (
	"fmt"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/tui/status"
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
// The session value those helpers read lives on the shell; see
// advisories_shell.go for where the machine-local setting and the
// `--no-advisories` flag feed it.

// setAdvisoriesHidden injects the shell's setting, mirroring
// setOutstandingConflicts: a nil accessor is inert and reads as showing, which
// is what a struct-literal test model and the Hub's preview painter get.
func (m *OutlinerModel) SetAdvisoriesHidden(accessor func() bool) {
	m.advisoriesHiddenAccessor = accessor
}

// advisoriesSuppressed answers the two helpers below.
func (m *OutlinerModel) AdvisoriesSuppressed() bool {
	if m == nil || m.advisoriesHiddenAccessor == nil {
		return false
	}
	return m.advisoriesHiddenAccessor()
}

// advisoryLine gates one status-strip row. Returning the empty string is what
// keeps statusStripHeight and the strip itself measuring the same body, so a
// suppressed advisory costs no reserved row either.
func (m *OutlinerModel) AdvisoryLine(text string) string {
	if m.AdvisoriesSuppressed() {
		return ""
	}
	return text
}

// advisoryStatus gates one ambient status message. A nil Cmd is the whole
// suppression: the message is never sent, so it neither reaches the footer nor
// supersedes the line already there. An advisory that is shown is informational
// and therefore quiet — it says what the workspace holds and moves nothing.
func (m *OutlinerModel) AdvisoryStatus(text string) tea.Cmd {
	if text == "" || m.AdvisoriesSuppressed() {
		return nil
	}
	return func() tea.Msg { return status.InfoStatusMsg(text) }
}

// loadFailureAdvisory is the notice a partly-loaded population reports: the
// projects that did not read are workspace state, not an answer to a keystroke,
// so both the cross-project load and the lens load route their wording through
// this one helper and inherit the gate with it.
func (m *OutlinerModel) LoadFailureAdvisory(failures []ProjectLoadFailure) tea.Cmd {
	if len(failures) == 0 {
		return nil
	}
	noun := "project"
	if len(failures) > 1 {
		noun = "projects"
	}
	return m.AdvisoryStatus(fmt.Sprintf("%d %s failed to load: %s", len(failures), noun, failures[0].Detail))
}

// The outstanding-conflict indicator is workspace state, not an event: it is a
// line the status strip carries for as long as `.tt/conflicts/` holds a bundle,
// and it disappears once the CLI resolves the last one. The count itself is
// cached on the shell and injected as an accessor; see conflict_indicator_shell.go
// for where it is read.

// conflictIndicatorText is the strip's line for a given count. Zero renders
// nothing at all, so the strip keeps its height only while there is state to
// report.
func conflictIndicatorText(count int) string {
	switch {
	case count <= 0:
		return ""
	case count == 1:
		return "1 unresolved merge conflict — gc to review"
	default:
		return fmt.Sprintf("%d unresolved merge conflicts — gc to review", count)
	}
}

// setOutstandingConflicts injects the shell's conflict accessor. Like the note
// index it is a setter rather than a constructor parameter, because the Hub's
// preview painter builds an Outliner that must keep the nil accessor.
func (m *OutlinerModel) SetOutstandingConflicts(accessor func() int) {
	m.outstandingConflicts = accessor
}

// conflictIndicator is the strip's line for the injected count.
func (m *OutlinerModel) conflictIndicator() string {
	if m == nil || m.outstandingConflicts == nil {
		return ""
	}
	return conflictIndicatorText(m.outstandingConflicts())
}
