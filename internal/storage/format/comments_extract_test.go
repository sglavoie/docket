package format

import (
	"strings"
	"testing"

	"github.com/sglavoie/tt/internal/models"
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
			wantComments:    0,
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
			task := &models.Task{Description: tt.description}

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

	task := &models.Task{Description: description}

	ExtractCommentsFromDescription(task)

	if len(task.Comments) != 1 {
		t.Fatalf("got %d comments, want 1", len(task.Comments))
	}

	if task.Comments[0].Text != "Comment with <!-- and --> markers" {
		t.Errorf("Comment text = %q, want %q",
			task.Comments[0].Text,
			"Comment with <!-- and --> markers")
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