package format

import (
	"strings"
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/models"
)

func TestRoundTrip_Attachments(t *testing.T) {
	now := time.Now()
	task := &models.Task{
		ID:       "att-1",
		Type:     models.TaskTypeTask,
		Status:   models.StatusTodo,
		Priority: 2,
		Attachments: []models.Attachment{
			{
				Filename:     "report.pdf",
				IsURL:        false,
				IsCodeRef:    false,
				OriginalPath: "/home/user/docs/report.pdf",
				AddedAt:      now,
			},
			{
				Filename:     "https://example.com/page",
				IsURL:        true,
				OriginalPath: "https://example.com/page",
				AddedAt:      now,
			},
			{
				Filename:     "main.go",
				IsURL:        false,
				IsCodeRef:    true,
				LineNumber:   42,
				OriginalPath: "/home/user/src/main.go",
				AddedAt:      now,
			},
		},
	}

	metaStr := WriteMetadata(task)
	content := metaStr[5 : len(metaStr)-4]
	parsed := &models.Task{}
	warnings := ParseMetadata(parsed, content)

	if len(warnings) != 0 {
		t.Errorf("got %d warnings, want 0: %v", len(warnings), warnings)
	}

	if len(parsed.Attachments) != 3 {
		t.Fatalf("got %d attachments, want 3", len(parsed.Attachments))
	}

	// File attachment
	got := parsed.Attachments[0]
	if got.Filename != "report.pdf" {
		t.Errorf("att[0].Filename = %q, want %q", got.Filename, "report.pdf")
	}
	if got.IsURL {
		t.Errorf("att[0].IsURL = true, want false")
	}
	if got.IsCodeRef {
		t.Errorf("att[0].IsCodeRef = true, want false")
	}
	if got.OriginalPath != "/home/user/docs/report.pdf" {
		t.Errorf("att[0].OriginalPath = %q, want %q", got.OriginalPath, "/home/user/docs/report.pdf")
	}
	if got.AddedAt.Unix() != now.Unix() {
		t.Errorf("att[0].AddedAt.Unix() = %d, want %d", got.AddedAt.Unix(), now.Unix())
	}

	// URL attachment
	got = parsed.Attachments[1]
	if got.Filename != "https://example.com/page" {
		t.Errorf("att[1].Filename = %q, want %q", got.Filename, "https://example.com/page")
	}
	if !got.IsURL {
		t.Errorf("att[1].IsURL = false, want true")
	}
	if got.OriginalPath != "https://example.com/page" {
		t.Errorf("att[1].OriginalPath = %q, want %q", got.OriginalPath, "https://example.com/page")
	}
	if got.AddedAt.Unix() != now.Unix() {
		t.Errorf("att[1].AddedAt.Unix() = %d, want %d", got.AddedAt.Unix(), now.Unix())
	}

	// Code reference attachment
	got = parsed.Attachments[2]
	if got.Filename != "main.go" {
		t.Errorf("att[2].Filename = %q, want %q", got.Filename, "main.go")
	}
	if !got.IsCodeRef {
		t.Errorf("att[2].IsCodeRef = false, want true")
	}
	if got.LineNumber != 42 {
		t.Errorf("att[2].LineNumber = %d, want 42", got.LineNumber)
	}
	if got.OriginalPath != "/home/user/src/main.go" {
		t.Errorf("att[2].OriginalPath = %q, want %q", got.OriginalPath, "/home/user/src/main.go")
	}
	if got.AddedAt.Unix() != now.Unix() {
		t.Errorf("att[2].AddedAt.Unix() = %d, want %d", got.AddedAt.Unix(), now.Unix())
	}
}

