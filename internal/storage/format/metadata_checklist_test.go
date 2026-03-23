package format

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

// TestRoundTrip_ChecklistAdversarial verifies that checklist items containing
// special characters survive a write→parse round-trip without corruption.
func TestRoundTrip_ChecklistAdversarial(t *testing.T) {
	tests := []struct {
		name  string
		items []models.ChecklistItem
	}{
		{
			name: "semicolon in text",
			items: []models.ChecklistItem{
				{Text: "buy milk; eggs; bread", Done: false},
				{Text: "done; truly", Done: true},
			},
		},
		{
			name: "done marker in text",
			items: []models.ChecklistItem{
				{Text: "check [x] box", Done: false},
				{Text: "unchecked [ ] item", Done: true},
			},
		},
		{
			name: "backslash in text",
			items: []models.ChecklistItem{
				{Text: `path\to\file`, Done: false},
				{Text: `escaped \; semicolon`, Done: true},
			},
		},
		{
			name: "pipe delimiter in text",
			items: []models.ChecklistItem{
				{Text: "a || b", Done: false},
				{Text: "col1||col2", Done: true},
			},
		},
		{
			name: "html comment in text",
			items: []models.ChecklistItem{
				{Text: "<!-- not a comment -->", Done: false},
				{Text: "close with -->", Done: true},
			},
		},
		{
			name: "mixed special chars",
			items: []models.ChecklistItem{
				{Text: `a\;b[x]c[ ]d||e<!-- f -->`, Done: false},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			task := &models.Task{
				ID:        "cl-test",
				Type:      models.TaskTypeTask,
				Status:    models.StatusTodo,
				Priority:  1,
				Checklist: tc.items,
			}

			metaStr := WriteMetadata(task)
			content := metaStr[5 : len(metaStr)-4]
			parsed := &models.Task{}
			warnings := ParseMetadata(parsed, content)
			if len(warnings) != 0 {
				t.Errorf("got %d warnings: %v", len(warnings), warnings)
			}

			if len(parsed.Checklist) != len(tc.items) {
				t.Fatalf("got %d checklist items, want %d", len(parsed.Checklist), len(tc.items))
			}
			for i, want := range tc.items {
				got := parsed.Checklist[i]
				if got.Text != want.Text {
					t.Errorf("item[%d].Text = %q, want %q", i, got.Text, want.Text)
				}
				if got.Done != want.Done {
					t.Errorf("item[%d].Done = %v, want %v", i, got.Done, want.Done)
				}
			}
		})
	}
}
