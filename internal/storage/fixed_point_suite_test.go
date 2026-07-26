package storage

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/testutil"
)

func TestRoundTrip_KitchenSinkParseWriteFixedPoint(t *testing.T) {
	project := newStorageFixedPointProject(t, "", "peer-project")

	seed := WriteTasksMarkdownWithSections(project.Name, project.Tasks, project.GetSections(), project.SectionColors)

	result1, err := ParseTasksMarkdownWithSections(seed)
	if err != nil {
		t.Fatalf("ParseTasksMarkdownWithSections(seed): %v", err)
	}
	if len(result1.Warnings) != 0 {
		t.Fatalf("expected no parse warnings on first parse, got %v", result1.Warnings)
	}

	written1 := WriteTasksMarkdownWithSections(result1.ProjectName, result1.Tasks, result1.Sections, result1.SectionColors)
	if written1 != seed {
		t.Fatalf("parse -> write changed canonical markdown:\n--- seed ---\n%s\n--- written1 ---\n%s", seed, written1)
	}

	result2, err := ParseTasksMarkdownWithSections(written1)
	if err != nil {
		t.Fatalf("ParseTasksMarkdownWithSections(written1): %v", err)
	}
	if len(result2.Warnings) != 0 {
		t.Fatalf("expected no parse warnings on second parse, got %v", result2.Warnings)
	}

	written2 := WriteTasksMarkdownWithSections(result2.ProjectName, result2.Tasks, result2.Sections, result2.SectionColors)
	if written2 != written1 {
		t.Fatalf("parse -> write -> parse -> write did not reach fixed point:\n--- written1 ---\n%s\n--- written2 ---\n%s", written1, written2)
	}

	if result1.ProjectName != project.Name {
		t.Fatalf("project name = %q, want %q", result1.ProjectName, project.Name)
	}
	assertSectionsAndColors(t, result1.Sections, []string{models.SectionNext}, result1.SectionColors, map[string]string{})
	assertSectionsAndColors(t, result2.Sections, []string{models.SectionNext}, result2.SectionColors, map[string]string{})
	assertKitchenSinkRoundTrip(t, result1.Tasks, project.Tasks)
	assertKitchenSinkRoundTrip(t, result2.Tasks, project.Tasks)
}

func TestRoundTrip_KitchenSinkSaveLoadSaveFixedPoint(t *testing.T) {
	dir, cleanup := testutil.TempProjectDir(t)
	defer cleanup()

	projectsDir := filepath.Join(dir, "projects")
	projectPath := filepath.Join(projectsDir, "storage-fixed-point")
	peerPath := filepath.Join(projectsDir, "peer-project")

	if err := os.MkdirAll(peerPath, 0755); err != nil {
		t.Fatalf("MkdirAll(peerPath): %v", err)
	}

	project := newStorageFixedPointProject(t, projectPath, "peer-project")
	if err := SaveProject(project); err != nil {
		t.Fatalf("SaveProject(first): %v", err)
	}

	firstSave, err := os.ReadFile(tasksFilePath(projectPath))
	if err != nil {
		t.Fatalf("ReadFile(first save): %v", err)
	}

	loaded := testutil.NewTestProject(project.Name)
	loaded.Slug = project.Slug
	loaded.Path = projectPath

	result, err := LoadProjectWithWarnings(loaded)
	if err != nil {
		t.Fatalf("LoadProjectWithWarnings: %v", err)
	}
	if len(result.Warnings) != 0 {
		t.Fatalf("expected no load warnings, got %v", result.Warnings)
	}

	assertSectionsAndColors(t, loaded.GetSections(), project.GetSections(), loaded.SectionColors, map[string]string{})
	assertKitchenSinkRoundTrip(t, loaded.Tasks, project.Tasks)
	assertLoadedHierarchy(t, loaded.Tasks)

	err = SaveProject(loaded)
	if err != nil {
		t.Fatalf("SaveProject(second): %v", err)
	}

	secondSave, err := os.ReadFile(tasksFilePath(projectPath))
	if err != nil {
		t.Fatalf("ReadFile(second save): %v", err)
	}

	if string(secondSave) != string(firstSave) {
		t.Fatalf("Save -> Load -> Save did not preserve exact file content:\n--- first ---\n%s\n--- second ---\n%s", firstSave, secondSave)
	}
}