func TestRoundTrip_RecurrenceSubFields(t *testing.T) {
	task := &models.Task{
		ID:       "recur-sub-1",
		Type:     models.TaskTypeTask,
		Status:   models.StatusTodo,
		Priority: 1,
		Recurrence: &models.RecurrencePattern{
			Interval:  1,
			Unit:      models.RecurrenceDaily,
			Skipped:   3,
			Streak:    5,
			MaxStreak: 10,
		},
	}

	metaStr := WriteMetadata(task)
	content := metaStr[5 : len(metaStr)-4]
	parsed := &models.Task{}
	warnings := ParseMetadata(parsed, content)

	if len(warnings) != 0 {
		t.Errorf("got %d warnings, want 0: %v", len(warnings), warnings)
	}

	if parsed.Recurrence == nil {
		t.Fatal("Recurrence is nil after round-trip")
	}
	if parsed.Recurrence.Interval != 1 {
		t.Errorf("Interval = %d, want 1", parsed.Recurrence.Interval)
	}
	if parsed.Recurrence.Unit != models.RecurrenceDaily {
		t.Errorf("Unit = %s, want %s", parsed.Recurrence.Unit, models.RecurrenceDaily)
	}
	if parsed.Recurrence.Skipped != 3 {
		t.Errorf("Skipped = %d, want 3", parsed.Recurrence.Skipped)
	}
	if parsed.Recurrence.Streak != 5 {
		t.Errorf("Streak = %d, want 5", parsed.Recurrence.Streak)
	}
	if parsed.Recurrence.MaxStreak != 10 {
		t.Errorf("MaxStreak = %d, want 10", parsed.Recurrence.MaxStreak)
	}
}

func TestRoundTrip_SnoozedUntil(t *testing.T) {
	snoozed := time.Date(2026, 6, 15, 0, 0, 0, 0, time.Local)
	task := &models.Task{
		ID:           "snooze-1",
		Type:         models.TaskTypeTask,
		Status:       models.StatusTodo,
		Priority:     2,
		SnoozedUntil: &snoozed,
	}

	metaStr := WriteMetadata(task)
	content := metaStr[5 : len(metaStr)-4]
	parsed := &models.Task{}
	warnings := ParseMetadata(parsed, content)

	if len(warnings) != 0 {
		t.Errorf("got %d warnings, want 0: %v", len(warnings), warnings)
	}

	if parsed.SnoozedUntil == nil {
		t.Fatal("SnoozedUntil is nil after round-trip")
	}

	// Snoozed uses date-only format, so compare dates only
	wantDate := snoozed.Format("2006-01-02")
	gotDate := parsed.SnoozedUntil.Format("2006-01-02")
	if gotDate != wantDate {
		t.Errorf("SnoozedUntil date = %q, want %q", gotDate, wantDate)
	}
}

func TestRoundTrip_PinnedAt(t *testing.T) {
	pinnedAt := time.Date(2026, 3, 10, 14, 30, 45, 0, time.Local)
	task := &models.Task{
		ID:       "pin-1",
		Type:     models.TaskTypeTask,
		Status:   models.StatusTodo,
		Priority: 1,
		Pinned:   true,
		PinnedAt: &pinnedAt,
	}

	metaStr := WriteMetadata(task)
	content := metaStr[5 : len(metaStr)-4]
	parsed := &models.Task{}
	warnings := ParseMetadata(parsed, content)

	if len(warnings) != 0 {
		t.Errorf("got %d warnings, want 0: %v", len(warnings), warnings)
	}

	if parsed.PinnedAt == nil {
		t.Fatal("PinnedAt is nil after round-trip")
	}

	// PinnedAt uses second-level precision
	wantStr := pinnedAt.Format("2006-01-02 15:04:05")
	gotStr := parsed.PinnedAt.Format("2006-01-02 15:04:05")
	if gotStr != wantStr {
		t.Errorf("PinnedAt = %q, want %q", gotStr, wantStr)
	}
}

func TestRoundTrip_RelatedTo(t *testing.T) {
	task := &models.Task{
		ID:        "rel-1",
		Type:      models.TaskTypeTask,
		Status:    models.StatusTodo,
		Priority:  2,
		RelatedTo: []string{"id1", "id2"},
	}

	metaStr := WriteMetadata(task)
	content := metaStr[5 : len(metaStr)-4]
	parsed := &models.Task{}
	warnings := ParseMetadata(parsed, content)

	if len(warnings) != 0 {
		t.Errorf("got %d warnings, want 0: %v", len(warnings), warnings)
	}

	if !stringSlicesEqual(parsed.RelatedTo, []string{"id1", "id2"}) {
		t.Errorf("RelatedTo = %v, want %v", parsed.RelatedTo, []string{"id1", "id2"})
	}
}

