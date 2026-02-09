package format

import (
	"strings"
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/util"
)

func TestExtractCommentsFromDescription(t *testing.T) {
	tests := []struct {
		name            string
		description     string
		wantDescription string
		wantComments    int
	}{
		{
			name:            "no comments",
			description:     "This is a regular description",
			wantDescription: "This is a regular description",
			wantComments:    0,
		},
		{
			name: "single comment",
			description: `Task description here

<!-- COMMENTS -->
<!-- comment: 2025-01-15 10:30 -->
This is a comment
<!-- /COMMENTS -->`,
			wantDescription: "Task description here",
			wantComments:    1,
		},
		{
			name: "multiple comments",
			description: `Task description

<!-- COMMENTS -->
<!-- comment: 2025-01-15 10:30 -->
First comment
<!-- comment: 2025-01-16 14:20 -->
Second comment
<!-- /COMMENTS -->`,
			wantDescription: "Task description",
			wantComments:    2,
		},
		{
			name: "comment with multiple lines",
			description: `Task description

<!-- COMMENTS -->
<!-- comment: 2025-01-15 10:30 -->
This is a comment
with multiple lines
and some details
<!-- /COMMENTS -->`,
			wantDescription: "Task description",
			wantComments:    1,
		},
		{
			name: "empty comment",
			description: `Task description

<!-- COMMENTS -->
<!-- comment: 2025-01-15 10:30 -->

<!-- /COMMENTS -->`,
			wantDescription: "Task description",
			wantComments:    0, // Empty comments are not added
		},
		{
			name: "comment with HTML escaping",
			description: `Task description

<!-- COMMENTS -->
<!-- comment: 2025-01-15 10:30 -->
Comment with &lt;!-- and --&gt; markers
<!-- /COMMENTS -->`,
			wantDescription: "Task description",
			wantComments:    1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			task := &models.Task{
				Description: tt.description,
			}

			ExtractCommentsFromDescription(task)

			gotDesc := strings.TrimSpace(task.Description)
			wantDesc := strings.TrimSpace(tt.wantDescription)
			if gotDesc != wantDesc {
				t.Errorf("Description = %q, want %q", gotDesc, wantDesc)
			}

			if len(task.Comments) != tt.wantComments {
				t.Errorf("got %d comments, want %d", len(task.Comments), tt.wantComments)
			}
		})
	}
}

func TestExtractCommentsFromDescription_EscapedText(t *testing.T) {
	description := `Task description

<!-- COMMENTS -->
<!-- comment: 2025-01-15 10:30 -->
Comment with &lt;!-- and --&gt; markers
<!-- /COMMENTS -->`

	task := &models.Task{
		Description: description,
	}

	ExtractCommentsFromDescription(task)

	if len(task.Comments) != 1 {
		t.Fatalf("got %d comments, want 1", len(task.Comments))
	}

	// Check that HTML escaping was reversed
	if task.Comments[0].Text != "Comment with <!-- and --> markers" {
		t.Errorf("Comment text = %q, want %q",
			task.Comments[0].Text,
			"Comment with <!-- and --> markers")
	}
}

func TestWriteComments(t *testing.T) {
	tests := []struct {
		name     string
		comments []*models.Comment
		wantHas  []string // Strings that should be in the output
	}{
		{
			name:     "no comments",
			comments: nil,
			wantHas:  nil,
		},
		{
			name: "single comment",
			comments: []*models.Comment{
				{
					Text:      "This is a comment",
					CreatedAt: time.Date(2025, 1, 15, 10, 30, 0, 0, time.Local),
				},
			},
			wantHas: []string{
				"<!-- COMMENTS -->",
				"<!-- comment: 2025-01-15 10:30 -->",
				"This is a comment",
				"<!-- /COMMENTS -->",
			},
		},
		{
			name: "multiple comments",
			comments: []*models.Comment{
				{
					Text:      "First comment",
					CreatedAt: time.Date(2025, 1, 15, 10, 30, 0, 0, time.Local),
				},
				{
					Text:      "Second comment",
					CreatedAt: time.Date(2025, 1, 16, 14, 20, 0, 0, time.Local),
				},
			},
			wantHas: []string{
				"<!-- COMMENTS -->",
				"<!-- comment: 2025-01-15 10:30 -->",
				"First comment",
				"<!-- comment: 2025-01-16 14:20 -->",
				"Second comment",
				"<!-- /COMMENTS -->",
			},
		},
		{
			name: "comment with HTML markers",
			comments: []*models.Comment{
				{
					Text:      "Comment with <!-- and --> markers",
					CreatedAt: time.Date(2025, 1, 15, 10, 30, 0, 0, time.Local),
				},
			},
			wantHas: []string{
				"<!-- COMMENTS -->",
				"<!-- comment: 2025-01-15 10:30 -->",
				"&lt;!--",
				"--&gt;",
				"<!-- /COMMENTS -->",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			task := &models.Task{
				Comments: tt.comments,
			}

			got := WriteComments(task)

			if len(tt.comments) == 0 {
				if got != "" {
					t.Errorf("WriteComments() = %q, want empty string", got)
				}
				return
			}

			for _, want := range tt.wantHas {
				if !strings.Contains(got, want) {
					t.Errorf("WriteComments() missing %q\ngot:\n%s", want, got)
				}
			}
		})
	}
}

