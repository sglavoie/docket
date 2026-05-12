package tui

import "testing"

func TestParseQuickCaptureBasicMetadata(t *testing.T) {
	runQuickCaptureParseTests(t, []quickCaptureParseTestCase{
		{
			name:         "simple title only",
			input:        "Fix login bug",
			wantTitle:    "Fix login bug",
			wantPriority: 3,
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
			input:        "Cleanup code !5",
			wantTitle:    "Cleanup code",
			wantPriority: 5,
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
			wantPriority: 3,
			wantTags:     []string{"backend"},
		},
		{
			name:         "multiple tags",
			input:        "Fix bug #backend #urgent #hotfix",
			wantTitle:    "Fix bug",
			wantPriority: 3,
			wantTags:     []string{"backend", "urgent", "hotfix"},
		},
		{
			name:         "tags at different positions",
			input:        "#frontend Add button #ui to sidebar",
			wantTitle:    "Add button to sidebar",
			wantPriority: 3,
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
			wantPriority: 3,
			wantTags:     []string{"my-tag"},
		},
		{
			name:         "dotted tag",
			input:        "Update #v2.0 release",
			wantTitle:    "Update release",
			wantPriority: 3,
			wantTags:     []string{"v2.0"},
		},
		{
			name:         "tag with multiple hyphens",
			input:        "Add feature #long-hyphen-tag",
			wantTitle:    "Add feature",
			wantPriority: 3,
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

func TestParseQuickCapturePriorityAbbreviations(t *testing.T) {
	runQuickCaptureParseTests(t, []quickCaptureParseTestCase{
		{name: "!h → P1", input: "Task !h", wantTitle: "Task", wantPriority: 1},
		{name: "!hi → P1", input: "Task !hi", wantTitle: "Task", wantPriority: 1},
		{name: "!c → P1", input: "Task !c", wantTitle: "Task", wantPriority: 1},
		{name: "!cr → P1", input: "Task !cr", wantTitle: "Task", wantPriority: 1},
		{name: "!u → P1", input: "Task !u", wantTitle: "Task", wantPriority: 1},
		{name: "!ur → P1", input: "Task !ur", wantTitle: "Task", wantPriority: 1},
		{name: "!m → P3", input: "Task !m", wantTitle: "Task", wantPriority: 3},
		{name: "!me → P3", input: "Task !me", wantTitle: "Task", wantPriority: 3},
		{name: "!med → P3", input: "Task !med", wantTitle: "Task", wantPriority: 3},
		{name: "!n → P3", input: "Task !n", wantTitle: "Task", wantPriority: 3},
		{name: "!no → P3", input: "Task !no", wantTitle: "Task", wantPriority: 3},
		{name: "!l → P5", input: "Task !l", wantTitle: "Task", wantPriority: 5},
		{name: "!lo → P5", input: "Task !lo", wantTitle: "Task", wantPriority: 5},
		{name: "!lo at start", input: "!lo Task title", wantTitle: "Task title", wantPriority: 5},
		{name: "!h case-insensitive", input: "Task !H", wantTitle: "Task", wantPriority: 1},
	})
}
