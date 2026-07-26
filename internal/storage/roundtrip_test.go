package storage

import (
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/testutil"
)

// TestRoundTrip_AllFields verifies that a project with varied tasks (different
// types, statuses, priorities, tags, descriptions, code refs, comments,
// dependencies, recurrence) survives a full SaveProject → LoadProject cycle.
func TestRoundTrip_AllFields(t *testing.T) { //nolint:gocyclo
	dir, cleanup := testutil.TempProjectDir(t)
	defer cleanup()

	now := time.Date(2024, 6, 15, 12, 0, 0, 0, time.UTC)
	due := time.Date(2024, 7, 1, 0, 0, 0, 0, time.Local)

	project := testutil.NewTestProject("Round-Trip Project")
	project.Path = filepath.Join(dir, "roundtrip")
	project.Sections = []string{"Next", "Inbox", "Waiting"}
	project.SectionColors = map[string]string{"Inbox": "blue", "Waiting": "orange"}
	project.Tags = []string{"backend", "release", "urgent"}
	project.PinnedTags = []string{"backend", "urgent"}

	// Epic with description and tags
	epic, err := models.NewTask(uuid.New().String(), 1, models.TaskTypeEpic, "Main Epic")
	if err != nil {
		t.Fatal(err)
	}
	epic.Description = "Top-level epic with a multi-line\ndescription."
	epic.Priority = 1
	epic.Tags = []string{"backend", "release"}
	// Under the v2 writer the section collapses into the derived state, so the
	// epic uses state=next rather than in_progress+Next: an in_progress state
	// would drop the Next bucket on round-trip.
	epic.WorkflowState = models.StateNext
	epic.CreatedAt = now
	epic.UpdatedAt = now
	epic.ScoreDimensions = map[string]int{"impact": 5, "effort": 3}

	// Story under epic
	story, err := models.NewTask(uuid.New().String(), 2, models.TaskTypeStory, "Auth Story")
	if err != nil {
		t.Fatal(err)
	}
	story.ParentID = &epic.ID
	story.Priority = 2
	story.Tags = []string{"auth"}
	story.CreatedAt = now
	story.UpdatedAt = now

	// Task with dependency, due date, pinned
	task1, err := models.NewTask(uuid.New().String(), 3, models.TaskTypeTask, "Implement login")
	if err != nil {
		t.Fatal(err)
	}
	task1.ParentID = &story.ID
	task1.WorkflowState = models.StateInProgress
	task1.Priority = 2
	task1.DependsOn = []string{story.ID}
	task1.DueDate = &due
	task1.Pinned = true
	pinnedAt := now
	task1.PinnedAt = &pinnedAt
	task1.CreatedAt = now
	task1.UpdatedAt = now

	// Task with comments
	commentTime := now.Add(time.Hour)
	task2, err := models.NewTask(uuid.New().String(), 4, models.TaskTypeTask, "Write tests")
	if err != nil {
		t.Fatal(err)
	}
	task2.ParentID = &story.ID
	task2.Comments = []*models.Comment{
		{
			Text:      "First comment here.",
			CreatedAt: commentTime,
		},
	}
	task2.CreatedAt = now
	task2.UpdatedAt = now

	// Bug with code ref attachment
	bug, err := models.NewTask(uuid.New().String(), 5, models.TaskTypeBug, "Fix null pointer")
	if err != nil {
		t.Fatal(err)
	}
	bug.Priority = 1
	bug.Tags = []string{"urgent"}
	bug.Attachments = []models.Attachment{
		{
			Filename:     "main.go",
			IsCodeRef:    true,
			LineNumber:   42,
			OriginalPath: "/src/main.go",
			AddedAt:      now,
		},
	}
	bug.CreatedAt = now
	bug.UpdatedAt = now

	// Recurring task
	recurring, err := models.NewTask(uuid.New().String(), 6, models.TaskTypeTask, "Weekly sync")
	if err != nil {
		t.Fatal(err)
	}
	recurring.Recurrence = &models.RecurrencePattern{
		Interval: 1,
		Unit:     models.RecurrenceWeekly,
		Weekdays: []int{1, 3},
	}
	recurring.CreatedAt = now
	recurring.UpdatedAt = now

	// Idea
	idea, err := models.NewTask(uuid.New().String(), 7, models.TaskTypeIdea, "New feature idea")
	if err != nil {
		t.Fatal(err)
	}
	idea.Description = "Could we add dark mode?"
	idea.CreatedAt = now
	idea.UpdatedAt = now

	// Completed task
	done, err := models.NewTask(uuid.New().String(), 8, models.TaskTypeTask, "Completed task")
	if err != nil {
		t.Fatal(err)
	}
	done.WorkflowState = models.StateDone
	doneAt := now.Add(2 * time.Hour)
	done.CompletedAt = &doneAt
	done.CreatedAt = now
	done.UpdatedAt = now

	project.Tasks = []*models.Task{epic, story, task1, task2, bug, recurring, idea, done}
	project.NextShortID = 9
	project.BuildTaskTree()

	// Save
	if err := SaveProject(project); err != nil {
		t.Fatalf("SaveProject: %v", err)
	}

	// Load into a fresh project
	loaded := testutil.NewTestProject("Round-Trip Project")
	loaded.Path = project.Path
	if err := LoadProject(loaded); err != nil {
		t.Fatalf("LoadProject: %v", err)
	}

	if len(loaded.Tasks) != len(project.Tasks) {
		t.Fatalf("task count: got %d, want %d", len(loaded.Tasks), len(project.Tasks))
	}

	byID := func(tasks []*models.Task, id string) *models.Task {
		for _, t := range tasks {
			if t.ID == id {
				return t
			}
		}
		return nil
	}

	checkTask := func(orig *models.Task) {
		t.Helper()
		got := byID(loaded.Tasks, orig.ID)
		if got == nil {
			t.Errorf("task %q (%s) not found after load", orig.Title, orig.ID)
			return
		}
		if got.Title != orig.Title {
			t.Errorf("%q: title got %q, want %q", orig.ID, got.Title, orig.Title)
		}
		if got.Type != orig.Type {
			t.Errorf("%q: type got %s, want %s", orig.ID, got.Type, orig.Type)
		}
		if got.State() != orig.State() {
			t.Errorf("%q: state got %s, want %s", orig.ID, got.State(), orig.State())
		}
		if got.Priority != orig.Priority {
			t.Errorf("%q: priority got %d, want %d", orig.ID, got.Priority, orig.Priority)
		}
		if got.Description != orig.Description {
			t.Errorf("%q: description got %q, want %q", orig.ID, got.Description, orig.Description)
		}
	}

	for _, orig := range project.Tasks {
		checkTask(orig)
	}

	// Tags preserved
	gotEpic := byID(loaded.Tasks, epic.ID)
	if gotEpic == nil {
		t.Fatal("epic not found")
	}
	if len(gotEpic.Tags) != 2 || gotEpic.Tags[0] != "backend" || gotEpic.Tags[1] != "release" {
		t.Errorf("epic tags: got %v, want [backend release]", gotEpic.Tags)
	}
	if !slices.Equal(loaded.PinnedTags, project.PinnedTags) {
		t.Errorf("project pinned tags: got %v, want %v", loaded.PinnedTags, project.PinnedTags)
	}

	// Section preserved (derived from the round-tripped state)
	if got := models.SectionForState(gotEpic.State()); got != "Next" {
		t.Errorf("epic section: got %q, want %q", got, "Next")
	}

	// Score dimensions preserved
	if gotEpic.ScoreDimensions["impact"] != 5 {
		t.Errorf("epic score impact: got %d, want 5", gotEpic.ScoreDimensions["impact"])
	}
	if gotEpic.ScoreDimensions["effort"] != 3 {
		t.Errorf("epic score effort: got %d, want 3", gotEpic.ScoreDimensions["effort"])
	}

	// Dependency preserved
	gotTask1 := byID(loaded.Tasks, task1.ID)
	if gotTask1 == nil {
		t.Fatal("task1 not found")
	}
	if len(gotTask1.DependsOn) != 1 || gotTask1.DependsOn[0] != story.ID {
		t.Errorf("task1 depends_on: got %v, want [%s]", gotTask1.DependsOn, story.ID)
	}
	if gotTask1.DueDate == nil || gotTask1.DueDate.Format("2006-01-02") != due.Format("2006-01-02") {
		t.Errorf("task1 due_date: got %v, want %v", gotTask1.DueDate, due)
	}
	if !gotTask1.Pinned {
		t.Errorf("task1 pinned: got false, want true")
	}
	// The legacy Active flag is retired and not serialized, so there is nothing
	// to assert on load.

	// Comments preserved
	gotTask2 := byID(loaded.Tasks, task2.ID)
	if gotTask2 == nil {
		t.Fatal("task2 not found")
	}
	if len(gotTask2.Comments) != 1 {
		t.Fatalf("task2 comments: got %d, want 1", len(gotTask2.Comments))
	}
	if gotTask2.Comments[0].Text != "First comment here." {
		t.Errorf("task2 comment text: got %q", gotTask2.Comments[0].Text)
	}

	// Code ref attachment preserved
	gotBug := byID(loaded.Tasks, bug.ID)
	if gotBug == nil {
		t.Fatal("bug not found")
	}
	if len(gotBug.Attachments) != 1 {
		t.Fatalf("bug attachments: got %d, want 1", len(gotBug.Attachments))
	}
	att := gotBug.Attachments[0]
	if !att.IsCodeRef {
		t.Errorf("bug attachment: IsCodeRef got false, want true")
	}
	if att.LineNumber != 42 {
		t.Errorf("bug attachment: LineNumber got %d, want 42", att.LineNumber)
	}

	// Recurrence preserved
	gotRecurring := byID(loaded.Tasks, recurring.ID)
	if gotRecurring == nil {
		t.Fatal("recurring task not found")
	}
	if gotRecurring.Recurrence == nil {
		t.Fatal("recurring task: Recurrence is nil after load")
	}
	if gotRecurring.Recurrence.Unit != models.RecurrenceWeekly {
		t.Errorf("recurring unit: got %s, want %s", gotRecurring.Recurrence.Unit, models.RecurrenceWeekly)
	}
	if len(gotRecurring.Recurrence.Weekdays) != 2 {
		t.Errorf("recurring weekdays: got %v, want [1 3]", gotRecurring.Recurrence.Weekdays)
	}

	// CompletedAt preserved
	gotDone := byID(loaded.Tasks, done.ID)
	if gotDone == nil {
		t.Fatal("done task not found")
	}
	if gotDone.CompletedAt == nil {
		t.Error("done task: CompletedAt is nil after load")
	}

	// Parent-child hierarchy preserved
	gotStory := byID(loaded.Tasks, story.ID)
	if gotStory == nil {
		t.Fatal("story not found")
	}
	if gotStory.ParentID == nil || *gotStory.ParentID != epic.ID {
		t.Errorf("story parent: got %v, want %s", gotStory.ParentID, epic.ID)
	}

	// Legacy project section colors are intentionally decode-only and are not
	// hydrated into the canonical runtime project.
	if len(loaded.SectionColors) != 0 {
		t.Errorf("section colors: got %v, want empty canonical runtime metadata", loaded.SectionColors)
	}
}