func TestWriteComments_RoundTrip(t *testing.T) {
	// Test that writing and extracting comments are inverse operations
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

	// Write comments to string
	commentBlock := WriteComments(task)

	// Create new task with comments block in description
	task2 := &models.Task{
		Description: "Task description\n\n" + commentBlock,
	}

	// Extract comments
	ExtractCommentsFromDescription(task2)

	// Verify comment count
	if len(task2.Comments) != len(comments) {
		t.Fatalf("got %d comments, want %d", len(task2.Comments), len(comments))
	}

	// Verify each comment
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

	// Verify description was cleaned
	if !strings.Contains(task2.Description, "Task description") {
		t.Errorf("Description missing original content: %q", task2.Description)
	}
	if strings.Contains(task2.Description, "<!-- COMMENTS -->") {
		t.Errorf("Description still contains comments block")
	}
}

func TestExtractCommentsFromDescription_Resolved(t *testing.T) {
	t.Run("parses resolved comment header", func(t *testing.T) {
		description := `Task description

<!-- COMMENTS -->
<!-- comment: 2025-01-15 10:30, resolved -->
This is a resolved comment
<!-- /COMMENTS -->`

		task := &models.Task{Description: description}
		ExtractCommentsFromDescription(task)

		if len(task.Comments) != 1 {
			t.Fatalf("got %d comments, want 1", len(task.Comments))
		}
		if !task.Comments[0].Resolved {
			t.Error("expected comment to be resolved")
		}
		if task.Comments[0].Text != "This is a resolved comment" {
			t.Errorf("Comment text = %q, want %q", task.Comments[0].Text, "This is a resolved comment")
		}
	})

	t.Run("old format parses as unresolved", func(t *testing.T) {
		description := `Task description

<!-- COMMENTS -->
<!-- comment: 2025-01-15 10:30 -->
Old format comment
<!-- /COMMENTS -->`

		task := &models.Task{Description: description}
		ExtractCommentsFromDescription(task)

		if len(task.Comments) != 1 {
			t.Fatalf("got %d comments, want 1", len(task.Comments))
		}
		if task.Comments[0].Resolved {
			t.Error("expected old-format comment to be unresolved")
		}
	})

	t.Run("mixed resolved and unresolved", func(t *testing.T) {
		description := `Task description

<!-- COMMENTS -->
<!-- comment: 2025-01-15 10:30 -->
Unresolved comment
<!-- comment: 2025-01-16 14:20, resolved -->
Resolved comment
<!-- comment: 2025-01-17 09:00 -->
Another unresolved
<!-- /COMMENTS -->`

		task := &models.Task{Description: description}
		ExtractCommentsFromDescription(task)

		if len(task.Comments) != 3 {
			t.Fatalf("got %d comments, want 3", len(task.Comments))
		}
		if task.Comments[0].Resolved {
			t.Error("comment 0 should be unresolved")
		}
		if !task.Comments[1].Resolved {
			t.Error("comment 1 should be resolved")
		}
		if task.Comments[2].Resolved {
			t.Error("comment 2 should be unresolved")
		}
	})
}

func TestWriteComments_Resolved(t *testing.T) {
	t.Run("writes resolved suffix", func(t *testing.T) {
		task := &models.Task{
			Comments: []*models.Comment{
				{
					Text:      "Resolved comment",
					CreatedAt: time.Date(2025, 1, 15, 10, 30, 0, 0, time.Local),
					Resolved:  true,
				},
			},
		}

		got := WriteComments(task)
		if !strings.Contains(got, "<!-- comment: 2025-01-15 10:30, resolved -->") {
			t.Errorf("WriteComments() missing resolved suffix\ngot:\n%s", got)
		}
	})

	t.Run("does not write resolved suffix for unresolved", func(t *testing.T) {
		task := &models.Task{
			Comments: []*models.Comment{
				{
					Text:      "A regular comment",
					CreatedAt: time.Date(2025, 1, 15, 10, 30, 0, 0, time.Local),
					Resolved:  false,
				},
			},
		}

		got := WriteComments(task)
		if strings.Contains(got, ", resolved") {
			t.Errorf("WriteComments() should not contain ', resolved' suffix\ngot:\n%s", got)
		}
	})
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

func TestEscapeUnescapeCommentText(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		escaped   string
		roundtrip bool
	}{
		{
			name:      "no special characters",
			input:     "Regular text",
			escaped:   "Regular text",
			roundtrip: true,
		},
		{
			name:      "HTML comment start",
			input:     "Text with <!-- marker",
			escaped:   "Text with &lt;!-- marker",
			roundtrip: true,
		},
		{
			name:      "HTML comment end",
			input:     "Text with --> marker",
			escaped:   "Text with --&gt; marker",
			roundtrip: true,
		},
		{
			name:      "both markers",
			input:     "Text with <!-- and --> markers",
			escaped:   "Text with &lt;!-- and --&gt; markers",
			roundtrip: true,
		},
		{
			name:      "multiple occurrences",
			input:     "<!-- first --> and <!-- second -->",
			escaped:   "&lt;!-- first --&gt; and &lt;!-- second --&gt;",
			roundtrip: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test escaping
			escaped := escapeCommentText(tt.input)
			if escaped != tt.escaped {
				t.Errorf("escapeCommentText() = %q, want %q", escaped, tt.escaped)
			}

			// Test round-trip if specified
			if tt.roundtrip {
				unescaped := unescapeCommentText(escaped)
				if unescaped != tt.input {
					t.Errorf("round-trip failed: got %q, want %q", unescaped, tt.input)
				}
			}
		})
	}
}
