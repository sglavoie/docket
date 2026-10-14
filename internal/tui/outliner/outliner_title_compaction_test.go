package outliner

import (
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/models"
)

func TestCompactRowTitle(t *testing.T) {
	const home = "/Users/me"
	tests := []struct {
		name, title, want string
	}{
		{"url keeps host and first segment", "Reply https://www.slack.com/archives/C024BE91L/p1700000000", "Reply slack.com/archives/…"},
		{"url with one segment", "Read http://example.com/post", "Read example.com/post"},
		{"url with a query", "See https://example.com/search?q=tt", "See example.com/search/…"},
		{"bare host", "Open https://example.com", "Open example.com"},
		{"wrapped url keeps its punctuation", "Notes (https://github.com/sglavoie/tt/issues/12).", "Notes (github.com/sglavoie/…)."},
		{"home path keeps last segment", "Clean /Users/me/Downloads/old/report.pdf", "Clean ~/…/report.pdf"},
		{"home path directly in home", "Edit /Users/me/notes.md", "Edit ~/notes.md"},
		{"path outside home is left alone", "Read /etc/hosts/file", "Read /etc/hosts/file"},
		{"prefix sharing home is not home", "Open /Users/meg/file.txt", "Open /Users/meg/file.txt"},
		{"plain text is left alone", "Buy milk and bread", "Buy milk and bread"},
		{"not a web scheme", "Mail mailto:me@example.com", "Mail mailto:me@example.com"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := compactRowTitle(tc.title, home); got != tc.want {
				t.Fatalf("compactRowTitle(%q) = %q, want %q", tc.title, got, tc.want)
			}
		})
	}
	if got := compactRowTitle("Edit /Users/me/a/b.md", ""); got != "Edit /Users/me/a/b.md" {
		t.Fatalf("an unknown home must compact no path, got %q", got)
	}
}

func TestRenderTaskTitleCompactsOnlyAnOverWideTitle(t *testing.T) {
	const url = "Reply https://www.slack.com/archives/C024BE91L/p1700000000"
	tests := []struct {
		name, title string
		rowWidth    int
		want        string
	}{
		{"a title that fits is left alone", url, lipgloss.Width(url) + 1, url},
		{"an over-wide url is compacted", url, 40, "Reply slack.com/archives/…"},
		{"a compacted title can still need a cut", url, 15, "Reply slack.c…"},
		{"an over-wide home path is compacted", "Clean /Users/me/Downloads/old/report.pdf", 30, "Clean ~/…/report.pdf"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m := NewOutlinerModel(nil, nil, nil)
			m.homeDir = "/Users/me"
			row := OutlinerRow{Task: &models.Task{Title: tc.title}}
			got := ansi.Strip(m.renderTaskTitle(row, lipgloss.NewStyle(), tc.rowWidth, taskRowSegments{}))
			if got != tc.want {
				t.Fatalf("renderTaskTitle(%q, %d) = %q, want %q", tc.title, tc.rowWidth, got, tc.want)
			}
		})
	}
}
