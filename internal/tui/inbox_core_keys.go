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

func (c *inboxCore) handlePrefixKey(key string) (tea.Model, tea.Cmd) {
	prefix := c.pendingPrefix
	c.pendingPrefix = ""

	if key == "esc" {
		return c.model, nil
	}

	switch prefix {
	case ",":
		if status, ok := StatusForPrefixKey(key); ok {
			return c.model, c.changeTaskStatus(status, StatusVerb(status))
		}
	case "g":
		if key == "g" {
			c.currentIndex = 0
			return c.model, nil
		}
	case "!":
		if priority, ok := PriorityForPrefixKey(key); ok {
			return c.model, c.changePriority(priority)
		}
	case "s":
		if key == "d" {
			return c.model, c.openSnoozePicker()
		}
		snoozeDate, _, ok := resolveQuickSnoozeDate(key)
		if !ok {
			return c.model, nil
		}
		return c.model, c.setSnooze(snoozeDate)
	case ".":
		stage := StageForPrefixKey(key, c.config.Stages)
		if stage != nil {
			return c.model, c.setStage(*stage)
		}
	case "m":
		section, ok := sectionForMoveKey(key)
		if ok {
			return c.model, c.moveToSection(section)
		}
	}

	return c.model, nil
}