func TestRoundTrip_BugAttrs(t *testing.T) {
	task := &models.Task{
		ID:       "bug-1",
		Type:     models.TaskTypeBug,
		Status:   models.StatusTodo,
		Priority: 1,
		BugAttrs: &models.BugAttrs{
			Reproducible: models.BugReproducibleYes,
			Severity:     models.BugSeverityCritical,
			Workaround:   "restart the service",
		},
	}

	metaStr := WriteMetadata(task)
	content := metaStr[5 : len(metaStr)-4]
	parsed := &models.Task{Type: models.TaskTypeBug}
	warnings := ParseMetadata(parsed, content)

	if len(warnings) != 0 {
		t.Errorf("got %d warnings, want 0: %v", len(warnings), warnings)
	}

	if parsed.BugAttrs == nil {
		t.Fatal("BugAttrs is nil after round-trip")
	}
	if parsed.BugAttrs.Reproducible != models.BugReproducibleYes {
		t.Errorf("Reproducible = %q, want %q", parsed.BugAttrs.Reproducible, models.BugReproducibleYes)
	}
	if parsed.BugAttrs.Severity != models.BugSeverityCritical {
		t.Errorf("Severity = %q, want %q", parsed.BugAttrs.Severity, models.BugSeverityCritical)
	}
	if parsed.BugAttrs.Workaround != "restart the service" {
		t.Errorf("Workaround = %q, want %q", parsed.BugAttrs.Workaround, "restart the service")
	}
}

func TestRoundTrip_IdeaAttrs(t *testing.T) {
	task := &models.Task{
		ID:       "idea-1",
		Type:     models.TaskTypeIdea,
		Status:   models.StatusTodo,
		Priority: 3,
		IdeaAttrs: &models.IdeaAttrs{
			Category:  models.IdeaCategoryFeature,
			Effort:    models.EffortMedium,
			Value:     models.IdeaValueHigh,
			Validated: true,
		},
	}

	metaStr := WriteMetadata(task)
	content := metaStr[5 : len(metaStr)-4]
	parsed := &models.Task{Type: models.TaskTypeIdea}
	warnings := ParseMetadata(parsed, content)

	if len(warnings) != 0 {
		t.Errorf("got %d warnings, want 0: %v", len(warnings), warnings)
	}

	if parsed.IdeaAttrs == nil {
		t.Fatal("IdeaAttrs is nil after round-trip")
	}
	if parsed.IdeaAttrs.Category != models.IdeaCategoryFeature {
		t.Errorf("Category = %q, want %q", parsed.IdeaAttrs.Category, models.IdeaCategoryFeature)
	}
	if parsed.IdeaAttrs.Effort != models.EffortMedium {
		t.Errorf("Effort = %q, want %q", parsed.IdeaAttrs.Effort, models.EffortMedium)
	}
	if parsed.IdeaAttrs.Value != models.IdeaValueHigh {
		t.Errorf("Value = %q, want %q", parsed.IdeaAttrs.Value, models.IdeaValueHigh)
	}
	if !parsed.IdeaAttrs.Validated {
		t.Errorf("Validated = false, want true")
	}
}

func TestRoundTrip_SourceAndURL(t *testing.T) {
	task := &models.Task{
		ID:       "src-1",
		Type:     models.TaskTypeTask,
		Status:   models.StatusTodo,
		Priority: 2,
		Source:   "cli",
		URL:      "https://example.com",
	}

	metaStr := WriteMetadata(task)
	content := metaStr[5 : len(metaStr)-4]
	parsed := &models.Task{}
	warnings := ParseMetadata(parsed, content)

	if len(warnings) != 0 {
		t.Errorf("got %d warnings, want 0: %v", len(warnings), warnings)
	}

	if parsed.Source != "cli" {
		t.Errorf("Source = %q, want %q", parsed.Source, "cli")
	}
	if parsed.URL != "https://example.com" {
		t.Errorf("URL = %q, want %q", parsed.URL, "https://example.com")
	}
}

