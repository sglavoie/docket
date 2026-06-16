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
<!-- sections: Next;Inbox -->
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

func TestParseTasksMarkdownWithSections_ProjectPinnedTags(t *testing.T) {
	content := `# Project: Test Project
<!-- sections: Next;Inbox -->
<!-- pinned_tags: backend; #frontend ; backend; urgent -->

## Task: My Task
<!-- id: task1, status: todo, priority: 3 -->
`

	result, err := ParseTasksMarkdownWithSections(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := []string{"backend", "frontend", "urgent"}
	if !slices.Equal(result.PinnedTags, want) {
		t.Fatalf("result.PinnedTags = %v, want %v", result.PinnedTags, want)
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

func TestParseTasksMarkdownWithSections_ProjectPinnedTagsAfterTaskIgnored(t *testing.T) {
	content := `# Project: Test Project

## Task: My Task
<!-- id: task1, status: todo, priority: 3 -->

<!-- pinned_tags: backend -->
`

	result, err := ParseTasksMarkdownWithSections(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.PinnedTags) != 0 {
		t.Fatalf("result.PinnedTags = %v, want empty", result.PinnedTags)
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

func TestParseTasksMarkdownWithSections_InvalidProjectPinnedTagsWarnAndSkip(t *testing.T) {
	content := `# Project: Test Project
<!-- pinned_tags: backend;bad,tag;#;frontend -->

## Task: My Task
<!-- id: task1, status: todo, priority: 3 -->
`

	result, err := ParseTasksMarkdownWithSections(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := []string{"backend", "frontend"}
	if !slices.Equal(result.PinnedTags, want) {
		t.Fatalf("result.PinnedTags = %v, want %v", result.PinnedTags, want)
	}

	tagWarnings := 0
	for _, warning := range result.Warnings {
		if warning.Type == format.WarningValidation && warning.Field == "pinned_tags" {
			tagWarnings++
		}
	}
	if tagWarnings != 2 {
		t.Fatalf("pinned tag validation warnings = %d, want 2; warnings: %v", tagWarnings, result.Warnings)
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
		[]string{models.SectionNext, models.SectionInbox},
		map[string]string{models.SectionInbox: "gray"},
		[]string{" backend ", "#frontend", "backend", "bad,tag"},
	)

	if !strings.Contains(content, "<!-- sections: Next;Inbox:gray -->") {
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

func TestWriteTasksMarkdownWithFullProjectMetadata_WritesPinnedTags(t *testing.T) {
	task, err := models.NewTask("task1", 1, models.TaskTypeTask, "Test Task")
	if err != nil {
		t.Fatal(err)
	}

	content := WriteTasksMarkdownWithFullProjectMetadata(
		"Test Project",
		[]*models.Task{task},
		[]string{models.SectionNext, models.SectionInbox},
		map[string]string{models.SectionInbox: "gray"},
		[]string{"backend"},
		[]string{" #frontend ", "backend", "frontend", "bad,tag"},
	)

	if !strings.Contains(content, "<!-- tags: backend -->") {
		t.Fatalf("content should contain tags metadata:\n%s", content)
	}
	if !strings.Contains(content, "<!-- pinned_tags: frontend;backend -->") {
		t.Fatalf("content should contain normalized pinned tag metadata:\n%s", content)
	}
	if strings.Contains(content, "bad,tag") {
		t.Fatalf("content should skip invalid project pinned tags:\n%s", content)
	}
	if strings.Index(content, "<!-- pinned_tags:") < strings.Index(content, "<!-- tags:") {
		t.Fatalf("pinned tags metadata should be written after tags metadata:\n%s", content)
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

	if saveErr := SaveProject(project); saveErr != nil {
		t.Fatalf("SaveProject failed: %v", saveErr)
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

func TestSaveProject_WithProjectPinnedTags(t *testing.T) {
	dir, cleanup := testutil.TempProjectDir(t)
	defer cleanup()

	project := testutil.NewTestProject("Pinned Tags Test")
	project.Path = filepath.Join(dir, "pinned-tags-test")
	project.Tags = []string{"backend", "zero-use"}
	project.PinnedTags = []string{" #backend ", "urgent", "backend"}

	task, err := models.NewTask("task-1", 1, models.TaskTypeTask, "Tagged Task")
	if err != nil {
		t.Fatal(err)
	}
	task.Tags = []string{"backend"}
	project.Tasks = []*models.Task{task}

	if saveErr := SaveProject(project); saveErr != nil {
		t.Fatalf("SaveProject failed: %v", saveErr)
	}

	tasksPath := filepath.Join(project.Path, "tasks.md")
	content, err := os.ReadFile(tasksPath)
	if err != nil {
		t.Fatalf("ReadFile(%s): %v", tasksPath, err)
	}
	if !strings.Contains(string(content), "<!-- pinned_tags: backend;urgent -->") {
		t.Fatalf("saved content should contain project pinned tags metadata:\n%s", content)
	}

	loadedProject := testutil.NewTestProject("Pinned Tags Test")
	loadedProject.Path = project.Path
	if err := LoadProject(loadedProject); err != nil {
		t.Fatalf("LoadProject failed: %v", err)
	}

	if !slices.Equal(loadedProject.PinnedTags, []string{"backend", "urgent"}) {
		t.Fatalf("loadedProject.PinnedTags = %v, want [backend urgent]", loadedProject.PinnedTags)
	}
	if !loadedProject.IsTagPinned("#backend") {
		t.Fatal("loadedProject.IsTagPinned(#backend) = false, want true")
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

func TestBuildTargetedRestorePreservesProjectPinnedTags(t *testing.T) {
	currentTask, err := models.NewTask("task-1", 1, models.TaskTypeTask, "Current Title")
	if err != nil {
		t.Fatal(err)
	}
	snapshotTask, err := models.NewTask("task-1", 1, models.TaskTypeTask, "Snapshot Title")
	if err != nil {
		t.Fatal(err)
	}

	currentContent := WriteTasksMarkdownWithFullProjectMetadata(
		"Restore Test",
		[]*models.Task{currentTask},
		[]string{"Next"},
		nil,
		[]string{"current-zero", "backend"},
		[]string{"current-pinned", "backend"},
	)
	snapshotContent := WriteTasksMarkdownWithFullProjectMetadata(
		"Restore Test",
		[]*models.Task{snapshotTask},
		[]string{"Next"},
		nil,
		[]string{"snapshot-only"},
		[]string{"snapshot-pinned"},
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

	if !slices.Equal(result.PinnedTags, []string{"current-pinned", "backend"}) {
		t.Fatalf("result.PinnedTags = %v, want [current-pinned backend]", result.PinnedTags)
	}
	if len(result.Tasks) != 1 || result.Tasks[0].Title != "Snapshot Title" {
		t.Fatalf("restored task title = %v, want Snapshot Title", result.Tasks)
	}
}