func newStorageFixedPointProject(t *testing.T, projectPath, peerSlug string) *models.Project {
	t.Helper()

	base := time.Date(2025, 1, 15, 9, 30, 0, 0, time.Local)
	dueAt := time.Date(2025, 1, 22, 14, 45, 0, 0, time.Local)
	startDate := time.Date(2025, 1, 20, 0, 0, 0, 0, time.Local)
	snoozedUntil := time.Date(2025, 1, 25, 0, 0, 0, 0, time.Local)
	scopeBaseline := time.Date(2025, 1, 16, 0, 0, 0, 0, time.Local)

	project := testutil.NewTestProject("Storage Fixed Point")
	project.Slug = "storage-fixed-point"
	project.Path = projectPath
	project.Sections = []string{
		models.SectionNext,
		models.SectionInbox,
		models.SectionWaiting,
		models.SectionSomeday,
		models.SectionReview,
		models.SectionStandby,
		models.SectionCompleted,
	}
	project.SectionColors = map[string]string{
		models.SectionInbox:   "blue",
		models.SectionWaiting: "orange",
	}

	epic, err := models.NewTask("epic-fixed", 1, models.TaskTypeEpic, "Storage invariants")
	if err != nil {
		t.Fatal(err)
	}
	// v2 is the sole writer: it serializes the derived `state` and drops the
	// legacy section/stage/active keys, so a round-trip is an identity only when
	// the lifecycle fields are the canonical representative of their state (empty
	// stage, legacy active metadata discarded). The epic is in_progress (canonical
	// section "").
	epic.WorkflowState = models.StateInProgress
	epic.Priority = 1
	epic.Tags = []string{"storage", "roundtrip"}
	epic.Description = "Line with commas, status: todo, id: placeholder\n## Task: embedded heading\n# Project: embedded project line\n<!-- COMMENTS -->\n<!-- literal html comment -->"
	epic.CreatedAt = base
	epic.UpdatedAt = base.Add(2 * time.Hour)
	epic.ScoreDimensions = map[string]int{"impact": 8, "risk": 5}
	epic.EpicAttrs = &models.EpicAttrs{
		Scope:             models.EpicScopeDefined,
		Milestone:         "Phase 1, status: steady",
		SuccessMetrics:    "Ship parser, priority: 1\n# Project: embedded\n<!-- metrics marker -->",
		InitialChildCount: 1,
		ScopeBaselinedAt:  &scopeBaseline,
	}

	story, err := models.NewTask("story-fixed", 2, models.TaskTypeStory, "Parser hierarchy")
	if err != nil {
		t.Fatal(err)
	}
	story.ParentID = &epic.ID
	story.WorkflowState = models.StateReady
	story.Priority = 2
	story.CreatedAt = base.Add(3 * time.Hour)
	story.UpdatedAt = base.Add(4 * time.Hour)
	story.StoryAttrs = &models.StoryAttrs{
		StoryPoints: 5,
		Persona:     "As operator, priority: 2\n### Story: keep literal\n<!-- persona marker -->",
	}

	task, err := models.NewTask("task-fixed", 3, models.TaskTypeTask, "Round-trip fixed point")
	if err != nil {
		t.Fatal(err)
	}
	task.ParentID = &story.ID
	// Canonical `waiting` state. Blocked status also derives waiting but its
	// canonical representative is todo+Waiting, so the writer emits `state: waiting`
	// and the round-trip stays an identity under the v2 writer.
	task.WorkflowState = models.StateWaiting
	task.Priority = 2
	task.CreatedAt = base.Add(5 * time.Hour)
	task.UpdatedAt = base.Add(10 * time.Hour)
	task.DueDate = &dueAt
	task.HasTime = true
	task.StartDate = &startDate
	task.EstimatedDays = 2.5
	task.DependsOn = []string{"bug-fixed", peerSlug + ":peer-dep"}
	task.RelatedTo = []string{"epic-fixed", peerSlug + ":peer-related"}
	task.Pinned = true
	task.PinnedAt = timePtr(base.Add(6 * time.Hour))
	// Legacy Active metadata and custom stages are retired: neither is stored on
	// the task or serialized, so this stays a fixed point without them.
	task.Dimmed = true
	task.SnoozedUntil = &snoozedUntil
	task.Source = "cli"
	task.URL = "https://example.com/issues/task-fixed?note=id:epic-fixed,status:todo"
	task.Recurrence = &models.RecurrencePattern{
		Interval:  2,
		Unit:      models.RecurrenceWeekly,
		Weekdays:  []int{1, 4},
		Skipped:   1,
		Streak:    3,
		MaxStreak: 4,
		Mode:      models.RecurModeFromCompletion,
	}
	task.Checklist = []models.ChecklistItem{
		{Text: "Verify commas, status: todo", Done: true},
		{Text: "Keep ## Task: literal", Done: false},
	}
	task.TaskAttrs = &models.TaskAttrs{
		Effort:   models.EffortMedium,
		WorkType: models.WorkTypeTesting,
	}
	task.Attachments = []models.Attachment{
		{
			Filename:     "parser.go",
			IsCodeRef:    true,
			LineNumber:   42,
			OriginalPath: "/repo/internal/storage/parser.go",
			AddedAt:      base.Add(7 * time.Hour),
		},
	}
	task.Activity = []*models.ActivityEntry{
		{
			Timestamp: task.CreatedAt,
			Field:     "created",
		},
		{
			Timestamp: base.Add(8 * time.Hour),
			Field:     "title",
			OldValue:  "Old > title, status: todo",
			NewValue:  "New > title, key: value, ## Task: pseudo <!-- keep -->",
		},
		{
			Timestamp: base.Add(9 * time.Hour),
			Field:     "source",
			OldValue:  "github",
			NewValue:  "cli, id: task-fixed",
		},
	}
	task.Comments = []*models.Comment{
		{
			Text:      "Comment, status: todo\n## Task: pseudo\n<!-- /COMMENTS -->",
			CreatedAt: base.Add(11 * time.Hour),
		},
		{
			Text:      "Resolved comment with key: value\n<!-- DECISIONS -->\n### Story: not real",
			CreatedAt: base.Add(12 * time.Hour),
			Resolved:  true,
		},
	}
	task.Decisions = []*models.Decision{
		{
			Text:      "Choose parser, source: cli\n## Task: pseudo",
			Rationale: "Because <!-- keep -->\n<!-- ACTIVITY -->",
			Category:  models.DecisionCategoryDesign,
			CreatedAt: base.Add(13 * time.Hour),
		},
		{
			Text:      "Ship fix, status: todo",
			Rationale: "Resolved, priority: 1",
			Category:  models.DecisionCategoryScope,
			CreatedAt: base.Add(14 * time.Hour),
			Resolved:  true,
		},
	}

	bug, err := models.NewTask("bug-fixed", 4, models.TaskTypeBug, "Metadata-safe bug")
	if err != nil {
		t.Fatal(err)
	}
	bug.WorkflowState = models.StateReady
	bug.Priority = 1
	bug.Tags = []string{"bug", "parser"}
	bug.CreatedAt = base.Add(15 * time.Hour)
	bug.UpdatedAt = base.Add(16 * time.Hour)
	bug.BugAttrs = &models.BugAttrs{
		Reproducible: models.BugReproducibleYes,
		Severity:     models.BugSeverityCritical,
		Workaround:   "Retry, status: todo, id: bug-fixed\n## Task: workaround heading\n<!-- keep literal -->",
	}
	bug.Attachments = []models.Attachment{
		{
			Filename:     "bug.go",
			IsCodeRef:    true,
			LineNumber:   7,
			OriginalPath: "/repo/internal/storage/format/metadata.go",
			AddedAt:      base.Add(17 * time.Hour),
		},
	}

	project.Tasks = []*models.Task{epic, story, task, bug}
	project.NextShortID = 5
	project.BuildTaskTree()

	return project
}

