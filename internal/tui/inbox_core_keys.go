package tui

import tea "charm.land/bubbletea/v2"

func (c *inboxCore) handleKeyMsg(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	c.err = nil

	if c.Dispatcher != nil {
		if result, cmd, handled := c.Dispatcher.Dispatch(msg); handled {
			return result, cmd
		}
	}

	key := msg.String()
	if key == "esc" {
		return c.model, func() tea.Msg {
			return c.backend.closeMsg(len(c.processedIDs), len(c.skippedIDs))
		}
	}

	return c.model, nil
}
