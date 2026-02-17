package tui

// ValidateCursor clamps the cursor position to valid bounds for the given task count.
// For an empty column (taskCount=0), cursor is set to 0.
// For non-empty columns, cursor is clamped to [0, taskCount-1].
// Call this after task mutations that may invalidate the cursor position.
func (cs *ColumnState) ValidateCursor(taskCount int) {
	cs.Cursor = ClampCursor(cs.Cursor, taskCount)
}

// Validate adjusts Cursor and ScrollOffset to be within valid bounds for the
// given task count. Call this after any mutation that changes the column's
// task list (task moves, status changes, deletions).
func (cs *ColumnState) Validate(taskCount int) {
	cs.ValidateCursor(taskCount)
	cs.validateScrollOffset(taskCount)
}

// validateScrollOffset ensures ScrollOffset keeps the selected task visible.
// This is called internally by Validate.
func (cs *ColumnState) validateScrollOffset(taskCount int) {
	if taskCount == 0 {
		cs.ScrollOffset = 0
		return
	}
	if cs.ScrollOffset < 0 {
		cs.ScrollOffset = 0
	}
	// Ensure cursor is visible (scroll offset should not exceed cursor)
	if cs.ScrollOffset > cs.Cursor {
		cs.ScrollOffset = cs.Cursor
	}
}

// InBounds returns true if the cursor is valid for the given task count.
func (cs *ColumnState) InBounds(taskCount int) bool {
	return CursorInBounds(cs.Cursor, taskCount)
}

// ValidateScroll adjusts the scroll offset to ensure the cursor row remains
// visible within the column's viewport. Call this after cursor movement or
// viewport resize to keep the selected task visible.
//
// The method ensures:
// - Cursor above viewport: scroll up so cursor is at top of viewport
// - Cursor below viewport: scroll down so cursor is at bottom of viewport
// - Scroll offset clamped to valid range [0, max(0, taskCount-viewportHeight)]
func (cs *ColumnState) ValidateScroll(taskCount, viewportHeight int) {
	if taskCount == 0 || viewportHeight <= 0 {
		cs.ScrollOffset = 0
		return
	}

	// If cursor is above the viewport, scroll up
	if cs.Cursor < cs.ScrollOffset {
		cs.ScrollOffset = cs.Cursor
	}

	// If cursor is below the viewport, scroll down
	if cs.Cursor >= cs.ScrollOffset+viewportHeight {
		cs.ScrollOffset = cs.Cursor - viewportHeight + 1
	}

	// Clamp scroll offset to valid range
	maxOffset := taskCount - viewportHeight
	if maxOffset < 0 {
		maxOffset = 0
	}
	if cs.ScrollOffset > maxOffset {
		cs.ScrollOffset = maxOffset
	}
	if cs.ScrollOffset < 0 {
		cs.ScrollOffset = 0
	}
}

// AdjustScrollForViewport is an alias for ValidateScroll for backwards compatibility.
// Prefer using ValidateScroll for new code.
func (cs *ColumnState) AdjustScrollForViewport(taskCount, viewportHeight int) {
	cs.ValidateScroll(taskCount, viewportHeight)
}
