package format

import (
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestConvertV5TaskMapsLifecycleAndPriority(t *testing.T) {
	now := time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC)
	states := []struct {
		from       string
		to         string
		nextMember bool
	}{
		{from: "inbox", to: "inbox"},
		{from: "ready", to: "backlog"},
		{from: "next", to: "backlog", nextMember: true},
		{from: "in_progress", to: "doing"},
		{from: "waiting", to: "waiting"},
		{from: "paused", to: "paused"},
		{from: "someday", to: "someday"},
		{from: "done", to: "done"},
		{from: "canceled", to: "canceled"},
	}
	priorities := map[int]int{0: 0, 1: 1, 2: 1, 3: 0, 4: 3, 5: 3}

	for _, state := range states {
		for fromPriority, toPriority := range priorities {
			name := state.from + "-priority-" + string(rune('0'+fromPriority))
			t.Run(name, func(t *testing.T) {
				source := baseV5TaskForConversion(now)
				source.State = state.from
				source.Priority = fromPriority
				result, err := ConvertV5Task(source)
				if err != nil {
					t.Fatalf("ConvertV5Task: %v", err)
				}
				if result.Task.State != state.to || result.NextMember != state.nextMember {
					t.Fatalf("state conversion = (%q, next=%v), want (%q, next=%v)", result.Task.State, result.NextMember, state.to, state.nextMember)
				}
				if result.Task.Priority != toPriority {
					t.Fatalf("priority conversion = %d, want %d", result.Task.Priority, toPriority)
				}
			})
		}
	}
}

func TestConvertV5TaskMergesCommentsAndDecisionsChronologically(t *testing.T) {
	now := time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC)
	source := baseV5TaskForConversion(now)
	source.Comments = []V5TaskComment{
		{Text: "same first comment", CreatedAt: now.Add(2 * time.Minute)},
		{Text: "earliest comment", CreatedAt: now},
		{Text: "same second comment", CreatedAt: now.Add(2 * time.Minute)},
	}
	source.Decisions = []V5TaskDecision{
		{Text: "middle decision", Rationale: "because\nwith detail", CreatedAt: now.Add(time.Minute)},
		{Text: "same first decision", CreatedAt: now.Add(2 * time.Minute)},
		{Text: "same second decision", Rationale: "same time", CreatedAt: now.Add(2 * time.Minute)},
	}

	result, err := ConvertV5Task(source)
	if err != nil {
		t.Fatalf("ConvertV5Task: %v", err)
	}
	want := []V6TaskComment{
		{Text: "earliest comment", CreatedAt: now},
		{Text: "Decision: middle decision\nRationale: because\nwith detail", CreatedAt: now.Add(time.Minute)},
		{Text: "same first comment", CreatedAt: now.Add(2 * time.Minute)},
		{Text: "same second comment", CreatedAt: now.Add(2 * time.Minute)},
		{Text: "Decision: same first decision", CreatedAt: now.Add(2 * time.Minute)},
		{Text: "Decision: same second decision\nRationale: same time", CreatedAt: now.Add(2 * time.Minute)},
	}
	if !reflect.DeepEqual(result.Task.Comments, want) {
		t.Fatalf("merged comments:\n got: %#v\nwant: %#v", result.Task.Comments, want)
	}
	wantAudit := []V6TaskMigrationAuditEntry{{
		TaskID: source.ID,
		Action: "converted",
		Field:  "decisions",
		From:   "3 decisions",
		To:     "3 comments",
		Reason: "schema 6 converts task decisions into comments",
	}}
	if !reflect.DeepEqual(result.Audit, wantAudit) {
		t.Fatalf("decision conversion audit:\n got: %#v\nwant: %#v", result.Audit, wantAudit)
	}
}

