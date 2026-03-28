package tui

import (
	"testing"
)

func TestParseSearchQuery_HyphenatedDottedTags(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantTags []string
		wantText string
	}{
		{
			name:     "simple tag",
			input:    "#backend",
			wantTags: []string{"backend"},
			wantText: "",
		},
		{
			name:     "hyphenated tag",
			input:    "#my-tag",
			wantTags: []string{"my-tag"},
			wantText: "",
		},
		{
			name:     "dotted tag",
			input:    "#v2.0",
			wantTags: []string{"v2.0"},
			wantText: "",
		},
		{
			name:     "hyphen and dot tag",
			input:    "#my-app.v2",
			wantTags: []string{"my-app.v2"},
			wantText: "",
		},
		{
			name:     "tag with text",
			input:    "search text #my-tag",
			wantTags: []string{"my-tag"},
			wantText: "search text",
		},
		{
			name:     "multiple hyphenated tags",
			input:    "#front-end #back-end",
			wantTags: []string{"front-end", "back-end"},
			wantText: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseSearchQuery(tt.input)

			if len(result.Tags) != len(tt.wantTags) {
				t.Fatalf("ParseSearchQuery(%q).Tags = %v, want %v", tt.input, result.Tags, tt.wantTags)
			}
			for i, tag := range result.Tags {
				if tag != tt.wantTags[i] {
					t.Errorf("ParseSearchQuery(%q).Tags[%d] = %q, want %q", tt.input, i, tag, tt.wantTags[i])
				}
			}

			if result.TextQuery != tt.wantText {
				t.Errorf("ParseSearchQuery(%q).TextQuery = %q, want %q", tt.input, result.TextQuery, tt.wantText)
			}
		})
	}
}
