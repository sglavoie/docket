package format

import (
	"strings"
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/models"
)

func TestWriteComments(t *testing.T) {
	tests := []struct {
		name     string
		comments []*models.Comment
		wantHas  []string
	}{
		{
			name:     "no comments",
			comments: nil,
			wantHas:  nil,
		},
		{
			name:     "only nil comments",
			comments: []*models.Comment{nil},
			wantHas:  nil,
		},
		{
			name: "single comment",
			comments: []*models.Comment{{
				Text:      "This is a comment",
				CreatedAt: time.Date(2025, 1, 15, 10, 30, 0, 0, time.Local),
			}},
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
			comments: []*models.Comment{{
				Text:      "Comment with <!-- and --> markers",
				CreatedAt: time.Date(2025, 1, 15, 10, 30, 0, 0, time.Local),
			}},
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
			task := &models.Task{Comments: tt.comments}

			got := WriteComments(task)

			if len(tt.wantHas) == 0 {
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

func TestWriteComments_Resolved(t *testing.T) {
	t.Run("writes resolved suffix", func(t *testing.T) {
		task := &models.Task{
			Comments: []*models.Comment{{
				Text:      "Resolved comment",
				CreatedAt: time.Date(2025, 1, 15, 10, 30, 0, 0, time.Local),
				Resolved:  true,
			}},
		}

		got := WriteComments(task)
		if !strings.Contains(got, "<!-- comment: 2025-01-15 10:30, resolved -->") {
			t.Errorf("WriteComments() missing resolved suffix\ngot:\n%s", got)
		}
	})

	t.Run("does not write resolved suffix for unresolved", func(t *testing.T) {
		task := &models.Task{
			Comments: []*models.Comment{{
				Text:      "A regular comment",
				CreatedAt: time.Date(2025, 1, 15, 10, 30, 0, 0, time.Local),
				Resolved:  false,
			}},
		}

		got := WriteComments(task)
		if strings.Contains(got, ", resolved") {
			t.Errorf("WriteComments() should not contain ', resolved' suffix\ngot:\n%s", got)
		}
	})
}
