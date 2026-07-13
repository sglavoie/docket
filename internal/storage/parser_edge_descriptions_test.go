package storage

import (
	"strings"
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/models"
)

// TestRoundTrip_DescriptionWithHeadingLikeLines verifies that descriptions
// containing lines that resemble task headings survive write→parse without
// creating phantom tasks or corrupting data.
func TestRoundTrip_DescriptionWithHeadingLikeLines(t *testing.T) {
	tests := []struct {
		name string
		desc string
	}{
		{
			name: "task heading in description",
			desc: "See the following example:\n## Task: Example heading in description\nEnd of example",
		},
		{
			name: "epic heading in description",
			desc: "Reference:\n## Epic: Some epic name\nMore text",
		},
		{
			name: "project header in description",
			desc: "Previous project:\n# Project: Old Name\nWas renamed",
		},
		{
			name: "multiple heading-like lines",
			desc: "## Task: First\n### Story: Second\n#### Bug: Third",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tasks := []*models.Task{
				{
					ID:            "t1",
					ShortID:       1,
					Type:          models.TaskTypeTask,
					Title:         "Test Task",
					Description:   tc.desc,
					WorkflowState: models.StateReady,
					Priority:      3,
					Tags:          []string{},
					DependsOn:     []string{},
					RelatedTo:     []string{},
					CreatedAt:     time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
				},
			}

			content := WriteTasksMarkdown("Test", tasks)
			_, parsed, err := ParseTasksMarkdown(content)
			if err != nil {
				t.Fatalf("parse error: %v", err)
			}

			if len(parsed) != 1 {
				t.Fatalf("expected 1 task (not %d) — heading-like line in description created phantom task", len(parsed))
			}

			if parsed[0].Description != tc.desc {
				t.Errorf("description mismatch:\n  got:  %q\n  want: %q", parsed[0].Description, tc.desc)
			}
		})
	}
}

// TestRoundTrip_DescriptionWithLiteralBackslashHash verifies that a description
// starting with literal `\#` does not get incorrectly unescaped on re-parse.
func TestRoundTrip_DescriptionWithLiteralBackslashHash(t *testing.T) {
	// This description has a line starting with \# which is NOT an escaped
	// heading — it's a literal backslash followed by a hashtag. The unescape
	// logic should leave it alone since stripping the backslash does NOT
	// produce a valid heading regex match.
	desc := "Some text\n\\#hashtag is cool\nMore text"

	tasks := []*models.Task{
		{
			ID:            "t1",
			ShortID:       1,
			Type:          models.TaskTypeTask,
			Title:         "Test Task",
			Description:   desc,
			WorkflowState: models.StateReady,
			Priority:      3,
			Tags:          []string{},
			DependsOn:     []string{},
			RelatedTo:     []string{},
			CreatedAt:     time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		},
	}

	content := WriteTasksMarkdown("Test", tasks)
	_, parsed, err := ParseTasksMarkdown(content)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	if len(parsed) != 1 {
		t.Fatalf("expected 1 task, got %d", len(parsed))
	}

	if parsed[0].Description != desc {
		t.Errorf("description mismatch:\n  got:  %q\n  want: %q", parsed[0].Description, desc)
	}
}

// TestRoundTrip_DescriptionEmptyAndWhitespace verifies that descriptions
// that are empty or whitespace-only round-trip correctly as empty.
func TestRoundTrip_DescriptionEmptyAndWhitespace(t *testing.T) {
	tests := []struct {
		name     string
		desc     string
		wantDesc string
	}{
		{"empty", "", ""},
		{"spaces only", "   ", ""},
		{"newlines only", "\n\n\n", ""},
		{"single newline", "\n", ""},
		{"whitespace mixed", "  \n  \n  ", ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tasks := []*models.Task{
				{
					ID:            "t1",
					ShortID:       1,
					Type:          models.TaskTypeTask,
					Title:         "Test Task",
					Description:   tc.desc,
					WorkflowState: models.StateReady,
					Priority:      3,
					Tags:          []string{},
					DependsOn:     []string{},
					RelatedTo:     []string{},
					CreatedAt:     time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
				},
			}

			content := WriteTasksMarkdown("Test", tasks)
			_, parsed, err := ParseTasksMarkdown(content)
			if err != nil {
				t.Fatalf("parse error: %v", err)
			}

			if len(parsed) != 1 {
				t.Fatalf("expected 1 task, got %d", len(parsed))
			}

			if parsed[0].Description != tc.wantDesc {
				t.Errorf("description = %q, want %q", parsed[0].Description, tc.wantDesc)
			}
		})
	}
}

