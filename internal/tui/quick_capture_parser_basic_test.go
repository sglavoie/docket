package tui

import "testing"

func TestParseQuickCaptureBasicMetadata(t *testing.T) {
	runQuickCaptureParseTests(t, []quickCaptureParseTestCase{
		{
			name:         "simple title only",
			input:        "Fix login bug",
			wantTitle:    "Fix login bug",
			wantPriority: 0,
			wantTags:     nil,
		},
		{
			name:         "title with priority",
			input:        "Fix critical bug !1",
			wantTitle:    "Fix critical bug",
			wantPriority: 1,
			wantTags:     nil,
		},
		{
			name:         "title with low priority",
			input:        "Cleanup code !3",
			wantTitle:    "Cleanup code",
			wantPriority: 3,
			wantTags:     nil,
		},
		{
			name:         "priority at start",
			input:        "!2 Update documentation",
			wantTitle:    "Update documentation",
			wantPriority: 2,
			wantTags:     nil,
		},
		{
			name:         "single tag",
			input:        "Add feature #backend",
			wantTitle:    "Add feature",
			wantPriority: 0,
			wantTags:     []string{"backend"},
		},
		{
			name:         "multiple tags",
			input:        "Fix bug #backend #urgent #hotfix",
			wantTitle:    "Fix bug",
			wantPriority: 0,
			wantTags:     []string{"backend", "urgent", "hotfix"},
		},
		{
			name:         "tags at different positions",
			input:        "#frontend Add button #ui to sidebar",
			wantTitle:    "Add button to sidebar",
			wantPriority: 0,
			wantTags:     []string{"frontend", "ui"},
		},
		{
			name:         "priority and tags combined",
			input:        "Critical fix !1 #hotfix #production",
			wantTitle:    "Critical fix",
			wantPriority: 1,
			wantTags:     []string{"hotfix", "production"},
		},
		{
			name:         "hyphenated tag",
			input:        "Fix bug #my-tag",
			wantTitle:    "Fix bug",
			wantPriority: 0,
			wantTags:     []string{"my-tag"},
		},
		{
			name:         "dotted tag",
			input:        "Update #v2.0 release",
			wantTitle:    "Update release",
			wantPriority: 0,
			wantTags:     []string{"v2-0"},
		},
		{
			name:         "tag with multiple hyphens",
			input:        "Add feature #long-hyphen-tag",
			wantTitle:    "Add feature",
			wantPriority: 0,
			wantTags:     []string{"long-hyphen-tag"},
		},
		{
			name:         "extra whitespace handling",
			input:        "  Fix   the   bug   !2   #backend   ",
			wantTitle:    "Fix the bug",
			wantPriority: 2,
			wantTags:     []string{"backend"},
		},
	})
}
