package tui

import tea "github.com/charmbracelet/bubbletea"

// ConfirmDialog provides reusable confirmation prompt functionality.
// Embed this in models that need yes/no confirmation for destructive actions.
type ConfirmDialog struct {
	action string // e.g., "delete", "archive", "bulk-delete"
	prompt string // custom prompt text to display
}

// Show activates the confirmation dialog with the given action and prompt.
func (c *ConfirmDialog) Show(action, prompt string) {
	c.action = action
	c.prompt = prompt
}

// Cancel deactivates the confirmation dialog.
func (c *ConfirmDialog) Cancel() {
	c.action = ""
	c.prompt = ""
}

// IsActive returns true if a confirmation is pending.
func (c *ConfirmDialog) IsActive() bool {
	return c.action != ""
}

// Action returns the current pending action.
func (c *ConfirmDialog) Action() string {
	return c.action
}

// Prompt returns the prompt text, or a default if not set.
func (c *ConfirmDialog) Prompt() string {
	if c.prompt != "" {
		return c.prompt
	}
	return "Are you sure? (y/n)"
}

// HandleKey processes a key press while confirmation is active.
// Returns: confirmed (true if user pressed y/Y), handled (true if key was processed)
func (c *ConfirmDialog) HandleKey(key string) (confirmed, handled bool) {
	if !c.IsActive() {
		return false, false
	}

	switch key {
	case "y", "Y":
		action := c.action
		c.Cancel()
		return action != "", true
	case "n", "N", "esc":
		c.Cancel()
		return false, true
	}
	// Block other keys while confirmation is active
	return false, true
}

// Render returns the styled confirmation prompt string.
func (c *ConfirmDialog) Render() string {
	if !c.IsActive() {
		return ""
	}
	return ConfirmStyle.Render(c.Prompt())
}

// ConfirmResult is returned when a confirmation is resolved.
type ConfirmResult struct {
	Action    string
	Confirmed bool
}

// HandleKeyMsg is a convenience wrapper for tea.KeyMsg.
func (c *ConfirmDialog) HandleKeyMsg(msg tea.KeyMsg) (confirmed, handled bool) {
	return c.HandleKey(msg.String())
}
