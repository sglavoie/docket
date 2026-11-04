package pulse

import tea "charm.land/bubbletea/v2"

// TerminalProgress is the pulse as the terminal's own progress indicator
// (OSC 9;4, drawn by Ghostty, Windows Terminal, and others in the tab): the
// day's completions against what Today still holds, so it fills as Today
// empties. It exists only while Today holds open work, since a bar standing
// full or empty all day says nothing; an overdue task turns it to the warning
// state at the same value. Terminals without support ignore the sequence.
func TerminalProgress(pulse WorkspacePulse) *tea.ProgressBar {
	if pulse.Today <= 0 {
		return nil
	}
	done := max(pulse.Done, 0)
	value := done * 100 / (done + pulse.Today)
	state := tea.ProgressBarDefault
	if pulse.Overdue > 0 {
		state = tea.ProgressBarWarning
	}
	return tea.NewProgressBar(state, value)
}