// TestRoundTrip_DescriptionWithHeadingPattern verifies that descriptions
// containing lines matching the task heading pattern (e.g., "## Task: ...")
// survive a round-trip without being mis-parsed as new tasks.
func TestRoundTrip_DescriptionWithHeadingPattern(t *testing.T) {
	now := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name string
		desc string
	}{
		{
			name: "task heading in description",
			desc: "Some text\n## Task: Looks like a heading\nMore text",
		},
		{
			name: "epic heading in description",
			desc: "Before\n## Epic: Fake epic\nAfter",
		},
		{
			name: "story heading in description",
			desc: "### Story: Not a real story",
		},
		{
			name: "bug heading in description",
			desc: "#### Bug: Not a bug heading\nstill description",
		},
		{
			name: "idea heading in description",
			desc: "## Idea: Just an idea mention",
		},
		{
			name: "project title in description",
			desc: "# Project: Embedded project name",
		},
		{
			name: "multiple heading-like lines",
			desc: "## Task: first\n### Story: second\n## Epic: third",
		},
		{
			name: "escaped backslash before heading mid-line",
			desc: `Line with \## Task: not a heading`,
		},
		{
			name: "backslash heading at line start",
			desc: `\## Task: not a heading`,
		},
		{
			name: "backslash project at line start",
			desc: `\# Project: not a project`,
		},
		{
			name: "double backslash heading at line start",
			desc: `\\## Task: double escaped`,
		},
		{
			name: "backslash heading in multiline",
			desc: "Some text\n\\## Task: Looks like a heading\nMore text",
		},
		{
			name: "no heading pattern (safe)",
			desc: "Normal description\nwith multiple lines",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			task, err := models.NewTask("id-1", 1, models.TaskTypeTask, "My Task")
			if err != nil {
				t.Fatal(err)
			}
			task.Description = tc.desc
			task.CreatedAt = now
			task.UpdatedAt = now

			md := WriteTasksMarkdown("Test", []*models.Task{task})
			_, tasks, err := ParseTasksMarkdown(md)
			if err != nil {
				t.Fatalf("parse error: %v", err)
			}
			if len(tasks) != 1 {
				t.Fatalf("expected 1 task, got %d (description with heading pattern was split into multiple tasks)", len(tasks))
			}
			if tasks[0].Description != tc.desc {
				t.Errorf("description mismatch:\n  got:  %q\n  want: %q", tasks[0].Description, tc.desc)
			}
		})
	}
}
