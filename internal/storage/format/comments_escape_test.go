package format

import "testing"

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
		{
			name:      "pre-existing entity-like &lt;!--",
			input:     "Use &lt;!-- for HTML comments",
			escaped:   "Use &amp;lt;!-- for HTML comments",
			roundtrip: true,
		},
		{
			name:      "pre-existing &amp; entity",
			input:     "Use &amp; for ampersands",
			escaped:   "Use &amp;amp; for ampersands",
			roundtrip: true,
		},
		{
			name:      "bare ampersand",
			input:     "Tom & Jerry",
			escaped:   "Tom &amp; Jerry",
			roundtrip: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			escaped := escapeCommentText(tt.input)
			if escaped != tt.escaped {
				t.Errorf("escapeCommentText() = %q, want %q", escaped, tt.escaped)
			}

			if tt.roundtrip {
				unescaped := unescapeCommentText(escaped)
				if unescaped != tt.input {
					t.Errorf("round-trip failed: got %q, want %q", unescaped, tt.input)
				}
			}
		})
	}
}
