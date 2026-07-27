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

func TestParseTasksMarkdownDocument_ProjectTags(t *testing.T) {
	content := `# Project: Test Project
<!-- sections: Next;Inbox -->
<!-- tags: backend; #frontend ; backend; urgent -->

## Task: My Task
<!-- id: task1, status: todo, priority: 3 -->
`

	result, err := ParseTasksMarkdownDocument(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := []string{"backend", "frontend", "urgent"}
	if !slices.Equal(result.Tags, want) {
		t.Fatalf("result.Tags = %v, want %v", result.Tags, want)
	}
}

func TestParseTasksMarkdownDocument_ProjectPinnedTags(t *testing.T) {
	content := `# Project: Test Project
<!-- sections: Next;Inbox -->
<!-- pinned_tags: backend; #frontend ; backend; urgent -->

## Task: My Task
<!-- id: task1, status: todo, priority: 3 -->
`

	result, err := ParseTasksMarkdownDocument(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := []string{"backend", "frontend", "urgent"}
	if !slices.Equal(result.PinnedTags, want) {
		t.Fatalf("result.PinnedTags = %v, want %v", result.PinnedTags, want)
	}
}

func TestParseTasksMarkdownDocument_ProjectTagsAfterTaskIgnored(t *testing.T) {
	content := `# Project: Test Project

## Task: My Task
<!-- id: task1, status: todo, priority: 3 -->

<!-- tags: backend -->
`

	result, err := ParseTasksMarkdownDocument(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Tags) != 0 {
		t.Fatalf("result.Tags = %v, want empty", result.Tags)
	}
}

func TestParseTasksMarkdownDocument_ProjectPinnedTagsAfterTaskIgnored(t *testing.T) {
	content := `# Project: Test Project

## Task: My Task
<!-- id: task1, status: todo, priority: 3 -->

<!-- pinned_tags: backend -->
`

	result, err := ParseTasksMarkdownDocument(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.PinnedTags) != 0 {
		t.Fatalf("result.PinnedTags = %v, want empty", result.PinnedTags)
	}
}

func TestParseTasksMarkdownDocument_InvalidProjectTagsWarnAndSkip(t *testing.T) {
	content := `# Project: Test Project
<!-- tags: backend;bad,tag;#;frontend -->

## Task: My Task
<!-- id: task1, status: todo, priority: 3 -->
`

	result, err := ParseTasksMarkdownDocument(content)
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

func TestParseTasksMarkdownDocument_InvalidProjectPinnedTagsWarnAndSkip(t *testing.T) {
	content := `# Project: Test Project
<!-- pinned_tags: backend;bad,tag;#;frontend -->

## Task: My Task
<!-- id: task1, status: todo, priority: 3 -->
`

	result, err := ParseTasksMarkdownDocument(content)
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
		[]string{" backend ", "#frontend", "backend", "bad,tag"},
		nil,
	)

	if strings.Contains(content, "<!-- sections:") {
		t.Fatalf("canonical content should omit legacy sections metadata:\n%s", content)
	}
	if !strings.Contains(content, "<!-- tags: backend;frontend -->") {
		t.Fatalf("content should contain normalized tag metadata:\n%s", content)
	}
	if strings.Contains(content, "bad,tag") {
		t.Fatalf("content should skip invalid project tags:\n%s", content)
	}
	if strings.Index(content, "<!-- tags:") < strings.Index(content, "<!-- tt-schema: 2 -->") {
		t.Fatalf("tags metadata should be written after the schema marker:\n%s", content)
	}
}

func TestWriteTasksMarkdownWithProjectMetadata_WritesPinnedTags(t *testing.T) {
	task, err := models.NewTask("task1", 1, models.TaskTypeTask, "Test Task")
	if err != nil {
		t.Fatal(err)
	}

	content := WriteTasksMarkdownWithProjectMetadata(
		"Test Project",
		[]*models.Task{task},
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

func TestSaveProject_AfterDeleteTagPersistsUnassignmentAndActivity(t *testing.T) {
	dir, cleanup := testutil.TempProjectDir(t)
	defer cleanup()

	project := testutil.NewTestProject("Delete Tag Storage Test")
	project.Path = filepath.Join(dir, "delete-tag-storage-test")
	project.Tags = []string{"backend", "frontend", "zero-use"}
	project.PinnedTags = []string{"backend", "frontend"}

	task1, err := models.NewTask("task-1", 1, models.TaskTypeTask, "Backend and Frontend")
	if err != nil {
		t.Fatal(err)
	}
	task1.Tags = []string{"backend", "frontend"}
	task2, err := models.NewTask("task-2", 2, models.TaskTypeTask, "Backend Only")
	if err != nil {
		t.Fatal(err)
	}
	task2.Tags = []string{"backend"}
	task3, err := models.NewTask("task-3", 3, models.TaskTypeTask, "Frontend Only")
	if err != nil {
		t.Fatal(err)
	}
	task3.Tags = []string{"frontend"}
	project.Tasks = []*models.Task{task1, task2, task3}

	if saveErr := SaveProject(project); saveErr != nil {
		t.Fatalf("initial SaveProject failed: %v", saveErr)
	}

	result, err := project.DeleteTag("#backend")
	if err != nil {
		t.Fatalf("DeleteTag failed: %v", err)
	}
	if result.Name != "backend" || !result.CatalogChanged || !result.PinnedChanged || result.TasksUnassigned != 2 {
		t.Fatalf("DeleteTag result = %#v, want catalog/pinned changed and 2 tasks unassigned", result)
	}

	if saveErr := SaveProject(project); saveErr != nil {
		t.Fatalf("SaveProject after DeleteTag failed: %v", saveErr)
	}

	tasksPath := filepath.Join(project.Path, "tasks.md")
	content, err := os.ReadFile(tasksPath)
	if err != nil {
		t.Fatalf("ReadFile(%s): %v", tasksPath, err)
	}
	if !strings.Contains(string(content), "<!-- tags: frontend;zero-use -->") {
		t.Fatalf("saved content should retain remaining project tags metadata:\n%s", content)
	}
	if !strings.Contains(string(content), "<!-- pinned_tags: frontend -->") {
		t.Fatalf("saved content should retain remaining pinned tag metadata:\n%s", content)
	}

	loadedProject := testutil.NewTestProject("Delete Tag Storage Test")
	loadedProject.Path = project.Path
	if err := LoadProject(loadedProject); err != nil {
		t.Fatalf("LoadProject failed: %v", err)
	}

	if !slices.Equal(loadedProject.Tags, []string{"frontend", "zero-use"}) {
		t.Fatalf("loadedProject.Tags = %v, want [frontend zero-use]", loadedProject.Tags)
	}
	if !slices.Equal(loadedProject.PinnedTagList(), []string{"frontend"}) {
		t.Fatalf("loadedProject.PinnedTagList() = %v, want [frontend]", loadedProject.PinnedTagList())
	}

	wantTaskTags := map[int][]string{
		1: {"frontend"},
		2: {},
		3: {"frontend"},
	}
	for _, task := range loadedProject.Tasks {
		want, ok := wantTaskTags[task.ShortID]
		if !ok {
			continue
		}
		if !slices.Equal(task.Tags, want) {
			t.Fatalf("loaded task #%d tags = %v, want %v", task.ShortID, task.Tags, want)
		}
	}

	for _, shortID := range []int{1, 2} {
		task := findLoadedTaskByShortID(t, loadedProject, shortID)
		entry := findActivity(task, "tags")
		if entry == nil {
			t.Fatalf("loaded task #%d missing tags activity entry", shortID)
		}
		if entry.OldValue != "" || entry.NewValue != "-backend" {
			t.Fatalf("loaded task #%d tags activity = %#v, want NewValue -backend", shortID, entry)
		}
	}
	task := findLoadedTaskByShortID(t, loadedProject, 3)
	if entry := findActivity(task, "tags"); entry != nil {
		t.Fatalf("loaded task #3 tags activity = %#v, want none", entry)
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
		[]string{"current-zero", "backend"},
		nil,
	)
	snapshotContent := WriteTasksMarkdownWithProjectMetadata(
		"Restore Test",
		[]*models.Task{snapshotTask},
		[]string{"snapshot-only"},
		nil,
	)
	snapshotResult, err := ParseTasksMarkdownDocument(snapshotContent)
	if err != nil {
		t.Fatalf("ParseTasksMarkdownDocument(snapshotContent): %v", err)
	}

	restored, err := buildTargetedRestore(currentContent, snapshotResult, []string{"task-1"})
	if err != nil {
		t.Fatalf("buildTargetedRestore failed: %v", err)
	}

	result, err := ParseTasksMarkdownDocument(restored)
	if err != nil {
		t.Fatalf("ParseTasksMarkdownDocument(restored): %v", err)
	}

	if !slices.Equal(result.Tags, []string{"current-zero", "backend"}) {
		t.Fatalf("result.Tags = %v, want [current-zero backend]", result.Tags)
	}
	if len(result.Tasks) != 1 || result.Tasks[0].Title != "Snapshot Title" {
		t.Fatalf("restored task title = %v, want Snapshot Title", result.Tasks)
	}
}

func findLoadedTaskByShortID(t *testing.T, project *models.Project, shortID int) *models.Task {
	t.Helper()
	for _, task := range project.Tasks {
		if task.ShortID == shortID {
			return task
		}
	}
	t.Fatalf("task #%d not found in loaded project", shortID)
	return nil
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

	currentContent := WriteTasksMarkdownWithProjectMetadata(
		"Restore Test",
		[]*models.Task{currentTask},
		[]string{"current-zero", "backend"},
		[]string{"current-pinned", "backend"},
	)
	snapshotContent := WriteTasksMarkdownWithProjectMetadata(
		"Restore Test",
		[]*models.Task{snapshotTask},
		[]string{"snapshot-only"},
		[]string{"snapshot-pinned"},
	)
	snapshotResult, err := ParseTasksMarkdownDocument(snapshotContent)
	if err != nil {
		t.Fatalf("ParseTasksMarkdownDocument(snapshotContent): %v", err)
	}

	restored, err := buildTargetedRestore(currentContent, snapshotResult, []string{"task-1"})
	if err != nil {
		t.Fatalf("buildTargetedRestore failed: %v", err)
	}

	result, err := ParseTasksMarkdownDocument(restored)
	if err != nil {
		t.Fatalf("ParseTasksMarkdownDocument(restored): %v", err)
	}

	if !slices.Equal(result.PinnedTags, []string{"current-pinned", "backend"}) {
		t.Fatalf("result.PinnedTags = %v, want [current-pinned backend]", result.PinnedTags)
	}
	if len(result.Tasks) != 1 || result.Tasks[0].Title != "Snapshot Title" {
		t.Fatalf("restored task title = %v, want Snapshot Title", result.Tasks)
	}
}
