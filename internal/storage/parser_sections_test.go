package storage

import (
	"strings"
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

func TestParseTasksMarkdownWithSections_DefaultNext(t *testing.T) {
	content := `# Project: Test Project

## Task: My Task
<!-- id: task1, status: todo, priority: 3 -->
`
	result, err := ParseTasksMarkdownWithSections(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should default to Next when no sections specified
	if len(result.Sections) != 1 {
		t.Errorf("expected 1 default section, got %d", len(result.Sections))
	}
	if result.Sections[0] != models.SectionNext {
		t.Errorf("expected default section to be Next, got %s", result.Sections[0])
	}
}

func TestParseTasksMarkdownWithSections_ExplicitSections(t *testing.T) {
	content := `# Project: Test Project
<!-- sections: Next;Inbox;Review;Completed -->

## Task: My Task
<!-- id: task1, status: todo, priority: 3 -->
`
	result, err := ParseTasksMarkdownWithSections(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expectedSections := []string{models.SectionNext, models.SectionInbox, models.SectionReview, models.SectionCompleted}
	if len(result.Sections) != len(expectedSections) {
		t.Errorf("expected %d sections, got %d", len(expectedSections), len(result.Sections))
	}

	for i, expected := range expectedSections {
		if i >= len(result.Sections) || result.Sections[i] != expected {
			t.Errorf("section %d: expected %s, got %s", i, expected, result.Sections[i])
		}
	}
}

func TestParseTasksMarkdownWithSections_SectionsAfterTaskIgnored(t *testing.T) {
	content := `# Project: Test Project

## Task: My Task
<!-- id: task1, status: todo, priority: 3 -->

<!-- sections: Next;Inbox;Completed -->
`
	result, err := ParseTasksMarkdownWithSections(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Sections after first task should be ignored, defaulting to Next
	if len(result.Sections) != 1 {
		t.Errorf("expected 1 section (sections after task ignored), got %d", len(result.Sections))
	}
	if result.Sections[0] != models.SectionNext {
		t.Errorf("expected default Next, got %s", result.Sections[0])
	}
}

func TestParseTasksMarkdownWithSections_TaskWithSection(t *testing.T) {
	content := `# Project: Test Project
<!-- sections: Next;Inbox;Completed -->

## Task: My Task
<!-- id: task1, status: todo, priority: 3, section: Next -->

Description here.
`
	result, err := ParseTasksMarkdownWithSections(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(result.Tasks))
	}

	if models.SectionForState(result.Tasks[0].State()) != models.SectionNext {
		t.Errorf("expected task section Next, got '%s'", models.SectionForState(result.Tasks[0].State()))
	}
}

func TestWriteTasksMarkdownWithSections(t *testing.T) {
	task, err := models.NewTask("task1", 1, models.TaskTypeTask, "Test Task")
	if err != nil {
		t.Fatal(err)
	}
	task.SetSection(models.SectionNext)
	tasks := []*models.Task{task}
	sections := []string{models.SectionNext, models.SectionInbox, models.SectionWaiting}

	content := WriteTasksMarkdownWithSections("Test Project", tasks, sections, nil)

	// Check project header
	if !strings.Contains(content, "# Project: Test Project") {
		t.Error("content should contain project header")
	}

	// Check sections metadata
	if !strings.Contains(content, "<!-- sections: Next;Inbox;Waiting -->") {
		t.Error("content should contain sections metadata")
	}

	// v2 collapses the per-task section into the derived `state`; the Next
	// section derives to state=next. The project-level sections metadata above
	// is still written.
	if !strings.Contains(content, "state: next") {
		t.Error("content should contain task state derived from Next section")
	}
	if strings.Contains(content, "section: Next") {
		t.Error("v2 output should not contain the per-task section field")
	}
}

func TestWriteTasksMarkdownWithSections_EmptySections(t *testing.T) {
	task, err := models.NewTask("task1", 1, models.TaskTypeTask, "Test Task")
	if err != nil {
		t.Fatal(err)
	}
	tasks := []*models.Task{task}

	content := WriteTasksMarkdownWithSections("Test Project", tasks, nil, nil)

	// Should not contain sections line when nil
	if strings.Contains(content, "<!-- sections:") {
		t.Error("content should not contain sections metadata when sections is nil")
	}

	content = WriteTasksMarkdownWithSections("Test Project", tasks, []string{}, nil)

	// Should not contain sections line when empty
	if strings.Contains(content, "<!-- sections:") {
		t.Error("content should not contain sections metadata when sections is empty")
	}
}

func TestSectionsRoundTrip(t *testing.T) {
	task1, err := models.NewTask("task1", 1, models.TaskTypeTask, "Task in Next")
	if err != nil {
		t.Fatal(err)
	}
	task1.SetSection(models.SectionNext)
	task2, err := models.NewTask("task2", 2, models.TaskTypeTask, "Task in Inbox")
	if err != nil {
		t.Fatal(err)
	}
	task2.SetSection(models.SectionInbox)
	task3, err := models.NewTask("task3", 3, models.TaskTypeTask, "Task in Waiting")
	if err != nil {
		t.Fatal(err)
	}
	task3.SetSection(models.SectionWaiting)

	tasks := []*models.Task{task1, task2, task3}
	sections := []string{models.SectionNext, models.SectionInbox, models.SectionReview, models.SectionWaiting}

	// Write to markdown
	content := WriteTasksMarkdownWithSections("Round Trip Test", tasks, sections, nil)

	// Parse back
	result, err := ParseTasksMarkdownWithSections(content)
	if err != nil {
		t.Fatalf("failed to parse: %v", err)
	}

	// Check project name
	if result.ProjectName != "Round Trip Test" {
		t.Errorf("project name mismatch: got '%s'", result.ProjectName)
	}

	// Check sections
	if len(result.Sections) != len(sections) {
		t.Errorf("expected %d sections, got %d", len(sections), len(result.Sections))
	}
	for i, expected := range sections {
		if i >= len(result.Sections) || result.Sections[i] != expected {
			t.Errorf("section %d: expected %s, got %s", i, expected, result.Sections[i])
		}
	}

	// Check tasks
	if len(result.Tasks) != 3 {
		t.Fatalf("expected 3 tasks, got %d", len(result.Tasks))
	}

	if models.SectionForState(result.Tasks[0].State()) != models.SectionNext {
		t.Errorf("task1 section should be Next, got %s", models.SectionForState(result.Tasks[0].State()))
	}
	if models.SectionForState(result.Tasks[1].State()) != models.SectionInbox {
		t.Errorf("task2 section should be Inbox, got %s", models.SectionForState(result.Tasks[1].State()))
	}
	if models.SectionForState(result.Tasks[2].State()) != models.SectionWaiting {
		t.Errorf("task3 section should be Waiting, got %s", models.SectionForState(result.Tasks[2].State()))
	}
}

func TestSectionColorsRoundTrip(t *testing.T) {
	task, err := models.NewTask("task1", 1, models.TaskTypeTask, "Test Task")
	if err != nil {
		t.Fatal(err)
	}
	task.SetSection(models.SectionInbox)
	tasks := []*models.Task{task}
	sections := []string{models.SectionNext, models.SectionInbox, models.SectionReview, "Legacy"}
	colors := map[string]string{
		models.SectionInbox:  "green",
		models.SectionReview: "blue",
		"Legacy":             "gray",
	}

	// Write to markdown with colors
	content := WriteTasksMarkdownWithSections("Color Test", tasks, sections, colors)

	// Check that colors are in the output
	if !strings.Contains(content, "Inbox:green") {
		t.Error("content should contain Inbox:green")
	}
	if !strings.Contains(content, "Review:blue") {
		t.Error("content should contain Review:blue")
	}
	if strings.Contains(content, "Legacy:gray") {
		t.Error("content should not contain non-GTD Legacy section color")
	}

	// Parse back
	result, err := ParseTasksMarkdownWithSections(content)
	if err != nil {
		t.Fatalf("failed to parse: %v", err)
	}

	// Check section colors
	if result.SectionColors[models.SectionInbox] != "green" {
		t.Errorf("Inbox color should be green, got %s", result.SectionColors[models.SectionInbox])
	}
	if result.SectionColors[models.SectionReview] != "blue" {
		t.Errorf("Review color should be blue, got %s", result.SectionColors[models.SectionReview])
	}
	if _, ok := result.SectionColors["Legacy"]; ok {
		t.Errorf("Legacy color should be filtered, got %s", result.SectionColors["Legacy"])
	}
}

func TestParseTasksMarkdownWithSections_DeduplicatesSectionNames(t *testing.T) {
	// A sections metadata line containing duplicate names (whether typed by
	// hand or merged from multiple lines) must not produce duplicate entries
	// in the returned Sections slice. Downstream UI assumes unique section
	// names, and a duplicate would render the same tab twice.
	content := `# Project: Dedup Test
<!-- sections: Next;Inbox;Next;Completed -->
<!-- sections: Completed;Review -->

## Task: T
<!-- id: t1, status: todo, priority: 2 -->
`
	result, err := ParseTasksMarkdownWithSections(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := []string{models.SectionNext, models.SectionInbox, models.SectionCompleted, models.SectionReview}
	if len(result.Sections) != len(expected) {
		t.Fatalf("expected %d unique sections, got %d (%v)", len(expected), len(result.Sections), result.Sections)
	}
	for i, want := range expected {
		if result.Sections[i] != want {
			t.Errorf("section %d: expected %q, got %q", i, want, result.Sections[i])
		}
	}
}

func TestParseTasksMarkdownWithSections_FiltersNonGTDSections(t *testing.T) {
	content := `# Project: GTD Filter Test
<!-- sections: Backlog;Next;Active;Inbox;Done;Completed -->

## Task: T
<!-- id: t1, status: todo, priority: 2 -->
`
	result, err := ParseTasksMarkdownWithSections(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := []string{models.SectionNext, models.SectionInbox, models.SectionCompleted}
	if len(result.Sections) != len(expected) {
		t.Fatalf("expected %d sections, got %d (%v)", len(expected), len(result.Sections), result.Sections)
	}
	for i, want := range expected {
		if result.Sections[i] != want {
			t.Errorf("section %d: expected %q, got %q", i, want, result.Sections[i])
		}
	}
}
