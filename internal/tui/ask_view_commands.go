package tui

import (
	"os/exec"
	"strings"

	tea "charm.land/bubbletea/v2"
)

func (m *AskViewModel) applyCommands() tea.Cmd {
	commands := make([]string, len(m.commands))
	copy(commands, m.commands)
	m.commands = nil

	return func() tea.Msg {
		var results []AskCommandResultMsg
		for _, c := range commands {
			parts := strings.Fields(c)
			if len(parts) < 2 {
				continue
			}
			cmd := exec.Command(parts[0], parts[1:]...)
			out, err := cmd.CombinedOutput()
			results = append(results, AskCommandResultMsg{Command: c, Output: string(out), Err: err})
		}
		if len(results) == 0 {
			return nil
		}
		return AskCommandBatchMsg{Results: results}
	}
}
