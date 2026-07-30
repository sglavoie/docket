package format

import (
	"slices"
	"strings"
	"testing"
	"time"
)

func TestV5TaskBodyRoundTripExact(t *testing.T) {
	created := time.Date(2026, 7, 28, 12, 34, 56, 789123000, time.FixedZone("CST", -6*60*60))
	updated := created.Add(2*time.Hour + 17*time.Nanosecond)
	completed := updated.Add(24 * time.Hour)
	pinnedAt := created.Add(time.Minute)
	attachmentAddedAt := created.Add(2 * time.Minute)
	task := V5TaskRecord{
		ID:           "task-v5-rich",
		ShortID:      7,
		Title:        "Canonical generic record",
		Description:  "First line\n## Task: not a child\n<!-- COMMENTS -->",
		State:        "done",
		Priority:     1,
		ParentID:     "task-parent",
		Tags:         []string{"type:bug", "source:github", "café"},
		CreatedAt:    created,
		UpdatedAt:    updated,
		CompletedAt:  &completed,
		DueDate:      "2026-07-30",
		SnoozedUntil: "2026-07-29",
		DependsOn:    []string{"other-project-task", "local-task"},
		Pinned:       true,
		PinnedAt:     &pinnedAt,
		Recurrence:   &V5TaskRecurrence{Interval: 2, Unit: "week"},
		Attachments: []V5TaskAttachment{
			{
				Kind:         "file",
				Filename:     "spec, final;v5.txt",
				OriginalPath: "/tmp/spec | final.txt",
				AddedAt:      &attachmentAddedAt,
			},
			{
				Kind:         "url",
				Filename:     "https://example.com/a?x=1,y=2",
				OriginalPath: "https://example.com/a?x=1,y=2",
			},
			{
				Kind:         "code",
				Filename:     "codec.go",
				OriginalPath: "internal/storage/codec.go",
				LineNumber:   42,
			},
		},
		Checklist: []V5ChecklistItem{
			{Text: "Keep commas, semicolons; and | pipes", Done: true},
			{Text: "Preserve café", Done: false},
		},
		Comments: []V5TaskComment{
			{Text: "Comment one\n## Task: still comment text", CreatedAt: created.Add(3 * time.Minute)},
			{Text: "Comment with <!-- marker -->", CreatedAt: created.Add(4 * time.Minute)},
		},
		Decisions: []V5TaskDecision{
			{
				Text:      "Keep the target codec isolated",
				Rationale: "Activation belongs to session 44.\n<!-- rationale --> stays text.",
				CreatedAt: created.Add(5 * time.Minute),
			},
		},
	}

	body, err := WriteV5TaskBody(task)
	if err != nil {
		t.Fatalf("WriteV5TaskBody: %v", err)
	}
	parsed, err := ParseV5TaskBody(task.Title, body)
	if err != nil {
		t.Fatalf("ParseV5TaskBody: %v\nbody:\n%s", err, body)
	}
	if !EqualV5TaskRecord(parsed, task) {
		t.Fatalf("v5 task mismatch after round trip:\n got: %#v\nwant: %#v", parsed, task)
	}

	second, err := WriteV5TaskBody(parsed)
	if err != nil {
		t.Fatalf("second WriteV5TaskBody: %v", err)
	}
	if second != body {
		t.Fatalf("v5 body is not a fixed point:\nfirst:\n%s\nsecond:\n%s", body, second)
	}
}

func TestV5MetadataContainsOnlyTargetFields(t *testing.T) {
	now := time.Date(2026, 7, 28, 12, 0, 0, 0, time.UTC)
	task := V5TaskRecord{
		ID:        "target-only",
		ShortID:   1,
		Title:     "Target only",
		State:     "ready",
		Priority:  3,
		CreatedAt: now,
		UpdatedAt: now,
		Tags:      []string{"type:bug", "source:github"},
	}
	metadata, err := WriteV5TaskMetadata(task)
	if err != nil {
		t.Fatalf("WriteV5TaskMetadata: %v", err)
	}
	for _, retired := range []string{
		", status:", ", section:", ", stage:", ", active:", ", related:", ", dimmed:",
		", parked:", ", start:", ", estimated_days:", ", source:", ", url:", ", score.",
		", bug.", ", story.", ", epic.", ", idea.", ", task.",
	} {
		if strings.Contains(metadata, retired) {
			t.Errorf("v5 metadata contains retired field %q: %s", retired, metadata)
		}
	}
	if !strings.Contains(metadata, "tags: type:bug;source:github") {
		t.Fatalf("v5 metadata lost namespaced tags: %s", metadata)
	}
	parsed, err := ParseV5TaskMetadata(metadata)
	if err != nil {
		t.Fatalf("ParseV5TaskMetadata: %v", err)
	}
	second, err := WriteV5TaskMetadata(parsed)
	if err != nil {
		t.Fatalf("second WriteV5TaskMetadata: %v", err)
	}
	if second != metadata {
		t.Fatalf("v5 metadata is not a fixed point:\nfirst:  %s\nsecond: %s", metadata, second)
	}

	if _, err := ParseV5TaskMetadata("<!-- id: old, short_id: 1, state: ready, priority: 3, created: 2026-07-28T12:00:00Z, updated: 2026-07-28T12:00:00Z, source: github -->"); err == nil {
		t.Fatal("ParseV5TaskMetadata accepted retired source field")
	}
}

func TestDerivedV5TaskTagsUsesTaskRecordsOnly(t *testing.T) {
	tasks := []V5TaskRecord{
		{Tags: []string{"source:github", "backend", "type:bug"}},
		{Tags: []string{"backend", "#team:platform", "bad,tag"}},
	}
	got := DerivedV5TaskTags(tasks)
	want := []string{"backend", "source:github", "team:platform", "type:bug"}
	if !slices.Equal(got, want) {
		t.Fatalf("DerivedV5TaskTags() = %v, want %v", got, want)
	}
}

func TestValidateV5TaskRecordRequiresCanonicalTags(t *testing.T) {
	now := time.Date(2026, 7, 28, 12, 0, 0, 0, time.UTC)
	base := V5TaskRecord{
		ID:        "tag-validation",
		ShortID:   1,
		Title:     "Tag validation",
		State:     "ready",
		Priority:  3,
		CreatedAt: now,
		UpdatedAt: now,
	}
	for _, tags := range [][]string{
		{"#type:bug"},
		{"type:bug", "type:bug"},
		{"bad,tag"},
	} {
		task := base
		task.Tags = tags
		if err := ValidateV5TaskRecord(task); err == nil {
			t.Errorf("ValidateV5TaskRecord accepted non-canonical tags %v", tags)
		}
	}
}
