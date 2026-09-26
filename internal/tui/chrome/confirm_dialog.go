package chrome

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/icons"
	"github.com/sglavoie/tt/internal/tui/theme"
)

// ConfirmDialogProvider is implemented by view models that have a confirm dialog.
type ConfirmDialogProvider interface {
	GetConfirmDialog() *ConfirmDialog
}

// ConfirmDetail holds optional rich content shown below the confirm prompt.
type ConfirmDetail struct {
	Items       []string // task names to list (up to 5)
	TotalCount  int      // total items; if > len(Items), shows "and N more"
	CascadeNote string   // e.g., "This epic has 3 subtasks that will also be deleted"
	UndoHint    bool     // show "Undo available (u)"
}

// ConfirmDialog provides reusable confirmation prompt functionality.
// Embed this in models that need yes/no confirmation for destructive actions.
type ConfirmDialog struct {
	action string         // e.g., "delete", "archive", "bulk-delete"
	prompt string         // custom prompt text to display
	Detail *ConfirmDetail // optional rich detail shown below prompt
}

// Show activates the confirmation dialog with the given action and prompt.
func (c *ConfirmDialog) Show(action, prompt string) {
	c.action = action
	c.prompt = prompt
	c.Detail = nil
}

// ShowWithDetail activates the confirmation dialog with rich detail content.
func (c *ConfirmDialog) ShowWithDetail(action, prompt string, detail *ConfirmDetail) {
	c.action = action
	c.prompt = prompt
	c.Detail = detail
}