func TestRoundTrip_KitchenSink(t *testing.T) { //nolint:gocyclo
	now := time.Now()
	created := time.Date(2026, 1, 10, 8, 0, 0, 0, time.Local)
	updated := time.Date(2026, 1, 15, 12, 30, 0, 0, time.Local)
	completed := time.Date(2026, 1, 20, 9, 15, 0, 0, time.Local)
	due := time.Date(2026, 2, 1, 0, 0, 0, 0, time.Local)
	snoozed := time.Date(2026, 3, 1, 0, 0, 0, 0, time.Local)
	pinnedAt := time.Date(2026, 1, 12, 14, 30, 45, 0, time.Local)

	task := &models.Task{
		ID:            "kitchen-sink-1",
		Type:          models.TaskTypeBug,
		Status:        models.StatusDone,
		Priority:      1,
		Tags:          []string{"backend", "urgent", "p0"},
		Section:       "Next",
		CustomSection: "Infrastructure",
		CreatedAt:     created,
		UpdatedAt:     updated,
		CompletedAt:   &completed,
		DueDate:       &due,
		DependsOn:     []string{"dep-1", "dep-2", "dep-3"},
		RelatedTo:     []string{"rel-1", "rel-2"},
		Pinned:        true,
		PinnedAt:      &pinnedAt,
		Active:        true,
		SnoozedUntil:  &snoozed,
		Recurrence: &models.RecurrencePattern{
			Interval:  2,
			Unit:      models.RecurrenceWeekly,
			Weekdays:  []int{1, 3, 5},
			Skipped:   7,
			Streak:    12,
			MaxStreak: 20,
		},
		Attachments: []models.Attachment{
			{
				Filename:     "notes.txt",
				OriginalPath: "/tmp/notes.txt",
				AddedAt:      now,
			},
			{
				Filename:     "https://github.com/issue/1",
				IsURL:        true,
				OriginalPath: "https://github.com/issue/1",
				AddedAt:      now,
			},
		},
		Checklist: []models.ChecklistItem{
			{Text: "Step one", Done: true},
			{Text: "Step two", Done: false},
		},
		Stage:  "Implement",
		Source: "github",
		URL:    "https://github.com/org/repo/issues/42",
		ScoreDimensions: map[string]int{
			"impact": 4,
			"effort": 2,
		},
		BugAttrs: &models.BugAttrs{
			Reproducible: models.BugReproducibleYes,
			Severity:     models.BugSeverityMajor,
			Workaround:   "use the old endpoint",
		},
	}

	metaStr := WriteMetadata(task)
	content := metaStr[5 : len(metaStr)-4]
	parsed := &models.Task{Type: task.Type}
	warnings := ParseMetadata(parsed, content)

	if len(warnings) != 0 {
		t.Errorf("got %d warnings, want 0: %v", len(warnings), warnings)
	}

	// Core fields
	if parsed.ID != task.ID {
		t.Errorf("ID = %q, want %q", parsed.ID, task.ID)
	}
	if parsed.Status != task.Status {
		t.Errorf("Status = %q, want %q", parsed.Status, task.Status)
	}
	if parsed.Priority != task.Priority {
		t.Errorf("Priority = %d, want %d", parsed.Priority, task.Priority)
	}

	// Tags
	if !stringSlicesEqual(parsed.Tags, task.Tags) {
		t.Errorf("Tags = %v, want %v", parsed.Tags, task.Tags)
	}

	// Sections
	if parsed.Section != task.Section {
		t.Errorf("Section = %q, want %q", parsed.Section, task.Section)
	}
	if parsed.CustomSection != task.CustomSection {
		t.Errorf("CustomSection = %q, want %q", parsed.CustomSection, task.CustomSection)
	}

	// Dates (minute-level precision)
	if parsed.CreatedAt.Format("2006-01-02 15:04") != created.Format("2006-01-02 15:04") {
		t.Errorf("CreatedAt = %v, want %v", parsed.CreatedAt, created)
	}
	if parsed.UpdatedAt.Format("2006-01-02 15:04") != updated.Format("2006-01-02 15:04") {
		t.Errorf("UpdatedAt = %v, want %v", parsed.UpdatedAt, updated)
	}
	if parsed.CompletedAt == nil {
		t.Fatal("CompletedAt is nil")
	}
	if parsed.CompletedAt.Format("2006-01-02 15:04") != completed.Format("2006-01-02 15:04") {
		t.Errorf("CompletedAt = %v, want %v", *parsed.CompletedAt, completed)
	}

	// Due date (date-only)
	if parsed.DueDate == nil {
		t.Fatal("DueDate is nil")
	}
	if parsed.DueDate.Format("2006-01-02") != due.Format("2006-01-02") {
		t.Errorf("DueDate = %v, want %v", *parsed.DueDate, due)
	}

	// Dependencies and related
	if !stringSlicesEqual(parsed.DependsOn, task.DependsOn) {
		t.Errorf("DependsOn = %v, want %v", parsed.DependsOn, task.DependsOn)
	}
	if !stringSlicesEqual(parsed.RelatedTo, task.RelatedTo) {
		t.Errorf("RelatedTo = %v, want %v", parsed.RelatedTo, task.RelatedTo)
	}

	// Boolean flags
	if !parsed.Pinned {
		t.Errorf("Pinned = false, want true")
	}
	if !parsed.Active {
		t.Errorf("Active = false, want true")
	}

	// PinnedAt (second-level)
	if parsed.PinnedAt == nil {
		t.Fatal("PinnedAt is nil")
	}
	if parsed.PinnedAt.Format("2006-01-02 15:04:05") != pinnedAt.Format("2006-01-02 15:04:05") {
		t.Errorf("PinnedAt = %q, want %q", parsed.PinnedAt.Format("2006-01-02 15:04:05"), pinnedAt.Format("2006-01-02 15:04:05"))
	}

	// SnoozedUntil (date-only)
	if parsed.SnoozedUntil == nil {
		t.Fatal("SnoozedUntil is nil")
	}
	if parsed.SnoozedUntil.Format("2006-01-02") != snoozed.Format("2006-01-02") {
		t.Errorf("SnoozedUntil = %q, want %q", parsed.SnoozedUntil.Format("2006-01-02"), snoozed.Format("2006-01-02"))
	}

	// Recurrence with sub-fields
	if parsed.Recurrence == nil {
		t.Fatal("Recurrence is nil")
	}
	if !recurrenceEqual(parsed.Recurrence, task.Recurrence) {
		t.Errorf("Recurrence = %+v, want %+v", parsed.Recurrence, task.Recurrence)
	}

	// Attachments
	if len(parsed.Attachments) != 2 {
		t.Fatalf("got %d attachments, want 2", len(parsed.Attachments))
	}
	if parsed.Attachments[0].Filename != "notes.txt" {
		t.Errorf("att[0].Filename = %q, want %q", parsed.Attachments[0].Filename, "notes.txt")
	}
	if parsed.Attachments[0].OriginalPath != "/tmp/notes.txt" {
		t.Errorf("att[0].OriginalPath = %q, want %q", parsed.Attachments[0].OriginalPath, "/tmp/notes.txt")
	}
	if !parsed.Attachments[1].IsURL {
		t.Errorf("att[1].IsURL = false, want true")
	}

	// Checklist
	if len(parsed.Checklist) != 2 {
		t.Fatalf("got %d checklist items, want 2", len(parsed.Checklist))
	}
	if parsed.Checklist[0].Text != "Step one" || !parsed.Checklist[0].Done {
		t.Errorf("checklist[0] = {%q, %v}, want {%q, true}", parsed.Checklist[0].Text, parsed.Checklist[0].Done, "Step one")
	}
	if parsed.Checklist[1].Text != "Step two" || parsed.Checklist[1].Done {
		t.Errorf("checklist[1] = {%q, %v}, want {%q, false}", parsed.Checklist[1].Text, parsed.Checklist[1].Done, "Step two")
	}

	// Stage, Source, URL
	if parsed.Stage != "Implement" {
		t.Errorf("Stage = %q, want %q", parsed.Stage, "Implement")
	}
	if parsed.Source != "github" {
		t.Errorf("Source = %q, want %q", parsed.Source, "github")
	}
	if parsed.URL != "https://github.com/org/repo/issues/42" {
		t.Errorf("URL = %q, want %q", parsed.URL, "https://github.com/org/repo/issues/42")
	}

	// Score dimensions
	if len(parsed.ScoreDimensions) != 2 {
		t.Fatalf("got %d score dimensions, want 2", len(parsed.ScoreDimensions))
	}
	if parsed.ScoreDimensions["impact"] != 4 {
		t.Errorf("score.impact = %d, want 4", parsed.ScoreDimensions["impact"])
	}
	if parsed.ScoreDimensions["effort"] != 2 {
		t.Errorf("score.effort = %d, want 2", parsed.ScoreDimensions["effort"])
	}

	// Bug attrs
	if parsed.BugAttrs == nil {
		t.Fatal("BugAttrs is nil")
	}
	if parsed.BugAttrs.Reproducible != models.BugReproducibleYes {
		t.Errorf("bug.Reproducible = %q, want %q", parsed.BugAttrs.Reproducible, models.BugReproducibleYes)
	}
	if parsed.BugAttrs.Severity != models.BugSeverityMajor {
		t.Errorf("bug.Severity = %q, want %q", parsed.BugAttrs.Severity, models.BugSeverityMajor)
	}
	if parsed.BugAttrs.Workaround != "use the old endpoint" {
		t.Errorf("bug.Workaround = %q, want %q", parsed.BugAttrs.Workaround, "use the old endpoint")
	}
}

