package tui

import (
	"testing"

	"github.com/sglavoie/tt/internal/tui/listsurface"
)

func TestJumpCursorEmptyTotal(t *testing.T) {
	cursor := 5
	listsurface.JumpCursor(&cursor, 0, 10)
	if cursor != 0 {
		t.Errorf("empty total with positive delta: cursor = %d, want 0", cursor)
	}

	cursor = -3
	listsurface.JumpCursor(&cursor, 0, -5)
	if cursor != 0 {
		t.Errorf("empty total with negative delta: cursor = %d, want 0", cursor)
	}

	cursor = 2
	listsurface.JumpCursor(&cursor, -1, 3)
	if cursor != 0 {
		t.Errorf("negative total: cursor = %d, want 0", cursor)
	}
}

func TestJumpCursorClampUpper(t *testing.T) {
	cursor := 1
	listsurface.JumpCursor(&cursor, 5, 100)
	if cursor != 4 {
		t.Errorf("over-jump up: cursor = %d, want 4", cursor)
	}
}

func TestJumpCursorClampLower(t *testing.T) {
	cursor := 2
	listsurface.JumpCursor(&cursor, 5, -100)
	if cursor != 0 {
		t.Errorf("over-jump down: cursor = %d, want 0", cursor)
	}
}

func TestJumpCursorNormal(t *testing.T) {
	cursor := 3
	listsurface.JumpCursor(&cursor, 10, 2)
	if cursor != 5 {
		t.Errorf("normal jump: cursor = %d, want 5", cursor)
	}
}
