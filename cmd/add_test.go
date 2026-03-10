package cmd

import (
	"testing"
)

func TestParseInlineMetadata(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		wantTitle    string
		wantPriority int
		wantTags     []string
	}{
		{
			name:         "plain title",
			input:        "Fix login bug",
			wantTitle:    "Fix login bug",
			wantPriority: 0,
			wantTags:     nil,
		},
		{
			name:         "title with priority",
			input:        "Fix login bug !1",
			wantTitle:    "Fix login bug",
			wantPriority: 1,
			wantTags:     nil,
		},
		{
			name:         "priority in middle",
			input:        "Fix !2 login bug",
			wantTitle:    "Fix login bug",
			wantPriority: 2,
			wantTags:     nil,
		},
		{
			name:         "title with tag",
			input:        "Fix login bug #backend",
			wantTitle:    "Fix login bug",
			wantPriority: 0,
			wantTags:     []string{"backend"},
		},
		{
			name:         "title with multiple tags",
			input:        "Add feature #backend #urgent",
			wantTitle:    "Add feature",
			wantPriority: 0,
			wantTags:     []string{"backend", "urgent"},
		},
		{
			name:         "title with priority and tags",
			input:        "Critical fix !1 #hotfix #backend",
			wantTitle:    "Critical fix",
			wantPriority: 1,
			wantTags:     []string{"hotfix", "backend"},
		},
		{
			name:         "mixed order",
			input:        "#frontend Fix the button !3 #urgent",
			wantTitle:    "Fix the button",
			wantPriority: 3,
			wantTags:     []string{"frontend", "urgent"},
		},
		{
			name:         "priority at start",
			input:        "!5 Low priority task",
			wantTitle:    "Low priority task",
			wantPriority: 5,
			wantTags:     nil,
		},
		{
			name:         "invalid priority not matched",
			input:        "Fix bug !9",
			wantTitle:    "Fix bug !9",
			wantPriority: 0,
			wantTags:     nil,
		},
		{
			name:         "tag with underscore",
			input:        "Add feature #code_review",
			wantTitle:    "Add feature",
			wantPriority: 0,
			wantTags:     []string{"code_review"},
		},
		{
			name:         "priority 0 is invalid",
			input:        "!0 not matched",
			wantTitle:    "!0 not matched",
			wantPriority: 0,
			wantTags:     nil,
		},
		{
			name:         "priority 6 is invalid",
			input:        "!6 out of range",
			wantTitle:    "!6 out of range",
			wantPriority: 0,
			wantTags:     nil,
		},
		{
			name:         "multiple spaces normalized",
			input:        "Multiple   spaces   here",
			wantTitle:    "Multiple spaces here",
			wantPriority: 0,
			wantTags:     nil,
		},
		{
			name:         "empty input",
			input:        "",
			wantTitle:    "",
			wantPriority: 0,
			wantTags:     nil,
		},
		{
			name:         "only metadata leaves empty title",
			input:        "!1 #tag",
			wantTitle:    "",
			wantPriority: 1,
			wantTags:     []string{"tag"},
		},
		{
			name:         "hash without word not a tag",
			input:        "Fix bug # comment",
			wantTitle:    "Fix bug # comment",
			wantPriority: 0,
			wantTags:     nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotTitle, gotPriority, gotTags := parseInlineMetadata(tt.input)

			if gotTitle != tt.wantTitle {
				t.Errorf("parseInlineMetadata() title = %q, want %q", gotTitle, tt.wantTitle)
			}

			if gotPriority != tt.wantPriority {
				t.Errorf("parseInlineMetadata() priority = %d, want %d", gotPriority, tt.wantPriority)
			}

			if len(gotTags) != len(tt.wantTags) {
				t.Errorf("parseInlineMetadata() tags = %v, want %v", gotTags, tt.wantTags)
				return
			}

			for i, tag := range gotTags {
				if tag != tt.wantTags[i] {
					t.Errorf("parseInlineMetadata() tag[%d] = %q, want %q", i, tag, tt.wantTags[i])
				}
			}
		})
	}
}
