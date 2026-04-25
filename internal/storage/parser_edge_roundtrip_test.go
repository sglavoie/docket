package storage

import (
	"strings"
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/models"
)

// TestRoundTrip_DeepHierarchyPastLevel6 verifies that tasks nested deeper than
// 5 levels (heading ##-######) round-trip correctly using parent: metadata.
func TestRoundTrip_DeepHierarchyPastLevel6(t *testing.T) {
	epicID := "epic1"
	storyID := "story1"
	taskID := "task1"
	bugID := "bug1"
	ideaID := "idea1"

	tasks := []*models.Task{
		{
			ID: "epic1", ShortID: 1, Type: models.TaskTypeEpic,
			Title: "Root Epic", Status: models.StatusTodo, Priority: 1,
			Tags: []string{}, DependsOn: []string{}, RelatedTo: []string{},
			CreatedAt: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		},
		{
			ID: "story1", ShortID: 2, Type: models.TaskTypeStory,
			Title: "Child Story", Status: models.StatusTodo, Priority: 2,
			ParentID: &epicID,
			Tags:     []string{}, DependsOn: []string{}, RelatedTo: []string{},
			CreatedAt: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		},
		{
			ID: "task1", ShortID: 3, Type: models.TaskTypeTask,
			Title: "Grandchild Task", Status: models.StatusTodo, Priority: 3,
			ParentID: &storyID,
			Tags:     []string{}, DependsOn: []string{}, RelatedTo: []string{},
			CreatedAt: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		},
		{
			ID: "bug1", ShortID: 4, Type: models.TaskTypeBug,
			Title: "Great-grandchild Bug", Status: models.StatusTodo, Priority: 3,
			ParentID: &taskID,
			Tags:     []string{}, DependsOn: []string{}, RelatedTo: []string{},
			CreatedAt: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		},
		{
			ID: "idea1", ShortID: 5, Type: models.TaskTypeIdea,
			Title: "Deep Idea", Status: models.StatusTodo, Priority: 3,
			ParentID: &bugID,
			Tags:     []string{}, DependsOn: []string{}, RelatedTo: []string{},
			CreatedAt: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		},
		{
			ID: "task2", ShortID: 6, Type: models.TaskTypeTask,
			Title: "Very Deep Task", Status: models.StatusTodo, Priority: 3,
			ParentID: &ideaID,
			Tags:     []string{}, DependsOn: []string{}, RelatedTo: []string{},
			CreatedAt: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		},
	}

	content := WriteTasksMarkdown("Deep Test", tasks)

	for line := range strings.SplitSeq(content, "\n") {
		if strings.HasPrefix(line, "#") {
			hashes := 0
			for _, c := range line {
				if c == '#' {
					hashes++
				} else {
					break
				}
			}
			if hashes > 6 {
				t.Errorf("heading has %d # characters (max 6): %s", hashes, line)
			}
		}
	}

	_, parsed, err := ParseTasksMarkdown(content)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	if len(parsed) != len(tasks) {
		t.Fatalf("expected %d tasks, got %d", len(tasks), len(parsed))
	}

	parentMap := map[string]string{
		"story1": "epic1",
		"task1":  "story1",
		"bug1":   "task1",
		"idea1":  "bug1",
		"task2":  "idea1",
	}
	for _, p := range parsed {
		wantParent, ok := parentMap[p.ID]
		if !ok {
			if p.ParentID != nil {
				t.Errorf("task %s should have no parent, got %s", p.ID, *p.ParentID)
			}
			continue
		}
		if p.ParentID == nil {
			t.Errorf("task %s should have parent %s, got nil", p.ID, wantParent)
		} else if *p.ParentID != wantParent {
			t.Errorf("task %s parent = %s, want %s", p.ID, *p.ParentID, wantParent)
		}
	}
}

