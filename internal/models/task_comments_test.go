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

func TestAddComment_RejectsEmptyAndWhitespace(t *testing.T) {
	cases := []struct {
		name string
		text string
	}{
		{"empty", ""},
		{"spaces", "   "},
		{"tabs", "\t\t"},
		{"newlines", "\n\n"},
		{"mixed whitespace", " \t\n  "},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			task := &Task{}
			task.AddComment(tc.text)
			if len(task.Comments) != 0 {
				t.Fatalf("expected 0 comments after AddComment(%q), got %d", tc.text, len(task.Comments))
			}
		})
	}
}

func TestRemoveComment_NilsOutTailToReleasePointer(t *testing.T) {
	task := &Task{}
	task.AddComment("first")
	task.AddComment("second")
	task.AddComment("third")

	// Capture the underlying backing array via a slice that includes the
	// tail slot. After RemoveComment shrinks the visible length by one,
	// the backing array's last slot must be nil — otherwise the removed
	// *Comment is leaked through the underlying array.
	full := task.Comments[:3]

	if !task.RemoveComment(0) {
		t.Fatal("RemoveComment(0) returned false")
	}
	if len(task.Comments) != 2 {
		t.Fatalf("expected 2 comments after removal, got %d", len(task.Comments))
	}
	if task.Comments[0].Text != "second" || task.Comments[1].Text != "third" {
		t.Fatalf("unexpected remaining comments: %q, %q",
			task.Comments[0].Text, task.Comments[1].Text)
	}
	if full[2] != nil {
		t.Errorf("expected backing-array tail slot to be nil after RemoveComment, "+
			"but it still references a *Comment with text %q", full[2].Text)
	}
}

func TestAddComment_KeepsNonEmptyTextWithSurroundingSpace(t *testing.T) {
	task := &Task{}
	task.AddComment("  meaningful note  ")
	if len(task.Comments) != 1 {
		t.Fatalf("expected 1 comment, got %d", len(task.Comments))
	}
	if task.Comments[0].Text != "  meaningful note  " {
		t.Errorf("expected text to be preserved verbatim, got %q", task.Comments[0].Text)
	}
}
