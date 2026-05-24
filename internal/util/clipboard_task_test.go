package util

import (
	"strings"
	"testing"
)

func TestParseClipboard(t *testing.T) {
	tests := []struct {
		name  string
		input string
		title string
		url   string
		desc  string
	}{
		// Empty / whitespace
		{name: "empty string", input: "", title: "", url: "", desc: ""},
		{name: "whitespace only", input: "   \n\t\n  ", title: "", url: "", desc: ""},

		// Single-line plain text
		{name: "plain text", input: "Buy milk", title: "Buy milk", url: "", desc: ""},
		{name: "plain text trimmed", input: "  Buy milk  ", title: "Buy milk", url: "", desc: ""},

		// Single-line bare URL
		{
			name:  "bare https URL",
			input: "https://example.com/path?q=1",
			title: "https://example.com/path?q=1",
			url:   "https://example.com/path?q=1",
			desc:  "",
		},
		{
			name:  "bare http URL",
			input: "http://example.com",
			title: "http://example.com",
			url:   "http://example.com",
			desc:  "",
		},
		{
			name:  "bare file URL",
			input: "file:///home/user/doc.pdf",
			title: "file:///home/user/doc.pdf",
			url:   "file:///home/user/doc.pdf",
			desc:  "",
		},
		{
			name:  "bare URL trimmed",
			input: "  https://example.com  ",
			title: "https://example.com",
			url:   "https://example.com",
			desc:  "",
		},

		// Single-line markdown link
		{
			name:  "markdown link",
			input: "[Example Site](https://example.com)",
			title: "Example Site",
			url:   "https://example.com",
			desc:  "",
		},
		{
			name:  "markdown link with path",
			input: "[Go docs](https://pkg.go.dev/fmt)",
			title: "Go docs",
			url:   "https://pkg.go.dev/fmt",
			desc:  "",
		},
		{
			name:  "markdown link file scheme",
			input: "[Local file](file:///home/user/notes.md)",
			title: "Local file",
			url:   "file:///home/user/notes.md",
			desc:  "",
		},

		// Partial markdown — no URL → treated as plain text
		{
			name:  "plain text with brackets not a link",
			input: "[not a link]",
			title: "[not a link]",
			url:   "",
			desc:  "",
		},

		// Multi-line: plain title + description
		{
			name:  "multi-line plain",
			input: "Buy groceries\nmilk, eggs, bread",
			title: "Buy groceries",
			url:   "",
			desc:  "milk, eggs, bread",
		},
		{
			name:  "multi-line with blank lines in body",
			input: "Task title\n\nFirst paragraph\n\nSecond paragraph",
			title: "Task title",
			url:   "",
			desc:  "First paragraph\n\nSecond paragraph",
		},
		{
			name:  "leading blank lines before title",
			input: "\n\nActual Title\nsome details",
			title: "Actual Title",
			url:   "",
			desc:  "some details",
		},

		// Multi-line: URL in title line
		{
			name:  "multi-line bare URL title",
			input: "https://example.com\nCheck this out",
			title: "https://example.com",
			url:   "https://example.com",
			desc:  "Check this out",
		},
		{
			name:  "multi-line markdown link title",
			input: "[Example](https://example.com)\nGreat resource",
			title: "Example",
			url:   "https://example.com",
			desc:  "Great resource",
		},

		// Title cap at 700 runes
		{
			name:  "title capped at 700",
			input: strings.Repeat("a", 800),
			title: strings.Repeat("a", 700),
			url:   "",
			desc:  "",
		},
		{
			name:  "title exactly 700 not truncated",
			input: strings.Repeat("b", 700),
			title: strings.Repeat("b", 700),
			url:   "",
			desc:  "",
		},

		// Trailing newline normalisation
		{
			name:  "trailing newline stripped",
			input: "Single line\n",
			title: "Single line",
			url:   "",
			desc:  "",
		},

		// CRLF / CR line-ending normalisation (Windows / classic Mac clipboards)
		{
			name:  "CRLF line endings normalised",
			input: "Title line\r\nBody line one\r\nBody line two\r\n",
			title: "Title line",
			url:   "",
			desc:  "Body line one\nBody line two",
		},
		{
			name:  "CR-only line endings normalised",
			input: "Title\rBody\r",
			title: "Title",
			url:   "",
			desc:  "Body",
		},
		{
			name:  "single line with trailing CRLF",
			input: "Just title\r\n",
			title: "Just title",
			url:   "",
			desc:  "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ParseClipboard(tc.input)
			if got.Title != tc.title {
				t.Errorf("Title: got %q, want %q", got.Title, tc.title)
			}
			if got.URL != tc.url {
				t.Errorf("URL: got %q, want %q", got.URL, tc.url)
			}
			if got.Description != tc.desc {
				t.Errorf("Description: got %q, want %q", got.Description, tc.desc)
			}
		})
	}
}
