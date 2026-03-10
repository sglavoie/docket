package storage

import (
	"strings"
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

func TestParseTasksMarkdownWithSections_DefaultBacklog(t *testing.T) {
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
<!-- sections: Backlog;Active;Review;Done -->

## Task: My Task
<!-- id: task1, status: todo, priority: 3 -->
`
	result, err := ParseTasksMarkdownWithSections(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expectedSections := []string{"Backlog", "Active", "Review", "Done"}
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

<!-- sections: Backlog;Active;Done -->
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
<!-- sections: Backlog;Active;Done -->

## Task: My Task
<!-- id: task1, status: todo, priority: 3, section: Active -->

Description here.
`
	result, err := ParseTasksMarkdownWithSections(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(result.Tasks))
	}

	if result.Tasks[0].CustomSection != "Active" {
		t.Errorf("expected task custom section 'Active', got '%s'", result.Tasks[0].CustomSection)
	}
}

func TestWriteTasksMarkdownWithSections(t *testing.T) {
	task := models.NewTask("task1", 1, models.TaskTypeTask, "Test Task")
	task.CustomSection = "Active"
	tasks := []*models.Task{task}
	sections := []string{"Backlog", "Active", "Done"}

	content := WriteTasksMarkdownWithSections("Test Project", tasks, sections, nil)

	// Check project header
	if !strings.Contains(content, "# Project: Test Project") {
		t.Error("content should contain project header")
	}

	// Check sections metadata
	if !strings.Contains(content, "<!-- sections: Backlog;Active;Done -->") {
		t.Error("content should contain sections metadata")
	}

	// Check task custom section
	if !strings.Contains(content, "custom_section: Active") {
		t.Error("content should contain task custom_section")
	}
}

func TestWriteTasksMarkdownWithSections_EmptySections(t *testing.T) {
	task := models.NewTask("task1", 1, models.TaskTypeTask, "Test Task")
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
	task1 := models.NewTask("task1", 1, models.TaskTypeTask, "Task in Active")
	task1.CustomSection = "Active"
	task2 := models.NewTask("task2", 2, models.TaskTypeTask, "Task in Backlog")
	task2.CustomSection = "Backlog"
	task3 := models.NewTask("task3", 3, models.TaskTypeTask, "Task in Done")
	task3.CustomSection = "Done"

	tasks := []*models.Task{task1, task2, task3}
	sections := []string{"Backlog", "Active", "Review", "Done"}

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

	if result.Tasks[0].CustomSection != "Active" {
		t.Errorf("task1 custom section should be Active, got %s", result.Tasks[0].CustomSection)
	}
	if result.Tasks[1].CustomSection != "Backlog" {
		t.Errorf("task2 custom section should be Backlog, got %s", result.Tasks[1].CustomSection)
	}
	if result.Tasks[2].CustomSection != "Done" {
		t.Errorf("task3 custom section should be Done, got %s", result.Tasks[2].CustomSection)
	}
}

func TestSectionColorsRoundTrip(t *testing.T) {
	task := models.NewTask("task1", 1, models.TaskTypeTask, "Test Task")
	task.CustomSection = "Active"
	tasks := []*models.Task{task}
	sections := []string{"Backlog", "Active", "Done"}
	colors := map[string]string{
		"Active":  "green",
		"Done":    "blue",
		"Backlog": "gray",
	}

	// Write to markdown with colors
	content := WriteTasksMarkdownWithSections("Color Test", tasks, sections, colors)

	// Check that colors are in the output
	if !strings.Contains(content, "Active:green") {
		t.Error("content should contain Active:green")
	}
	if !strings.Contains(content, "Done:blue") {
		t.Error("content should contain Done:blue")
	}
	// Gray should now be preserved in the output
	if !strings.Contains(content, "Backlog:gray") {
		t.Error("content should contain Backlog:gray (gray color should be preserved)")
	}

	// Parse back
	result, err := ParseTasksMarkdownWithSections(content)
	if err != nil {
		t.Fatalf("failed to parse: %v", err)
	}

	// Check section colors
	if result.SectionColors["Active"] != "green" {
		t.Errorf("Active color should be green, got %s", result.SectionColors["Active"])
	}
	if result.SectionColors["Done"] != "blue" {
		t.Errorf("Done color should be blue, got %s", result.SectionColors["Done"])
	}
	if result.SectionColors["Backlog"] != "gray" {
		t.Errorf("Backlog color should be gray, got %s", result.SectionColors["Backlog"])
	}
}
