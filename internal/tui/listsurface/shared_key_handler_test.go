package listsurface

import (
	"testing"
)

func TestJumpCursorEmptyTotal(t *testing.T) {
	cursor := 5
	jumpCursor(&cursor, 0, 10)
	if cursor != 0 {
		t.Errorf("empty total with positive delta: cursor = %d, want 0", cursor)
	}

	cursor = -3
	jumpCursor(&cursor, 0, -5)
	if cursor != 0 {
		t.Errorf("empty total with negative delta: cursor = %d, want 0", cursor)
	}

	cursor = 2
	jumpCursor(&cursor, -1, 3)
	if cursor != 0 {
		t.Errorf("negative total: cursor = %d, want 0", cursor)
	}
}

func TestJumpCursorClampUpper(t *testing.T) {
	cursor := 1
	jumpCursor(&cursor, 5, 100)
	if cursor != 4 {
		t.Errorf("over-jump up: cursor = %d, want 4", cursor)
	}
}

func TestJumpCursorClampLower(t *testing.T) {
	cursor := 2
	jumpCursor(&cursor, 5, -100)
	if cursor != 0 {
		t.Errorf("over-jump down: cursor = %d, want 0", cursor)
	}
}

func TestJumpCursorNormal(t *testing.T) {
	cursor := 3
	jumpCursor(&cursor, 10, 2)
	if cursor != 5 {
		t.Errorf("normal jump: cursor = %d, want 5", cursor)
	}
}
