package format

import (
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestV6TaskBodyRoundTripPreservesRichRetainedData(t *testing.T) {
	created := time.Date(2026, 8, 1, 12, 34, 56, 789123000, time.UTC)
	updated := created.Add(2*time.Hour + 17*time.Nanosecond)
	completed := updated.Add(24 * time.Hour)
	pinnedAt := created.Add(time.Minute)
	attachmentAddedAt := created.Add(2 * time.Minute)
	task := V6TaskRecord{
		ID:           "task-v6-rich",
		ShortID:      7,
		Title:        "Canonical generic record",
		Description:  "First line\n## Task: not a child\n<!-- COMMENTS -->",
		State:        "done",
		Priority:     0,
		ParentID:     "task-parent",
		Tags:         []string{"bug", "feature", "idea", "source:github", "café"},
		CreatedAt:    created,
		UpdatedAt:    updated,
		CompletedAt:  &completed,
		DueDate:      "2026-08-03",
		SnoozedUntil: "2026-08-02",
		Pinned:       true,
		PinnedAt:     &pinnedAt,
		Attachments: []V6TaskAttachment{
			{
				Kind:         "file",
				Filename:     "spec, final;v6.txt",
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
		Checklist: []V6ChecklistItem{
			{Text: "Keep commas, semicolons; and | pipes", Done: true},
			{Text: "Preserve café"},
		},
		Comments: []V6TaskComment{
			{Text: "Comment one\n## Task: still comment text", CreatedAt: created.Add(3 * time.Minute)},
			{Text: "Comment with <!-- marker -->", CreatedAt: created.Add(4 * time.Minute)},
		},
	}

	body, err := WriteV6TaskBody(task)
	if err != nil {
		t.Fatalf("WriteV6TaskBody: %v", err)
	}
	for _, retired := range []string{"depends:", "recur:", "<!-- DECISIONS -->"} {
		if strings.Contains(body, retired) {
			t.Errorf("v6 body contains retired syntax %q:\n%s", retired, body)
		}
	}
	parsed, err := ParseV6TaskBody(task.Title, body)
	if err != nil {
		t.Fatalf("ParseV6TaskBody: %v\nbody:\n%s", err, body)
	}
	if !reflect.DeepEqual(parsed, task) {
		t.Fatalf("v6 task mismatch after round trip:\n got: %#v\nwant: %#v", parsed, task)
	}
	second, err := WriteV6TaskBody(parsed)
	if err != nil {
		t.Fatalf("second WriteV6TaskBody: %v", err)
	}
	if second != body {
		t.Fatalf("v6 body is not a fixed point:\nfirst:\n%s\nsecond:\n%s", body, second)
	}
}

func TestParseV6TaskMetadataRejectsUnknownAndRetiredFields(t *testing.T) {
	base := "<!-- id: task, short_id: 1, state: backlog, priority: 0, created: 2026-08-01T12:00:00Z, updated: 2026-08-01T12:00:00Z"
	for _, field := range []string{
		"depends: other",
		"recur: 1w",
		"decision: retired",
		"source: github",
		"unknown: value",
	} {
		t.Run(strings.Split(field, ":")[0], func(t *testing.T) {
			if _, err := ParseV6TaskMetadata(base + ", " + field + " -->"); err == nil {
				t.Fatalf("ParseV6TaskMetadata accepted field %q", field)
			}
		})
	}
}

func TestV6TaskRecordHasNoRetiredFields(t *testing.T) {
	typeOf := reflect.TypeOf(V6TaskRecord{})
	for _, retired := range []string{"DependsOn", "Recurrence", "Decisions"} {
		if _, ok := typeOf.FieldByName(retired); ok {
			t.Errorf("V6TaskRecord exposes retired field %q", retired)
		}
	}
}

func TestValidateV6TaskRecordLifecyclePriorityAndDateContract(t *testing.T) {
	now := time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC)
	base := V6TaskRecord{
		ID:        "contract",
		ShortID:   1,
		Title:     "Contract",
		State:     "backlog",
		Priority:  0,
		CreatedAt: now,
		UpdatedAt: now,
	}

	for _, state := range []string{"inbox", "backlog", "doing", "waiting", "paused", "someday", "done", "canceled"} {
		t.Run("state-"+state, func(t *testing.T) {
			task := base
			task.State = state
			if err := ValidateV6TaskRecord(task); err != nil {
				t.Fatalf("ValidateV6TaskRecord rejected state %q: %v", state, err)
			}
		})
	}
	for _, state := range []string{"ready", "next", "in_progress", "review", "Backlog", ""} {
		t.Run("retired-state-"+state, func(t *testing.T) {
			task := base
			task.State = state
			if err := ValidateV6TaskRecord(task); err == nil {
				t.Fatalf("ValidateV6TaskRecord accepted state %q", state)
			}
		})
	}
	for priority := 0; priority <= 3; priority++ {
		task := base
		task.Priority = priority
		if err := ValidateV6TaskRecord(task); err != nil {
			t.Errorf("ValidateV6TaskRecord rejected priority %d: %v", priority, err)
		}
	}
	for _, priority := range []int{-1, 4, 5} {
		task := base
		task.Priority = priority
		if err := ValidateV6TaskRecord(task); err == nil {
			t.Errorf("ValidateV6TaskRecord accepted priority %d", priority)
		}
	}

	validDates := base
	validDates.DueDate = "2026-08-03"
	validDates.SnoozedUntil = "2026-08-02"
	if err := ValidateV6TaskRecord(validDates); err != nil {
		t.Fatalf("ValidateV6TaskRecord rejected canonical date-only values: %v", err)
	}
	for name, value := range map[string]string{
		"timestamp": "2026-08-03T00:00:00Z",
		"unpadded":  "2026-8-3",
		"invalid":   "2026-02-30",
	} {
		t.Run("date-"+name, func(t *testing.T) {
			task := base
			task.DueDate = value
			if err := ValidateV6TaskRecord(task); err == nil {
				t.Fatalf("ValidateV6TaskRecord accepted due date %q", value)
			}
		})
	}
}

func TestParseV6TaskBodyRejectsRetiredAndMalformedBlocks(t *testing.T) {
	metadata := "<!-- id: task, short_id: 1, state: backlog, priority: 0, created: 2026-08-01T12:00:00Z, updated: 2026-08-01T12:00:00Z -->\n"
	tests := map[string]string{
		"decision block": metadata + "<!-- DECISIONS -->\n<!-- decision: 2026-08-01T12:00:00Z -->\nNo\n<!-- rationale -->\nNo\n<!-- /DECISIONS -->\n",
		"unknown block":  metadata + "<!-- DEPENDENCIES -->\nother\n<!-- /DEPENDENCIES -->\n",
		"unterminated":   metadata + "<!-- DESCRIPTION -->\ntext\n",
		"duplicate":      metadata + "<!-- DESCRIPTION -->\none\n<!-- /DESCRIPTION -->\n<!-- DESCRIPTION -->\ntwo\n<!-- /DESCRIPTION -->\n",
	}
	for name, body := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := ParseV6TaskBody("Task", body); err == nil {
				t.Fatalf("ParseV6TaskBody accepted invalid body:\n%s", body)
			}
		})
	}
}

