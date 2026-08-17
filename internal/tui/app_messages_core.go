package tui

import (
	"time"

	tea "charm.land/bubbletea/v2"
)

// =============================================================================
// Close-View Interface
// =============================================================================

// popStyle indicates how a close-view message should pop the view stack.
type popStyle int

const (
	popOne      popStyle = iota // pop a single view (popView)
	popToAnchor                 // pop to the nearest anchor (popToAnchorView)
)

// closeViewMsg is implemented by close messages whose only effect is popping
// the view stack. The handler in Update checks this interface first, avoiding
// individual case arms for each Close*Msg type.
type closeViewMsg interface {
	closePopStyle() popStyle
}

// PopViewMsg is embedded in Close*Msg types that need popView().
type PopViewMsg struct{}

func (PopViewMsg) closePopStyle() popStyle { return popOne }

// =============================================================================
// Navigation Messages
// =============================================================================

// BackMsg requests navigation back to the previous view.
type BackMsg struct{}

// QuitMsg requests application termination.
type QuitMsg struct{}

// ErrorMsg reports an error that should be displayed to the user.
type ErrorMsg struct{ Err error }

// StatusClearMsg clears the status message after a timeout. ScheduledAt
// identifies the timer generation so an older timer cannot clear newer text.
type StatusClearMsg struct{ ScheduledAt time.Time }

// UndoToastTimeoutMsg fires when an undo-available toast's window has
// elapsed. The handler clears the toast only if it has not been replaced
// by a newer toast (checked via statusExpiresAt) so back-to-back
// destructive actions do not cancel each other prematurely.
type UndoToastTimeoutMsg struct{}

// ViewStatusClearMsg clears a view-owned status message after a timeout. View
// and Seq identify the message generation, so an older tick can never dismiss a
// newer message or a message belonging to a different view.
type ViewStatusClearMsg struct {
	View PresentationID
	Seq  uint64
}

// CursorFlashClearMsg clears the cursor flash highlight after a timeout.
type CursorFlashClearMsg struct{}

// FooterFlashTickMsg advances the footer divider flash one frame. Seq
// identifies the flash generation so a tick from a superseded flash is ignored.
type FooterFlashTickMsg struct{ Seq uint64 }

// periodicSaveMsg triggers a periodic save of spatial state (cursor/fold).
type periodicSaveMsg struct{}

// StatusMsg sets a temporary status message.
type StatusMsg string

// InfoStatusMsg sets a temporary informational status message (neutral styling, no checkmark).
type InfoStatusMsg string

// SaveCompleteMsg signals that an async save completed successfully.
type SaveCompleteMsg struct {
	SuccessMessage string
}

// =============================================================================
// Helper Functions
// =============================================================================

// StatusUpdateTimeout is the default duration for status messages before they auto-clear.
const StatusUpdateTimeout = 2 * time.Second

// statusTimeout returns a command that clears the status message after the given duration.
func statusTimeout(d time.Duration) tea.Cmd {
	scheduledAt := time.Now()
	return tea.Tick(d, func(time.Time) tea.Msg {
		return StatusClearMsg{ScheduledAt: scheduledAt}
	})
}

// defaultStatusTimeout returns a command that clears the status message after the default timeout.
func defaultStatusTimeout() tea.Cmd {
	return statusTimeout(StatusUpdateTimeout)
}

// viewStatusTimeout returns a command that clears one generation of a
// view-owned status message after the default timeout.
func viewStatusTimeout(view PresentationID, seq uint64) tea.Cmd {
	return tea.Tick(StatusUpdateTimeout, func(time.Time) tea.Msg {
		return ViewStatusClearMsg{View: view, Seq: seq}
	})
}

// CursorFlashDuration is how long the cursor flash highlight stays visible.
const CursorFlashDuration = 150 * time.Millisecond

// cursorFlashTimeout returns a command that clears the cursor flash after CursorFlashDuration.
func cursorFlashTimeout() tea.Cmd {
	return tea.Tick(CursorFlashDuration, func(time.Time) tea.Msg {
		return CursorFlashClearMsg{}
	})
}

// footerFlashTick schedules the next frame of the footer divider flash.
func footerFlashTick(seq uint64) tea.Cmd {
	return tea.Tick(footerFlashFrameInterval, func(time.Time) tea.Msg {
		return FooterFlashTickMsg{Seq: seq}
	})
}
