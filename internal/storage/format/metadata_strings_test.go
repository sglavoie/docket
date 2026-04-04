package format

import (
	"testing"

	"github.com/sglavoie/tt/internal/util"
)

func TestParseTimeField(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		wantOk  bool
		wantStr string
	}{
		{
			name:    "datetime format",
			value:   "2025-01-15 10:30",
			wantOk:  true,
			wantStr: "2025-01-15 10:30",
		},
		{
			name:    "date only format",
			value:   "2025-01-15",
			wantOk:  true,
			wantStr: "2025-01-15 00:00",
		},
		{
			name:   "invalid format",
			value:  "not-a-date",
			wantOk: false,
		},
		{
			name:   "empty string",
			value:  "",
			wantOk: false,
		},
		{
			name:   "partial date",
			value:  "2025-01",
			wantOk: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := ParseTimeField(tt.value)
			if ok != tt.wantOk {
				t.Errorf("ParseTimeField(%q) ok = %v, want %v", tt.value, ok, tt.wantOk)
			}
			if tt.wantOk {
				gotStr := got.Format(util.DateTimeFormat)
				if gotStr != tt.wantStr {
					t.Errorf("ParseTimeField(%q) = %q, want %q", tt.value, gotStr, tt.wantStr)
				}
			}
		})
	}
}

func TestEscapeUnescapeNewlines(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"no newlines", "simple text"},
		{"single newline", "line1\nline2"},
		{"multiple newlines", "line1\nline2\nline3"},
		{"newline at start", "\ntext"},
		{"newline at end", "text\n"},
		{"consecutive newlines", "text\n\nmore"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			escaped := escapeNewlines(tc.input)
			unescaped := unescapeNewlines(escaped)
			if unescaped != tc.input {
				t.Errorf("roundtrip failed: input=%q, escaped=%q, unescaped=%q", tc.input, escaped, unescaped)
			}
		})
	}
}