// TestRoundTrip_RecurrenceWeekdaysInMetadata verifies that recurrence patterns
// with comma-separated weekdays survive the metadata write→parse round-trip
// without the commas being misinterpreted as metadata field separators.
func TestRoundTrip_RecurrenceWeekdaysInMetadata(t *testing.T) {
	dueDate := time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)

	tasks := []*models.Task{
		{
			ID:       "t1",
			ShortID:  1,
			Type:     models.TaskTypeTask,
			Title:    "Weekly Task",
			Status:   models.StatusTodo,
			Priority: 3,
			DueDate:  &dueDate,
			Recurrence: &models.RecurrencePattern{
				Interval: 1,
				Unit:     models.RecurrenceWeekly,
				Weekdays: []int{1, 3, 5},
			},
			Tags:      []string{},
			DependsOn: []string{},
			RelatedTo: []string{},
			CreatedAt: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
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

	r := parsed[0].Recurrence
	if r == nil {
		t.Fatal("recurrence should not be nil after round-trip")
	}
	if r.Interval != 1 || r.Unit != models.RecurrenceWeekly {
		t.Errorf("recurrence base: got interval=%d unit=%s, want 1 week", r.Interval, r.Unit)
	}
	if len(r.Weekdays) != 3 {
		t.Fatalf("expected 3 weekdays, got %d: %v", len(r.Weekdays), r.Weekdays)
	}
	expected := []int{1, 3, 5}
	for i, wd := range r.Weekdays {
		if wd != expected[i] {
			t.Errorf("weekday[%d] = %d, want %d", i, wd, expected[i])
		}
	}
}

// TestRoundTrip_TaskWithAllMetadataFields exercises a task with every possible
// metadata field set to verify no field interactions cause data loss.
func TestRoundTrip_TaskWithAllMetadataFields(t *testing.T) { //nolint:gocyclo
	epicID := "parent-epic"
	due := time.Date(2024, 6, 15, 0, 0, 0, 0, time.Local)
	completed := time.Date(2024, 6, 10, 14, 30, 0, 0, time.Local)
	snoozed := time.Date(2024, 7, 1, 0, 0, 0, 0, time.Local)
	pinnedAt := time.Date(2024, 5, 1, 12, 0, 0, 0, time.Local)

	tasks := []*models.Task{
		{
			ID: "parent-epic", ShortID: 1, Type: models.TaskTypeEpic,
			Title: "Parent", Status: models.StatusInProgress, Priority: 1,
			Tags: []string{}, DependsOn: []string{}, RelatedTo: []string{},
			CreatedAt: time.Date(2024, 1, 1, 0, 0, 0, 0, time.Local),
		},
		{
			ID:           "full-task",
			ShortID:      2,
			Type:         models.TaskTypeBug,
			Title:        "Fully Loaded Bug",
			Description:  "A bug with every field set.",
			Status:       models.StatusDone,
			Priority:     1,
			ParentID:     &epicID,
			Tags:         []string{"urgent", "backend"},
			Section:      models.SectionNext,
			DueDate:      &due,
			CompletedAt:  &completed,
			DependsOn:    []string{"dep-1", "dep-2"},
			RelatedTo:    []string{"rel-1"},
			Pinned:       true,
			PinnedAt:     &pinnedAt,
			Active:       true,
			Dimmed:       true,
			SnoozedUntil: &snoozed,
			Stage:        "verification",
			Source:       "github",
			URL:          "https://example.com/issue/123",
			CreatedAt:    time.Date(2024, 1, 1, 10, 0, 0, 0, time.Local),
			UpdatedAt:    time.Date(2024, 6, 10, 14, 30, 0, 0, time.Local),
			Recurrence: &models.RecurrencePattern{
				Interval:    2,
				Unit:        models.RecurrenceWeekly,
				Weekdays:    []int{1, 5},
				Skipped:     3,
				Streak:      5,
				MaxStreak:   10,
				OriginalDay: 0,
			},
			Checklist: []models.ChecklistItem{
				{Text: "Step 1", Done: true},
				{Text: "Step 2", Done: false},
			},
			ScoreDimensions: map[string]int{
				"impact":  8,
				"urgency": 5,
			},
			BugAttrs: &models.BugAttrs{
				Reproducible: "always",
				Severity:     "critical",
				Workaround:   "Restart the service",
			},
		},
	}

	content := WriteTasksMarkdown("Full Test", tasks)
	_, parsed, err := ParseTasksMarkdown(content)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	if len(parsed) != 2 {
		t.Fatalf("expected 2 tasks, got %d", len(parsed))
	}

	p := parsed[1]
	if p.ID != "full-task" {
		t.Errorf("ID = %q, want 'full-task'", p.ID)
	}
	if p.Type != models.TaskTypeBug {
		t.Errorf("Type = %q, want 'bug'", p.Type)
	}
	if p.Status != models.StatusDone {
		t.Errorf("Status = %q, want 'done'", p.Status)
	}
	if p.Priority != 1 {
		t.Errorf("Priority = %d, want 1", p.Priority)
	}
	if p.ParentID == nil || *p.ParentID != "parent-epic" {
		t.Errorf("ParentID = %v, want 'parent-epic'", p.ParentID)
	}
	if len(p.Tags) != 2 {
		t.Errorf("Tags count = %d, want 2", len(p.Tags))
	}
	if p.Section != models.SectionNext {
		t.Errorf("Section = %q, want %q", p.Section, models.SectionNext)
	}
	if p.DueDate == nil {
		t.Error("DueDate should not be nil")
	}
	if p.CompletedAt == nil {
		t.Error("CompletedAt should not be nil")
	}
	if len(p.DependsOn) != 2 {
		t.Errorf("DependsOn count = %d, want 2", len(p.DependsOn))
	}
	if len(p.RelatedTo) != 1 {
		t.Errorf("RelatedTo count = %d, want 1", len(p.RelatedTo))
	}
	if !p.Pinned {
		t.Error("Pinned should be true")
	}
	if p.PinnedAt == nil {
		t.Error("PinnedAt should not be nil")
	}
	if !p.Active {
		t.Error("Active should be true")
	}
	if !p.Dimmed {
		t.Error("Dimmed should be true")
	}
	if p.SnoozedUntil == nil {
		t.Error("SnoozedUntil should not be nil")
	}
	if p.Stage != "verification" {
		t.Errorf("Stage = %q, want 'verification'", p.Stage)
	}
	if p.Source != "github" {
		t.Errorf("Source = %q, want 'github'", p.Source)
	}
	if p.URL != "https://example.com/issue/123" {
		t.Errorf("URL = %q, want 'https://example.com/issue/123'", p.URL)
	}
	if p.Recurrence == nil {
		t.Fatal("Recurrence should not be nil")
	}
	if p.Recurrence.Interval != 2 || p.Recurrence.Unit != models.RecurrenceWeekly {
		t.Errorf("Recurrence = %d %s, want 2 week", p.Recurrence.Interval, p.Recurrence.Unit)
	}
	if len(p.Recurrence.Weekdays) != 2 {
		t.Errorf("Recurrence.Weekdays = %v, want [1, 5]", p.Recurrence.Weekdays)
	}
	if p.Recurrence.Skipped != 3 {
		t.Errorf("Recurrence.Skipped = %d, want 3", p.Recurrence.Skipped)
	}
	if p.Recurrence.Streak != 5 {
		t.Errorf("Recurrence.Streak = %d, want 5", p.Recurrence.Streak)
	}
	if p.Recurrence.MaxStreak != 10 {
		t.Errorf("Recurrence.MaxStreak = %d, want 10", p.Recurrence.MaxStreak)
	}
	if len(p.Checklist) != 2 {
		t.Errorf("Checklist count = %d, want 2", len(p.Checklist))
	}
	if len(p.ScoreDimensions) != 2 {
		t.Errorf("ScoreDimensions count = %d, want 2", len(p.ScoreDimensions))
	}
	if p.BugAttrs == nil {
		t.Fatal("BugAttrs should not be nil")
	}
	if p.BugAttrs.Reproducible != "always" {
		t.Errorf("BugAttrs.Reproducible = %q, want 'always'", p.BugAttrs.Reproducible)
	}
	if p.BugAttrs.Severity != "critical" {
		t.Errorf("BugAttrs.Severity = %q, want 'critical'", p.BugAttrs.Severity)
	}
	if p.BugAttrs.Workaround != "Restart the service" {
		t.Errorf("BugAttrs.Workaround = %q, want 'Restart the service'", p.BugAttrs.Workaround)
	}
}