func assertKitchenSinkRoundTrip(t *testing.T, gotTasks, wantTasks []*models.Task) {
	t.Helper()

	wantOrder := make([]string, len(wantTasks))
	for i, want := range wantTasks {
		wantOrder[i] = want.ID
	}
	gotOrder := make([]string, len(gotTasks))
	for i, got := range gotTasks {
		gotOrder[i] = got.ID
	}
	if !reflect.DeepEqual(gotOrder, wantOrder) {
		t.Fatalf("task order = %v, want %v", gotOrder, wantOrder)
	}

	for _, want := range wantTasks {
		got := findTaskByID(gotTasks, want.ID)
		if got == nil {
			t.Fatalf("task %q missing after round-trip", want.ID)
		}
		assertTaskPersistentFields(t, got, want)
	}
}

func assertTaskPersistentFields(t *testing.T, got, want *models.Task) {
	t.Helper()

	assertTaskCoreFields(t, got, want)
	assertTaskScheduleFields(t, got, want)
	assertTaskRelationshipFields(t, got, want)
	assertTaskContentFields(t, got, want)
	assertTaskTypedAttributeFields(t, got, want)
}

func assertTaskCoreFields(t *testing.T, got, want *models.Task) {
	t.Helper()

	assertTaskComparableField(t, want.ID, "ShortID", got.ShortID, want.ShortID)
	assertTaskComparableField(t, want.ID, "Type", got.Type, want.Type)
	assertTaskComparableField(t, want.ID, "Title", got.Title, want.Title)
	assertTaskComparableField(t, want.ID, "Description", got.Description, want.Description)
	// WorkflowState is the single source of truth for the lifecycle: asserting it
	// covers the retired Status/Section/Stage reads (each derives from this state).
	assertTaskComparableField(t, want.ID, "WorkflowState", got.State(), want.State())
	assertTaskComparableField(t, want.ID, "Priority", got.Priority, want.Priority)
	assertTaskStringPtrField(t, want.ID, "ParentID", got.ParentID, want.ParentID)
	assertTaskDeepEqualField(t, want.ID, "Tags", got.Tags, want.Tags)
}

