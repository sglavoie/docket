package chrome

// CursorInBounds returns true if cursor is valid for the given slice length.
// Use this to guard operations that require a valid selection.
// HalfPage returns a scroll jump size of half the given height, minimum 1.
func HalfPage(height int) int {
	return max(height/2, 1)
}

// ClampCursor ensures cursor is within [0, length-1] range.
// Returns 0 if length is 0.
func ClampCursor(cursor, length int) int {
	if length == 0 {
		return 0
	}
	if cursor < 0 {
		return 0
	}
	if cursor >= length {
		return length - 1
	}
	return cursor
}
