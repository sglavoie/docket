package storage

import (
	"strings"
	"testing"
)

func TestEscapeDescription(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "no heading pattern",
			input: "Normal description text",
			want:  "Normal description text",
		},
		{
			name:  "task heading",
			input: "## Task: Fake heading",
			want:  `\## Task: Fake heading`,
		},
		{
			name:  "epic heading",
			input: "## Epic: Not real",
			want:  `\## Epic: Not real`,
		},
		{
			name:  "story heading deeper level",
			input: "### Story: Deep story",
			want:  `\### Story: Deep story`,
		},
		{
			name:  "project title",
			input: "# Project: Embedded",
			want:  `\# Project: Embedded`,
		},
		{
			name:  "multiline with heading in middle",
			input: "Before\n## Bug: Not a bug\nAfter",
			want:  "Before\n\\## Bug: Not a bug\nAfter",
		},
		{
			name:  "hash without task type not escaped",
			input: "## Not a heading type",
			want:  "## Not a heading type",
		},
		{
			name:  "single hash without Project not escaped",
			input: "# Just a heading",
			want:  "# Just a heading",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := escapeDescription(tc.input)
			if got != tc.want {
				t.Errorf("escapeDescription(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

func TestUnescapeDescriptionLine(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "no escape needed",
			input: "Normal line",
			want:  "Normal line",
		},
		{
			name:  "escaped task heading",
			input: `\## Task: Fake heading`,
			want:  "## Task: Fake heading",
		},
		{
			name:  "escaped project title",
			input: `\# Project: Embedded`,
			want:  "# Project: Embedded",
		},
		{
			name:  "backslash not before heading pattern",
			input: `\Some other text`,
			want:  `\Some other text`,
		},
		{
			name:  "backslash-hash but not a heading pattern",
			input: `\## Not a heading type`,
			want:  `\## Not a heading type`,
		},
		{
			name:  "empty string",
			input: "",
			want:  "",
		},
		{
			name:  "single char",
			input: `\`,
			want:  `\`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := unescapeDescriptionLine(tc.input)
			if got != tc.want {
				t.Errorf("unescapeDescriptionLine(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

func TestEscapeUnescapeRoundTrip(t *testing.T) {
	descs := []string{
		"## Task: Something",
		"### Story: A story",
		"# Project: My Project",
		"Normal text\n## Epic: Fake\nMore text",
		"## Not a heading type",
		"No special content",
		"",
	}

	for _, desc := range descs {
		escaped := escapeDescription(desc)
		// Unescape line by line (as the parser does)
		lines := splitLines(escaped)
		for i, line := range lines {
			lines[i] = unescapeDescriptionLine(line)
		}
		got := joinLines(lines)
		if got != desc {
			t.Errorf("round-trip failed:\n  input:   %q\n  escaped: %q\n  got:     %q", desc, escaped, got)
		}
	}
}

// splitLines splits on newlines preserving the count (no trailing empty removal).
func splitLines(s string) []string {
	if s == "" {
		return []string{""}
	}
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	lines = append(lines, s[start:])
	return lines
}

func joinLines(lines []string) string {
	if len(lines) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString(lines[0])
	for _, line := range lines[1:] {
		b.WriteByte('\n')
		b.WriteString(line)
	}
	return b.String()
}
