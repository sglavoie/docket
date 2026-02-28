package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
)

// errorCmd creates a tea.Cmd that returns an ErrorMsg with a formatted error.
func errorCmd(format string, args ...any) tea.Cmd {
	return func() tea.Msg {
		return ErrorMsg{Err: fmt.Errorf(format, args...)}
	}
}
