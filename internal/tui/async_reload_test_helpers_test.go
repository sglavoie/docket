package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"
)

func runSession2AppCommand(t *testing.T, app *AppModel, cmd tea.Cmd) {
	t.Helper()
	queue := []tea.Cmd{cmd}
	for steps := 0; len(queue) > 0; steps++ {
		if steps > 200 {
			t.Fatal("command queue did not settle")
		}
		next := queue[0]
		queue = queue[1:]
		if next == nil {
			continue
		}
		msg := next()
		if msg == nil {
			continue
		}
		if batch, ok := msg.(tea.BatchMsg); ok {
			queue = append(queue, batch...)
			continue
		}
		_, followup := app.Update(msg)
		if followup != nil {
			queue = append(queue, followup)
		}
	}
}
