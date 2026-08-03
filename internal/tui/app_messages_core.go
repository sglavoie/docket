package tui

import (
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
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

// RefreshMsg requests a refresh of the current view's data.
type RefreshMsg struct{}

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

// CursorFlashClearMsg clears the cursor flash highlight after a timeout.
type CursorFlashClearMsg struct{}

// TodayClearedBannerClearMsg dismisses the Today-cleared banner once its TTL
// elapses. Distinct from StatusClearMsg so a normal status message coming in
// later does not get wiped by the banner timer.
type TodayClearedBannerClearMsg struct{}

// periodicSaveMsg triggers a periodic save of spatial state (cursor/fold).
type periodicSaveMsg struct{}

// outlinerFilterStateChangedMsg signals that the outliner's filter state was
// modified (e.g. Fx clear) and should be persisted to config immediately.
type outlinerFilterStateChangedMsg struct{}

// StatusMsg sets a temporary status message.
type StatusMsg string

// InfoStatusMsg sets a temporary informational status message (neutral styling, no checkmark).
type InfoStatusMsg string

// =============================================================================
// Misc Messages
// =============================================================================

// OpenDataFolderMsg opens the data folder in the configured editor.
// If Project is nil, opens root data directory. Otherwise opens project directory.
type OpenDataFolderMsg struct {
	Project *models.Project
}

// OpenLogDirMsg opens the log directory in Finder.
type OpenLogDirMsg struct{}

// =============================================================================
// Async Save Messages
// =============================================================================

// SaveStartMsg signals that an async save operation is starting.
type SaveStartMsg struct {
	Message string
}

// SaveCompleteMsg signals that an async save completed successfully.
type SaveCompleteMsg struct {
	SuccessMessage string
}

// SaveErrorMsg signals that an async save failed.
type SaveErrorMsg struct {
	Err error
}

// SaveTimeoutMsg is sent when a save operation has not completed within the timeout window.
type SaveTimeoutMsg struct{}

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

// CursorFlashDuration is how long the cursor flash highlight stays visible.
const CursorFlashDuration = 150 * time.Millisecond

// cursorFlashTimeout returns a command that clears the cursor flash after CursorFlashDuration.
func cursorFlashTimeout() tea.Cmd {
	return tea.Tick(CursorFlashDuration, func(time.Time) tea.Msg {
		return CursorFlashClearMsg{}
	})
}

// TodayClearedBannerTimeout is how long the Today-cleared banner stays visible.
const TodayClearedBannerTimeout = 5 * time.Second

// todayClearedBannerTimeout returns a command that dismisses the Today-cleared
// banner after TodayClearedBannerTimeout.
func todayClearedBannerTimeout() tea.Cmd {
	return tea.Tick(TodayClearedBannerTimeout, func(time.Time) tea.Msg {
		return TodayClearedBannerClearMsg{}
	})
}
