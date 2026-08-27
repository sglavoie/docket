package models

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestAddCommentAtCreatesLeanTimestampedRecord(t *testing.T) {
	task := &Task{}
	now := time.Date(2026, 7, 28, 12, 30, 0, 0, time.UTC)

	task.AddCommentAt("  meaningful note  ", now)

	if len(task.Comments) != 1 {
		t.Fatalf("comments = %d, want 1", len(task.Comments))
	}
	comment := task.Comments[0]
	if comment.Text != "  meaningful note  " {
		t.Errorf("text = %q, want preserved input", comment.Text)
	}
	if !comment.CreatedAt.Equal(now) || !task.UpdatedAt.Equal(now) {
		t.Errorf("timestamps = comment %v task %v, want %v", comment.CreatedAt, task.UpdatedAt, now)
	}

	encoded, err := json.Marshal(comment)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "resolved") {
		t.Fatalf("canonical comment JSON contains retired state: %s", encoded)
	}
}

func TestAddCommentRejectsWhitespace(t *testing.T) {
	for _, text := range []string{"", "   ", "\t\t", "\n\n", " \t\n  "} {
		task := &Task{}
		task.AddComment(text)
		if len(task.Comments) != 0 {
			t.Fatalf("AddComment(%q) created %d comments", text, len(task.Comments))
		}
	}
}

func TestRemoveCommentNilsBackingArrayTail(t *testing.T) {
	task := &Task{}
	task.AddComment("first")
	task.AddComment("second")
	task.AddComment("third")
	full := task.Comments[:3]

	if !task.RemoveComment(0) {
		t.Fatal("RemoveComment(0) returned false")
	}
	if len(task.Comments) != 2 || task.Comments[0].Text != "second" || task.Comments[1].Text != "third" {
		t.Fatalf("remaining comments = %+v", task.Comments)
	}
	if full[2] != nil {
		t.Fatalf("removed comment remains in backing-array tail: %+v", full[2])
	}
	if task.RemoveComment(-1) || task.RemoveComment(2) {
		t.Fatal("out-of-range removal succeeded")
	}
}
