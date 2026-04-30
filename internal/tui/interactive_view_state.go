package tui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"
)

// InteractiveViewState provides common interactive state for views that
// support cursor navigation, prefix key sequences, status messages, and
// confirmation dialogs. Embed this in view models to eliminate duplicate
// field declarations and HasPendingPrefix() methods.
type InteractiveViewState struct {
	cursor        int
	cursorFlash   bool
	pendingPrefix string
	statusMessage string
	confirm       ConfirmDialog
}

// setStatus sets the status message.
func (s *InteractiveViewState) setStatus(msg string) {
	s.statusMessage = msg
}

// clearStatus clears the status message.
func (s *InteractiveViewState) clearStatus() {
	s.statusMessage = ""
}

// HasPendingPrefix returns true if waiting for second key in a prefix sequence.
func (s *InteractiveViewState) HasPendingPrefix() bool {
	return s.pendingPrefix != ""
}

// SkipModifierInPrefix returns true when a prefix is pending and the key event
// is a bare modifier press (Shift, Ctrl, Alt, etc.). Views should call this
// before dispatching prefix keys and return early to keep the popup open.
func (s *InteractiveViewState) SkipModifierInPrefix(msg tea.KeyPressMsg) bool {
	return s.pendingPrefix != "" && isModifierOnly(msg)
}

// DefaultShouldCaptureEsc returns true if a pending prefix or confirm dialog is active.
// Views should call this in their ShouldCaptureEsc() override and OR in their own conditions.
// Views that also embed BaseSearchableView should additionally OR in m.SearchShouldCaptureEsc().
func (s *InteractiveViewState) DefaultShouldCaptureEsc() bool {
	return s.pendingPrefix != "" || s.confirm.IsActive()
}

// ClampCursorTo ensures cursor is within [0, length-1] range.
func (s *InteractiveViewState) ClampCursorTo(length int) {
	s.cursor = ClampCursor(s.cursor, length)
}

// ResetCursor sets the cursor to position 0.
func (s *InteractiveViewState) ResetCursor() {
	s.cursor = 0
}

// JumpToTop resets the cursor to 0 and returns a flash cmd if the jump
// was more than 1 position.
func (s *InteractiveViewState) JumpToTop() tea.Cmd {
	old := s.cursor
	s.cursor = 0
	return flashIfJumped(old, s.cursor, &s.cursorFlash)
}

// SectionGoToNext moves the cursor to the next section header (with wrap)
// using the provided section start positions. Returns a flash cmd on jump.
func (s *InteractiveViewState) SectionGoToNext(sectionStarts []int) tea.Cmd {
	old := s.cursor
	if pos := navigateToNextSection(s.cursor, sectionStarts); pos >= 0 {
		s.cursor = pos
	}
	return flashIfJumped(old, s.cursor, &s.cursorFlash)
}

// SectionGoToPrev moves the cursor to the previous section header (with wrap)
// using the provided section start positions. Returns a flash cmd on jump.
func (s *InteractiveViewState) SectionGoToPrev(sectionStarts []int) tea.Cmd {
	old := s.cursor
	if pos := navigateToPrevSection(s.cursor, sectionStarts); pos >= 0 {
		s.cursor = pos
	}
	return flashIfJumped(old, s.cursor, &s.cursorFlash)
}

// IsConfirmActive returns true if a confirm dialog is currently active.
func (s *InteractiveViewState) IsConfirmActive() bool {
	return s.confirm.IsActive()
}

// GetConfirmDialog returns a pointer to the embedded confirm dialog.
func (s *InteractiveViewState) GetConfirmDialog() *ConfirmDialog {
	return &s.confirm
}

// showDeleteConfirm activates the delete confirmation dialog for the given task.
func (s *InteractiveViewState) showDeleteConfirm(shortID int) {
	s.confirm.Show("delete", fmt.Sprintf("Delete task #%d?", shortID))
}

// HandleConfirmKey checks if a confirm dialog is active and processes the key.
// Returns (confirmed, handled). On cancel, clears the status message.
// When handled is false, no confirm dialog was active and the caller should
// continue normal key processing.
func (s *InteractiveViewState) HandleConfirmKey(key string) (confirmed, handled bool) {
	if !s.confirm.IsActive() {
		return false, false
	}
	c, h := s.confirm.HandleKey(key)
	return c, h
}
