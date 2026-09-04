package tui

import (
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/tui/status"
)

// periodicSaveInterval is the cadence of the one recurring app timer. It is the
// span a scenario advances to observe something only the tick can produce, such
// as an outstanding-conflict count another machine's sync wrote.
const periodicSaveInterval = 60 * time.Second

// lifecycleController is the shell's session-lifecycle cluster: the one message
// the composition root arms for the first frame, and the flag a quit sets so the
// final paint renders nothing.
//
// It is the smallest cluster on AppModel deliberately. A session has exactly two
// pieces of state between "constructed" and "gone": what it still owes the first
// frame, and whether it is on the way out. Everything else about the lifecycle is
// behavior, not state — RunApp acquires the process lock and owns the panic
// recovery (app_run.go), NewAppModel runs the fifteen-step composition
// (app_init.go), and teardown is the deferred lock release beside the Bubble Tea
// program. None of that is retained, so none of it belongs to a field.
//
// It also owns the scheduling of the one recurring timer, because the two places
// that arm it — Init and the handler that re-arms it on each fire — must arm the
// same one. App timers go through status.TickCmd and never tea.Tick, so the tick
// stays virtualizable by the scenario harness; keeping the construction here is
// what stops a second, differently-scheduled periodic save from appearing.
type lifecycleController struct {
	// startup is the message the composition root owes the first frame. It is
	// held rather than sent because NewAppModel returns a model, not a command:
	// Init is the first place a tea.Cmd can exist.
	startup tea.Msg

	// quitting records that the session is on the way out, so the final View
	// paints an empty frame instead of a torn one over the restored terminal.
	quitting bool
}

// startupCmd returns the command that delivers the armed startup message, or nil
// when nothing was armed. The message is captured by value so a later write to
// the field cannot change what this command already promised to deliver.
func (c *lifecycleController) startupCmd() tea.Cmd {
	if c == nil || c.startup == nil {
		return nil
	}
	msg := c.startup
	return func() tea.Msg { return msg }
}

// periodicSaveCmd schedules the next periodic tick. It is the single
// construction of the app's one recurring timer: Init arms it, and the handler
// for each fire re-arms it through this same call.
func (c *lifecycleController) periodicSaveCmd() tea.Cmd {
	return status.TickCmd(periodicSaveInterval, func(at time.Time) tea.Msg {
		return status.PeriodicSaveMsg{At: at}
	})
}
