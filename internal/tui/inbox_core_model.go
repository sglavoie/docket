package tui

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// IsFullHeight returns true so the inbox processor controls its own height and footer.
func (c *inboxCore) IsFullHeight() bool { return true }

// Init initializes the model.
func (c *inboxCore) Init() tea.Cmd {
	return nil
}

// ShouldCaptureEsc returns true when prefix mode or confirm dialog is active.
func (c *inboxCore) ShouldCaptureEsc() bool {
	return c.DefaultShouldCaptureEsc()
}

// Update handles messages for the inbox processor.
func (c *inboxCore) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		return c.handleKeyMsg(msg)

	case SectionPickerDoneMsg:
		return c.model, c.handleSectionPickerDone(msg)

	case TaskUpdatedMsg:
		return c.model, inboxHandleTaskUpdated(c)

	case TaskPropertyChangedMsg:
		return c.model, inboxHandleTaskUpdated(c)

	case DueDateChangedMsg:
		return c.model, inboxHandleDueDateChanged(c)

	case DependencySelectorDoneMsg:
		return c.model, inboxHandleDependencySelectorDone(c)

	case TagSelectorDoneMsg:
		_, cmd := inboxHandleTagSelectorDone(c, msg)
		return c.model, cmd

	case TaskCreatedMsg:
		c.statusMessage = "Subtask created"
		return c.model, c.backend.reloadCmd()

	case ErrorMsg:
		c.err = msg.Err
		return c.model, nil

	default:
		if matched, err := c.backend.handleSavedMsg(msg); matched {
			if err != nil {
				c.err = err
				return c.model, nil
			}
			c.backend.rebuildTasks()
			c.realignCurrentIndex()
			return c.model, nil
		}
	}

	return c.model, nil
}

// View renders the inbox processor.
func (c *inboxCore) View() tea.View {
	if c.backend.taskCount() == 0 {
		return tea.NewView(c.renderEmptyInbox())
	}

	var b strings.Builder

	b.WriteString(c.renderHeader())
	b.WriteString("\n")

	task, _ := c.currentTaskRef()
	if task != nil {
		b.WriteString(c.renderTaskDisplay())
	} else {
		b.WriteString(c.renderEmptyState())
	}

	b.WriteString("\n")
	if c.height <= minTerminalHeight {
		b.WriteString(renderInboxOperationsPanelCompact(c.width))
	} else {
		b.WriteString(renderInboxOperationsPanel())
	}
	// Single newline instead of \n\n keeps the global inbox under the 80x24
	// height ceiling. The footer already prefixes its own horizontal rule, so
	// dropping the blank line does not hurt the visual separation.
	b.WriteString("\n")
	b.WriteString(renderInboxFooter(c.width))

	if c.statusMessage != "" {
		b.WriteString("\n")
		b.WriteString(SuccessStyle.Render(c.statusMessage))
	}

	if c.err != nil {
		b.WriteString("\n")
		b.WriteString(ValidationErrorStyle.Render(c.err.Error()))
	}

	output := b.String()
	if outputHeight := lipgloss.Height(output); outputHeight < c.height {
		output += strings.Repeat("\n", c.height-outputHeight)
	}
	return tea.NewView(output)
}

// GetPrefixPopupInfo returns prefix popup data for overlay rendering.
func (c *inboxCore) GetPrefixPopupInfo() *PrefixPopupInfo {
	if c.Dispatcher != nil && c.Dispatcher.HasPendingPrefix() {
		prefix := c.Dispatcher.PendingPrefix()
		var hints []PrefixHint
		for _, h := range c.Dispatcher.PrefixHints() {
			hints = append(hints, PrefixHint{Key: h.Key, Description: h.Description})
		}
		return &PrefixPopupInfo{
			Prefix: prefix,
			Hints:  PrefixHints{prefix: hints},
		}
	}
	if c.pendingPrefix == "" {
		return nil
	}
	var currentPriority int
	if task, _ := c.currentTaskRef(); task != nil {
		currentPriority = task.Priority
	}
	hints := CommonTaskActionPrefixHints(currentPriority)
	hints["g"] = NavigationPrefixHints
	// In Inbox, ss (double-tap s) opens the section picker
	hints["s"] = []PrefixHint{{Key: "s", Description: "section", Detail: "section picker"}}
	return &PrefixPopupInfo{
		Prefix: c.pendingPrefix,
		Hints:  hints,
	}
}

// GetHelpItems returns help items for this view.
func (c *inboxCore) GetHelpItems() []HelpItem {
	return GetKeybindingRegistry().GetForView(c.backend.viewType())
}