// TestBackwardCompat_NoDescriptionMarkers verifies that old-format markdown
// (no <!-- DESCRIPTION --> markers) is parsed correctly: the description text
// is preserved as-is without any mutation.
func TestBackwardCompat_NoDescriptionMarkers(t *testing.T) {
	content := `# Project: Test

## Task: Old Task
<!-- id: old-1, status: todo, priority: 2 -->

This is a plain description
with multiple lines
and no DESCRIPTION markers.
`

	_, tasks, err := ParseTasksMarkdown(content)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if len(tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(tasks))
	}

	want := "This is a plain description\nwith multiple lines\nand no DESCRIPTION markers."
	if tasks[0].Description != want {
		t.Errorf("description = %q, want %q", tasks[0].Description, want)
	}
}

// TestBackwardCompat_MixedDescriptionMarkers verifies that a file containing
// some tasks with <!-- DESCRIPTION --> markers and some without (old format)
// is parsed correctly — both styles coexist without data loss.
func TestBackwardCompat_MixedDescriptionMarkers(t *testing.T) {
	content := `# Project: Test

## Task: New Task
<!-- id: new-1, status: todo, priority: 1 -->

<!-- DESCRIPTION -->
New-format description.
<!-- /DESCRIPTION -->

## Task: Old Task
<!-- id: old-1, status: todo, priority: 2 -->

Old-format description without markers.
`

	_, tasks, err := ParseTasksMarkdown(content)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if len(tasks) != 2 {
		t.Fatalf("expected 2 tasks, got %d", len(tasks))
	}

	byID := func(id string) *models.Task {
		for _, t := range tasks {
			if t.ID == id {
				return t
			}
		}
		return nil
	}

	newTask := byID("new-1")
	if newTask == nil {
		t.Fatal("new-1 not found")
	}
	if newTask.Description != "New-format description." {
		t.Errorf("new task description = %q, want %q", newTask.Description, "New-format description.")
	}

	oldTask := byID("old-1")
	if oldTask == nil {
		t.Fatal("old-1 not found")
	}
	if oldTask.Description != "Old-format description without markers." {
		t.Errorf("old task description = %q, want %q", oldTask.Description, "Old-format description without markers.")
	}
}

