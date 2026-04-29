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

func TestWriteComments_ResolvedRoundTrip(t *testing.T) {
	comments := []*models.Comment{
		{
			Text:      "Unresolved comment",
			CreatedAt: time.Date(2025, 1, 15, 10, 30, 0, 0, time.Local),
			Resolved:  false,
		},
		{
			Text:      "Resolved comment",
			CreatedAt: time.Date(2025, 1, 16, 14, 20, 0, 0, time.Local),
			Resolved:  true,
		},
		{
			Text:      "Another unresolved",
			CreatedAt: time.Date(2025, 1, 17, 9, 15, 0, 0, time.Local),
			Resolved:  false,
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
		if got.Resolved != want.Resolved {
			t.Errorf("Comment[%d].Resolved = %v, want %v", i, got.Resolved, want.Resolved)
		}
		gotTime := got.CreatedAt.Format(util.DateTimeFormat)
		wantTime := want.CreatedAt.Format(util.DateTimeFormat)
		if gotTime != wantTime {
			t.Errorf("Comment[%d].CreatedAt = %s, want %s", i, gotTime, wantTime)
		}
	}
}
