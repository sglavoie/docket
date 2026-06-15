package storage

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage/format"
	"github.com/sglavoie/tt/internal/testutil"
)

func TestParseTasksMarkdownWithSections_ProjectTags(t *testing.T) {
	content := `# Project: Test Project
<!-- sections: Backlog;Active -->
<!-- tags: backend; #frontend ; backend; urgent -->

## Task: My Task
<!-- id: task1, status: todo, priority: 3 -->
`

	result, err := ParseTasksMarkdownWithSections(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := []string{"backend", "frontend", "urgent"}
	if !slices.Equal(result.Tags, want) {
		t.Fatalf("result.Tags = %v, want %v", result.Tags, want)
	}
}

func TestParseTasksMarkdownWithSections_ProjectTagsAfterTaskIgnored(t *testing.T) {
	content := `# Project: Test Project

## Task: My Task
<!-- id: task1, status: todo, priority: 3 -->

<!-- tags: backend -->
`

	result, err := ParseTasksMarkdownWithSections(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Tags) != 0 {
		t.Fatalf("result.Tags = %v, want empty", result.Tags)
	}
}

func TestParseTasksMarkdownWithSections_InvalidProjectTagsWarnAndSkip(t *testing.T) {
	content := `# Project: Test Project
<!-- tags: backend;bad,tag;#;frontend -->

## Task: My Task
<!-- id: task1, status: todo, priority: 3 -->
`

	result, err := ParseTasksMarkdownWithSections(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := []string{"backend", "frontend"}
	if !slices.Equal(result.Tags, want) {
		t.Fatalf("result.Tags = %v, want %v", result.Tags, want)
	}

	tagWarnings := 0
	for _, warning := range result.Warnings {
		if warning.Type == format.WarningValidation && warning.Field == "tags" {
			tagWarnings++
		}
	}
	if tagWarnings != 2 {
		t.Fatalf("tag validation warnings = %d, want 2; warnings: %v", tagWarnings, result.Warnings)
	}
}

func TestWriteTasksMarkdownWithProjectMetadata_WritesTags(t *testing.T) {
	task, err := models.NewTask("task1", 1, models.TaskTypeTask, "Test Task")
	if err != nil {
		t.Fatal(err)
	}

	content := WriteTasksMarkdownWithProjectMetadata(
		"Test Project",
		[]*models.Task{task},
		[]string{"Backlog", "Active"},
		map[string]string{"Backlog": "gray"},
		[]string{" backend ", "#frontend", "backend", "bad,tag"},
	)

	if !strings.Contains(content, "<!-- sections: Backlog:gray;Active -->") {
		t.Fatalf("content should contain sections metadata:\n%s", content)
	}
	if !strings.Contains(content, "<!-- tags: backend;frontend -->") {
		t.Fatalf("content should contain normalized tag metadata:\n%s", content)
	}
	if strings.Contains(content, "bad,tag") {
		t.Fatalf("content should skip invalid project tags:\n%s", content)
	}
	if strings.Index(content, "<!-- tags:") < strings.Index(content, "<!-- sections:") {
		t.Fatalf("tags metadata should be written after sections metadata:\n%s", content)
	}
}

func TestSaveProject_WithProjectTags(t *testing.T) {
	dir, cleanup := testutil.TempProjectDir(t)
	defer cleanup()

	project := testutil.NewTestProject("Tags Test")
	project.Path = filepath.Join(dir, "tags-test")
	project.Tags = []string{"backend", "zero-use"}

	task, err := models.NewTask("task-1", 1, models.TaskTypeTask, "Tagged Task")
	if err != nil {
		t.Fatal(err)
	}
	task.Tags = []string{"backend"}
	project.Tasks = []*models.Task{task}

	if err := SaveProject(project); err != nil {
		t.Fatalf("SaveProject failed: %v", err)
	}

	tasksPath := filepath.Join(project.Path, "tasks.md")
	content, err := os.ReadFile(tasksPath)
	if err != nil {
		t.Fatalf("ReadFile(%s): %v", tasksPath, err)
	}
	if !strings.Contains(string(content), "<!-- tags: backend;zero-use -->") {
		t.Fatalf("saved content should contain project tags metadata:\n%s", content)
	}

	loadedProject := testutil.NewTestProject("Tags Test")
	loadedProject.Path = project.Path
	if err := LoadProject(loadedProject); err != nil {
		t.Fatalf("LoadProject failed: %v", err)
	}

	if !slices.Equal(loadedProject.Tags, []string{"backend", "zero-use"}) {
		t.Fatalf("loadedProject.Tags = %v, want [backend zero-use]", loadedProject.Tags)
	}
	if !slices.Equal(loadedProject.AllTags(), []string{"backend", "zero-use"}) {
		t.Fatalf("loadedProject.AllTags() = %v, want [backend zero-use]", loadedProject.AllTags())
	}
}

func TestBuildTargetedRestorePreservesProjectTags(t *testing.T) {
	currentTask, err := models.NewTask("task-1", 1, models.TaskTypeTask, "Current Title")
	if err != nil {
		t.Fatal(err)
	}
	snapshotTask, err := models.NewTask("task-1", 1, models.TaskTypeTask, "Snapshot Title")
	if err != nil {
		t.Fatal(err)
	}

	currentContent := WriteTasksMarkdownWithProjectMetadata(
		"Restore Test",
		[]*models.Task{currentTask},
		[]string{"Next"},
		nil,
		[]string{"current-zero", "backend"},
	)
	snapshotContent := WriteTasksMarkdownWithProjectMetadata(
		"Restore Test",
		[]*models.Task{snapshotTask},
		[]string{"Next"},
		nil,
		[]string{"snapshot-only"},
	)
	snapshotResult, err := ParseTasksMarkdownWithSections(snapshotContent)
	if err != nil {
		t.Fatalf("ParseTasksMarkdownWithSections(snapshotContent): %v", err)
	}

	restored, err := buildTargetedRestore(currentContent, snapshotResult, []string{"task-1"})
	if err != nil {
		t.Fatalf("buildTargetedRestore failed: %v", err)
	}

	result, err := ParseTasksMarkdownWithSections(restored)
	if err != nil {
		t.Fatalf("ParseTasksMarkdownWithSections(restored): %v", err)
	}

	if !slices.Equal(result.Tags, []string{"current-zero", "backend"}) {
		t.Fatalf("result.Tags = %v, want [current-zero backend]", result.Tags)
	}
	if len(result.Tasks) != 1 || result.Tasks[0].Title != "Snapshot Title" {
		t.Fatalf("restored task title = %v, want Snapshot Title", result.Tasks)
	}
}