// TestRoundTrip_DescriptionWithBlockMarkers verifies that descriptions
// containing literal block-marker lines (`<!-- COMMENTS -->`, etc.) survive
// write→parse without being consumed by the block extractors and without
// losing content between markers.
func TestRoundTrip_DescriptionWithBlockMarkers(t *testing.T) {
	tests := []struct {
		name string
		desc string
	}{
		{
			name: "comments markers in body",
			desc: "Foo\n<!-- COMMENTS -->\nbar\n<!-- /COMMENTS -->\nBaz",
		},
		{
			name: "activity markers in body",
			desc: "Outer\n<!-- ACTIVITY -->\ninner activity-looking line\n<!-- /ACTIVITY -->\nTail",
		},
		{
			name: "decisions markers in body",
			desc: "Pre\n<!-- DECISIONS -->\nmid\n<!-- /DECISIONS -->\nPost",
		},
		{
			name: "description markers in body",
			desc: "Before\n<!-- DESCRIPTION -->\nshould not nest\n<!-- /DESCRIPTION -->\nAfter",
		},
		{
			name: "mixed block markers",
			desc: "Top\n<!-- COMMENTS -->\nc1\n<!-- /COMMENTS -->\nMid\n<!-- ACTIVITY -->\na1\n<!-- /ACTIVITY -->\nEnd",
		},
		{
			name: "literal backslash before block marker",
			desc: "X\n\\<!-- COMMENTS -->\nY",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tasks := []*models.Task{
				{
					ID:            "t1",
					ShortID:       1,
					Type:          models.TaskTypeTask,
					Title:         "Test Task",
					Description:   tc.desc,
					WorkflowState: models.StateReady,
					Priority:      3,
					Tags:          []string{},
					DependsOn:     []string{},
					RelatedTo:     []string{},
					CreatedAt:     time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
				},
			}

			content := WriteTasksMarkdown("Test", tasks)
			_, parsed, err := ParseTasksMarkdown(content)
			if err != nil {
				t.Fatalf("parse error: %v", err)
			}

			if len(parsed) != 1 {
				t.Fatalf("expected 1 task, got %d", len(parsed))
			}

			if parsed[0].Description != tc.desc {
				t.Errorf("description mismatch:\n  got:  %q\n  want: %q", parsed[0].Description, tc.desc)
			}

			if len(parsed[0].Comments) != 0 {
				t.Errorf("phantom Comments parsed: %+v", parsed[0].Comments)
			}
			if len(parsed[0].Activity) != 0 {
				t.Errorf("phantom Activity parsed: %+v", parsed[0].Activity)
			}
			if len(parsed[0].Decisions) != 0 {
				t.Errorf("phantom Decisions parsed: %+v", parsed[0].Decisions)
			}
		})
	}
}

// TestRoundTrip_DescriptionWithHTMLCommentLikeMetadata verifies that
// descriptions containing lines that look like metadata comments don't
// get incorrectly parsed as task metadata.
func TestRoundTrip_DescriptionWithHTMLCommentLikeMetadata(t *testing.T) {
	desc := "Here is a code example:\n<!-- id: fake-id, status: done, priority: 1 -->\nThis is not metadata."

	tasks := []*models.Task{
		{
			ID:            "t1",
			ShortID:       1,
			Type:          models.TaskTypeTask,
			Title:         "Test Task",
			Description:   desc,
			WorkflowState: models.StateReady,
			Priority:      3,
			Tags:          []string{},
			DependsOn:     []string{},
			RelatedTo:     []string{},
			CreatedAt:     time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		},
	}

	content := WriteTasksMarkdown("Test", tasks)
	_, parsed, err := ParseTasksMarkdown(content)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	if len(parsed) != 1 {
		t.Fatalf("expected 1 task, got %d", len(parsed))
	}

	if parsed[0].ID != "t1" {
		t.Errorf("task ID = %q, want %q — metadata-like line in description corrupted the ID", parsed[0].ID, "t1")
	}

	if !strings.Contains(parsed[0].Description, "<!-- id: fake-id") {
		t.Error("description should preserve the HTML comment-like line")
	}
}

// TestParse_ProjectHeaderInDescriptionDoesNotOverrideName verifies that a
// `# Project: ...` line appearing inside an old-format description (no escape
// applied, e.g. files predating the description-line escape logic, or content
// pasted from another project file) does NOT silently overwrite the project
// name or get consumed from the description. Regression test for a real
// data-loss bug.
func TestParse_ProjectHeaderInDescriptionDoesNotOverrideName(t *testing.T) {
	content := `# Project: Real Name

## Task: Test
<!-- id: t1, status: todo, priority: 3 -->

# Project: Pasted From Elsewhere
Just describing something.
`
	pname, tasks, err := ParseTasksMarkdown(content)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if pname != "Real Name" {
		t.Errorf("project name was overridden by description text: got %q, want %q", pname, "Real Name")
	}
	if len(tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(tasks))
	}
	if !strings.Contains(tasks[0].Description, "# Project: Pasted From Elsewhere") {
		t.Errorf("description should preserve the project-header-like line, got %q", tasks[0].Description)
	}
	if !strings.Contains(tasks[0].Description, "Just describing something.") {
		t.Errorf("description should preserve following lines, got %q", tasks[0].Description)
	}
}
