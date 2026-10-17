package status

import (
	"time"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/tui/presentation"
)

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

// WorkspaceUndoMsg asks the shell to undo (Redo false) or redo (Redo true) the
// newest workspace-ledger transaction on behalf of a panel. The Project Hub and
// the Attachments panel hold no AppModel, and the core undo descriptors answer
// only to the Outliner, so this message is how a panel key reaches the one
// ledger without a second key router: the shell runs coreUndo/coreRedo, which
// names what moved in the status strip and refreshes every open surface.
type WorkspaceUndoMsg struct{ Redo bool }

// WorkspaceUndoCmd is the command a panel returns for its u / Ctrl+R press.
func WorkspaceUndoCmd(redo bool) tea.Cmd {
	return func() tea.Msg { return WorkspaceUndoMsg{Redo: redo} }
}

// ViewStatusClearMsg clears a view-owned status message after a timeout. View
// and Seq identify the message generation, so an older tick can never dismiss a
// newer message or a message belonging to a different view.
type ViewStatusClearMsg struct {
	View presentation.PresentationID
	Seq  uint64
}

// CursorFlashClearMsg clears the cursor flash highlight after a timeout.
type CursorFlashClearMsg struct{}

// PeriodicSaveMsg triggers a periodic save of spatial state (cursor/fold).
type PeriodicSaveMsg struct{ At time.Time }

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

// StatusTimeout returns a command that clears the status message after the given duration.
func StatusTimeout(d time.Duration) tea.Cmd {
	scheduledAt := time.Now()
	return TickCmd(d, func(time.Time) tea.Msg {
		return StatusClearMsg{ScheduledAt: scheduledAt}
	})
}

// DefaultStatusTimeout returns a command that clears the status message after the default timeout.
func DefaultStatusTimeout() tea.Cmd {
	return StatusTimeout(StatusUpdateTimeout)
}

// Reading time ordinary shell and view-owned feedback are given: the default
// floor, a little more per rune, and a ceiling so a long refusal still leaves.
const (
	statusPerRune    = 40 * time.Millisecond
	statusMaxTimeout = 6 * time.Second
)

// StatusDuration is how long ordinary feedback of this text stays, so an
// error or a long refusal is not gone before it can be read.
func StatusDuration(text string) time.Duration {
	d := StatusUpdateTimeout + time.Duration(utf8.RuneCountInString(text))*statusPerRune
	return min(d, statusMaxTimeout)
}

// RefusalDuration leaves time to read the reason and use its recovery action.
// rows is the actual wrapped feedback height, measured by the renderer's own
// layout. Routine successes and the undo deadline keep their existing timing.
func RefusalDuration(text string, rows int) time.Duration {
	d := StatusUpdateTimeout + time.Duration(utf8.RuneCountInString(text))*statusPerRune
	d += time.Duration(max(rows-1, 0)) * time.Second
	return min(max(d, 6*time.Second), 12*time.Second)
}

// ViewStatusTimeout returns a command that clears one generation of a
// view-owned status message once its text has had time to be read.
func ViewStatusTimeout(view presentation.PresentationID, seq uint64, text string) tea.Cmd {
	return TickCmd(StatusDuration(text), func(time.Time) tea.Msg {
		return ViewStatusClearMsg{View: view, Seq: seq}
	})
}

// CursorFlashDuration is how long the cursor flash highlight stays visible.
const CursorFlashDuration = 150 * time.Millisecond

// CursorFlashTimeout returns a command that clears the cursor flash after CursorFlashDuration.
func CursorFlashTimeout() tea.Cmd {
	return TickCmd(CursorFlashDuration, func(time.Time) tea.Msg {
		return CursorFlashClearMsg{}
	})
}
