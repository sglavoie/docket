package tui

import (
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
)

// =============================================================================
// Navigation Messages
// =============================================================================

// BackMsg requests navigation back to the previous view.
type BackMsg struct{}

// NavigateHistoryBackMsg requests navigation backward through view history.
type NavigateHistoryBackMsg struct{}

// NavigateHistoryForwardMsg requests navigation forward through view history.
type NavigateHistoryForwardMsg struct{}

// RefreshMsg requests a refresh of the current view's data.
type RefreshMsg struct{}

// QuitMsg requests application termination.
type QuitMsg struct{}

// ErrorMsg reports an error that should be displayed to the user.
type ErrorMsg struct{ Err error }

// StatusClearMsg clears the status message after a timeout.
type StatusClearMsg struct{}

// ClearSemicolonPrefixMsg clears a pending semicolon prefix if it has not been resolved.
type ClearSemicolonPrefixMsg struct{}

// CursorFlashClearMsg clears the cursor flash highlight after a timeout.
type CursorFlashClearMsg struct{}

// periodicSaveMsg triggers a periodic save of spatial state (cursor/fold).
type periodicSaveMsg struct{}

// outlinerFilterStateChangedMsg signals that the outliner's filter state was
// modified (e.g. Fx clear) and should be persisted to config immediately.
type outlinerFilterStateChangedMsg struct{}

// StatusMsg sets a temporary status message (used by command palette handlers).
type StatusMsg string

// InfoStatusMsg sets a temporary informational status message (neutral styling, no checkmark).
type InfoStatusMsg string

// =============================================================================
// Misc Messages
// =============================================================================

// MoveBlockedMsg signals a task move was blocked (e.g., completion boundary).
type MoveBlockedMsg struct{ Reason string }

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
	return tea.Tick(d, func(time.Time) tea.Msg {
		return StatusClearMsg{}
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