func TestRoundTrip_ParentID(t *testing.T) {
	parentID := "parent-abc-123"
	task := &models.Task{
		ID:       "child-xyz-456",
		Type:     models.TaskTypeTask,
		Status:   models.StatusTodo,
		Priority: 2,
		ParentID: &parentID,
	}

	metaStr := WriteMetadata(task)

	// Verify parent: appears in output
	if !strings.Contains(metaStr, "parent: parent-abc-123") {
		t.Errorf("WriteMetadata should include parent field, got: %s", metaStr)
	}

	content := metaStr[5 : len(metaStr)-4]
	parsed := &models.Task{}
	warnings := ParseMetadata(parsed, content)

	if len(warnings) != 0 {
		t.Errorf("got %d warnings, want 0: %v", len(warnings), warnings)
	}

	if parsed.ParentID == nil {
		t.Fatal("ParentID is nil after round-trip")
	}
	if *parsed.ParentID != parentID {
		t.Errorf("ParentID = %q, want %q", *parsed.ParentID, parentID)
	}
}

func TestRoundTrip_NilParentID(t *testing.T) {
	task := &models.Task{
		ID:       "root-task-1",
		Type:     models.TaskTypeTask,
		Status:   models.StatusTodo,
		Priority: 1,
	}

	metaStr := WriteMetadata(task)

	// Verify parent: does NOT appear for root tasks
	if strings.Contains(metaStr, "parent:") {
		t.Errorf("WriteMetadata should not include parent field for nil ParentID, got: %s", metaStr)
	}

	content := metaStr[5 : len(metaStr)-4]
	parsed := &models.Task{}
	warnings := ParseMetadata(parsed, content)

	if len(warnings) != 0 {
		t.Errorf("got %d warnings, want 0: %v", len(warnings), warnings)
	}

	if parsed.ParentID != nil {
		t.Errorf("ParentID should be nil, got %q", *parsed.ParentID)
	}
}

