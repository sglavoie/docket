package format

import (
	"strings"
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/util"
)

func TestWriteComments_RoundTrip(t *testing.T) {
	comments := []*models.Comment{
		{
			Text:      "First comment",
			CreatedAt: time.Date(2025, 1, 15, 10, 30, 0, 0, time.Local),
		},
		{
			Text:      "Second comment\nwith multiple lines",
			CreatedAt: time.Date(2025, 1, 16, 14, 20, 0, 0, time.Local),
		},
		{
			Text:      "Comment with <!-- and --> markers",
			CreatedAt: time.Date(2025, 1, 17, 9, 15, 0, 0, time.Local),
		},
	}

	task := &models.Task{
		Description: "Task description",
		Comments:    comments,
	}

	commentBlock := WriteComments(task)

	task2 := &models.Task{
		Description: "Task description\n\n" + commentBlock,
	}

	ExtractCommentsFromDescription(task2)

	if len(task2.Comments) != len(comments) {
		t.Fatalf("got %d comments, want %d", len(task2.Comments), len(comments))
	}

	for i, got := range task2.Comments {
		want := comments[i]
		if got.Text != want.Text {
			t.Errorf("Comment[%d].Text = %q, want %q", i, got.Text, want.Text)
		}
		gotTime := got.CreatedAt.Format(util.DateTimeFormat)
		wantTime := want.CreatedAt.Format(util.DateTimeFormat)
		if gotTime != wantTime {
			t.Errorf("Comment[%d].CreatedAt = %s, want %s", i, gotTime, wantTime)
		}
	}

	if !strings.Contains(task2.Description, "Task description") {
		t.Errorf("Description missing original content: %q", task2.Description)
	}
	if strings.Contains(task2.Description, "<!-- COMMENTS -->") {
		t.Errorf("Description still contains comments block")
	}
}

func TestWriteComments_AmpersandRoundTrip(t *testing.T) {
	comments := []*models.Comment{
		{
			Text:      "Use &lt;!-- for HTML comments",
			CreatedAt: time.Date(2025, 3, 1, 10, 0, 0, 0, time.Local),
		},
		{
			Text:      "Tom & Jerry",
			CreatedAt: time.Date(2025, 3, 2, 10, 0, 0, 0, time.Local),
		},
		{
			Text:      "Code: <!-- inject --> end",
			CreatedAt: time.Date(2025, 3, 3, 10, 0, 0, 0, time.Local),
		},
	}

	task := &models.Task{
		Description: "Task description",
		Comments:    comments,
	}

	commentBlock := WriteComments(task)

	task2 := &models.Task{
		Description: "Task description\n\n" + commentBlock,
	}

	ExtractCommentsFromDescription(task2)

	if len(task2.Comments) != len(comments) {
		t.Fatalf("got %d comments, want %d", len(task2.Comments), len(comments))
	}

	for i, got := range task2.Comments {
		want := comments[i]
		if got.Text != want.Text {
			t.Errorf("Comment[%d].Text = %q, want %q", i, got.Text, want.Text)
		}
	}
}

func TestWriteComments_HeadingLikeLinesRoundTrip(t *testing.T) {
	comments := []*models.Comment{
		{
			Text:      "Comment, status: todo\n## Task: pseudo heading\n# Project: embedded project line",
			CreatedAt: time.Date(2025, 3, 4, 9, 0, 0, 0, time.Local),
		},
		{
			Text:      "Literal escaped heading\n\\## Task: not a heading",
			CreatedAt: time.Date(2025, 3, 4, 10, 0, 0, 0, time.Local),
		},
	}

	task := &models.Task{Comments: comments}
	block := WriteComments(task)

	task2 := &models.Task{Description: block}
	ExtractCommentsFromDescription(task2)

	if len(task2.Comments) != len(comments) {
		t.Fatalf("got %d comments, want %d", len(task2.Comments), len(comments))
	}
	for i, want := range comments {
		if got := task2.Comments[i].Text; got != want.Text {
			t.Errorf("Comment[%d].Text = %q, want %q", i, got, want.Text)
		}
	}
	if rewritten := WriteComments(task2); rewritten != block {
		t.Fatalf("comments block did not reach fixed point:\n--- block ---\n%s\n--- rewritten ---\n%s", block, rewritten)
	}
}

func TestWriteComments_PreservesOuterWhitespaceRoundTrip(t *testing.T) {
	comments := []*models.Comment{
		{
			Text:      "    indented code\nnext line has trailing spaces  \n",
			CreatedAt: time.Date(2025, 3, 5, 9, 0, 0, 0, time.Local),
		},
	}

	task := &models.Task{Comments: comments}
	block := WriteComments(task)

	task2 := &models.Task{Description: block}
	ExtractCommentsFromDescription(task2)

	if len(task2.Comments) != 1 {
		t.Fatalf("got %d comments, want 1", len(task2.Comments))
	}
	if got := task2.Comments[0].Text; got != comments[0].Text {
		t.Errorf("Comment.Text = %q, want %q", got, comments[0].Text)
	}
}