func TestValidateV6TaskRecordRequiresCanonicalGenericTags(t *testing.T) {
	now := time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC)
	base := V6TaskRecord{
		ID:        "tag-validation",
		ShortID:   1,
		Title:     "Tag validation",
		State:     "backlog",
		Priority:  0,
		CreatedAt: now,
		UpdatedAt: now,
	}
	valid := base
	valid.Tags = []string{"bug", "feature", "idea"}
	if err := ValidateV6TaskRecord(valid); err != nil {
		t.Fatalf("ValidateV6TaskRecord rejected independent semantic tags: %v", err)
	}
	for _, tags := range [][]string{
		{"#bug"},
		{"bug", "bug"},
		{"bad,tag"},
		{"bad;tag"},
		{" untrimmed"},
	} {
		task := base
		task.Tags = tags
		if err := ValidateV6TaskRecord(task); err == nil {
			t.Errorf("ValidateV6TaskRecord accepted non-canonical tags %v", tags)
		}
	}
}

func TestValidateDeferValueRejectsValuesTheLayoutCannotRoundTrip(t *testing.T) {
	for _, tc := range []struct {
		name     string
		encoding deferEncoding
		value    string
		wantErr  string
	}{
		{name: "empty is absent", encoding: deferDateLegacy, value: ""},
		{name: "canonical date", encoding: deferDateLegacy, value: "2026-05-14"},
		{
			name:     "unpadded date",
			encoding: deferDateLegacy,
			value:    "2026-5-14",
			wantErr:  `snoozed date "2026-5-14" is not canonical YYYY-MM-DD`,
		},
		{
			name:     "renamed key names itself",
			encoding: deferDateRenamed,
			value:    "2026-05-14T09:00:00Z",
			wantErr:  `deferred date "2026-05-14T09:00:00Z" is not canonical YYYY-MM-DD`,
		},
		{name: "canonical instant", encoding: deferInstant, value: "2026-05-14T09:00:00Z"},
		{
			name:     "instant layout refuses a bare date",
			encoding: deferInstant,
			value:    "2026-05-14",
			wantErr:  `deferred date "2026-05-14" is not canonical RFC3339Nano`,
		},
		{
			name:     "instant layout refuses a trailing zero nanosecond",
			encoding: deferInstant,
			value:    "2026-05-14T09:00:00.000Z",
			wantErr:  `deferred date "2026-05-14T09:00:00.000Z" is not canonical RFC3339Nano`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := validateDeferValue(tc.encoding, tc.value)
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("validateDeferValue(%+v, %q) = %v, want nil", tc.encoding, tc.value, err)
				}
				return
			}
			if err == nil || err.Error() != tc.wantErr {
				t.Fatalf("validateDeferValue(%+v, %q) = %v, want %q", tc.encoding, tc.value, err, tc.wantErr)
			}
		})
	}
}