func assertTaskScheduleFields(t *testing.T, got, want *models.Task) {
	t.Helper()

	assertTaskTimeField(t, want.ID, "CreatedAt", got.CreatedAt, want.CreatedAt)
	assertTaskTimeField(t, want.ID, "UpdatedAt", got.UpdatedAt, want.UpdatedAt)
	assertTaskTimePtrField(t, want.ID, "CompletedAt", got.CompletedAt, want.CompletedAt)
	assertTaskTimePtrField(t, want.ID, "DueDate", got.DueDate, want.DueDate)
	assertTaskComparableField(t, want.ID, "HasTime", got.HasTime, want.HasTime)
	assertTaskComparableField(t, want.ID, "Pinned", got.Pinned, want.Pinned)
	assertTaskTimePtrField(t, want.ID, "PinnedAt", got.PinnedAt, want.PinnedAt)
	assertTaskTimePtrField(t, want.ID, "SnoozedUntil", got.SnoozedUntil, want.SnoozedUntil)
	assertTaskTimePtrField(t, want.ID, "StartDate", got.StartDate, want.StartDate)
	assertTaskComparableField(t, want.ID, "EstimatedDays", got.EstimatedDays, want.EstimatedDays)
}

func assertTaskRelationshipFields(t *testing.T, got, want *models.Task) {
	t.Helper()

	assertTaskDeepEqualField(t, want.ID, "DependsOn", got.DependsOn, want.DependsOn)
	assertTaskDeepEqualField(t, want.ID, "RelatedTo", got.RelatedTo, want.RelatedTo)
	// Legacy Active metadata and Stage are retired and not serialized, so there
	// is nothing to round-trip here.
	assertTaskComparableField(t, want.ID, "Dimmed", got.Dimmed, want.Dimmed)
	assertTaskComparableField(t, want.ID, "Source", got.Source, want.Source)
	assertTaskComparableField(t, want.ID, "URL", got.URL, want.URL)
}

