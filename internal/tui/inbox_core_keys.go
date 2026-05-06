package tui

import tea "charm.land/bubbletea/v2"

func (c *inboxCore) handleKeyMsg(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	if confirmed, handled := c.confirm.HandleKey(key); handled {
		if confirmed {
			return c.model, c.executeDelete()
		}
		c.statusMessage = ""
		return c.model, nil
	}

	if c.pendingPrefix != "" && isModifierOnly(msg) {
		return c.model, nil
	}
	if c.pendingPrefix != "" {
		return c.handlePrefixKey(key)
	}

	c.err = nil

	if prefix, ok := activateSecondaryPrefix(key, "g", ",!sm.", true); ok {
		c.pendingPrefix = prefix
		return c.model, nil
	}

	switch key {
	case "esc":
		return c.model, func() tea.Msg {
			return c.backend.closeMsg(len(c.processedIDs), len(c.skippedIDs))
		}

	case "n":
		return c.model, c.skipCurrent()

	case "j", "down":
		return c.model, c.advanceToNext(false)

	case "k", "up":
		c.goToPrevious()
		return c.model, nil

	case "G":
		if c.backend.taskCount() > 0 {
			c.currentIndex = c.backend.taskCount() - 1
		}
		return c.model, nil

	case "1", "2", "3", "4", "5":
		return c.model, c.setPriorityQuick(key)

	case "e":
		return c.model, inboxOpenTaskEditor(c)

	case "D":
		inboxDeletePrompt(c, &c.confirm)
		return c.model, nil

	case "ctrl+w":
		return c.model, c.sendToWaiting()

	case "ctrl+s":
		return c.model, inboxOpenSectionPicker(c)

	case "ctrl+t":
		return c.model, inboxOpenTagSelector(c)

	case "u":
		return c.model, c.undoLastAction()

	case "enter":
		task, project := c.currentTaskRef()
		if task != nil {
			return c.model, func() tea.Msg {
				return ShowTaskDetailMsg{Task: task, Project: project}
			}
		}
		return c.model, nil

	case "?":
		return c.model, helpCmd(c.backend.viewType(), c.backend.helpViewName(), c.GetHelpItems())
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