func TestConvertV5TaskRetainsSupportedFieldsWithoutAliasing(t *testing.T) {
	now := time.Date(2026, 8, 1, 12, 0, 0, 123, time.FixedZone("CST", -6*60*60))
	completed := now.Add(4 * time.Hour)
	pinnedAt := now.Add(time.Minute)
	addedAt := now.Add(2 * time.Minute)
	source := baseV5TaskForConversion(now)
	source.Description = "Description\nwith detail"
	source.ParentID = "parent-task"
	source.Tags = []string{"bug", "source:github"}
	source.UpdatedAt = now.Add(3 * time.Hour)
	source.CompletedAt = &completed
	source.DueDate = "2026-08-03"
	source.SnoozedUntil = "2026-08-02"
	source.Pinned = true
	source.PinnedAt = &pinnedAt
	source.Attachments = []V5TaskAttachment{{
		Kind: "code", Filename: "convert.go", OriginalPath: "internal/convert.go", LineNumber: 42, AddedAt: &addedAt,
	}}
	source.Checklist = []V5ChecklistItem{{Text: "Retain this", Done: true}}
	source.Comments = []V5TaskComment{{Text: "Retain this too", CreatedAt: now.Add(3 * time.Minute)}}

	result, err := ConvertV5Task(source)
	if err != nil {
		t.Fatalf("ConvertV5Task: %v", err)
	}
	want := V6TaskRecord{
		ID: source.ID, ShortID: source.ShortID, Title: source.Title,
		Description: source.Description, State: "backlog", Priority: 0,
		ParentID: source.ParentID, Tags: []string{"bug", "source:github"},
		CreatedAt: source.CreatedAt, UpdatedAt: source.UpdatedAt, CompletedAt: &completed,
		DueDate: source.DueDate, SnoozedUntil: source.SnoozedUntil,
		Pinned: true, PinnedAt: &pinnedAt,
		Attachments: []V6TaskAttachment{{
			Kind: "code", Filename: "convert.go", OriginalPath: "internal/convert.go", LineNumber: 42, AddedAt: &addedAt,
		}},
		Checklist: []V6ChecklistItem{{Text: "Retain this", Done: true}},
		Comments:  []V6TaskComment{{Text: "Retain this too", CreatedAt: now.Add(3 * time.Minute)}},
	}
	if !reflect.DeepEqual(result.Task, want) {
		t.Fatalf("retained task:\n got: %#v\nwant: %#v", result.Task, want)
	}

	*source.CompletedAt = time.Time{}
	*source.PinnedAt = time.Time{}
	*source.Attachments[0].AddedAt = time.Time{}
	source.Tags[0] = "changed"
	source.Attachments[0].Filename = "changed"
	source.Checklist[0].Text = "changed"
	source.Comments[0].Text = "changed"
	if result.Task.CompletedAt.IsZero() || result.Task.PinnedAt.IsZero() || result.Task.Attachments[0].AddedAt.IsZero() {
		t.Fatal("converted timestamp pointers alias the source")
	}
	if result.Task.Tags[0] != "bug" || result.Task.Attachments[0].Filename != "convert.go" || result.Task.Checklist[0].Text != "Retain this" || result.Task.Comments[0].Text != "Retain this too" {
		t.Fatal("converted collections alias the source")
	}
}

func TestConvertV5TaskAuditsEveryRetiredFieldValue(t *testing.T) {
	now := time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC)
	source := baseV5TaskForConversion(now)
	source.DependsOn = []string{"dependency-a", "dependency-b"}
	source.Recurrence = &V5TaskRecurrence{Interval: 2, Unit: "week"}

	result, err := ConvertV5Task(source)
	if err != nil {
		t.Fatalf("ConvertV5Task: %v", err)
	}
	want := []V6TaskMigrationAuditEntry{
		{TaskID: source.ID, Action: "discarded", Field: "depends_on[0]", From: "dependency-a", Reason: "schema 6 retires task dependencies"},
		{TaskID: source.ID, Action: "discarded", Field: "depends_on[1]", From: "dependency-b", Reason: "schema 6 retires task dependencies"},
		{TaskID: source.ID, Action: "discarded", Field: "recurrence", From: "2w", Reason: "schema 6 retires task recurrence"},
	}
	if !reflect.DeepEqual(result.Audit, want) {
		t.Fatalf("migration audit:\n got: %#v\nwant: %#v", result.Audit, want)
	}

	encoded, err := WriteV6TaskBody(result.Task)
	if err != nil {
		t.Fatalf("WriteV6TaskBody: %v", err)
	}
	for _, retired := range []string{"dependency-a", "dependency-b", "depends:", "recur:", "<!-- DECISIONS -->"} {
		if strings.Contains(encoded, retired) {
			t.Errorf("converted v6 body contains retired value %q:\n%s", retired, encoded)
		}
	}
}

func TestConvertV5TaskRejectsInvalidSource(t *testing.T) {
	source := baseV5TaskForConversion(time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC))
	source.State = "backlog"
	if _, err := ConvertV5Task(source); err == nil {
		t.Fatal("ConvertV5Task accepted an invalid schema-5 source")
	}
}

func baseV5TaskForConversion(now time.Time) V5TaskRecord {
	return V5TaskRecord{
		ID: "conversion-task", ShortID: 7, Title: "Conversion task",
		State: "ready", Priority: 3, CreatedAt: now, UpdatedAt: now,
	}
}