func assertTaskContentFields(t *testing.T, got, want *models.Task) {
	t.Helper()

	assertTaskDeepEqualField(t, want.ID, "Comments", got.Comments, want.Comments)
	assertTaskDeepEqualField(t, want.ID, "Decisions", got.Decisions, want.Decisions)
	assertTaskDeepEqualField(t, want.ID, "Recurrence", got.Recurrence, want.Recurrence)
	assertTaskDeepEqualField(t, want.ID, "Attachments", got.Attachments, want.Attachments)
	assertTaskDeepEqualField(t, want.ID, "Activity", got.Activity, want.Activity)
	assertTaskDeepEqualField(t, want.ID, "Checklist", got.Checklist, want.Checklist)
	assertTaskDeepEqualField(t, want.ID, "ScoreDimensions", got.ScoreDimensions, want.ScoreDimensions)
}

func assertTaskTypedAttributeFields(t *testing.T, got, want *models.Task) {
	t.Helper()

	assertTaskDeepEqualField(t, want.ID, "BugAttrs", got.BugAttrs, want.BugAttrs)
	assertTaskDeepEqualField(t, want.ID, "TaskAttrs", got.TaskAttrs, want.TaskAttrs)
	assertTaskDeepEqualField(t, want.ID, "StoryAttrs", got.StoryAttrs, want.StoryAttrs)
	assertTaskDeepEqualField(t, want.ID, "EpicAttrs", got.EpicAttrs, want.EpicAttrs)
}

func assertTaskComparableField[T comparable](t *testing.T, taskID, field string, got, want T) {
	t.Helper()
	if got != want {
		t.Errorf("%s %s = %v, want %v", taskID, field, got, want)
	}
}

func assertTaskDeepEqualField(t *testing.T, taskID, field string, got, want any) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Errorf("%s %s = %#v, want %#v", taskID, field, got, want)
	}
}

func assertTaskStringPtrField(t *testing.T, taskID, field string, got, want *string) {
	t.Helper()
	if !equalStringPtr(got, want) {
		t.Errorf("%s %s = %v, want %v", taskID, field, got, want)
	}
}

func assertTaskTimeField(t *testing.T, taskID, field string, got, want time.Time) {
	t.Helper()
	if !got.Equal(want) {
		t.Errorf("%s %s = %v, want %v", taskID, field, got, want)
	}
}

func assertTaskTimePtrField(t *testing.T, taskID, field string, got, want *time.Time) {
	t.Helper()
	if !equalTimePtr(got, want) {
		t.Errorf("%s %s = %v, want %v", taskID, field, got, want)
	}
}

func assertSectionsAndColors(t *testing.T, gotSections, wantSections []string, gotColors, wantColors map[string]string) {
	t.Helper()
	if !reflect.DeepEqual(gotSections, wantSections) {
		t.Fatalf("sections = %v, want %v", gotSections, wantSections)
	}
	if !reflect.DeepEqual(gotColors, wantColors) {
		t.Fatalf("section colors = %#v, want %#v", gotColors, wantColors)
	}
}

func assertLoadedHierarchy(t *testing.T, tasks []*models.Task) {
	t.Helper()

	epic := findTaskByID(tasks, "epic-fixed")
	story := findTaskByID(tasks, "story-fixed")
	task := findTaskByID(tasks, "task-fixed")
	if epic == nil || story == nil || task == nil {
		t.Fatalf("expected hierarchy tasks to exist, got epic=%v story=%v task=%v", epic != nil, story != nil, task != nil)
	}

	if len(epic.Children) != 1 || epic.Children[0].ID != story.ID {
		t.Fatalf("epic children = %#v, want [%s]", epic.Children, story.ID)
	}
	if len(story.Children) != 1 || story.Children[0].ID != task.ID {
		t.Fatalf("story children = %#v, want [%s]", story.Children, task.ID)
	}
	if story.Parent == nil || story.Parent.ID != epic.ID {
		t.Fatalf("story parent = %#v, want %s", story.Parent, epic.ID)
	}
	if task.Parent == nil || task.Parent.ID != story.ID {
		t.Fatalf("task parent = %#v, want %s", task.Parent, story.ID)
	}
}

func equalStringPtr(a, b *string) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

func equalTimePtr(a, b *time.Time) bool {
	if a == nil || b == nil {
		return a == b
	}
	return a.Equal(*b)
}

func timePtr(t time.Time) *time.Time {
	return &t
}
