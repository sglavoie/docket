package tui

import (
	"time"

	tea "charm.land/bubbletea/v2"
)

// tickScheduler is an optional, package-private scheduling seam used by the
// scenario harness, which owns a virtual clock so a scenario's outcome does not
// depend on how fast the machine running it happens to be. Normal application
// scheduling leaves it nil.
var tickScheduler func(delay time.Duration, fire func(time.Time) tea.Msg) tea.Cmd

// tickCmd is the single place the app schedules a delayed message. Every timer
// must go through it, or the harness cannot see it and falls back to waiting on
// the wall clock.
func tickCmd(delay time.Duration, fire func(time.Time) tea.Msg) tea.Cmd {
	if tickScheduler != nil {
		return tickScheduler(delay, fire)
	}
	return tea.Tick(delay, fire)
}
