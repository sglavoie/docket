package status

import (
	"time"

	tea "charm.land/bubbletea/v2"
)

// TickScheduler is an optional scheduling seam used by the scenario harness,
// which owns a virtual clock so a scenario's outcome does not depend on how
// fast the machine running it happens to be. It is exported only because the
// harness lives in the shell package; nothing in production writes it, and
// normal application scheduling leaves it nil.
var TickScheduler func(delay time.Duration, fire func(time.Time) tea.Msg) tea.Cmd

// TickCmd is the single place the app schedules a delayed message. Every timer
// must go through it, or the harness cannot see it and falls back to waiting on
// the wall clock.
func TickCmd(delay time.Duration, fire func(time.Time) tea.Msg) tea.Cmd {
	if TickScheduler != nil {
		return TickScheduler(delay, fire)
	}
	return tea.Tick(delay, fire)
}
