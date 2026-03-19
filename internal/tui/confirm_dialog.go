package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// ConfirmDialogProvider is implemented by view models that have a confirm dialog.
type ConfirmDialogProvider interface {
	GetConfirmDialog() *ConfirmDialog
}

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
	return "Are you sure?"
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

// Render returns empty string. Confirm dialogs are rendered as floating
// overlays via RenderOverlay, called from AppModel.View().
func (c *ConfirmDialog) Render() string {
	return ""
}

// RenderOverlay renders the confirm dialog as a floating bordered popup box.
// Uses orange (ColorWarning) borders to distinguish from blue informational popups.
func (c *ConfirmDialog) RenderOverlay(width int) string {
	if !c.IsActive() {
		return ""
	}

	borderStyle := WarningIconStyle

	// Render prompt text inside a bordered box
	promptText := c.Prompt()

	// Build box using lipgloss border
	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorWarning).
		Padding(0, 1)

	// Title and footer text (computed early so we can size the box to fit)
	titleStyle := lipgloss.NewStyle().Foreground(ColorWarning).Bold(true)
	title := titleStyle.Render("Confirm")
	footerText := MutedStyle.Render("y confirm · n/Esc cancel")

	// Minimum inner width must accommodate title and footer border chrome.
	// renderTitledTopBorder/renderTitledBottomBorder use e.g. "╭─ " + text + " ─╮"
	// which is 5 chars of decoration + text visual width + at least 1 dash,
	// minus 2 for the lipgloss border columns that are part of actualWidth
	// but not innerWidth.
	titleMinInner := lipgloss.Width(title) + 6 - 2
	footerMinInner := lipgloss.Width(footerText) + 6 - 2
	minInner := max(titleMinInner, footerMinInner)

	// Size the box: fit prompt with padding, but cap at half the screen width
	promptWidth := lipgloss.Width(promptText)
	innerWidth := promptWidth + 2 // 1 padding each side
	maxWidth := width / 2
	if maxWidth < 30 {
		maxWidth = width - 4
	}
	if innerWidth > maxWidth {
		innerWidth = maxWidth
	}
	if innerWidth < minInner {
		innerWidth = minInner
	}
	boxStyle = boxStyle.Width(innerWidth)

	rendered := boxStyle.Render(promptText)

	// Split into lines to replace top and bottom borders
	lines := strings.Split(rendered, "\n")
	if len(lines) < 2 {
		return rendered
	}

	actualWidth := lipgloss.Width(lines[0])
	lines[0] = renderTitledTopBorder(title, actualWidth, borderStyle)
	lines[len(lines)-1] = renderTitledBottomBorder(footerText, actualWidth, borderStyle)

	return strings.Join(lines, "\n")
}

// ConfirmResult is returned when a confirmation is resolved.
type ConfirmResult struct {
	Action    string
	Confirmed bool
}
