package models

import (
	"testing"
)

func TestToggleCommentResolved(t *testing.T) {
	t.Run("toggles resolved state", func(t *testing.T) {
		task := &Task{}
		task.AddComment("First comment")
		task.AddComment("Second comment")

		if task.Comments[0].Resolved {
			t.Fatal("new comment should not be resolved")
		}

		ok := task.ToggleCommentResolved(0)
		if !ok {
			t.Fatal("ToggleCommentResolved returned false for valid index")
		}
		if !task.Comments[0].Resolved {
			t.Error("comment should be resolved after toggle")
		}

		// Toggle back
		ok = task.ToggleCommentResolved(0)
		if !ok {
			t.Fatal("ToggleCommentResolved returned false for valid index")
		}
		if task.Comments[0].Resolved {
			t.Error("comment should be unresolved after second toggle")
		}
	})

	t.Run("out of range returns false", func(t *testing.T) {
		task := &Task{}
		task.AddComment("Comment")

		if task.ToggleCommentResolved(-1) {
			t.Error("ToggleCommentResolved should return false for negative index")
		}
		if task.ToggleCommentResolved(1) {
			t.Error("ToggleCommentResolved should return false for out-of-range index")
		}
		if task.ToggleCommentResolved(99) {
			t.Error("ToggleCommentResolved should return false for far out-of-range index")
		}
	})

	t.Run("empty comments returns false", func(t *testing.T) {
		task := &Task{}
		if task.ToggleCommentResolved(0) {
			t.Error("ToggleCommentResolved should return false for empty comments")
		}
	})
}

func TestUnresolvedCommentCount(t *testing.T) {
	t.Run("all unresolved", func(t *testing.T) {
		task := &Task{}
		task.AddComment("First")
		task.AddComment("Second")
		task.AddComment("Third")

		if got := task.UnresolvedCommentCount(); got != 3 {
			t.Errorf("UnresolvedCommentCount() = %d, want 3", got)
		}
	})

	t.Run("some resolved", func(t *testing.T) {
		task := &Task{}
		task.AddComment("First")
		task.AddComment("Second")
		task.AddComment("Third")
		task.Comments[1].Resolved = true

		if got := task.UnresolvedCommentCount(); got != 2 {
			t.Errorf("UnresolvedCommentCount() = %d, want 2", got)
		}
	})

	t.Run("all resolved", func(t *testing.T) {
		task := &Task{}
		task.AddComment("First")
		task.AddComment("Second")
		task.Comments[0].Resolved = true
		task.Comments[1].Resolved = true

		if got := task.UnresolvedCommentCount(); got != 0 {
			t.Errorf("UnresolvedCommentCount() = %d, want 0", got)
		}
	})

	t.Run("no comments", func(t *testing.T) {
		task := &Task{}
		if got := task.UnresolvedCommentCount(); got != 0 {
			t.Errorf("UnresolvedCommentCount() = %d, want 0", got)
		}
	})
}