func TestRoundTrip_DependsOnCrossProject(t *testing.T) {
	task := &models.Task{
		ID:       "local-task-1",
		Type:     models.TaskTypeTask,
		Status:   models.StatusTodo,
		Priority: 2,
		DependsOn: []string{
			"local-dep-abc",
			"projID:task-xyz",        // cross-project reference
			"other-proj:another-123", // another cross-project reference
		},
	}

	metaStr := WriteMetadata(task)
	content := metaStr[5 : len(metaStr)-4]
	parsed := &models.Task{}
	warnings := ParseMetadata(parsed, content)

	if len(warnings) != 0 {
		t.Errorf("got %d warnings, want 0: %v", len(warnings), warnings)
	}

	want := []string{"local-dep-abc", "projID:task-xyz", "other-proj:another-123"}
	if !stringSlicesEqual(parsed.DependsOn, want) {
		t.Errorf("DependsOn = %v, want %v", parsed.DependsOn, want)
	}
}

func TestRoundTrip_RecurrenceMonthly(t *testing.T) {
	task := &models.Task{
		ID:       "monthly-task-1",
		Type:     models.TaskTypeTask,
		Status:   models.StatusTodo,
		Priority: 2,
		Recurrence: &models.RecurrencePattern{
			Interval:  2,
			Unit:      models.RecurrenceMonthly,
			Skipped:   1,
			Streak:    4,
			MaxStreak: 8,
		},
	}

	metaStr := WriteMetadata(task)
	content := metaStr[5 : len(metaStr)-4]
	parsed := &models.Task{}
	warnings := ParseMetadata(parsed, content)

	if len(warnings) != 0 {
		t.Errorf("got %d warnings, want 0: %v", len(warnings), warnings)
	}
	if parsed.Recurrence == nil {
		t.Fatal("Recurrence is nil after round-trip")
	}
	if parsed.Recurrence.Unit != models.RecurrenceMonthly {
		t.Errorf("Unit = %q, want %q", parsed.Recurrence.Unit, models.RecurrenceMonthly)
	}
	if parsed.Recurrence.Interval != 2 {
		t.Errorf("Interval = %d, want 2", parsed.Recurrence.Interval)
	}
	if parsed.Recurrence.Skipped != 1 {
		t.Errorf("Skipped = %d, want 1", parsed.Recurrence.Skipped)
	}
	if parsed.Recurrence.Streak != 4 {
		t.Errorf("Streak = %d, want 4", parsed.Recurrence.Streak)
	}
	if parsed.Recurrence.MaxStreak != 8 {
		t.Errorf("MaxStreak = %d, want 8", parsed.Recurrence.MaxStreak)
	}
}

