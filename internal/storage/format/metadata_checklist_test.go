package format

import (
	"testing"
	"time"

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
			name: "metadata key pattern after comma",
			items: []models.ChecklistItem{
				{Text: "verify parser, stage: review", Done: false},
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

func TestChecklistRoundtripWithSemicolon(t *testing.T) {
	task := &models.Task{
		ID:       "abc",
		Status:   models.StatusTodo,
		Priority: 1,
		Checklist: []models.ChecklistItem{
			{Text: "Buy eggs; milk; bread", Done: false},
			{Text: "Normal item", Done: true},
			{Text: "Has [x] in text", Done: false},
			{Text: "Has [ ] in text", Done: true},
		},
		CreatedAt: time.Date(2025, 1, 1, 0, 0, 0, 0, time.Local),
	}

	meta := WriteMetadata(task)
	metaStr := meta[5 : len(meta)-4]

	parsed := &models.Task{}
	ParseMetadata(parsed, metaStr)

	if len(parsed.Checklist) != len(task.Checklist) {
		t.Fatalf("expected %d checklist items, got %d", len(task.Checklist), len(parsed.Checklist))
	}

	for i, want := range task.Checklist {
		got := parsed.Checklist[i]
		if got.Text != want.Text {
			t.Errorf("item %d text: got %q, want %q", i, got.Text, want.Text)
		}
		if got.Done != want.Done {
			t.Errorf("item %d done: got %v, want %v", i, got.Done, want.Done)
		}
	}
}

func TestChecklistEscapeUnescape(t *testing.T) {
	tests := []struct {
		input string
	}{
		{"simple text"},
		{"has;semicolon"},
		{"has, comma"},
		{"has [x] marker"},
		{"has [ ] marker"},
		{`has\backslash`},
		{"combo; [x] and [ ] all;together"},
	}

	for _, tc := range tests {
		escaped := escapeChecklistText(tc.input)
		unescaped := unescapeChecklistText(escaped)
		if unescaped != tc.input {
			t.Errorf("roundtrip failed: input=%q, escaped=%q, unescaped=%q", tc.input, escaped, unescaped)
		}
	}
}
