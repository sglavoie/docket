package tui

import (
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/appearance"
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

// PopToAnchorMsg is embedded in Close*Msg types that need popToAnchorView().
type PopToAnchorMsg struct{}

func (PopToAnchorMsg) closePopStyle() popStyle { return popToAnchor }

// =============================================================================
// Navigation Messages
// =============================================================================

// BackMsg requests navigation back to the previous view.
type BackMsg struct{}

// ShowAskMsg requests opening the AI assistant view.
type ShowAskMsg struct{}

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

// MutationFlashClearMsg clears the mutation flash highlight after a timeout.
type MutationFlashClearMsg struct{}

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

// systemAppearanceCheckMsg triggers a background check of the OS appearance.
type systemAppearanceCheckMsg struct{}

// systemAppearanceResultMsg carries the result of an OS appearance detection.
type systemAppearanceResultMsg struct{ Appearance appearance.SystemAppearance }

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

// MutationFlashDuration is how long the mutation flash highlight stays visible.
const MutationFlashDuration = 500 * time.Millisecond

// mutationFlashTimeout returns a command that clears the mutation flash after MutationFlashDuration.
func mutationFlashTimeout() tea.Cmd {
	return tea.Tick(MutationFlashDuration, func(time.Time) tea.Msg {
		return MutationFlashClearMsg{}
	})
}
