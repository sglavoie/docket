package status

import (
	"fmt"

	tea "charm.land/bubbletea/v2"
)

// ErrorCmd creates a tea.Cmd that returns an ErrorMsg with a formatted error.
func ErrorCmd(format string, args ...any) tea.Cmd {
	return func() tea.Msg {
		return ErrorMsg{Err: fmt.Errorf(format, args...)}
	}
}
