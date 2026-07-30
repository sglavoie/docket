package tui

import (
	"testing"
)

func TestParseQuickCaptureEmbeddedTokenLiterals(t *testing.T) {
	runQuickCaptureParseTests(t, []quickCaptureParseTestCase{
		{
			name:         "slash in title without space should not trigger section",
			input:        "research/development task",
			wantTitle:    "research/development task",
			wantPriority: 3,
			wantSection:  "",
		},
		{
			name:         "slash in title with other metadata",
			input:        "word1/word2 !2 #backend",
			wantTitle:    "word1/word2",
			wantPriority: 2,
			wantTags:     []string{"backend"},
			wantSection:  "",
		},
		{
			name:         "space before slash remains title text",
			input:        "Fix bug /Next",
			wantTitle:    "Fix bug /Next",
			wantPriority: 3,
		},
		{
			name:         "embedded hash should not trigger tag",
			input:        "Fix C#code issue",
			wantTitle:    "Fix C#code issue",
			wantPriority: 3,
			wantTags:     nil,
		},
		{
			name:         "embedded exclamation should not trigger priority",
			input:        "Fix wow!1 bug",
			wantTitle:    "Fix wow!1 bug",
			wantPriority: 3,
		},
		{
			name:         "embedded at should not trigger date",
			input:        "Email user@tomorrow.com",
			wantTitle:    "Email user@tomorrow.com",
			wantPriority: 3,
		},
		{
			name:         "embedded tilde should not trigger recurrence",
			input:        "Path to~daily file",
			wantTitle:    "Path to~daily file",
			wantPriority: 3,
			wantHasRecur: false,
		},
		{
			name:         "embedded caret remains title text",
			input:        "Fix regex a^bug pattern",
			wantTitle:    "Fix regex a^bug pattern",
			wantPriority: 3,
		},
		{
			name:         "embedded dollar should not trigger status",
			input:        "Cost is 5$start today",
			wantTitle:    "Cost is 5$start today",
			wantPriority: 3,
			wantStatus:   "",
		},
		{
			name:         "embedded greater than should not trigger parent",
			input:        "Value a>1 check",
			wantTitle:    "Value a>1 check",
			wantPriority: 3,
			wantParentID: nil,
		},
		{
			name:         "all section sigils remain title text",
			input:        "Fix bug /Next /Backlog",
			wantTitle:    "Fix bug /Next /Backlog",
			wantPriority: 3,
		},
	})
}
