package tui

// CursorInBounds returns true if cursor is valid for the given slice length.
// Use this to guard operations that require a valid selection.
func CursorInBounds(cursor, length int) bool {
	return length > 0 && cursor >= 0 && cursor < length
}

// HalfPage returns a scroll jump size of half the given height, minimum 1.
func HalfPage(height int) int {
	return max(height/2, 1)
}

// HandleGGSequence handles vim-style gg→top navigation.
// Call this at the TOP of key handling, before any switch statement.
// Returns (consumed, goTop). If consumed=true caller must return immediately.
// If goTop=true caller should additionally reset cursor to 0.
func HandleGGSequence(pending *bool, key string) (consumed, goTop bool) {
	if *pending {
		*pending = false
		return true, key == "g"
	}
	if key == "g" {
		*pending = true
		return true, false
	}
	return false, false
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
