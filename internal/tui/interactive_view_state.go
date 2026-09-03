package tui

import (
	tea "charm.land/bubbletea/v2"
	"github.com/sglavoie/tt/internal/tui/chrome"
)

// InteractiveViewState provides common interactive state for views that
// support cursor navigation, status messages, and
// confirmation dialogs. Embed this in view models to eliminate duplicate
// field declarations.
type InteractiveViewState struct {
	cursor           int
	viewScrollOffset int
	cursorFlash      bool
	statusMessage    string
	statusSeq        uint64
	statusTransient  bool
	confirm          chrome.ConfirmDialog
}

// setStatus sets the status message. The message is sticky: it stays until the
// view replaces or clears it.
func (s *InteractiveViewState) setStatus(msg string) {
	s.statusMessage = msg
	s.statusSeq++
	s.statusTransient = false
}

// setTransientStatus sets a status message that auto-clears like the shell's own.
func (s *InteractiveViewState) setTransientStatus(msg string) {
	s.setStatus(msg)
	s.statusTransient = true
}

// StatusIsTransient reports whether a live status message should expire on the
// shell's timer.
func (s *InteractiveViewState) StatusIsTransient() bool {
	return s.statusTransient && s.statusMessage != ""
}

// StatusStamp reports the generation of this view's own status line so the
// shell can flash the footer divider when it changes.
func (s *InteractiveViewState) StatusStamp() uint64 {
	return s.statusSeq
}

// clearStatus clears the status message.
func (s *InteractiveViewState) clearStatus() {
	s.statusMessage = ""
	s.statusTransient = false
}

// DefaultShouldCaptureEsc returns true if a confirm dialog is active.
// Views should call this in their ShouldCaptureEsc() override and OR in their own conditions.
// Views that also embed BaseSearchableView should additionally OR in m.SearchShouldCaptureEsc().
func (s *InteractiveViewState) DefaultShouldCaptureEsc() bool {
	return s.confirm.IsActive()
}

// ClampCursorTo ensures cursor is within [0, length-1] range.
func (s *InteractiveViewState) ClampCursorTo(length int) {
	s.cursor = chrome.ClampCursor(s.cursor, length)
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
func (s *InteractiveViewState) GetConfirmDialog() *chrome.ConfirmDialog {
	return &s.confirm
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