func TestRoundTrip_KitchenSinkWithParentID(t *testing.T) {
	parentID := "parent-epic-001"
	task := &models.Task{
		ID:        "full-task-1",
		Type:      models.TaskTypeTask,
		Status:    models.StatusInProgress,
		Priority:  1,
		ParentID:  &parentID,
		Tags:      []string{"alpha", "beta"},
		Section:   "Next",
		DependsOn: []string{"dep-local", "proj2:dep-cross"},
		RelatedTo: []string{"rel-1"},
		Recurrence: &models.RecurrencePattern{
			Interval: 3,
			Unit:     models.RecurrenceMonthly,
		},
		Stage:  "Design",
		Source: "jira",
		URL:    "https://jira.example.com/browse/PROJ-1",
		ScoreDimensions: map[string]int{
			"impact": 5,
		},
	}

	metaStr := WriteMetadata(task)
	content := metaStr[5 : len(metaStr)-4]
	parsed := &models.Task{}
	warnings := ParseMetadata(parsed, content)

	if len(warnings) != 0 {
		t.Errorf("got %d warnings, want 0: %v", len(warnings), warnings)
	}
	if parsed.ParentID == nil {
		t.Fatal("ParentID is nil after round-trip")
	}
	if *parsed.ParentID != parentID {
		t.Errorf("ParentID = %q, want %q", *parsed.ParentID, parentID)
	}
	if !stringSlicesEqual(parsed.DependsOn, task.DependsOn) {
		t.Errorf("DependsOn = %v, want %v", parsed.DependsOn, task.DependsOn)
	}
	if !stringSlicesEqual(parsed.RelatedTo, task.RelatedTo) {
		t.Errorf("RelatedTo = %v, want %v", parsed.RelatedTo, task.RelatedTo)
	}
	if parsed.Recurrence == nil {
		t.Fatal("Recurrence is nil after round-trip")
	}
	if parsed.Recurrence.Unit != models.RecurrenceMonthly {
		t.Errorf("Recurrence.Unit = %q, want %q", parsed.Recurrence.Unit, models.RecurrenceMonthly)
	}
	if parsed.Recurrence.Interval != 3 {
		t.Errorf("Recurrence.Interval = %d, want 3", parsed.Recurrence.Interval)
	}
}

// TestParse_AttachmentMalformedDoubleBar verifies that an attachment entry whose
// original path contains "||" (i.e. was manually edited or produced by a buggy
// writer) is flagged with a warning instead of being silently corrupted.
func TestParse_AttachmentMalformedDoubleBar(t *testing.T) {
	// Craft a raw metadata string with a malformed attachment: original path
	// contains a second "||" sequence, which the validator should catch.
	// Format: "filename||originalPath@epoch" where originalPath has "||" in it.
	metaStr := "id: att-bad, status: todo, priority: 1, attachments: report.pdf||/path/with||double@1609459200"

	task := &models.Task{}
	warnings := ParseMetadata(task, metaStr)

	if len(warnings) == 0 {
		t.Error("expected at least one warning for malformed attachment, got none")
	}

	found := false
	for _, w := range warnings {
		if w.Field == "attachments" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected a warning with Field=attachments, got: %v", warnings)
	}

	// The attachment should still be parsed (graceful degradation), not dropped.
	if len(task.Attachments) == 0 {
		t.Error("expected attachment to be present even when malformed (graceful degradation)")
	}
}
