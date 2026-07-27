package tui

import (
	"image/color"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/tui/keymap"
)

// FlashKind classifies a mutation flash so renderers and frame schedulers can
// distinguish a normal status/priority change (single solid background) from
// a task-completion peak moment (multi-frame green ramp).
type FlashKind int

const (
	// FlashMutation is the default single-frame flash for status/priority/type/etc.
	FlashMutation FlashKind = iota
	// FlashCompletion is a 3-frame green-ramp pulse for task completion.
	FlashCompletion
)

// InteractiveViewState provides common interactive state for views that
// support cursor navigation, status messages, and
// confirmation dialogs. Embed this in view models to eliminate duplicate
// field declarations.
type InteractiveViewState struct {
	cursor            int
	viewScrollOffset  int
	cursorFlash       bool
	mutationFlash     bool
	mutationFlashBg   color.Color
	mutationFlashKind FlashKind
	statusMessage     string
	confirm           ConfirmDialog
	// Dispatcher is the declarative keymap dispatcher for this view.
	Dispatcher *keymap.Dispatcher
}

// setStatus sets the status message.
func (s *InteractiveViewState) setStatus(msg string) {
	s.statusMessage = msg
}

// clearStatus clears the status message.
func (s *InteractiveViewState) clearStatus() {
	s.statusMessage = ""
}

// DefaultShouldCaptureEsc returns true if a confirm dialog is active.
// Views should call this in their ShouldCaptureEsc() override and OR in their own conditions.
// Views that also embed BaseSearchableView should additionally OR in m.SearchShouldCaptureEsc().
func (s *InteractiveViewState) DefaultShouldCaptureEsc() bool {
	return s.confirm.IsActive()
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

// advanceCompletionFlashFrame updates the background to the given frame's color.
// No-op when the current flash is not a completion ramp.
func (s *InteractiveViewState) advanceCompletionFlashFrame(frame int) {
	if !s.mutationFlash || s.mutationFlashKind != FlashCompletion {
		return
	}
	s.mutationFlashBg = completionFlashBg(frame)
}