// Cancel deactivates the confirmation dialog.
func (c *ConfirmDialog) Cancel() {
	c.action = ""
	c.prompt = ""
	c.Detail = nil
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

// guidanceSpec is the answer line the box draws. The unsaved-draft question is
// the one confirmation whose "no" keeps working rather than cancelling, and the
// footer under it already says so, so the box spells that same line instead of
// a second, generic one beside it.
func (c *ConfirmDialog) guidanceSpec() GuidanceSpec {
	if c.action == DiscardConfirmAction {
		return DiscardGuidanceSpec()
	}
	return ConfirmationGuidanceSpec()
}

// RenderOverlay renders the confirm dialog as a floating bordered popup box.
// Uses orange (ColorWarning) borders to distinguish from blue informational popups.
func (c *ConfirmDialog) RenderOverlay(width int) string {
	if !c.IsActive() {
		return ""
	}

	borderStyle := theme.WarningIconStyle

	// Render prompt text inside a bordered box
	promptText := c.Prompt()

	// Build detail section if present.
	// Track max plain-text width separately from styled lines so the box
	// is sized based on visual widths, avoiding ANSI-aware wrapping bugs.
	var detailLines []string
	maxPlainWidth := lipgloss.Width(promptText)
	if c.Detail != nil {
		d := c.Detail
		for _, item := range d.Items {
			plain := "  " + item
			if w := lipgloss.Width(plain); w > maxPlainWidth {
				maxPlainWidth = w
			}
			detailLines = append(detailLines, theme.MutedStyle.Render(plain))
		}
		if d.TotalCount > len(d.Items) {
			remaining := d.TotalCount - len(d.Items)
			plain := fmt.Sprintf("  ...and %d more", remaining)
			if w := lipgloss.Width(plain); w > maxPlainWidth {
				maxPlainWidth = w
			}
			detailLines = append(detailLines, theme.MutedStyle.Render(plain))
		}
		if d.CascadeNote != "" {
			plain := "  " + icons.Get("warning") + " " + d.CascadeNote
			if w := lipgloss.Width(plain); w > maxPlainWidth {
				maxPlainWidth = w
			}
			detailLines = append(detailLines, theme.WarningIconStyle.Render(plain))
		}
		if d.UndoHint {
			plain := "  Undo available (u)"
			if w := lipgloss.Width(plain); w > maxPlainWidth {
				maxPlainWidth = w
			}
			detailLines = append(detailLines, theme.MutedStyle.Render(plain))
		}
	}

	// Combine prompt and detail into box content
	content := promptText
	if len(detailLines) > 0 {
		content = promptText + "\n\n" + strings.Join(detailLines, "\n")
	}

	// Title and footer text (computed early so we can size the box to fit)
	titleStyle := lipgloss.NewStyle().Foreground(theme.ColorWarning).Bold(true)
	title := titleStyle.Render("Confirm")
	footerText := RenderGuidanceText(c.guidanceSpec(), max(width-8, 1))

	// Minimum Width() value so titled borders fit (╭─ text ─…─╮ needs
	// textWidth + 5 decoration chars + at least 1 dash = textWidth + 6).
	// Width() in lipgloss v2 = total rendered width including border+padding.
	titleMinWidth := lipgloss.Width(title) + 6
	footerMinWidth := lipgloss.Width(footerText) + 6
	minWidth := max(titleMinWidth, footerMinWidth)

	// Size the box to fit widest content line.
	// Width(W) wraps content at W - 2(border) - 2(padding) = W - 4.
	innerWidth := maxPlainWidth + 4 // 2 border + 2 padding
	maxWidth := width / 2
	if maxWidth < 30 {
		maxWidth = width - 4
	}
	if innerWidth > maxWidth {
		innerWidth = maxWidth
	}
	if innerWidth < minWidth {
		innerWidth = minWidth
	}
	// Reserve one terminal cell outside the one-cell halo on each side.
	innerWidth = min(innerWidth, max(width-4, 1))

	return RenderOverlayBox(content, innerWidth, title, footerText, borderStyle)
}

// DiscardConfirmAction is the action every unsaved-draft question is shown
// under, which is how the box knows to answer it with DiscardGuidanceSpec.
const DiscardConfirmAction = "discard"

// DiscardConfirmState bundles the ConfirmDiscard flag with a ConfirmDialog and
// provides a standard GetConfirmDialog that shows "Discard unsaved changes?"
// when ConfirmDiscard is true. Embed in view models to satisfy ConfirmDialogProvider.
type DiscardConfirmState struct {
	ConfirmDiscard bool
	ConfirmOverlay ConfirmDialog
}

func (s *DiscardConfirmState) GetConfirmDialog() *ConfirmDialog {
	if s.ConfirmDiscard {
		s.ConfirmOverlay.Show(DiscardConfirmAction, "Discard unsaved changes?")
	} else {
		s.ConfirmOverlay.Cancel()
	}
	return &s.ConfirmOverlay
}

// ConfirmAnswer is what a confirmation made of one key press. It separates the
// key a confirmation swallowed from the key it never saw, so a key chain can
// keep giving an armed prompt modal priority without re-reading the dialog
// itself. It lives beside the dialog because every cluster that arms one reads
// its answer, and the reading is the same reading everywhere.
type ConfirmAnswer int

const (
	// ConfirmAnswerUnarmed means no confirmation was pending: the key belongs
	// to whatever sits underneath.
	ConfirmAnswerUnarmed ConfirmAnswer = iota
	// ConfirmAnswerSwallowed means the prompt consumed a key that answered
	// neither yes nor no, so nothing leaks into the view beneath it.
	ConfirmAnswerSwallowed
	ConfirmAnswerConfirmed
	ConfirmAnswerCanceled
)

// ResolveConfirmKey reads one key press against one dialog. An inactive dialog
// and a key the dialog does not handle are the same answer, because both leave
// the key to the surface below.
func ResolveConfirmKey(dialog *ConfirmDialog, key string) ConfirmAnswer {
	if !dialog.IsActive() {
		return ConfirmAnswerUnarmed
	}
	confirmed, handled := dialog.HandleKey(key)
	if !handled {
		return ConfirmAnswerUnarmed
	}
	if confirmed {
		return ConfirmAnswerConfirmed
	}
	switch key {
	case "n", "N", "esc":
		return ConfirmAnswerCanceled
	}
	return ConfirmAnswerSwallowed
}
