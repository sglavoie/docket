package tui

import "testing"

func TestTaskIDPattern(t *testing.T) {
	tests := []struct {
		query string
		want  bool
	}{
		{"#123", true},
		{"#1", true},
		{"#0", true},
		{"#abc", false},
		{"123", false},
		{"#", false},
		{"# 123", false},
	}

	for _, tc := range tests {
		t.Run(tc.query, func(t *testing.T) {
			got := taskIDPattern.MatchString(tc.query)
			if got != tc.want {
				t.Errorf("taskIDPattern.MatchString(%q) = %v, want %v", tc.query, got, tc.want)
			}
		})
	}
}

func TestInlineTaskPattern(t *testing.T) {
	tests := []struct {
		query     string
		wantMatch bool
		wantTitle string
	}{
		{"+New task", true, "New task"},
		{"+Task with spaces", true, "Task with spaces"},
		{"+", false, ""},
		{"+ ", true, " "},
		{"No plus", false, ""},
	}

	for _, tc := range tests {
		t.Run(tc.query, func(t *testing.T) {
			matches := inlineTaskPattern.FindStringSubmatch(tc.query)
			gotMatch := matches != nil
			if gotMatch != tc.wantMatch {
				t.Errorf("inlineTaskPattern.MatchString(%q) = %v, want %v", tc.query, gotMatch, tc.wantMatch)
			}
			if gotMatch && len(matches) > 1 && matches[1] != tc.wantTitle {
				t.Errorf("captured title = %q, want %q", matches[1], tc.wantTitle)
			}
		})
	}
}
