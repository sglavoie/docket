package storage

import (
	"strings"
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

func TestParseTasksMarkdown_BrokenMetadataComment(t *testing.T) {
	tests := []struct {
		name    string
		content string
		desc    string
	}{
		{
			name: "unclosed HTML comment",
			content: `# Project: Test

## Task: Task With Unclosed Comment
<!-- id: t1, status: todo, priority: 2

Some description that follows.

## Task: Normal Task
<!-- id: t2, status: todo, priority: 3 -->
`,
			desc: "unclosed comment should not crash parser",
		},
		{
			name: "truncated comment opener",
			content: `# Project: Test

## Task: Task With Bad Opener
<!- id: t1, status: todo, priority: 2 -->

## Task: Normal Task
<!-- id: t2, status: todo, priority: 3 -->
`,
			desc: "truncated opener should not crash parser",
		},
		{
			name: "nested HTML comments",
			content: `# Project: Test

## Task: Nested Comments
<!-- id: t1, status: todo, priority: 2 -->

Description with <!-- nested --> comment markers.

## Task: After Nested
<!-- id: t2, status: done, priority: 1 -->
`,
			desc: "nested comment markers in description",
		},
		{
			name: "only closing comment tag",
			content: `# Project: Test

## Task: Just Closing
--> id: t1, status: todo

## Task: Normal After
<!-- id: t2, status: todo, priority: 3 -->
`,
			desc: "only closing tag should not crash parser",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, tasks, err := ParseTasksMarkdown(tc.content)
			if err != nil {
				t.Fatalf("parser should not return error for %s: %v", tc.desc, err)
			}

			if len(tasks) < 1 {
				t.Fatalf("expected at least 1 task for %s, got 0", tc.desc)
			}

			var foundNormal bool
			for _, task := range tasks {
				if task.ID == "t2" {
					foundNormal = true
				}
			}
			if !foundNormal {
				t.Errorf("expected to find task t2 in %s", tc.desc)
			}
		})
	}
}

func TestParseTasksMarkdown_LineExceedingScannerBuffer(t *testing.T) {
	longLine := strings.Repeat("x", 2*1024*1024)
	content := "# Project: Test\n\n## Task: Normal\n<!-- id: t1, status: todo, priority: 3 -->\n\n" + longLine + "\n"

	_, err := ParseTasksMarkdownWithSections(content)
	if err == nil {
		t.Fatal("expected error for line exceeding scanner buffer, got nil")
	}
	if !strings.Contains(err.Error(), "parse") {
		t.Errorf("expected parse error, got: %v", err)
	}
}

func TestParseTasksMarkdown_CommentMarkerInDescription(t *testing.T) {
	content := `# Project: Test

## Task: Has Comment Markers
<!-- id: t1, status: todo, priority: 2 -->

This description has <!-- an HTML comment --> embedded in it.
And another <!-- partial marker without closing.
Plus a real code example: <!-- TODO: fix this -->

## Task: After Comment Markers
<!-- id: t2, status: in_progress, priority: 1 -->

Normal description.
`
	result, err := ParseTasksMarkdownWithSections(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Tasks) != 2 {
		t.Fatalf("expected 2 tasks, got %d", len(result.Tasks))
	}

	t1 := result.Tasks[0]
	if t1.ID != "t1" {
		t.Errorf("expected first task ID 't1', got %q", t1.ID)
	}
	if !strings.Contains(t1.Description, "<!-- an HTML comment -->") {
		t.Error("expected description to preserve comment markers")
	}

	t2 := result.Tasks[1]
	if t2.ID != "t2" {
		t.Errorf("expected second task ID 't2', got %q", t2.ID)
	}
	if t2.Status != models.StatusInProgress {
		t.Errorf("expected status in_progress, got %q", t2.Status)
	}
}
